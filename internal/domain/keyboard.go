package domain

// KeyboardLayout represents a keyboard layout identifier.
type KeyboardLayout string

// Supported keyboard layouts.
const (
	LayoutUS KeyboardLayout = "us" // US QWERTY
	LayoutFR KeyboardLayout = "fr" // French AZERTY
	LayoutDE KeyboardLayout = "de" // German QWERTZ
	LayoutES KeyboardLayout = "es" // Spanish QWERTY
	LayoutUK KeyboardLayout = "uk" // UK QWERTY
	LayoutIT KeyboardLayout = "it" // Italian QWERTY
)

// String returns the string representation of KeyboardLayout.
func (l KeyboardLayout) String() string {
	return string(l)
}

// IsValid returns true if the layout is a known supported layout.
func (l KeyboardLayout) IsValid() bool {
	switch l {
	case LayoutUS, LayoutFR, LayoutDE, LayoutES, LayoutUK, LayoutIT:
		return true
	default:
		return false
	}
}

// KeyModifier represents keyboard modifier keys.
type KeyModifier uint8

// Modifier key flags.
const (
	ModNone  KeyModifier = 0
	ModShift KeyModifier = 1 << 0
	ModAltGr KeyModifier = 1 << 1
	ModCtrl  KeyModifier = 1 << 2
	ModAlt   KeyModifier = 1 << 3
)

// HasShift returns true if the Shift modifier is set.
func (m KeyModifier) HasShift() bool {
	return m&ModShift != 0
}

// HasAltGr returns true if the AltGr modifier is set.
func (m KeyModifier) HasAltGr() bool {
	return m&ModAltGr != 0
}

// HasCtrl returns true if the Ctrl modifier is set.
func (m KeyModifier) HasCtrl() bool {
	return m&ModCtrl != 0
}

// HasAlt returns true if the Alt modifier is set.
func (m KeyModifier) HasAlt() bool {
	return m&ModAlt != 0
}

// String returns a human-readable representation of the modifiers.
func (m KeyModifier) String() string {
	if m == ModNone {
		return "none"
	}
	var parts []string
	if m.HasCtrl() {
		parts = append(parts, "Ctrl")
	}
	if m.HasAlt() {
		parts = append(parts, "Alt")
	}
	if m.HasShift() {
		parts = append(parts, "Shift")
	}
	if m.HasAltGr() {
		parts = append(parts, "AltGr")
	}
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += "+"
		}
		result += p
	}
	return result
}

// KeySequence represents a single keystroke with its modifiers.
// For simple characters, a sequence contains one keystroke.
// For dead key combinations, multiple KeySequences are needed.
type KeySequence struct {
	Keycode  uint16
	Modifier KeyModifier
}

// KeyMapping represents a character-to-keycode mapping.
type KeyMapping struct {
	Keycode  uint16
	Modifier KeyModifier
}
