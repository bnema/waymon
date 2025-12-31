package keyboard

import (
	"context"

	"github.com/bnema/waymon/internal/domain"
)

// Adapter implements the KeyboardLayoutPort interface.
// It provides keyboard layout translation services using a registry of layouts.
type Adapter struct {
	registry *Registry
}

// NewAdapter creates a new keyboard layout adapter.
func NewAdapter() *Adapter {
	return &Adapter{
		registry: NewRegistry(),
	}
}

// CharToKeySequence converts a Unicode character to a sequence of keystrokes
// for the specified keyboard layout.
func (a *Adapter) CharToKeySequence(ctx context.Context, char rune, layout domain.KeyboardLayout) ([]domain.KeySequence, error) {
	l, err := a.registry.Get(layout)
	if err != nil {
		// Fall back to US layout
		l = a.registry.Default()
	}
	return l.CharToKeySequence(ctx, char)
}

// KeycodeToChar converts a keycode with modifiers to a Unicode character
// for the specified keyboard layout.
func (a *Adapter) KeycodeToChar(_ context.Context, keycode uint16, modifier domain.KeyModifier, layout domain.KeyboardLayout) *rune {
	l, err := a.registry.Get(layout)
	if err != nil {
		l = a.registry.Default()
	}
	return l.KeycodeToChar(keycode, modifier)
}

// DetectLayout detects the system's current keyboard layout.
func (a *Adapter) DetectLayout(ctx context.Context) domain.KeyboardLayout {
	return DetectSystemLayout(ctx)
}

// AvailableLayouts returns the list of supported keyboard layouts.
func (a *Adapter) AvailableLayouts() []domain.KeyboardLayout {
	return a.registry.Available()
}

// IsLayoutSupported returns true if the given layout is supported.
func (a *Adapter) IsLayoutSupported(layout domain.KeyboardLayout) bool {
	return a.registry.IsSupported(layout)
}

// GetLayout returns a specific layout by name.
func (a *Adapter) GetLayout(name domain.KeyboardLayout) (Layout, error) {
	return a.registry.Get(name)
}

// Compile-time check that Adapter implements the port interface.
// This import is intentionally inside the file to verify interface compliance.
var _ interface {
	CharToKeySequence(ctx context.Context, char rune, layout domain.KeyboardLayout) ([]domain.KeySequence, error)
	KeycodeToChar(ctx context.Context, keycode uint16, modifier domain.KeyModifier, layout domain.KeyboardLayout) *rune
	DetectLayout(ctx context.Context) domain.KeyboardLayout
	AvailableLayouts() []domain.KeyboardLayout
	IsLayoutSupported(layout domain.KeyboardLayout) bool
} = (*Adapter)(nil)
