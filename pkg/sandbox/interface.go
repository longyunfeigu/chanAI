package sandbox

import (
	"context"
	"time"
)

// ExecOptions describes how a command should be executed inside the sandbox.
type ExecOptions struct {
	Timeout time.Duration
	WorkDir string
	Env     map[string]string
}

// ExecResult captures the outcome of a sandbox Exec call.
type ExecResult struct {
	Code   int
	Stdout string
	Stderr string
}

// FileChangeEvent represents a single file change notification.
type FileChangeEvent struct {
	Path  string
	Mtime time.Time
}

// FileChangeListener receives file change events from the sandbox.
type FileChangeListener func(event FileChangeEvent)

// SandboxFS is the file-system interface exposed to tools.
// Implementations are responsible for enforcing any path/boundary constraints.
type SandboxFS interface {
	// Resolve resolves a path into an absolute path within the sandbox.
	Resolve(path string) string

	// IsInside checks whether the given path is inside the sandbox boundary.
	IsInside(path string) bool

	// Read reads file content as text.
	Read(ctx context.Context, path string) (string, error)

	// Write writes file content as text, creating directories as needed.
	Write(ctx context.Context, path string, content string) error

	// Temp returns a path suitable for temporary files.
	Temp(name string) string

	// Stat returns metadata for a file.
	Stat(ctx context.Context, path string) (FileInfo, error)

	// Glob performs pattern-based file matching.
	Glob(ctx context.Context, pattern string, opts *GlobOptions) ([]string, error)
}

// FileInfo describes a file or directory in the sandbox.
type FileInfo struct {
	Path    string
	Size    int64
	ModTime time.Time
	IsDir   bool
	Mode    int
}

// GlobOptions controls how Glob behaves.
type GlobOptions struct {
	CWD      string
	Ignore   []string
	Dot      bool
	Absolute bool
}

// Sandbox is the main interface tools should depend on.
// It abstracts command execution and filesystem access behind a safe boundary.
type Sandbox interface {
	// Kind returns the sandbox type (e.g., "local", "remote", "mock").
	Kind() string

	// WorkDir returns the sandbox working directory.
	WorkDir() string

	// FS returns the filesystem interface.
	FS() SandboxFS

	// Exec executes a command within the sandbox.
	Exec(ctx context.Context, cmd string, opts *ExecOptions) (*ExecResult, error)

	// Watch subscribes to file changes for the given paths.
	Watch(paths []string, listener FileChangeListener) (watchID string, err error)

	// Unwatch cancels a previous watch.
	Unwatch(watchID string) error

	// Dispose releases any resources held by the sandbox.
	Dispose() error
}

