package keyboard

import "github.com/bnema/waymon/internal/domain"

// CommonMappings contains truly universal character mappings that work
// identically across ALL keyboard layouts.
var CommonMappings = map[rune]domain.KeyMapping{
	' ':  {Keycode: KeySpace, Modifier: domain.ModNone},
	'\t': {Keycode: KeyTab, Modifier: domain.ModNone},
	'\n': {Keycode: KeyEnter, Modifier: domain.ModNone},
	'€':  {Keycode: KeyE, Modifier: domain.ModAltGr},
}

// QWERTYBaseMappings contains the standard QWERTY letter positions.
// This is used by US, UK, and other QWERTY-based layouts where letters
// map directly to their corresponding key positions (a→KeyA, b→KeyB, etc.).
//
// Note: AZERTY (French) and QWERTZ (German) layouts should NOT use this
// as their letter positions are different.
var QWERTYBaseMappings = map[rune]domain.KeyMapping{
	// Lowercase letters
	'a': {Keycode: KeyA, Modifier: domain.ModNone},
	'b': {Keycode: KeyB, Modifier: domain.ModNone},
	'c': {Keycode: KeyC, Modifier: domain.ModNone},
	'd': {Keycode: KeyD, Modifier: domain.ModNone},
	'e': {Keycode: KeyE, Modifier: domain.ModNone},
	'f': {Keycode: KeyF, Modifier: domain.ModNone},
	'g': {Keycode: KeyG, Modifier: domain.ModNone},
	'h': {Keycode: KeyH, Modifier: domain.ModNone},
	'i': {Keycode: KeyI, Modifier: domain.ModNone},
	'j': {Keycode: KeyJ, Modifier: domain.ModNone},
	'k': {Keycode: KeyK, Modifier: domain.ModNone},
	'l': {Keycode: KeyL, Modifier: domain.ModNone},
	'm': {Keycode: KeyM, Modifier: domain.ModNone},
	'n': {Keycode: KeyN, Modifier: domain.ModNone},
	'o': {Keycode: KeyO, Modifier: domain.ModNone},
	'p': {Keycode: KeyP, Modifier: domain.ModNone},
	'q': {Keycode: KeyQ, Modifier: domain.ModNone},
	'r': {Keycode: KeyR, Modifier: domain.ModNone},
	's': {Keycode: KeyS, Modifier: domain.ModNone},
	't': {Keycode: KeyT, Modifier: domain.ModNone},
	'u': {Keycode: KeyU, Modifier: domain.ModNone},
	'v': {Keycode: KeyV, Modifier: domain.ModNone},
	'w': {Keycode: KeyW, Modifier: domain.ModNone},
	'x': {Keycode: KeyX, Modifier: domain.ModNone},
	'y': {Keycode: KeyY, Modifier: domain.ModNone},
	'z': {Keycode: KeyZ, Modifier: domain.ModNone},

	// Uppercase letters
	'A': {Keycode: KeyA, Modifier: domain.ModShift},
	'B': {Keycode: KeyB, Modifier: domain.ModShift},
	'C': {Keycode: KeyC, Modifier: domain.ModShift},
	'D': {Keycode: KeyD, Modifier: domain.ModShift},
	'E': {Keycode: KeyE, Modifier: domain.ModShift},
	'F': {Keycode: KeyF, Modifier: domain.ModShift},
	'G': {Keycode: KeyG, Modifier: domain.ModShift},
	'H': {Keycode: KeyH, Modifier: domain.ModShift},
	'I': {Keycode: KeyI, Modifier: domain.ModShift},
	'J': {Keycode: KeyJ, Modifier: domain.ModShift},
	'K': {Keycode: KeyK, Modifier: domain.ModShift},
	'L': {Keycode: KeyL, Modifier: domain.ModShift},
	'M': {Keycode: KeyM, Modifier: domain.ModShift},
	'N': {Keycode: KeyN, Modifier: domain.ModShift},
	'O': {Keycode: KeyO, Modifier: domain.ModShift},
	'P': {Keycode: KeyP, Modifier: domain.ModShift},
	'Q': {Keycode: KeyQ, Modifier: domain.ModShift},
	'R': {Keycode: KeyR, Modifier: domain.ModShift},
	'S': {Keycode: KeyS, Modifier: domain.ModShift},
	'T': {Keycode: KeyT, Modifier: domain.ModShift},
	'U': {Keycode: KeyU, Modifier: domain.ModShift},
	'V': {Keycode: KeyV, Modifier: domain.ModShift},
	'W': {Keycode: KeyW, Modifier: domain.ModShift},
	'X': {Keycode: KeyX, Modifier: domain.ModShift},
	'Y': {Keycode: KeyY, Modifier: domain.ModShift},
	'Z': {Keycode: KeyZ, Modifier: domain.ModShift},
}

// StandardNumberMappings contains the number row for standard QWERTY layouts.
// On QWERTY layouts (US, UK, etc.), numbers 0-9 are typed without shift.
//
// Note: French AZERTY requires shift for numbers, so this should NOT be used
// for French layouts.
var StandardNumberMappings = map[rune]domain.KeyMapping{
	'0': {Keycode: Key0, Modifier: domain.ModNone},
	'1': {Keycode: Key1, Modifier: domain.ModNone},
	'2': {Keycode: Key2, Modifier: domain.ModNone},
	'3': {Keycode: Key3, Modifier: domain.ModNone},
	'4': {Keycode: Key4, Modifier: domain.ModNone},
	'5': {Keycode: Key5, Modifier: domain.ModNone},
	'6': {Keycode: Key6, Modifier: domain.ModNone},
	'7': {Keycode: Key7, Modifier: domain.ModNone},
	'8': {Keycode: Key8, Modifier: domain.ModNone},
	'9': {Keycode: Key9, Modifier: domain.ModNone},
}

// MergeKeymaps merges multiple keymaps into a single map.
// Later maps override earlier ones in case of conflicts.
// This allows layouts to compose their mappings from shared bases
// plus layout-specific overrides.
func MergeKeymaps(maps ...map[rune]domain.KeyMapping) map[rune]domain.KeyMapping {
	result := make(map[rune]domain.KeyMapping)
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

// BuildReverseMapping creates a keycode+modifier → character mapping
// from a character → keycode mapping. This is used for translating
// captured keycodes back to characters on the server side.
func BuildReverseMapping(charToKey map[rune]domain.KeyMapping) map[uint32]rune {
	result := make(map[uint32]rune)
	for char, mapping := range charToKey {
		// Create a unique key combining keycode and modifier
		key := uint32(mapping.Keycode) | (uint32(mapping.Modifier) << 16)
		result[key] = char
	}
	return result
}

// MakeReverseKey creates the lookup key for reverse mapping.
func MakeReverseKey(keycode uint16, modifier domain.KeyModifier) uint32 {
	return uint32(keycode) | (uint32(modifier) << 16)
}
