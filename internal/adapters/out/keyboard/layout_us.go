package keyboard

import "github.com/bnema/waymon/internal/domain"

// USLayout implements US QWERTY keyboard layout.
type USLayout struct {
	*BaseLayout
}

// NewUSLayout creates a new US QWERTY layout.
func NewUSLayout() *USLayout {
	// Build the base mappings by merging shared mappings
	base := MergeKeymaps(
		CommonMappings,         // Universal: space, tab, enter, €
		QWERTYBaseMappings,     // Standard QWERTY letter positions
		StandardNumberMappings, // Numbers 0-9 without shift
		usShiftedSymbols,       // US-specific shifted symbols
		usPunctuation,          // US punctuation layout
		usSymbols,              // US symbols and brackets
	)

	return &USLayout{
		BaseLayout: NewBaseLayout(
			domain.LayoutUS,
			base,
			make(map[rune]domain.KeyMapping), // US layout has no dead keys
		),
	}
}

// usShiftedSymbols contains US-specific shifted symbols on the number row.
var usShiftedSymbols = map[rune]domain.KeyMapping{
	'!': {Keycode: Key1, Modifier: domain.ModShift},
	'@': {Keycode: Key2, Modifier: domain.ModShift},
	'#': {Keycode: Key3, Modifier: domain.ModShift},
	'$': {Keycode: Key4, Modifier: domain.ModShift},
	'%': {Keycode: Key5, Modifier: domain.ModShift},
	'^': {Keycode: Key6, Modifier: domain.ModShift},
	'&': {Keycode: Key7, Modifier: domain.ModShift},
	'*': {Keycode: Key8, Modifier: domain.ModShift},
	'(': {Keycode: Key9, Modifier: domain.ModShift},
	')': {Keycode: Key0, Modifier: domain.ModShift},
}

// usPunctuation contains US punctuation layout.
var usPunctuation = map[rune]domain.KeyMapping{
	',': {Keycode: KeyComma, Modifier: domain.ModNone},
	'<': {Keycode: KeyComma, Modifier: domain.ModShift},
	'.': {Keycode: KeyDot, Modifier: domain.ModNone},
	'>': {Keycode: KeyDot, Modifier: domain.ModShift},
	'/': {Keycode: KeySlash, Modifier: domain.ModNone},
	'?': {Keycode: KeySlash, Modifier: domain.ModShift},
}

// usSymbols contains US symbols and brackets.
var usSymbols = map[rune]domain.KeyMapping{
	'-':  {Keycode: KeyMinus, Modifier: domain.ModNone},
	'_':  {Keycode: KeyMinus, Modifier: domain.ModShift},
	'=':  {Keycode: KeyEqual, Modifier: domain.ModNone},
	'+':  {Keycode: KeyEqual, Modifier: domain.ModShift},
	'[':  {Keycode: KeyLeftBrace, Modifier: domain.ModNone},
	'{':  {Keycode: KeyLeftBrace, Modifier: domain.ModShift},
	']':  {Keycode: KeyRightBrace, Modifier: domain.ModNone},
	'}':  {Keycode: KeyRightBrace, Modifier: domain.ModShift},
	'\\': {Keycode: KeyBackslash, Modifier: domain.ModNone},
	'|':  {Keycode: KeyBackslash, Modifier: domain.ModShift},
	';':  {Keycode: KeySemicolon, Modifier: domain.ModNone},
	':':  {Keycode: KeySemicolon, Modifier: domain.ModShift},
	'\'': {Keycode: KeyApostrophe, Modifier: domain.ModNone},
	'"':  {Keycode: KeyApostrophe, Modifier: domain.ModShift},
	'`':  {Keycode: KeyGrave, Modifier: domain.ModNone},
	'~':  {Keycode: KeyGrave, Modifier: domain.ModShift},
}
