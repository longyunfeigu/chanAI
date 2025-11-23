package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// dangerousPatterns contain commands that should be blocked outright in a local sandbox.
var dangerousPatterns = []*regexp.Regexp{
	regexp.MustCompile(`rm\s+-rf\s+/($|\s)`),
	regexp.MustCompile(`sudo\s+`),
	regexp.MustCompile(`shutdown`),
	regexp.MustCompile(`reboot`),
	regexp.MustCompile(`mkfs\.`),
	regexp.MustCompile(`dd\s+.*of=`),
	regexp.MustCompile(`:\(\)\{\s*:\|\:&\s*\};:`),
	regexp.MustCompile(`chmod\s+777\s+/`),
	regexp.MustCompile(`curl\s+.*\|\s*(bash|sh)`),
	regexp.MustCompile(`wget\s+.*\|\s*(bash|sh)`),
	regexp.MustCompile(`>\s*/dev/sda`),
	regexp.MustCompile(`mkswap`),
	regexp.MustCompile(`swapon`),
}

// LocalSandboxConfig configures a local sandbox instance.
type LocalSandboxConfig struct {
	WorkDir         string
	EnforceBoundary bool
	AllowPaths      []string
	WatchFiles      bool
}

// LocalSandbox is a Sandbox implementation that executes directly on the host,
// enforcing a working directory boundary and blocking dangerous commands.
// It embeds LocalFS so that filesystem configuration (workDir/boundaries)
// is defined in a single place.
type LocalSandbox struct {
	*LocalFS
	watchEnabled bool
	watchers     map[string]*fileWatcher
	watcherMu    sync.Mutex
}

// fileWatcher tracks one fsnotify watcher and listener.
type fileWatcher struct {
	paths    []string
	listener FileChangeListener
	watcher  *fsnotify.Watcher
	done     chan struct{}
}

// NewLocalSandbox creates a LocalSandbox from config.
func NewLocalSandbox(config *LocalSandboxConfig) (*LocalSandbox, error) {
	if config == nil {
		config = &LocalSandboxConfig{}
	}

	// Resolve workDir
	workDir := config.WorkDir
	if workDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
		workDir = wd
	}

	workDir, err := filepath.Abs(workDir)
	if err != nil {
		return nil, fmt.Errorf("resolve work directory: %w", err)
	}

	// Resolve allow paths
	allowPaths := make([]string, 0, len(config.AllowPaths))
	for _, p := range config.AllowPaths {
		abs, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		allowPaths = append(allowPaths, abs)
	}

	fs := &LocalFS{
		workDir:         workDir,
		enforceBoundary: config.EnforceBoundary,
		allowPaths:      allowPaths,
	}

	ls := &LocalSandbox{
		LocalFS:     fs,
		watchEnabled: config.WatchFiles,
		watchers:     make(map[string]*fileWatcher),
	}

	return ls, nil
}

// Kind identifies this sandbox as local.
func (ls *LocalSandbox) Kind() string {
	return "local"
}

// WorkDir returns the sandbox working directory.
func (ls *LocalSandbox) WorkDir() string {
	return ls.workDir
}

// FS returns the sandbox filesystem.
func (ls *LocalSandbox) FS() SandboxFS {
	return ls.LocalFS
}

// Exec runs a command inside the sandbox with basic safety checks.
func (ls *LocalSandbox) Exec(ctx context.Context, cmd string, opts *ExecOptions) (*ExecResult, error) {
	// Block obviously dangerous commands.
	for _, pattern := range dangerousPatterns {
		if pattern.MatchString(cmd) {
			return &ExecResult{
				Code:   1,
				Stdout: "",
				Stderr: fmt.Sprintf("Dangerous command blocked for security: %s", truncate(cmd, 100)),
			}, nil
		}
	}

	// Timeout
	timeout := 120 * time.Second
	if opts != nil && opts.Timeout > 0 {
		timeout = opts.Timeout
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	command := exec.CommandContext(execCtx, "sh", "-c", cmd)

	// WorkDir
	workDir := ls.workDir
	if opts != nil && opts.WorkDir != "" {
		workDir = ls.Resolve(opts.WorkDir)
	}
	command.Dir = workDir

	// Env
	if opts != nil && len(opts.Env) > 0 {
		env := os.Environ()
		for k, v := range opts.Env {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		command.Env = env
	}

	output, err := command.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return &ExecResult{
				Code:   exitErr.ExitCode(),
				Stdout: string(output),
				Stderr: string(output),
			}, nil
		}
		return &ExecResult{
			Code:   1,
			Stdout: "",
			Stderr: err.Error(),
		}, nil
	}

	return &ExecResult{
		Code:   0,
		Stdout: string(output),
		Stderr: "",
	}, nil
}

// Watch subscribes to filesystem changes.
func (ls *LocalSandbox) Watch(paths []string, listener FileChangeListener) (string, error) {
	if !ls.watchEnabled {
		return fmt.Sprintf("watch-disabled-%d", time.Now().UnixNano()), nil
	}

	ls.watcherMu.Lock()
	defer ls.watcherMu.Unlock()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return "", fmt.Errorf("create file watcher: %w", err)
	}

	watchID := fmt.Sprintf("watch-%d-%s", time.Now().UnixNano(), randomString(8))

	for _, path := range paths {
		resolved := ls.Resolve(path)
		if !ls.IsInside(resolved) {
			continue
		}
		if err := watcher.Add(resolved); err != nil {
			continue
		}
	}

	fw := &fileWatcher{
		paths:    paths,
		listener: listener,
		watcher:  watcher,
		done:     make(chan struct{}),
	}

	ls.watchers[watchID] = fw

	go ls.watchLoop(watchID, fw)

	return watchID, nil
}

// watchLoop listens for fsnotify events and dispatches them to the listener.
func (ls *LocalSandbox) watchLoop(watchID string, fw *fileWatcher) {
	defer fw.watcher.Close()

	for {
		select {
		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}
			if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
				var mtime time.Time
				if stat, err := os.Stat(event.Name); err == nil {
					mtime = stat.ModTime()
				} else {
					mtime = time.Now()
				}

				fw.listener(FileChangeEvent{
					Path:  event.Name,
					Mtime: mtime,
				})
			}
		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			_ = err
		case <-fw.done:
			return
		}
	}
}

// Unwatch cancels a watch.
func (ls *LocalSandbox) Unwatch(watchID string) error {
	ls.watcherMu.Lock()
	defer ls.watcherMu.Unlock()

	fw, ok := ls.watchers[watchID]
	if !ok {
		return nil
	}

	close(fw.done)
	delete(ls.watchers, watchID)
	return nil
}

// Dispose stops all watchers.
func (ls *LocalSandbox) Dispose() error {
	ls.watcherMu.Lock()
	defer ls.watcherMu.Unlock()

	for _, fw := range ls.watchers {
		close(fw.done)
	}
	ls.watchers = make(map[string]*fileWatcher)
	return nil
}

// truncate safely cuts long strings for logging.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// randomString generates a simple pseudo-random string.
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		time.Sleep(time.Nanosecond)
	}
	return string(b)
}

