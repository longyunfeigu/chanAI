package sandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// RemoteClient is a minimal HTTP client used by RemoteSandbox.
type RemoteClient struct {
	baseURL    string
	apiKey     string
	apiSecret  string
	httpClient *http.Client
	headers    map[string]string
}

// RemoteClientConfig configures RemoteClient.
type RemoteClientConfig struct {
	BaseURL   string
	APIKey    string
	APISecret string
	Timeout   time.Duration
	Headers   map[string]string
}

// NewRemoteClient constructs a RemoteClient.
func NewRemoteClient(config *RemoteClientConfig) *RemoteClient {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	return &RemoteClient{
		baseURL:   config.BaseURL,
		apiKey:    config.APIKey,
		apiSecret: config.APISecret,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		headers: config.Headers,
	}
}

// RemoteResponse wraps a raw HTTP response.
type RemoteResponse struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
}

// JSON decodes the response body into v.
func (rr *RemoteResponse) JSON(v interface{}) error {
	return json.Unmarshal(rr.Body, v)
}

// String returns the body as string.
func (rr *RemoteResponse) String() string {
	return string(rr.Body)
}

// Call performs an HTTP request against the remote sandbox API.
func (rc *RemoteClient) Call(ctx context.Context, method, path string, body interface{}) (*RemoteResponse, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(jsonData)
	}

	url := rc.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if rc.apiKey != "" {
		req.Header.Set("X-API-Key", rc.apiKey)
	}

	for k, v := range rc.headers {
		req.Header.Set(k, v)
	}

	resp, err := rc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("api error: %d - %s", resp.StatusCode, string(respBody))
	}

	return &RemoteResponse{
		StatusCode: resp.StatusCode,
		Body:       respBody,
		Headers:    resp.Header,
	}, nil
}

// RemoteSandboxConfig configures a RemoteSandbox.
type RemoteSandboxConfig struct {
	BaseURL     string
	APIKey      string
	APISecret   string
	WorkDir     string
	Image       string            // sandbox image identifier
	Region      string            // region
	Timeout     time.Duration     // timeout
	Environment map[string]string // env vars
	Properties  map[string]interface{}
}

// RemoteSandbox is a base implementation of Sandbox that delegates to a remote service.
// Concrete cloud providers can extend this type and implement Exec/FS operations.
type RemoteSandbox struct {
	config     *RemoteSandboxConfig
	client     *RemoteClient
	sessionID  string
	workDir    string
	fs         *RemoteFS
	properties map[string]interface{}
}

// NewRemoteSandbox constructs a RemoteSandbox with a RemoteClient and RemoteFS.
func NewRemoteSandbox(config *RemoteSandboxConfig) (*RemoteSandbox, error) {
	client := NewRemoteClient(&RemoteClientConfig{
		BaseURL:   config.BaseURL,
		APIKey:    config.APIKey,
		APISecret: config.APISecret,
		Timeout:   config.Timeout,
	})

	rs := &RemoteSandbox{
		config:     config,
		client:     client,
		workDir:    config.WorkDir,
		properties: config.Properties,
	}

	rs.fs = &RemoteFS{
		sandbox: rs,
		workDir: config.WorkDir,
	}

	return rs, nil
}

// Kind returns "remote".
func (rs *RemoteSandbox) Kind() string {
	return "remote"
}

// Exec is intentionally left unimplemented; concrete cloud sandboxes should override it.
func (rs *RemoteSandbox) Exec(ctx context.Context, cmd string, opts *ExecOptions) (*ExecResult, error) {
	return nil, fmt.Errorf("exec not implemented in base RemoteSandbox")
}

// FS returns a basic RemoteFS implementation.
func (rs *RemoteSandbox) FS() SandboxFS {
	return rs.fs
}

// WorkDir returns the remote sandbox workDir.
func (rs *RemoteSandbox) WorkDir() string {
	return rs.workDir
}

// Watch is not supported for remote sandboxes by default.
func (rs *RemoteSandbox) Watch(paths []string, listener FileChangeListener) (string, error) {
	return "", fmt.Errorf("watch not supported in remote sandbox")
}

// Unwatch is not supported for remote sandboxes by default.
func (rs *RemoteSandbox) Unwatch(watchID string) error {
	return fmt.Errorf("unwatch not supported in remote sandbox")
}

// Dispose releases resources; concrete implementations can override.
func (rs *RemoteSandbox) Dispose() error {
	return nil
}

// SessionID returns the remote session ID.
func (rs *RemoteSandbox) SessionID() string {
	return rs.sessionID
}

// SetSessionID sets the remote session ID.
func (rs *RemoteSandbox) SetSessionID(id string) {
	rs.sessionID = id
}

// RemoteFS is a thin placeholder implementation for remote FS operations.
// Concrete remote sandboxes should override these methods via composition or embedding.
type RemoteFS struct {
	sandbox *RemoteSandbox
	workDir string
}

func (rfs *RemoteFS) Resolve(path string) string {
	return path
}

func (rfs *RemoteFS) IsInside(path string) bool {
	// remote sandboxes enforce boundaries server-side
	return true
}

func (rfs *RemoteFS) Read(ctx context.Context, path string) (string, error) {
	return "", fmt.Errorf("read not implemented in base RemoteFS")
}

func (rfs *RemoteFS) Write(ctx context.Context, path string, content string) error {
	return fmt.Errorf("write not implemented in base RemoteFS")
}

func (rfs *RemoteFS) Temp(name string) string {
	return "/tmp/" + name
}

func (rfs *RemoteFS) Stat(ctx context.Context, path string) (FileInfo, error) {
	return FileInfo{}, fmt.Errorf("stat not implemented in base RemoteFS")
}

func (rfs *RemoteFS) Glob(ctx context.Context, pattern string, opts *GlobOptions) ([]string, error) {
	return nil, fmt.Errorf("glob not implemented in base RemoteFS")
}

