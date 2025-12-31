package out

import (
	"context"

	"github.com/bnema/waymon/internal/domain"
)

// KeyboardLayoutPort defines the interface for keyboard layout operations.
// This port abstracts the keyboard layout translation logic, allowing the use cases
// to work with characters semantically while the adapter handles the keycode mapping.
type KeyboardLayoutPort interface {
	// CharToKeySequence converts a Unicode character to a sequence of keystrokes
	// for the specified keyboard layout. Returns an error if the character
	// cannot be typed on the given layout.
	//
	// For simple characters, returns a single-element slice.
	// For characters requiring dead keys (like ô = ^ + o), returns multiple keystrokes.
	CharToKeySequence(ctx context.Context, char rune, layout domain.KeyboardLayout) ([]domain.KeySequence, error)

	// KeycodeToChar converts a keycode with modifiers to a Unicode character
	// for the specified keyboard layout. Returns nil if the keycode doesn't
	// produce a printable character (e.g., modifier keys, function keys).
	//
	// This is the reverse operation of CharToKeySequence for simple characters.
	KeycodeToChar(ctx context.Context, keycode uint16, modifier domain.KeyModifier, layout domain.KeyboardLayout) *rune

	// DetectLayout detects the system's current keyboard layout.
	// Returns LayoutUS as default if detection fails.
	DetectLayout(ctx context.Context) domain.KeyboardLayout

	// AvailableLayouts returns the list of supported keyboard layouts.
	AvailableLayouts() []domain.KeyboardLayout

	// IsLayoutSupported returns true if the given layout is supported.
	IsLayoutSupported(layout domain.KeyboardLayout) bool
}
