package tool

import (
	"fmt"
	"strings"
	"sync"
)

// ToolFactory defines a function that creates a Tool instance from configuration.
type ToolFactory func(config map[string]any) (Tool, error)

// Registry manages tool factories and instances.
// It supports both pre-built instances (legacy/simple mode) and dynamic factories.
type Registry struct {
	mu        sync.RWMutex
	factories map[string]ToolFactory
	instances map[string]Tool // Cache or manually registered instances
}

// NewRegistry creates a new empty registry.
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[string]ToolFactory),
		instances: make(map[string]Tool),
	}
}

// RegisterFactory adds a tool factory to the registry.
// This allows dynamic creation of tools with different configurations.
func (r *Registry) RegisterFactory(name string, factory ToolFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[name] = factory
}

// RegisterInstance adds a pre-built tool instance.
// Useful for stateless tools or singletons.
func (r *Registry) RegisterInstance(t Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.instances[t.Name()] = t
}

// Create builds a new tool instance using the registered factory.
// If no factory is found, it checks if a singleton instance exists.
func (r *Registry) Create(name string, config map[string]any) (Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 1. Try Factory (Preferred for dynamic config)
	if factory, ok := r.factories[name]; ok {
		return factory(config)
	}

	// 2. Fallback to Singleton Instance
	// Note: Config is ignored here because the instance is already built.
	if instance, ok := r.instances[name]; ok {
		return instance, nil
	}

	return nil, &ToolNotFoundError{Name: name}
}

// Get returns a tool instance by name.
// It prioritizes existing instances. If only a factory exists, it attempts to create
// a default instance with nil config (which might fail depending on the tool).
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	// check instance first
	if t, ok := r.instances[name]; ok {
		r.mu.RUnlock()
		return t, true
	}
	
	// check factory
	factory, ok := r.factories[name]
	r.mu.RUnlock()
	
	if ok {
		// Try to instantiate with empty config
		// This handles the case where we just want "the tool" and don't have specific config
		t, err := factory(nil)
		if err == nil {
			return t, true
		}
	}
	
	return nil, false
}

// List returns all available tools.
// For factory-based tools, it attempts to create a default instance to get metadata.
func (r *Registry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	var list []Tool
	seen := make(map[string]bool)

	// 1. Add instances
	for name, t := range r.instances {
		list = append(list, t)
		seen[name] = true
	}

	// 2. Add from factories (if not already added)
	for name, factory := range r.factories {
		if seen[name] {
			continue
		}
		// Try to create a default instance to show in the list
		if t, err := factory(nil); err == nil {
			list = append(list, t)
		}
	}
	return list
}

// Remove deletes a tool (both factory and instance) from the registry.
func (r *Registry) Remove(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.factories, name)
	delete(r.instances, name)
}

// Find returns a tool by name (case-insensitive search).
// Supports both instances and factories.
func (r *Registry) Find(name string) Tool {
	// Try exact match first via Get
	if t, ok := r.Get(name); ok {
		return t
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Case-insensitive search logic
	target := strings.ToLower(name)

	// Check instances
	for n, t := range r.instances {
		if strings.ToLower(n) == target {
			return t
		}
	}

	// Check factories
	for n, factory := range r.factories {
		if strings.ToLower(n) == target {
			if t, err := factory(nil); err == nil {
				return t
			}
		}
	}

	return nil
}

// ToolNotFoundError indicates a requested tool is missing.
type ToolNotFoundError struct {
	Name string
}

func (e *ToolNotFoundError) Error() string {
	return fmt.Sprintf("tool not found: %s", e.Name)
}
