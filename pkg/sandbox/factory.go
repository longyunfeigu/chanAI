package sandbox

import (
	"fmt"
	"time"
)

// Kind identifies sandbox kind.
type Kind string

const (
	KindLocal  Kind = "local"
	KindMock   Kind = "mock"
	KindRemote Kind = "remote"
)

// RemoteConfig holds configuration specific to remote sandboxes.
type RemoteConfig struct {
	BaseURL   string
	APIKey    string
	APISecret string
	Timeout   time.Duration
}

// Config describes how to create a sandbox instance.
type Config struct {
	Kind            Kind
	WorkDir         string
	EnforceBoundary bool
	AllowPaths      []string
	WatchFiles      bool
	Remote          *RemoteConfig
}

// NewSandbox builds a Sandbox according to the given config.
// If cfg is nil, it defaults to a local sandbox rooted at the current directory.
func NewSandbox(cfg *Config) (Sandbox, error) {
	if cfg == nil {
		cfg = &Config{
			Kind:    KindLocal,
			WorkDir: ".",
		}
	}

	switch cfg.Kind {
	case KindLocal:
		return NewLocalSandbox(&LocalSandboxConfig{
			WorkDir:         cfg.WorkDir,
			EnforceBoundary: cfg.EnforceBoundary,
			AllowPaths:      cfg.AllowPaths,
			WatchFiles:      cfg.WatchFiles,
		})

	case KindMock:
		return NewMockSandbox(), nil

	case KindRemote:
		if cfg.Remote == nil {
			return nil, fmt.Errorf("remote sandbox requires Remote configuration")
		}

		timeout := cfg.Remote.Timeout
		if timeout <= 0 {
			timeout = 30 * time.Second
		}

		return NewRemoteSandbox(&RemoteSandboxConfig{
			BaseURL:   cfg.Remote.BaseURL,
			APIKey:    cfg.Remote.APIKey,
			APISecret: cfg.Remote.APISecret,
			WorkDir:   cfg.WorkDir,
			Timeout:   timeout,
		})

	default:
		return nil, fmt.Errorf("unknown sandbox kind: %s", cfg.Kind)
	}
}

