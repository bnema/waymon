package input

import (
	"testing"

	evdev "github.com/gvalkov/golang-evdev"
)

func TestKeyboardLayoutTranslator_TranslateKeyCode(t *testing.T) {
	tests := []struct {
		name         string
		sourceLayout string
		targetLayout string
		inputKey     uint32
		expectedKey  uint32
		description  string
	}{
		// Same layout - no translation
		{
			name:         "same_layout_us",
			sourceLayout: "us",
			targetLayout: "us",
			inputKey:     evdev.KEY_A,
			expectedKey:  evdev.KEY_A,
			description:  "Same layout should not translate",
		},
		{
			name:         "same_layout_fr",
			sourceLayout: "fr",
			targetLayout: "fr",
			inputKey:     evdev.KEY_Q,
			expectedKey:  evdev.KEY_Q,
			description:  "Same layout should not translate",
		},
		
		// QWERTY to AZERTY translations
		{
			name:         "qwerty_to_azerty_Q",
			sourceLayout: "us",
			targetLayout: "fr",
			inputKey:     evdev.KEY_Q,
			expectedKey:  evdev.KEY_A,
			description:  "Q on QWERTY should map to A on AZERTY",
		},
		{
			name:         "qwerty_to_azerty_A",
			sourceLayout: "us",
			targetLayout: "fr",
			inputKey:     evdev.KEY_A,
			expectedKey:  evdev.KEY_Q,
			description:  "A on QWERTY should map to Q on AZERTY",
		},
		{
			name:         "qwerty_to_azerty_W",
			sourceLayout: "us",
			targetLayout: "fr",
			inputKey:     evdev.KEY_W,
			expectedKey:  evdev.KEY_Z,
			description:  "W on QWERTY should map to Z on AZERTY",
		},
		{
			name:         "qwerty_to_azerty_Z",
			sourceLayout: "us",
			targetLayout: "fr",
			inputKey:     evdev.KEY_Z,
			expectedKey:  evdev.KEY_W,
			description:  "Z on QWERTY should map to W on AZERTY",
		},
		{
			name:         "qwerty_to_azerty_M",
			sourceLayout: "us",
			targetLayout: "fr",
			inputKey:     evdev.KEY_M,
			expectedKey:  evdev.KEY_SEMICOLON,
			description:  "M on QWERTY should map to ; on AZERTY",
		},
		{
			name:         "qwerty_to_azerty_semicolon",
			sourceLayout: "us",
			targetLayout: "fr",
			inputKey:     evdev.KEY_SEMICOLON,
			expectedKey:  evdev.KEY_M,
			description:  "; on QWERTY should map to M on AZERTY",
		},
		
		// AZERTY to QWERTY translations (reverse)
		{
			name:         "azerty_to_qwerty_A",
			sourceLayout: "fr",
			targetLayout: "us",
			inputKey:     evdev.KEY_A,
			expectedKey:  evdev.KEY_Q,
			description:  "A on AZERTY should map to Q on QWERTY",
		},
		{
			name:         "azerty_to_qwerty_Q",
			sourceLayout: "fr",
			targetLayout: "us",
			inputKey:     evdev.KEY_Q,
			expectedKey:  evdev.KEY_A,
			description:  "Q on AZERTY should map to A on QWERTY",
		},
		{
			name:         "azerty_to_qwerty_Z",
			sourceLayout: "fr",
			targetLayout: "us",
			inputKey:     evdev.KEY_Z,
			expectedKey:  evdev.KEY_W,
			description:  "Z on AZERTY should map to W on QWERTY",
		},
		{
			name:         "azerty_to_qwerty_W",
			sourceLayout: "fr",
			targetLayout: "us",
			inputKey:     evdev.KEY_W,
			expectedKey:  evdev.KEY_Z,
			description:  "W on AZERTY should map to Z on QWERTY",
		},
		
		// Keys that should NOT be translated (same position)
		{
			name:         "number_keys_no_translation",
			sourceLayout: "us",
			targetLayout: "fr",
			inputKey:     evdev.KEY_1,
			expectedKey:  evdev.KEY_1,
			description:  "Number keys should not be translated (physical position same)",
		},
		{
			name:         "common_keys_E",
			sourceLayout: "us",
			targetLayout: "fr",
			inputKey:     evdev.KEY_E,
			expectedKey:  evdev.KEY_E,
			description:  "E key is in same position on both layouts",
		},
		{
			name:         "common_keys_R",
			sourceLayout: "us",
			targetLayout: "fr",
			inputKey:     evdev.KEY_R,
			expectedKey:  evdev.KEY_R,
			description:  "R key is in same position on both layouts",
		},
		{
			name:         "modifier_keys_ctrl",
			sourceLayout: "us",
			targetLayout: "fr",
			inputKey:     evdev.KEY_LEFTCTRL,
			expectedKey:  evdev.KEY_LEFTCTRL,
			description:  "Modifier keys should not be translated",
		},
		{
			name:         "modifier_keys_alt",
			sourceLayout: "us",
			targetLayout: "fr",
			inputKey:     evdev.KEY_LEFTALT,
			expectedKey:  evdev.KEY_LEFTALT,
			description:  "Modifier keys should not be translated",
		},
		
		// Unknown layout combinations
		{
			name:         "unknown_layout_combination",
			sourceLayout: "us",
			targetLayout: "de",
			inputKey:     evdev.KEY_Y,
			expectedKey:  evdev.KEY_Y,
			description:  "Unknown layout combinations should not translate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			translator := NewKeyboardLayoutTranslator(tt.sourceLayout, tt.targetLayout)
			result := translator.TranslateKeyCode(tt.inputKey)
			
			if result != tt.expectedKey {
				t.Errorf("%s: expected key %d, got %d", tt.description, tt.expectedKey, result)
			}
		})
	}
}

func TestGetKeyMapping(t *testing.T) {
	tests := []struct {
		name         string
		sourceLayout string
		targetLayout string
		shouldBeNil  bool
	}{
		{
			name:         "qwerty_to_azerty",
			sourceLayout: "us",
			targetLayout: "fr",
			shouldBeNil:  false,
		},
		{
			name:         "azerty_to_qwerty",
			sourceLayout: "fr",
			targetLayout: "us",
			shouldBeNil:  false,
		},
		{
			name:         "unsupported_combination",
			sourceLayout: "us",
			targetLayout: "de",
			shouldBeNil:  true,
		},
		{
			name:         "same_layout",
			sourceLayout: "us",
			targetLayout: "us",
			shouldBeNil:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapping := getKeyMapping(tt.sourceLayout, tt.targetLayout)
			
			if tt.shouldBeNil && mapping != nil {
				t.Errorf("Expected nil mapping for %s -> %s, but got mapping", tt.sourceLayout, tt.targetLayout)
			}
			
			if !tt.shouldBeNil && mapping == nil {
				t.Errorf("Expected mapping for %s -> %s, but got nil", tt.sourceLayout, tt.targetLayout)
			}
		})
	}
}

func TestGetServerKeyboardLayout(t *testing.T) {
	// Test that GetServerKeyboardLayout returns a non-empty string
	layout := GetServerKeyboardLayout()
	
	if layout == "" {
		t.Error("GetServerKeyboardLayout should return a non-empty layout")
	}
	
	// For now, it should default to "us"
	if layout != "us" {
		t.Errorf("Expected default layout 'us', got '%s'", layout)
	}
}