package keyboard

import "github.com/bnema/waymon/internal/domain"

// FRLayout implements French AZERTY keyboard layout.
type FRLayout struct {
	*BaseLayout
}

// NewFRLayout creates a new French AZERTY layout.
func NewFRLayout() *FRLayout {
	// Build the base mappings by merging common and French-specific mappings
	base := MergeKeymaps(
		CommonMappings,       // Universal: space, tab, enter, €
		frAZERTYLetters,      // AZERTY letter positions
		frNumberRow,          // French number row (shifted)
		frPrecomposedAccents, // Direct keys for é, è, à, ç, ù
		frPunctuation,        // French punctuation layout
		frAltGrSymbols,       // AltGr combinations
		frRemainingSymbols,   // Other French-specific symbols
	)

	return &FRLayout{
		BaseLayout: NewBaseLayout(
			domain.LayoutFR,
			base,
			frDeadKeys,
		),
	}
}

// frDeadKeys maps dead key symbols to their physical location on French AZERTY keyboard.
var frDeadKeys = map[rune]domain.KeyMapping{
	'^': {Keycode: KeyLeftBrace, Modifier: domain.ModNone},  // Circumflex
	'¨': {Keycode: KeyLeftBrace, Modifier: domain.ModShift}, // Diaeresis
}

// frAZERTYLetters contains the AZERTY letter layout.
// Unlike QWERTY, several letters are in different positions:
// - First row: a→Q, z→W
// - Second row: q→A
// - Third row: w→Z
// - m is at Semicolon position
var frAZERTYLetters = map[rune]domain.KeyMapping{
	// First row - AZERTY specific positions
	'a': {Keycode: KeyQ, Modifier: domain.ModNone},
	'A': {Keycode: KeyQ, Modifier: domain.ModShift},
	'z': {Keycode: KeyW, Modifier: domain.ModNone},
	'Z': {Keycode: KeyW, Modifier: domain.ModShift},

	// First row - same as QWERTY
	'e': {Keycode: KeyE, Modifier: domain.ModNone},
	'E': {Keycode: KeyE, Modifier: domain.ModShift},
	'r': {Keycode: KeyR, Modifier: domain.ModNone},
	'R': {Keycode: KeyR, Modifier: domain.ModShift},
	't': {Keycode: KeyT, Modifier: domain.ModNone},
	'T': {Keycode: KeyT, Modifier: domain.ModShift},
	'y': {Keycode: KeyY, Modifier: domain.ModNone},
	'Y': {Keycode: KeyY, Modifier: domain.ModShift},
	'u': {Keycode: KeyU, Modifier: domain.ModNone},
	'U': {Keycode: KeyU, Modifier: domain.ModShift},
	'i': {Keycode: KeyI, Modifier: domain.ModNone},
	'I': {Keycode: KeyI, Modifier: domain.ModShift},
	'o': {Keycode: KeyO, Modifier: domain.ModNone},
	'O': {Keycode: KeyO, Modifier: domain.ModShift},
	'p': {Keycode: KeyP, Modifier: domain.ModNone},
	'P': {Keycode: KeyP, Modifier: domain.ModShift},

	// Second row - q is at KeyA position
	'q': {Keycode: KeyA, Modifier: domain.ModNone},
	'Q': {Keycode: KeyA, Modifier: domain.ModShift},

	// Second row - same as QWERTY
	's': {Keycode: KeyS, Modifier: domain.ModNone},
	'S': {Keycode: KeyS, Modifier: domain.ModShift},
	'd': {Keycode: KeyD, Modifier: domain.ModNone},
	'D': {Keycode: KeyD, Modifier: domain.ModShift},
	'f': {Keycode: KeyF, Modifier: domain.ModNone},
	'F': {Keycode: KeyF, Modifier: domain.ModShift},
	'g': {Keycode: KeyG, Modifier: domain.ModNone},
	'G': {Keycode: KeyG, Modifier: domain.ModShift},
	'h': {Keycode: KeyH, Modifier: domain.ModNone},
	'H': {Keycode: KeyH, Modifier: domain.ModShift},
	'j': {Keycode: KeyJ, Modifier: domain.ModNone},
	'J': {Keycode: KeyJ, Modifier: domain.ModShift},
	'k': {Keycode: KeyK, Modifier: domain.ModNone},
	'K': {Keycode: KeyK, Modifier: domain.ModShift},
	'l': {Keycode: KeyL, Modifier: domain.ModNone},
	'L': {Keycode: KeyL, Modifier: domain.ModShift},

	// Second row - m is at Semicolon position
	'm': {Keycode: KeySemicolon, Modifier: domain.ModNone},
	'M': {Keycode: KeySemicolon, Modifier: domain.ModShift},

	// Third row - w is at KeyZ position
	'w': {Keycode: KeyZ, Modifier: domain.ModNone},
	'W': {Keycode: KeyZ, Modifier: domain.ModShift},

	// Third row - same as QWERTY
	'x': {Keycode: KeyX, Modifier: domain.ModNone},
	'X': {Keycode: KeyX, Modifier: domain.ModShift},
	'c': {Keycode: KeyC, Modifier: domain.ModNone},
	'C': {Keycode: KeyC, Modifier: domain.ModShift},
	'v': {Keycode: KeyV, Modifier: domain.ModNone},
	'V': {Keycode: KeyV, Modifier: domain.ModShift},
	'b': {Keycode: KeyB, Modifier: domain.ModNone},
	'B': {Keycode: KeyB, Modifier: domain.ModShift},
	'n': {Keycode: KeyN, Modifier: domain.ModNone},
	'N': {Keycode: KeyN, Modifier: domain.ModShift},
}

