// Package spell checks English words against a frequency dictionary and
// suggests corrections. It knows nothing about HTTP.
package spell

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	symspell "github.com/eskriett/spell"
	"github.com/go-playground/validator/v10"
)

// Config controls suggestion count, edit distance and index size.
type Config struct {
	MaxSuggestions int `validate:"gt=0"`
	MaxDistance    uint32
	PrefixLength   uint32 `validate:"gtfield=MaxDistance"` // indexed prefix; distance uses the whole word
}

// Result is the outcome of checking one word.
type Result struct {
	Word        string
	Valid       bool
	Suggestions []string
}

// Checker checks words against a dictionary indexed for SymSpell lookups
// (github.com/eskriett/spell). It is read-only after construction and safe
// for concurrent use.
type Checker struct {
	maxSuggestions int
	index          *symspell.Spell
	acronyms       map[string]struct{}
}

// NewEnglish builds a Checker over the selected embedded English dictionary.
func NewEnglish(language Language, cfg Config) (*Checker, error) {
	language, err := ParseLanguage(string(language))
	if err != nil {
		return nil, fmt.Errorf("parse language: %w", err)
	}
	dict := americanEnglish
	if language == EnglishGB {
		dict = britishEnglish
	}
	c, err := New(dict, cfg)
	if err != nil {
		return nil, fmt.Errorf("load %s dictionary: %w", language, err)
	}
	c.acronyms, err = parseAcronyms(knownAcronyms)
	if err != nil {
		return nil, fmt.Errorf("load acronyms: %w", err)
	}
	return c, nil
}

// New validates index settings once and builds a Checker from "word count" lines.
func New(dict string, cfg Config) (*Checker, error) {
	if err := validator.New().Struct(cfg); err != nil {
		return nil, fmt.Errorf("validate checker config: %w", err)
	}

	entries, err := parseDictionary(dict)
	if err != nil {
		return nil, fmt.Errorf("build checker: %w", err)
	}

	index := symspell.New()
	index.MaxEditDistance = cfg.MaxDistance
	index.PrefixLength = cfg.PrefixLength

	for _, e := range entries {
		e.Word = key(e.Word)
		if _, err := index.AddEntry(e); err != nil {
			return nil, fmt.Errorf("add index entry %q: %w", e.Word, err)
		}
	}

	return &Checker{maxSuggestions: cfg.MaxSuggestions, index: index}, nil
}

// Check returns results for nonblank words in input order, ignoring blank entries.
// Cancellation is checked between word lookups; an in-progress lookup is not
// interruptible by the library.
func (c *Checker) Check(ctx context.Context, words []string) ([]Result, error) {
	results := make([]Result, 0, len(words))
	for _, w := range words {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("check words: %w", err)
		}
		if strings.TrimSpace(w) == "" {
			continue
		}
		r, err := c.check(w)
		if err != nil {
			return nil, fmt.Errorf("check %q: %w", w, err)
		}
		results = append(results, r)
	}

	return results, nil
}

// check reports w as valid when it is in the dictionary, regardless of case,
// and otherwise suggests the known words closest to it.
func (c *Checker) check(w string) (Result, error) {
	if _, ok := c.acronyms[w]; ok {
		return Result{Word: w, Valid: true}, nil
	}
	k := key(w)

	found, err := c.index.Lookup(k, symspell.SuggestionLevel(symspell.LevelClosest), symspell.SortFunc(byFrequency))
	if err != nil {
		return Result{}, fmt.Errorf("lookup suggestions: %w", err)
	}
	if len(found) > 0 && found[0].Distance == 0 {
		return Result{Word: w, Valid: true}, nil
	}

	// Keep up to MaxSuggestions ranked matches and extract their words.
	limit := min(len(found), c.maxSuggestions)
	selected := found[:limit]
	suggestions := selected.GetWords()
	for i, suggestion := range suggestions {
		suggestions[i] = matchCase(w, suggestion)
	}

	return Result{Word: w, Suggestions: suggestions}, nil
}

// byFrequency ranks suggestions most frequent first, then alphabetically.
// They are all at the same edit distance: the closest one that has any
// within the configured maximum, as the lookup asks for.
func byFrequency(list symspell.SuggestionList) {
	slices.SortFunc(list, func(x, y symspell.Suggestion) int {
		return cmp.Or(cmp.Compare(y.Frequency, x.Frequency), cmp.Compare(x.Word, y.Word))
	})
}

// key is the dictionary form of word: lowercase, straight apostrophes.
func key(word string) string {
	return strings.ReplaceAll(strings.ToLower(word), "’", "'")
}

// matchCase preserves uppercase words or an initial capital in suggestions.
func matchCase(typed string, suggestion string) string {
	if typed == strings.ToUpper(typed) && typed != strings.ToLower(typed) {
		return strings.ToUpper(suggestion)
	}

	if first, _ := utf8.DecodeRuneInString(typed); !unicode.IsUpper(first) {
		return suggestion
	}

	r, size := utf8.DecodeRuneInString(suggestion)
	return string(unicode.ToUpper(r)) + suggestion[size:]
}
