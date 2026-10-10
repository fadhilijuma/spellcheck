package spell

import (
	"fmt"
	"unicode"
	"unicode/utf8"
)

// MaxWordLen is the longest word, in runes, that ParseWord accepts.
const MaxWordLen = 64

// Word is a validated word as the user typed it; case is preserved.
type Word struct {
	value string
}

// String returns the word as typed.
func (w Word) String() string {
	return w.value
}

// ParseWord returns a Word if value is 1 to MaxWordLen runes of letters, with
// apostrophes (' or ’) allowed only between letters.
func ParseWord(value string) (Word, error) {
	n := utf8.RuneCountInString(value)
	if n == 0 || n > MaxWordLen {
		return Word{}, fmt.Errorf("invalid word %q: length must be 1 to %d", value, MaxWordLen)
	}

	runes := []rune(value)
	for i, r := range runes {
		switch {
		case unicode.IsLetter(r):
		case isApostrophe(r) && i > 0 && i < len(runes)-1 && unicode.IsLetter(runes[i-1]) && unicode.IsLetter(runes[i+1]):
		default:
			return Word{}, fmt.Errorf("invalid word %q: unexpected %q", value, r)
		}
	}

	return Word{value}, nil
}

func isApostrophe(r rune) bool {
	return r == '\'' || r == '’'
}
