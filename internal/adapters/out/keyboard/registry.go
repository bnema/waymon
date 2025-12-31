package keyboard

import (
	"fmt"
	"sync"

	"github.com/bnema/waymon/internal/domain"
)

// Registry manages available keyboard layouts.
type Registry struct {
	mu      sync.RWMutex
	layouts map[domain.KeyboardLayout]Layout
}

// NewRegistry creates a new layout registry with default layouts.
func NewRegistry() *Registry {
	r := &Registry{
		layouts: make(map[domain.KeyboardLayout]Layout),
	}

	// Register default layouts
	r.Register(NewUSLayout())
	r.Register(NewFRLayout())

	return r
}

// Register adds a layout to the registry.
func (r *Registry) Register(layout Layout) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.layouts[layout.Name()] = layout
}

// Get retrieves a layout by name.
func (r *Registry) Get(name domain.KeyboardLayout) (Layout, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	layout, ok := r.layouts[name]
	if !ok {
		return nil, fmt.Errorf("layout %q not found (available: %v)", name, r.Available())
	}

	return layout, nil
}

// Available returns a list of available layout names.
func (r *Registry) Available() []domain.KeyboardLayout {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]domain.KeyboardLayout, 0, len(r.layouts))
	for name := range r.layouts {
		names = append(names, name)
	}
	return names
}

// IsSupported returns true if the layout is registered.
func (r *Registry) IsSupported(name domain.KeyboardLayout) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.layouts[name]
	return ok
}

// Default returns the default US layout.
func (r *Registry) Default() Layout {
	layout, _ := r.Get(domain.LayoutUS)
	return layout
}