// frNumberRow contains the French AZERTY number row.
// In French AZERTY, numbers require SHIFT, and symbols are unshifted.
var frNumberRow = map[rune]domain.KeyMapping{
	// Shifted numbers
	'1': {Keycode: Key1, Modifier: domain.ModShift},
	'2': {Keycode: Key2, Modifier: domain.ModShift},
	'3': {Keycode: Key3, Modifier: domain.ModShift},
	'4': {Keycode: Key4, Modifier: domain.ModShift},
	'5': {Keycode: Key5, Modifier: domain.ModShift},
	'6': {Keycode: Key6, Modifier: domain.ModShift},
	'7': {Keycode: Key7, Modifier: domain.ModShift},
	'8': {Keycode: Key8, Modifier: domain.ModShift},
	'9': {Keycode: Key9, Modifier: domain.ModShift},
	'0': {Keycode: Key0, Modifier: domain.ModShift},

	// Unshifted symbols on number row
	'&': {Keycode: Key1, Modifier: domain.ModNone},
	// Note: é is in frPrecomposedAccents (Key2, ModNone)
	'"':  {Keycode: Key3, Modifier: domain.ModNone},
	'\'': {Keycode: Key4, Modifier: domain.ModNone},
	'(':  {Keycode: Key5, Modifier: domain.ModNone},
	'-':  {Keycode: Key6, Modifier: domain.ModNone},
	// Note: è is in frPrecomposedAccents (Key7, ModNone)
	'_': {Keycode: Key8, Modifier: domain.ModNone},
	// Note: ç is in frPrecomposedAccents (Key9, ModNone)
	// Note: à is in frPrecomposedAccents (Key0, ModNone)
}

// frPrecomposedAccents contains French characters that have dedicated keys
// (not requiring dead key combinations).
var frPrecomposedAccents = map[rune]domain.KeyMapping{
	'é': {Keycode: Key2, Modifier: domain.ModNone},
	'è': {Keycode: Key7, Modifier: domain.ModNone},
	'à': {Keycode: Key0, Modifier: domain.ModNone},
	'ç': {Keycode: Key9, Modifier: domain.ModNone},
	'ù': {Keycode: KeyApostrophe, Modifier: domain.ModNone},
}

// frPunctuation contains French punctuation layout.
var frPunctuation = map[rune]domain.KeyMapping{
	',': {Keycode: KeyM, Modifier: domain.ModNone},
	'?': {Keycode: KeyM, Modifier: domain.ModShift},
	';': {Keycode: KeyComma, Modifier: domain.ModNone},
	'.': {Keycode: KeyComma, Modifier: domain.ModShift},
	':': {Keycode: KeyDot, Modifier: domain.ModNone},
	'/': {Keycode: KeyDot, Modifier: domain.ModShift},
	'!': {Keycode: KeySlash, Modifier: domain.ModNone},
}

// frAltGrSymbols contains symbols accessible with AltGr.
var frAltGrSymbols = map[rune]domain.KeyMapping{
	'[':  {Keycode: Key5, Modifier: domain.ModAltGr},
	']':  {Keycode: KeyMinus, Modifier: domain.ModAltGr},
	'{':  {Keycode: Key4, Modifier: domain.ModAltGr},
	'}':  {Keycode: KeyEqual, Modifier: domain.ModAltGr},
	'@':  {Keycode: Key0, Modifier: domain.ModAltGr},
	'#':  {Keycode: Key3, Modifier: domain.ModAltGr},
	'~':  {Keycode: Key2, Modifier: domain.ModAltGr},
	'\\': {Keycode: Key8, Modifier: domain.ModAltGr},
	'|':  {Keycode: Key6, Modifier: domain.ModAltGr},
	'`':  {Keycode: Key7, Modifier: domain.ModAltGr},
}

// frRemainingSymbols contains other French-specific symbols.
var frRemainingSymbols = map[rune]domain.KeyMapping{
	'°': {Keycode: KeyMinus, Modifier: domain.ModShift},
	')': {Keycode: KeyEqual, Modifier: domain.ModNone},
	'=': {Keycode: KeyEqual, Modifier: domain.ModShift},
	'^': {Keycode: KeyLeftBrace, Modifier: domain.ModNone},  // Also a dead key
	'¨': {Keycode: KeyLeftBrace, Modifier: domain.ModShift}, // Also a dead key
	'$': {Keycode: KeyRightBrace, Modifier: domain.ModNone},
	'£': {Keycode: KeyRightBrace, Modifier: domain.ModShift},
	'*': {Keycode: KeyBackslash, Modifier: domain.ModNone},
	'µ': {Keycode: KeyBackslash, Modifier: domain.ModShift},
	'%': {Keycode: KeyApostrophe, Modifier: domain.ModShift},
	'<': {Keycode: Key102ND, Modifier: domain.ModNone},
	'>': {Keycode: Key102ND, Modifier: domain.ModShift},
}
