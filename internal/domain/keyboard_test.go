package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKeyboardLayout_String(t *testing.T) {
	tests := []struct {
		layout   KeyboardLayout
		expected string
	}{
		{LayoutUS, "us"},
		{LayoutFR, "fr"},
		{LayoutDE, "de"},
		{LayoutES, "es"},
		{LayoutUK, "uk"},
		{LayoutIT, "it"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.layout.String())
		})
	}
}

func TestKeyboardLayout_IsValid(t *testing.T) {
	tests := []struct {
		layout   KeyboardLayout
		expected bool
	}{
		{LayoutUS, true},
		{LayoutFR, true},
		{LayoutDE, true},
		{LayoutES, true},
		{LayoutUK, true},
		{LayoutIT, true},
		{KeyboardLayout("unknown"), false},
		{KeyboardLayout(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.layout), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.layout.IsValid())
		})
	}
}

func TestKeyModifier_HasShift(t *testing.T) {
	assert.True(t, ModShift.HasShift())
	assert.True(t, (ModShift | ModCtrl).HasShift())
	assert.False(t, ModNone.HasShift())
	assert.False(t, ModCtrl.HasShift())
}

func TestKeyModifier_HasAltGr(t *testing.T) {
	assert.True(t, ModAltGr.HasAltGr())
	assert.True(t, (ModAltGr | ModShift).HasAltGr())
	assert.False(t, ModNone.HasAltGr())
	assert.False(t, ModShift.HasAltGr())
}

func TestKeyModifier_HasCtrl(t *testing.T) {
	assert.True(t, ModCtrl.HasCtrl())
	assert.True(t, (ModCtrl | ModShift).HasCtrl())
	assert.False(t, ModNone.HasCtrl())
	assert.False(t, ModShift.HasCtrl())
}

func TestKeyModifier_HasAlt(t *testing.T) {
	assert.True(t, ModAlt.HasAlt())
	assert.True(t, (ModAlt | ModShift).HasAlt())
	assert.False(t, ModNone.HasAlt())
	assert.False(t, ModShift.HasAlt())
}

func TestKeyModifier_String(t *testing.T) {
	tests := []struct {
		modifier KeyModifier
		expected string
	}{
		{ModNone, "none"},
		{ModShift, "Shift"},
		{ModCtrl, "Ctrl"},
		{ModAlt, "Alt"},
		{ModAltGr, "AltGr"},
		{ModCtrl | ModShift, "Ctrl+Shift"},
		{ModCtrl | ModAlt | ModShift, "Ctrl+Alt+Shift"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.modifier.String())
		})
	}
}
