package spell

import (
	"bufio"
	_ "embed"
	"fmt"
	"strconv"
	"strings"
)

// english is the embedded, normalized US English spelling list; see README
// for its source and frequency ranking.
//
//go:embed words.txt
var english string

// entry is a dictionary word and how often it occurs.
type entry struct {
	word  Word
	count uint64
}

// parseDictionary reads "word count" lines. Blank lines are skipped; any other
// malformed line is an error that names its line number.
func parseDictionary(text string) ([]entry, error) {
	var entries []entry

	sc := bufio.NewScanner(strings.NewReader(text))
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}

		value, countStr, ok := strings.Cut(line, " ")
		if !ok {
			return nil, fmt.Errorf("line %d: want \"word count\", got %q", n, line)
		}

		count, err := strconv.ParseUint(countStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("line %d: invalid count %q", n, countStr)
		}

		w, err := ParseWord(value)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", n, err)
		}

		entries = append(entries, entry{word: w, count: count})
	}

	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}

	return entries, nil
}
