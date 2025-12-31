package keyboard

import (
	"context"
	"testing"

	"github.com/bnema/waymon/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdapter_CharToKeySequence_US(t *testing.T) {
	adapter := NewAdapter()
	ctx := context.Background()

	tests := []struct {
		name    string
		char    rune
		layout  domain.KeyboardLayout
		wantLen int
		wantKey uint16
		wantMod domain.KeyModifier
		wantErr bool
	}{
		{
			name:    "lowercase a",
			char:    'a',
			layout:  domain.LayoutUS,
			wantLen: 1,
			wantKey: KeyA,
			wantMod: domain.ModNone,
		},
		{
			name:    "uppercase A",
			char:    'A',
			layout:  domain.LayoutUS,
			wantLen: 1,
			wantKey: KeyA,
			wantMod: domain.ModShift,
		},
		{
			name:    "space",
			char:    ' ',
			layout:  domain.LayoutUS,
			wantLen: 1,
			wantKey: KeySpace,
			wantMod: domain.ModNone,
		},
		{
			name:    "number 1",
			char:    '1',
			layout:  domain.LayoutUS,
			wantLen: 1,
			wantKey: Key1,
			wantMod: domain.ModNone,
		},
		{
			name:    "exclamation mark (shifted 1)",
			char:    '!',
			layout:  domain.LayoutUS,
			wantLen: 1,
			wantKey: Key1,
			wantMod: domain.ModShift,
		},
		{
			name:    "at symbol (shifted 2)",
			char:    '@',
			layout:  domain.LayoutUS,
			wantLen: 1,
			wantKey: Key2,
			wantMod: domain.ModShift,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sequences, err := adapter.CharToKeySequence(ctx, tt.char, tt.layout)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Len(t, sequences, tt.wantLen)
			assert.Equal(t, tt.wantKey, sequences[0].Keycode)
			assert.Equal(t, tt.wantMod, sequences[0].Modifier)
		})
	}
}

func TestAdapter_CharToKeySequence_FR(t *testing.T) {
	adapter := NewAdapter()
	ctx := context.Background()

	tests := []struct {
		name    string
		char    rune
		layout  domain.KeyboardLayout
		wantLen int
		wantKey uint16
		wantMod domain.KeyModifier
	}{
		{
			name:    "lowercase a (at Q position in AZERTY)",
			char:    'a',
			layout:  domain.LayoutFR,
			wantLen: 1,
			wantKey: KeyQ, // AZERTY: 'a' is where 'q' is on QWERTY
			wantMod: domain.ModNone,
		},
		{
			name:    "lowercase q (at A position in AZERTY)",
			char:    'q',
			layout:  domain.LayoutFR,
			wantLen: 1,
			wantKey: KeyA, // AZERTY: 'q' is where 'a' is on QWERTY
			wantMod: domain.ModNone,
		},
		{
			name:    "lowercase z (at W position in AZERTY)",
			char:    'z',
			layout:  domain.LayoutFR,
			wantLen: 1,
			wantKey: KeyW, // AZERTY: 'z' is where 'w' is on QWERTY
			wantMod: domain.ModNone,
		},
		{
			name:    "lowercase w (at Z position in AZERTY)",
			char:    'w',
			layout:  domain.LayoutFR,
			wantLen: 1,
			wantKey: KeyZ, // AZERTY: 'w' is where 'z' is on QWERTY
			wantMod: domain.ModNone,
		},
		{
			name:    "number 1 (shifted in AZERTY)",
			char:    '1',
			layout:  domain.LayoutFR,
			wantLen: 1,
			wantKey: Key1,
			wantMod: domain.ModShift, // Numbers require shift in AZERTY
		},
		{
			name:    "é (precomposed, no shift)",
			char:    'é',
			layout:  domain.LayoutFR,
			wantLen: 1,
			wantKey: Key2, // é is directly on key 2 in FR
			wantMod: domain.ModNone,
		},
		{
			name:    "è (precomposed, no shift)",
			char:    'è',
			layout:  domain.LayoutFR,
			wantLen: 1,
			wantKey: Key7, // è is directly on key 7 in FR
			wantMod: domain.ModNone,
		},
		{
			name:    "à (precomposed, no shift)",
			char:    'à',
			layout:  domain.LayoutFR,
			wantLen: 1,
			wantKey: Key0, // à is directly on key 0 in FR
			wantMod: domain.ModNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sequences, err := adapter.CharToKeySequence(ctx, tt.char, tt.layout)

			require.NoError(t, err)
			require.Len(t, sequences, tt.wantLen)
			assert.Equal(t, tt.wantKey, sequences[0].Keycode)
			assert.Equal(t, tt.wantMod, sequences[0].Modifier)
		})
	}
}

func TestAdapter_CharToKeySequence_FR_DeadKeys(t *testing.T) {
	adapter := NewAdapter()
	ctx := context.Background()

	tests := []struct {
		name    string
		char    rune
		layout  domain.KeyboardLayout
		wantLen int
	}{
		{
			name:    "ô (circumflex + o)",
			char:    'ô',
			layout:  domain.LayoutFR,
			wantLen: 2, // Dead key ^ + o
		},
		{
			name:    "ê (circumflex + e)",
			char:    'ê',
			layout:  domain.LayoutFR,
			wantLen: 2, // Dead key ^ + e
		},
		{
			name:    "ë (diaeresis + e)",
			char:    'ë',
			layout:  domain.LayoutFR,
			wantLen: 2, // Dead key ¨ + e
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sequences, err := adapter.CharToKeySequence(ctx, tt.char, tt.layout)

			require.NoError(t, err)
			assert.Len(t, sequences, tt.wantLen, "expected %d keystrokes for dead key combination", tt.wantLen)
		})
	}
}

