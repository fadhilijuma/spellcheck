package spell

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
)

// plainEntry is an entry with plain values.
type plainEntry struct {
	Word  string
	Count uint64
}

func Test_ParseDictionary(t *testing.T) {
	t.Parallel()

	run(t, dictionary(), "dictionary")
	table := []Table{
		{
			Name: "syntax-cause", ExpResp: true, CmpFunc: cmpDiff,
			ExcFunc: func(context.Context) any {
				_, err := parseDictionary("the ten\n")
				cause, ok := errors.AsType[*strconv.NumError](err)
				return ok && cause.Num == "ten" && errors.Is(err, strconv.ErrSyntax)
			},
		},
		{
			Name: "overflow-cause", ExpResp: true, CmpFunc: cmpDiff,
			ExcFunc: func(context.Context) any {
				_, err := parseDictionary("the 18446744073709551616\n")
				cause, ok := errors.AsType[*strconv.NumError](err)
				return ok && cause.Num == "18446744073709551616" && errors.Is(err, strconv.ErrRange)
			},
		},
	}
	run(t, table, "dictionary-causes")
}

func Test_ParseLanguage(t *testing.T) {
	t.Parallel()

	table := []Table{
		{
			Name:    "default",
			ExpResp: EnglishUS,
			ExcFunc: func(context.Context) any { return parseLanguage("") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "American",
			ExpResp: EnglishUS,
			ExcFunc: func(context.Context) any { return parseLanguage("en-US") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "British",
			ExpResp: EnglishGB,
			ExcFunc: func(context.Context) any { return parseLanguage("en-GB") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "unsupported",
			ExpResp: `language "fr-FR": expected en-US or en-GB`,
			ExcFunc: func(context.Context) any { return parseLanguage("fr-FR") },
			CmpFunc: cmpDiff,
		},
	}
	run(t, table, "language")
}

func parseLanguage(value string) any {
	language, err := ParseLanguage(value)
	if err != nil {
		return err.Error()
	}
	return language
}

func Test_LanguageUnmarshalText(t *testing.T) {
	t.Parallel()
	table := []Table{
		{
			Name:    "default",
			ExpResp: EnglishUS,
			ExcFunc: func(context.Context) any { return unmarshalLanguage("") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "American",
			ExpResp: EnglishUS,
			ExcFunc: func(context.Context) any { return unmarshalLanguage("en-US") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "British",
			ExpResp: EnglishGB,
			ExcFunc: func(context.Context) any { return unmarshalLanguage("en-GB") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "invalid-does-not-overwrite",
			ExpResp: `en-GB: parse language: language "fr-FR": expected en-US or en-GB`,
			ExcFunc: func(context.Context) any { return unmarshalLanguage("fr-FR") },
			CmpFunc: cmpDiff,
		},
	}
	run(t, table, "configured-language")
}

func unmarshalLanguage(value string) any {
	language := EnglishGB
	if err := language.UnmarshalText([]byte(value)); err != nil {
		return string(language) + ": " + err.Error()
	}
	return language
}

func dictionary() []Table {
	table := []Table{
		{
			Name:    "empty-list",
			ExpResp: []plainEntry{},
			ExcFunc: func(context.Context) any { return parseDict(" \n\r\n") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "CRLF-and-whitespace",
			ExpResp: []plainEntry{{Word: "the", Count: 10}},
			ExcFunc: func(context.Context) any { return parseDict("  the 10 \r\n\r\n") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "scanner-error",
			ExpResp: "parse dictionary: scan word list: bufio.Scanner: token too long",
			ExcFunc: func(context.Context) any { return parseDict(strings.Repeat("a", 65536)) },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "entries",
			ExpResp: []plainEntry{{Word: "the", Count: 10}, {Word: "don’t", Count: 5}},
			ExcFunc: func(context.Context) any { return parseDict("the 10\ndon’t 5\n") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "blank-lines-skipped",
			ExpResp: []plainEntry{{Word: "the", Count: 10}, {Word: "ten", Count: 5}},
			ExcFunc: func(context.Context) any { return parseDict("the 10\n\nten 5\n") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "invalid-word",
			ExpResp: `parse dictionary: line 1: parse word: invalid word "'the": unexpected '\''`,
			ExcFunc: func(context.Context) any { return parseDict("'the 10\n") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "missing-count",
			ExpResp: `parse dictionary: line 2: want "word count", got "broken"`,
			ExcFunc: func(context.Context) any { return parseDict("the 10\nbroken\n") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "negative-count",
			ExpResp: `parse dictionary: line 1: parse count: strconv.ParseUint: parsing "-1": invalid syntax`,
			ExcFunc: func(context.Context) any { return parseDict("the -1\n") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "bad-count",
			ExpResp: `parse dictionary: line 1: parse count: strconv.ParseUint: parsing "ten": invalid syntax`,
			ExcFunc: func(context.Context) any { return parseDict("the ten\n") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "American-list",
			ExpResp: 107901,
			ExcFunc: func(context.Context) any { return embeddedCount(americanEnglish) },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "British-list",
			ExpResp: 107555,
			ExcFunc: func(context.Context) any { return embeddedCount(britishEnglish) },
			CmpFunc: cmpDiff,
		},
	}

	return table
}

func embeddedCount(text string) any {
	entries, err := parseDictionary(text)
	if err != nil {
		return err.Error()
	}
	return len(entries)
}

func Test_ParseAcronyms(t *testing.T) {
	t.Parallel()
	table := []Table{
		{
			Name:    "empty-list",
			ExpResp: map[string]struct{}{},
			ExcFunc: func(context.Context) any { return acronymEntries(" \n\r\n") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "CRLF-and-whitespace",
			ExpResp: map[string]struct{}{"JWT": {}},
			ExcFunc: func(context.Context) any { return acronymEntries(" JWT \r\n\r\n") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "scanner-error",
			ExpResp: "parse acronyms: scan word list: bufio.Scanner: token too long",
			ExcFunc: func(context.Context) any { return acronymEntries(strings.Repeat("A", 65536)) },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "entries-and-duplicates",
			ExpResp: map[string]struct{}{"JWT": {}, "DNS": {}},
			ExcFunc: func(context.Context) any { return acronymEntries("JWT\n\nDNS\nJWT\n") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "lowercase",
			ExpResp: `parse acronyms: line 1: acronym "jwt" must be uppercase`,
			ExcFunc: func(context.Context) any { return acronymEntries("jwt\n") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "invalid-word",
			ExpResp: `parse acronyms: line 2: parse word: invalid word "DNS1": unexpected '1'`,
			ExcFunc: func(context.Context) any { return acronymEntries("JWT\nDNS1\n") },
			CmpFunc: cmpDiff,
		},
	}
	run(t, table, "acronyms")
}

func acronymEntries(text string) any {
	acronyms, err := parseAcronyms(text)
	if err != nil {
		return err.Error()
	}
	return acronyms
}

// =============================================================================

// parseDict returns the entries parsed from text, or the error text.
func parseDict(text string) any {
	es, err := parseDictionary(text)
	if err != nil {
		return err.Error()
	}

	entries := make([]plainEntry, len(es))
	for i, e := range es {
		entries[i] = plainEntry{Word: e.word.String(), Count: e.count}
	}
	return entries
}
