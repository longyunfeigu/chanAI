package sandbox

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
)

// LocalFS provides a bounded view of the host filesystem,
// enforcing a workDir root and optional allow-list.
type LocalFS struct {
	workDir         string
	enforceBoundary bool
	allowPaths      []string
}

// Resolve turns a path into an absolute path rooted at workDir when needed.
func (lfs *LocalFS) Resolve(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(lfs.workDir, path)
}

// IsInside checks if the path is within the sandbox boundary.
func (lfs *LocalFS) IsInside(path string) bool {
	resolved, err := filepath.Abs(lfs.Resolve(path))
	if err != nil {
		return false
	}

	// 1. WorkDir boundary
	relativeToWork, err := filepath.Rel(lfs.workDir, resolved)
	if err == nil && !strings.HasPrefix(relativeToWork, "..") && !filepath.IsAbs(relativeToWork) {
		return true
	}

	// 2. If not enforcing boundary, allow everything
	if !lfs.enforceBoundary {
		return true
	}

	// 3. Allow-list
	for _, allowed := range lfs.allowPaths {
		resolvedAllowed, err := filepath.Abs(allowed)
		if err != nil {
			continue
		}
		relative, err := filepath.Rel(resolvedAllowed, resolved)
		if err == nil && !strings.HasPrefix(relative, "..") && !filepath.IsAbs(relative) {
			return true
		}
	}

	return false
}

// Read reads the content of a file as text.
func (lfs *LocalFS) Read(ctx context.Context, path string) (string, error) {
	resolved := lfs.Resolve(path)
	if !lfs.IsInside(resolved) {
		return "", fmt.Errorf("path outside sandbox: %s", path)
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	return string(data), nil
}

// Write writes text content to a file, creating parent directories as needed.
func (lfs *LocalFS) Write(ctx context.Context, path string, content string) error {
	resolved := lfs.Resolve(path)
	if !lfs.IsInside(resolved) {
		return fmt.Errorf("path outside sandbox: %s", path)
	}

	dir := filepath.Dir(resolved)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	if err := os.WriteFile(resolved, []byte(content), 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

// Temp returns a path for temporary files, located under a .temp folder in workDir.
func (lfs *LocalFS) Temp(name string) string {
	if name == "" {
		name = fmt.Sprintf("temp-%d-%s", time.Now().UnixNano(), randomString(8))
	}
	tempPath := filepath.Join(lfs.workDir, ".temp", name)
	relative, _ := filepath.Rel(lfs.workDir, tempPath)
	return relative
}

// Stat retrieves file metadata.
func (lfs *LocalFS) Stat(ctx context.Context, path string) (FileInfo, error) {
	resolved := lfs.Resolve(path)
	if !lfs.IsInside(resolved) {
		return FileInfo{}, fmt.Errorf("path outside sandbox: %s", path)
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return FileInfo{}, fmt.Errorf("stat file: %w", err)
	}

	return FileInfo{
		Path:    path,
		Size:    info.Size(),
		ModTime: info.ModTime(),
		IsDir:   info.IsDir(),
		Mode:    int(info.Mode()),
	}, nil
}

// Glob matches files under the sandbox according to pattern and options.
func (lfs *LocalFS) Glob(ctx context.Context, pattern string, opts *GlobOptions) ([]string, error) {
	if opts == nil {
		opts = &GlobOptions{}
	}

	// Determine search root
	cwd := lfs.workDir
	if opts.CWD != "" {
		cwd = lfs.Resolve(opts.CWD)
	}

	fsys := os.DirFS(cwd)
	matches, err := doublestar.Glob(
		fsys,
		pattern,
		doublestar.WithFilesOnly(),
		doublestar.WithNoFollow(),
	)
	if err != nil {
		return nil, fmt.Errorf("glob pattern: %w", err)
	}

	results := make([]string, 0, len(matches))
	for _, match := range matches {
		fullPath := filepath.Join(cwd, match)

		if !lfs.IsInside(fullPath) {
			continue
		}

		if opts.Ignore != nil && lfs.shouldIgnore(match, opts.Ignore) {
			continue
		}

		if opts.Absolute {
			results = append(results, fullPath)
		} else {
			rel, err := filepath.Rel(lfs.workDir, fullPath)
			if err != nil {
				results = append(results, match)
			} else {
				results = append(results, rel)
			}
		}
	}

	return results, nil
}

// shouldIgnore checks whether a path should be ignored according to patterns.
func (lfs *LocalFS) shouldIgnore(path string, ignorePatterns []string) bool {
	for _, pattern := range ignorePatterns {
		matched, err := doublestar.Match(pattern, path)
		if err == nil && matched {
			return true
		}
	}
	return false
}
