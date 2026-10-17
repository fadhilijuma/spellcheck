package spell

import (
	"bufio"
	"cmp"
	_ "embed"
	"fmt"
	"strconv"
	"strings"

	symspell "github.com/eskriett/spell"
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
		return fmt.Errorf("parse language: %w", err)
	}
	*language = value
	return nil
}

// parseAcronyms validates one uppercase acronym per line. Blank lines are skipped.
func parseAcronyms(text string) (map[string]struct{}, error) {
	entries, err := parseWordList(text, false)
	if err != nil {
		return nil, fmt.Errorf("parse acronyms: %w", err)
	}
	acronyms := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		acronyms[e.Word] = struct{}{}
	}
	return acronyms, nil
}

// parseDictionary reads "word count" lines. Blank lines are skipped; any other
// malformed line is an error that names its line number.
func parseDictionary(text string) ([]symspell.Entry, error) {
	entries, err := parseWordList(text, true)
	if err != nil {
		return nil, fmt.Errorf("parse dictionary: %w", err)
	}
	return entries, nil
}

// parseWordList shares word-list scanning. Dictionary lines have
// counts; acronym lines contain only an uppercase word.
func parseWordList(text string, withCounts bool) ([]symspell.Entry, error) {
	// One slot per possible line avoids repeatedly copying the growing list.
	entries := make([]symspell.Entry, 0, strings.Count(text, "\n")+1)

	sc := bufio.NewScanner(strings.NewReader(text))
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}

		e, err := parseEntry(line, withCounts)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", n, err)
		}

		entries = append(entries, e)
	}

	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan word list: %w", err)
	}

	return entries, nil
}

// parseEntry interprets one trimmed, nonblank dictionary or acronym line.
func parseEntry(line string, withCounts bool) (symspell.Entry, error) {
	if !withCounts {
		if line != strings.ToUpper(line) {
			return symspell.Entry{}, fmt.Errorf("acronym %q must be uppercase", line)
		}
		return symspell.Entry{Word: line}, nil
	}

	word, countText, ok := strings.Cut(line, " ")
	if !ok {
		return symspell.Entry{}, fmt.Errorf("want \"word count\", got %q", line)
	}
	count, err := strconv.ParseUint(countText, 10, 64)
	if err != nil {
		return symspell.Entry{}, fmt.Errorf("parse count: %w", err)
	}
	return symspell.Entry{Word: word, Frequency: count}, nil
}
