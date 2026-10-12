// Package spell checks English words against a frequency dictionary and
// suggests corrections. It knows nothing about HTTP.
package spell

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	symspell "github.com/eskriett/spell"
)

// Config controls suggestion count, edit distance and deletion-index size.
type Config struct {
	MaxSuggestions int
	MaxDistance    uint32
	PrefixLength   uint32 // indexed runes; distance checks still cover the whole word
}

// Result is the outcome of checking one word.
type Result struct {
	Word        Word
	Valid       bool
	Suggestions []Word
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
		return nil, err
	}
	dict := americanEnglish
	if language == EnglishGB {
		dict = britishEnglish
	}
	c, err := New(dict, cfg)
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", language, err)
	}
	c.acronyms, err = parseAcronyms(knownAcronyms)
	if err != nil {
		return nil, fmt.Errorf("acronyms: %w", err)
	}
	return c, nil
}

// New builds a Checker over dict, made of "word count" lines.
func New(dict string, cfg Config) (*Checker, error) {
	if cfg.MaxSuggestions <= 0 {
		return nil, errors.New("max suggestions: must be positive")
	}
	if cfg.PrefixLength <= cfg.MaxDistance {
		return nil, errors.New("prefix length: must be greater than max distance")
	}
	entries, err := parseDictionary(dict)
	if err != nil {
		return nil, fmt.Errorf("dictionary: %w", err)
	}

	index := symspell.New()
	index.MaxEditDistance = cfg.MaxDistance
	index.PrefixLength = cfg.PrefixLength

	for _, e := range entries {
		_, err := index.AddEntry(symspell.Entry{Word: key(e.word), Frequency: e.count})
		if err != nil {
			return nil, fmt.Errorf("index %q: %w", e.word, err)
		}
	}

	return &Checker{maxSuggestions: cfg.MaxSuggestions, index: index}, nil
}

// Check checks each word and returns one Result per word, in input order.
// Cancellation is checked between word lookups; an in-progress lookup is not
// interruptible by the library.
func (c *Checker) Check(ctx context.Context, words []Word) ([]Result, error) {
	results := make([]Result, 0, len(words))
	for _, w := range words {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		r, err := c.check(w)
		if err != nil {
			return nil, fmt.Errorf("check %q: %w", w, err)
		}
		results = append(results, r)
	}

	return results, nil
}

// =============================================================================

// check reports w as valid when it is in the dictionary, regardless of case,
// and otherwise suggests the known words closest to it.
func (c *Checker) check(w Word) (Result, error) {
	if _, ok := c.acronyms[w.value]; ok {
		return Result{Word: w, Valid: true}, nil
	}
	k := key(w)

	entry, err := c.index.GetEntry(k)
	if err != nil {
		return Result{}, err
	}
	if entry != nil {
		return Result{Word: w, Valid: true}, nil
	}

	found, err := c.index.Lookup(k, symspell.SuggestionLevel(symspell.LevelClosest), symspell.SortFunc(byFrequency))
	if err != nil {
		return Result{}, err
	}

	// Suggestions are dictionary words, validated when the dictionary was
	// loaded.
	suggestions := make([]Word, 0, min(len(found), c.maxSuggestions))
	for _, s := range found[:min(len(found), c.maxSuggestions)] {
		suggestions = append(suggestions, Word{value: matchCase(w.value, s.Word)})
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

// key is the dictionary form of w: lowercase, straight apostrophes.
func key(w Word) string {
	return strings.ReplaceAll(strings.ToLower(w.value), "’", "'")
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
