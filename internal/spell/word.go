package spell

import (
	"fmt"
	"unicode"
	"unicode/utf8"
)

// Word is a validated word as the user typed it; case is preserved.
type Word struct {
	value string
}

// String returns the word as typed.
func (w Word) String() string {
	return w.value
}

// ParseWord returns a Word if value is 1 to maxWordLen runes of letters, with
// apostrophes (' or ’) allowed only between letters.
func ParseWord(value string, maxWordLen int) (Word, error) {
	n := utf8.RuneCountInString(value)
	if n == 0 || n > maxWordLen {
		return Word{}, fmt.Errorf("invalid word %q: length must be 1 to %d", value, maxWordLen)
	}
	return parseWord(value)
}

// parseWord validates spelling-list syntax independently of the input limit.
func parseWord(value string) (Word, error) {
	if value == "" {
		return Word{}, fmt.Errorf("invalid word %q: word is empty", value)
	}
	previousLetter := false
	for i, r := range value {
		if unicode.IsLetter(r) {
			previousLetter = true
			continue
		}
		if isApostrophe(r) && previousLetter {
			next, _ := utf8.DecodeRuneInString(value[i+utf8.RuneLen(r):])
			if unicode.IsLetter(next) {
				previousLetter = false
				continue
			}
		}
		return Word{}, fmt.Errorf("invalid word %q: unexpected %q", value, r)
	}

	return Word{value}, nil
}

func isApostrophe(r rune) bool {
	return r == '\'' || r == '’'
}