func TestAdapter_KeycodeToChar_US(t *testing.T) {
	adapter := NewAdapter()
	ctx := context.Background()

	tests := []struct {
		name     string
		keycode  uint16
		modifier domain.KeyModifier
		layout   domain.KeyboardLayout
		wantChar *rune
	}{
		{
			name:     "KeyA no modifier -> 'a'",
			keycode:  KeyA,
			modifier: domain.ModNone,
			layout:   domain.LayoutUS,
			wantChar: runePtr('a'),
		},
		{
			name:     "KeyA with shift -> 'A'",
			keycode:  KeyA,
			modifier: domain.ModShift,
			layout:   domain.LayoutUS,
			wantChar: runePtr('A'),
		},
		{
			name:     "Key1 no modifier -> '1'",
			keycode:  Key1,
			modifier: domain.ModNone,
			layout:   domain.LayoutUS,
			wantChar: runePtr('1'),
		},
		{
			name:     "Key1 with shift -> '!'",
			keycode:  Key1,
			modifier: domain.ModShift,
			layout:   domain.LayoutUS,
			wantChar: runePtr('!'),
		},
		{
			name:     "KeySpace -> ' '",
			keycode:  KeySpace,
			modifier: domain.ModNone,
			layout:   domain.LayoutUS,
			wantChar: runePtr(' '),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := adapter.KeycodeToChar(ctx, tt.keycode, tt.modifier, tt.layout)

			if tt.wantChar == nil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Equal(t, *tt.wantChar, *result)
			}
		})
	}
}

func TestAdapter_DetectLayout(t *testing.T) {
	adapter := NewAdapter()
	ctx := context.Background()

	// DetectLayout should return a valid layout (at minimum US as default)
	layout := adapter.DetectLayout(ctx)
	assert.True(t, layout.IsValid(), "detected layout should be valid")
}

func TestAdapter_AvailableLayouts(t *testing.T) {
	adapter := NewAdapter()

	layouts := adapter.AvailableLayouts()
	assert.GreaterOrEqual(t, len(layouts), 2, "should have at least US and FR layouts")

	// Check that US and FR are available
	hasUS := false
	hasFR := false
	for _, l := range layouts {
		if l == domain.LayoutUS {
			hasUS = true
		}
		if l == domain.LayoutFR {
			hasFR = true
		}
	}
	assert.True(t, hasUS, "US layout should be available")
	assert.True(t, hasFR, "FR layout should be available")
}

func TestAdapter_IsLayoutSupported(t *testing.T) {
	adapter := NewAdapter()

	assert.True(t, adapter.IsLayoutSupported(domain.LayoutUS))
	assert.True(t, adapter.IsLayoutSupported(domain.LayoutFR))
	assert.False(t, adapter.IsLayoutSupported(domain.KeyboardLayout("unknown")))
}

func TestRegistry_Get(t *testing.T) {
	registry := NewRegistry()

	t.Run("get existing layout", func(t *testing.T) {
		layout, err := registry.Get(domain.LayoutUS)
		require.NoError(t, err)
		assert.Equal(t, domain.LayoutUS, layout.Name())
	})

	t.Run("get non-existent layout", func(t *testing.T) {
		_, err := registry.Get(domain.KeyboardLayout("nonexistent"))
		assert.Error(t, err)
	})
}

func TestUSLayout_CharToKeySequence(t *testing.T) {
	layout := NewUSLayout()
	ctx := context.Background()

	t.Run("simple character", func(t *testing.T) {
		sequences, err := layout.CharToKeySequence(ctx, 'a')
		require.NoError(t, err)
		require.Len(t, sequences, 1)
		assert.Equal(t, KeyA, sequences[0].Keycode)
		assert.Equal(t, domain.ModNone, sequences[0].Modifier)
	})

	t.Run("unsupported character", func(t *testing.T) {
		_, err := layout.CharToKeySequence(ctx, 'é')
		assert.Error(t, err)
		var errNotSupported *ErrCharNotSupported
		assert.ErrorAs(t, err, &errNotSupported)
	})
}

func TestFRLayout_CharToKeySequence(t *testing.T) {
	layout := NewFRLayout()
	ctx := context.Background()

	t.Run("precomposed accent é", func(t *testing.T) {
		sequences, err := layout.CharToKeySequence(ctx, 'é')
		require.NoError(t, err)
		require.Len(t, sequences, 1)
		assert.Equal(t, Key2, sequences[0].Keycode)
		assert.Equal(t, domain.ModNone, sequences[0].Modifier)
	})

	t.Run("dead key combination ô", func(t *testing.T) {
		sequences, err := layout.CharToKeySequence(ctx, 'ô')
		require.NoError(t, err)
		assert.Len(t, sequences, 2) // ^ + o
	})
}

// Helper function
func runePtr(r rune) *rune {
	return &r
}
