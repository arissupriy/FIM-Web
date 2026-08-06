// Package template provides template registry and management.
package template

import (
	"context"
	"sort"
	"sync"
)

// Registry manages all registered templates.
type Registry struct {
	mu        sync.RWMutex
	templates map[string]Template
	ordered   []Template // sorted by priority (descending)
}

// NewRegistry creates a new template registry.
func NewRegistry() *Registry {
	return &Registry{
		templates: make(map[string]Template),
		ordered:   make([]Template, 0),
	}
}

// Register adds a template to the registry.
// Panics if a template with the same name is already registered.
func (r *Registry) Register(t Template) {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := t.Name()
	if _, exists := r.templates[name]; exists {
		panic("template already registered: " + name)
	}

	r.templates[name] = t
	r.ordered = append(r.ordered, t)
	sort.Slice(r.ordered, func(i, j int) bool {
		return r.ordered[i].Priority() > r.ordered[j].Priority()
	})
}

// Get returns a template by name.
func (r *Registry) Get(name string) (Template, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	t, ok := r.templates[name]
	return t, ok
}

// List returns all registered templates sorted by priority.
func (r *Registry) List() []Template {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Template, len(r.ordered))
	copy(result, r.ordered)
	return result
}

// DetectBest tries each template in priority order until one matches.
func (r *Registry) DetectBest(ctx context.Context, db DBConnection) (Template, error) {
	templates := r.List()

	for _, t := range templates {
		compatible, err := t.Compatible(ctx, db)
		if err != nil {
			continue // Skip templates that error during detection
		}
		if compatible {
			return t, nil
		}
	}

	return nil, nil // No compatible template found
}

// MustGet returns a template by name, panics if not found.
func (r *Registry) MustGet(name string) Template {
	t, ok := r.Get(name)
	if !ok {
		panic("template not found: " + name)
	}
	return t
}

// Reset clears all registered templates (mainly for testing).
func (r *Registry) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.templates = make(map[string]Template)
	r.ordered = make([]Template, 0)
}

// DefaultRegistry is the global template registry.
var DefaultRegistry = NewRegistry()

// Register a template with the default registry.
func Register(t Template) {
	DefaultRegistry.Register(t)
}

// Get a template from the default registry.
func Get(name string) (Template, bool) {
	return DefaultRegistry.Get(name)
}

// List all templates from the default registry.
func List() []Template {
	return DefaultRegistry.List()
}

// Reset clears the global registry (mainly for testing).
func Reset() {
	DefaultRegistry.Reset()
}
