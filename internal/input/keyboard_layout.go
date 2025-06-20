package input

import (
	"github.com/bnema/waymon/internal/logger"
	evdev "github.com/gvalkov/golang-evdev"
)

// KeyboardLayoutTranslator handles keyboard layout translation between different layouts
type KeyboardLayoutTranslator struct {
	sourceLayout string
	targetLayout string
}

// NewKeyboardLayoutTranslator creates a new keyboard layout translator
func NewKeyboardLayoutTranslator(sourceLayout, targetLayout string) *KeyboardLayoutTranslator {
	return &KeyboardLayoutTranslator{
		sourceLayout: sourceLayout,
		targetLayout: targetLayout,
	}
}

// TranslateKeyCode translates a key code from source layout to target layout
func (k *KeyboardLayoutTranslator) TranslateKeyCode(keyCode uint32) uint32 {
	// If layouts are the same, no translation needed
	if k.sourceLayout == k.targetLayout {
		return keyCode
	}

	// Get the key mapping
	mapping := getKeyMapping(k.sourceLayout, k.targetLayout)
	if mapping == nil {
		logger.Debugf("No translation mapping found for %s -> %s", k.sourceLayout, k.targetLayout)
		return keyCode
	}

	// Translate the key code
	if translatedCode, exists := mapping[keyCode]; exists {
		logger.Debugf("Translated key %d to %d (%s -> %s)", keyCode, translatedCode, k.sourceLayout, k.targetLayout)
		return translatedCode
	}

	// No translation needed for this key
	return keyCode
}

// getKeyMapping returns the key mapping between two layouts
func getKeyMapping(sourceLayout, targetLayout string) map[uint32]uint32 {
	// For now, we'll handle the most common case: QWERTY to AZERTY
	if sourceLayout == "us" && targetLayout == "fr" {
		return qwertyToAzerty
	} else if sourceLayout == "fr" && targetLayout == "us" {
		return azertyToQwerty
	}

	// Add more mappings as needed
	return nil
}

// Key mappings for QWERTY to AZERTY
var qwertyToAzerty = map[uint32]uint32{
	// Number row differences
	evdev.KEY_1: evdev.KEY_1, // 1 -> & (same physical key, different symbol)
	evdev.KEY_2: evdev.KEY_2, // 2 -> é 
	evdev.KEY_3: evdev.KEY_3, // 3 -> "
	evdev.KEY_4: evdev.KEY_4, // 4 -> '
	evdev.KEY_5: evdev.KEY_5, // 5 -> (
	evdev.KEY_6: evdev.KEY_6, // 6 -> -
	evdev.KEY_7: evdev.KEY_7, // 7 -> è
	evdev.KEY_8: evdev.KEY_8, // 8 -> _
	evdev.KEY_9: evdev.KEY_9, // 9 -> ç
	evdev.KEY_0: evdev.KEY_0, // 0 -> à

	// Letter differences
	evdev.KEY_Q: evdev.KEY_A, // Q -> A
	evdev.KEY_W: evdev.KEY_Z, // W -> Z
	evdev.KEY_A: evdev.KEY_Q, // A -> Q
	evdev.KEY_Z: evdev.KEY_W, // Z -> W
	evdev.KEY_M: evdev.KEY_SEMICOLON, // M -> ;
	
	// Punctuation differences
	evdev.KEY_SEMICOLON: evdev.KEY_M,         // ; -> M
	evdev.KEY_APOSTROPHE: evdev.KEY_4,        // ' -> ù
	evdev.KEY_LEFTBRACE: evdev.KEY_5,         // [ -> ^
	evdev.KEY_MINUS: evdev.KEY_6,             // - -> )
	evdev.KEY_RIGHTBRACE: evdev.KEY_MINUS,    // ] -> $
}

// Reverse mapping for AZERTY to QWERTY
var azertyToQwerty = map[uint32]uint32{
	// Letter differences
	evdev.KEY_A: evdev.KEY_Q, // A -> Q
	evdev.KEY_Z: evdev.KEY_W, // Z -> W
	evdev.KEY_Q: evdev.KEY_A, // Q -> A
	evdev.KEY_W: evdev.KEY_Z, // W -> Z
	
	// M and semicolon swap
	evdev.KEY_SEMICOLON: evdev.KEY_M,         // ; -> M
	evdev.KEY_M: evdev.KEY_SEMICOLON,         // M -> ;
	
	// Punctuation differences (reverse of above)
	evdev.KEY_4: evdev.KEY_APOSTROPHE,        // ù -> '
	evdev.KEY_5: evdev.KEY_LEFTBRACE,         // ^ -> [
	evdev.KEY_6: evdev.KEY_MINUS,             // ) -> -
	evdev.KEY_MINUS: evdev.KEY_RIGHTBRACE,    // $ -> ]
}

// GetServerKeyboardLayout detects the keyboard layout on the server
// Since the server runs as root, we need to check system-wide settings
func GetServerKeyboardLayout() string {
	// For servers, we often can't detect the layout reliably
	// Default to US layout unless configured otherwise
	layout := "us"
	
	// You could add server-side detection here if needed
	// For now, we'll assume US layout for servers
	
	logger.Debugf("Server keyboard layout: %s", layout)
	return layout
}