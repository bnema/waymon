package keyboard

import (
	"context"
	"fmt"

	"github.com/bnema/waymon/internal/domain"
)

// Layout defines the interface for keyboard layout implementations.
// Each layout maps Unicode characters to Linux keycodes with appropriate modifiers.
type Layout interface {
	// Name returns the layout identifier (e.g., "us", "fr").
	Name() domain.KeyboardLayout

	// CharToKeySequence converts a Unicode character to a sequence of keystrokes.
	// For simple characters, returns a single-element slice.
	// For dead key combinations (like ô = ^ + o), returns multiple keystrokes.
	CharToKeySequence(ctx context.Context, char rune) ([]domain.KeySequence, error)

	// KeycodeToChar converts a keycode with modifiers to a character.
	// Returns nil if the keycode doesn't produce a printable character.
	KeycodeToChar(keycode uint16, modifier domain.KeyModifier) *rune

	// GetBaseMappings returns the base character-to-keycode mappings for this layout.
	// Used for building reverse mappings.
	GetBaseMappings() map[rune]domain.KeyMapping
}

// ErrCharNotSupported is returned when a character has no mapping in the layout.
type ErrCharNotSupported struct {
	Char   rune
	Layout domain.KeyboardLayout
}

func (e *ErrCharNotSupported) Error() string {
	return fmt.Sprintf("character %q (U+%04X) not supported in %s layout", e.Char, e.Char, e.Layout)
}

// BaseLayout provides common functionality for layout implementations.
type BaseLayout struct {
	name            domain.KeyboardLayout
	baseMappings    map[rune]domain.KeyMapping
	deadKeyRegistry DeadKeyRegistry
	deadKeys        map[rune]domain.KeyMapping
	reverseMapping  map[uint32]rune
}

// NewBaseLayout creates a new BaseLayout with the given mappings.
func NewBaseLayout(
	name domain.KeyboardLayout,
	baseMappings map[rune]domain.KeyMapping,
	deadKeys map[rune]domain.KeyMapping,
) *BaseLayout {
	return &BaseLayout{
		name:            name,
		baseMappings:    baseMappings,
		deadKeyRegistry: BuildDeadKeyRegistry(),
		deadKeys:        deadKeys,
		reverseMapping:  BuildReverseMapping(baseMappings),
	}
}

// Name returns the layout identifier.
func (l *BaseLayout) Name() domain.KeyboardLayout {
	return l.name
}

// GetBaseMappings returns the base mappings.
func (l *BaseLayout) GetBaseMappings() map[rune]domain.KeyMapping {
	return l.baseMappings
}

// CharToKeySequence converts a Unicode character to a sequence of keystrokes.
func (l *BaseLayout) CharToKeySequence(_ context.Context, char rune) ([]domain.KeySequence, error) {
	// First, check if it's a direct mapping
	if mapping, ok := l.baseMappings[char]; ok {
		return []domain.KeySequence{{Keycode: mapping.Keycode, Modifier: mapping.Modifier}}, nil
	}

	// Check if it needs a dead key combination
	if comp, ok := l.deadKeyRegistry[char]; ok {
		// Get the dead key mapping for this layout
		deadKeyMapping, hasDead := l.deadKeys[comp.DeadKey]
		if !hasDead {
			// This layout doesn't have this dead key
			return nil, &ErrCharNotSupported{Char: char, Layout: l.name}
		}

		// Get the base character mapping
		baseMapping, hasBase := l.baseMappings[comp.BaseChar]
		if !hasBase {
			return nil, &ErrCharNotSupported{Char: char, Layout: l.name}
		}

		// Return the sequence: dead key, then base character
		return []domain.KeySequence{
			{Keycode: deadKeyMapping.Keycode, Modifier: deadKeyMapping.Modifier},
			{Keycode: baseMapping.Keycode, Modifier: baseMapping.Modifier},
		}, nil
	}

	return nil, &ErrCharNotSupported{Char: char, Layout: l.name}
}

// KeycodeToChar converts a keycode with modifiers to a character.
func (l *BaseLayout) KeycodeToChar(keycode uint16, modifier domain.KeyModifier) *rune {
	key := MakeReverseKey(keycode, modifier)
	if char, ok := l.reverseMapping[key]; ok {
		return &char
	}
	return nil
}
