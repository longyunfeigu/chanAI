package sandbox

import (
	"context"
	"fmt"
	"time"
)

// MockSandbox is an in-memory sandbox implementation used for tests.
type MockSandbox struct {
	kind    string
	workDir string
	fs      *MockFS
}

// NewMockSandbox builds a new mock sandbox.
func NewMockSandbox() *MockSandbox {
	return &MockSandbox{
		kind:    "mock",
		workDir: "/mock/workspace",
		fs:      NewMockFS(),
	}
}

func (ms *MockSandbox) Kind() string {
	return ms.kind
}

func (ms *MockSandbox) WorkDir() string {
	return ms.workDir
}

func (ms *MockSandbox) FS() SandboxFS {
	return ms.fs
}

func (ms *MockSandbox) Exec(ctx context.Context, cmd string, opts *ExecOptions) (*ExecResult, error) {
	return &ExecResult{
		Code:   0,
		Stdout: fmt.Sprintf("Mock output for: %s", cmd),
		Stderr: "",
	}, nil
}

func (ms *MockSandbox) Watch(paths []string, listener FileChangeListener) (string, error) {
	return "mock-watch-id", nil
}

func (ms *MockSandbox) Unwatch(watchID string) error {
	return nil
}

func (ms *MockSandbox) Dispose() error {
	return nil
}

// MockFS is an in-memory SandboxFS implementation.
type MockFS struct {
	files map[string]string
}

// NewMockFS creates a new MockFS.
func NewMockFS() *MockFS {
	return &MockFS{
		files: make(map[string]string),
	}
}

func (mfs *MockFS) Resolve(path string) string {
	return path
}

func (mfs *MockFS) IsInside(path string) bool {
	return true
}

func (mfs *MockFS) Read(ctx context.Context, path string) (string, error) {
	if content, ok := mfs.files[path]; ok {
		return content, nil
	}
	return "", fmt.Errorf("file not found: %s", path)
}

func (mfs *MockFS) Write(ctx context.Context, path string, content string) error {
	mfs.files[path] = content
	return nil
}

func (mfs *MockFS) Temp(name string) string {
	return "/tmp/" + name
}

func (mfs *MockFS) Stat(ctx context.Context, path string) (FileInfo, error) {
	if content, ok := mfs.files[path]; ok {
		return FileInfo{
			Path:    path,
			Size:    int64(len(content)),
			ModTime: time.Now(),
			IsDir:   false,
			Mode:    0644,
		}, nil
	}
	return FileInfo{}, fmt.Errorf("file not found: %s", path)
}

func (mfs *MockFS) Glob(ctx context.Context, pattern string, opts *GlobOptions) ([]string, error) {
	results := make([]string, 0, len(mfs.files))
	for path := range mfs.files {
		results = append(results, path)
	}
	return results, nil
}

