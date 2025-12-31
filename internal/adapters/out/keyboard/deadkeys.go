package keyboard

// DeadKeyComposition defines a dead key combination that produces an accented character.
// Dead keys are modifier keys that don't produce output immediately but wait for
// the next keystroke to compose an accented character.
//
// Example: On a French keyboard, pressing circumflex (^) then 'o' produces 'ô'.
type DeadKeyComposition struct {
	DeadKey  rune // The dead key character (e.g., '^', '´', '`', '¨', '~')
	BaseChar rune // The base character to combine with (e.g., 'a', 'e', 'o')
	Result   rune // The resulting composed character (e.g., 'â', 'é', 'ò')
}

// CommonDeadKeyCompositions contains ALL universal dead key combinations.
// These combinations work the SAME across all keyboard layouts - only the
// physical location of the dead key varies by layout.
//
// For example: ^ + a → â works the same in French, German, Spanish, etc.
// The difference is WHERE the circumflex key is located on each layout.
var CommonDeadKeyCompositions = []DeadKeyComposition{
	// Circumflex (^) - lowercase
	{'^', 'a', 'â'},
	{'^', 'e', 'ê'},
	{'^', 'i', 'î'},
	{'^', 'o', 'ô'},
	{'^', 'u', 'û'},

	// Circumflex (^) - uppercase
	{'^', 'A', 'Â'},
	{'^', 'E', 'Ê'},
	{'^', 'I', 'Î'},
	{'^', 'O', 'Ô'},
	{'^', 'U', 'Û'},

	// Acute (´) - lowercase
	{'´', 'a', 'á'},
	{'´', 'e', 'é'},
	{'´', 'i', 'í'},
	{'´', 'o', 'ó'},
	{'´', 'u', 'ú'},
	{'´', 'y', 'ý'},

	// Acute (´) - uppercase
	{'´', 'A', 'Á'},
	{'´', 'E', 'É'},
	{'´', 'I', 'Í'},
	{'´', 'O', 'Ó'},
	{'´', 'U', 'Ú'},
	{'´', 'Y', 'Ý'},

	// Grave (`) - lowercase
	{'`', 'a', 'à'},
	{'`', 'e', 'è'},
	{'`', 'i', 'ì'},
	{'`', 'o', 'ò'},
	{'`', 'u', 'ù'},

	// Grave (`) - uppercase
	{'`', 'A', 'À'},
	{'`', 'E', 'È'},
	{'`', 'I', 'Ì'},
	{'`', 'O', 'Ò'},
	{'`', 'U', 'Ù'},

	// Diaeresis (¨) - lowercase
	{'¨', 'a', 'ä'},
	{'¨', 'e', 'ë'},
	{'¨', 'i', 'ï'},
	{'¨', 'o', 'ö'},
	{'¨', 'u', 'ü'},
	{'¨', 'y', 'ÿ'},

	// Diaeresis (¨) - uppercase
	{'¨', 'A', 'Ä'},
	{'¨', 'E', 'Ë'},
	{'¨', 'I', 'Ï'},
	{'¨', 'O', 'Ö'},
	{'¨', 'U', 'Ü'},
	{'¨', 'Y', 'Ÿ'},

	// Tilde (~) - lowercase
	{'~', 'a', 'ã'},
	{'~', 'n', 'ñ'},
	{'~', 'o', 'õ'},

	// Tilde (~) - uppercase
	{'~', 'A', 'Ã'},
	{'~', 'N', 'Ñ'},
	{'~', 'O', 'Õ'},
}

// DeadKeyRegistry provides fast lookup of dead key combinations.
// The key is the composed result character, the value is the composition rule.
type DeadKeyRegistry map[rune]DeadKeyComposition

// BuildDeadKeyRegistry creates a fast lookup map from the composition list.
// This allows O(1) lookup of "which dead key combination produces this character?"
func BuildDeadKeyRegistry() DeadKeyRegistry {
	registry := make(DeadKeyRegistry, len(CommonDeadKeyCompositions))
	for _, comp := range CommonDeadKeyCompositions {
		registry[comp.Result] = comp
	}
	return registry
}
