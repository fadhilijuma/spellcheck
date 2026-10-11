package spell

import (
	"bufio"
	"cmp"
	_ "embed"
	"fmt"
	"strconv"
	"strings"
)

// americanEnglish and britishEnglish contain the embedded spelling lists;
// see README for their sources and frequency ranking.
//
//go:embed languages/en-US.txt
var americanEnglish string

//go:embed languages/en-GB.txt
var britishEnglish string

// knownAcronyms contains the uppercase allowlist shared by both languages.
//
//go:embed languages/acronyms.txt
var knownAcronyms string

// Language identifies a supported English spelling convention.
type Language string

// entry is a dictionary word and how often it occurs.
type entry struct {
	word  Word
	count uint64
}

const (
	// EnglishUS uses American spelling; EnglishGB uses British -ise spelling.
	EnglishUS Language = "en-US"
	EnglishGB Language = "en-GB"
)

// ParseLanguage parses a locale, defaulting an empty value to US English.
func ParseLanguage(value string) (Language, error) {
	language := Language(cmp.Or(value, string(EnglishUS)))
	switch language {
	case EnglishUS, EnglishGB:
		return language, nil
	default:
		return "", fmt.Errorf("language %q: expected en-US or en-GB", value)
	}
}

// UnmarshalText validates a configured English convention.
func (language *Language) UnmarshalText(text []byte) error {
	value, err := ParseLanguage(string(text))
	if err != nil {
		return err
	}
	*language = value
	return nil
}

// parseAcronyms validates one uppercase acronym per line. Blank lines are skipped.
func parseAcronyms(text string) (map[string]struct{}, error) {
	entries, err := parseWordList(text, false)
	if err != nil {
		return nil, err
	}
	acronyms := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		acronyms[e.word.String()] = struct{}{}
	}
	return acronyms, nil
}

// parseDictionary reads "word count" lines. Blank lines are skipped; any other
// malformed line is an error that names its line number.
func parseDictionary(text string) ([]entry, error) {
	return parseWordList(text, true)
}

// parseWordList shares scanning and word validation. Dictionary lines have
// counts; acronym lines contain only an uppercase word.
func parseWordList(text string, withCounts bool) ([]entry, error) {
	var entries []entry

	sc := bufio.NewScanner(strings.NewReader(text))
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}

		value := line
		var count uint64
		if withCounts {
			var countStr string
			var ok bool
			value, countStr, ok = strings.Cut(line, " ")
			if !ok {
				return nil, fmt.Errorf("line %d: want \"word count\", got %q", n, line)
			}
			var err error
			count, err = strconv.ParseUint(countStr, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("line %d: invalid count %q", n, countStr)
			}
		}

		w, err := ParseWord(value)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", n, err)
		}
		if !withCounts && value != strings.ToUpper(value) {
			return nil, fmt.Errorf("line %d: acronym %q must be uppercase", n, value)
		}

		entries = append(entries, entry{word: w, count: count})
	}

	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}

	return entries, nil
}
