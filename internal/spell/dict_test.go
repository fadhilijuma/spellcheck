package spell

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	symspell "github.com/eskriett/spell"
)

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
	for _, tc := range []struct {
		name       string
		line       string
		withCounts bool
		expected   symspell.Entry
	}{
		{name: "counted-entry", line: "the 10", withCounts: true, expected: symspell.Entry{Word: "the", Frequency: 10}},
		{name: "acronym-entry", line: "NASA", expected: symspell.Entry{Word: "NASA"}},
	} {
		table = append(table, Table{
			Name: tc.name, ExpResp: tc.expected, CmpFunc: cmpDiff,
			ExcFunc: func(context.Context) any {
				e, err := parseEntry(tc.line, tc.withCounts)
				if err != nil {
					return err.Error()
				}
				return e
			},
		})
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
			ExpResp: []symspell.Entry{},
			ExcFunc: func(context.Context) any { return parseDict(" \n\r\n") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "CRLF-and-whitespace",
			ExpResp: []symspell.Entry{{Word: "the", Frequency: 10}},
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
			ExpResp: []symspell.Entry{{Word: "the", Frequency: 10}, {Word: "don’t", Frequency: 5}},
			ExcFunc: func(context.Context) any { return parseDict("the 10\ndon’t 5\n") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "blank-lines-skipped",
			ExpResp: []symspell.Entry{{Word: "the", Frequency: 10}, {Word: "ten", Frequency: 5}},
			ExcFunc: func(context.Context) any { return parseDict("the 10\n\nten 5\n") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "literal-word",
			ExpResp: []symspell.Entry{{Word: "'the", Frequency: 10}},
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
			Name:    "acronym-with-digit",
			ExpResp: map[string]struct{}{"JWT": {}, "DNS1": {}},
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

// BenchmarkParseWordList measures startup parsing without building the index.
func BenchmarkParseWordList(b *testing.B) {
	for _, tc := range []struct {
		name       string
		text       string
		withCounts bool
	}{
		{name: "dictionary-US", text: americanEnglish, withCounts: true},
		{name: "dictionary-GB", text: britishEnglish, withCounts: true},
		{name: "acronyms", text: knownAcronyms},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.SetBytes(int64(len(tc.text)))
			for b.Loop() {
				if _, err := parseWordList(tc.text, tc.withCounts); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// =============================================================================

// parseDict returns the entries parsed from text, or the error text.
func parseDict(text string) any {
	entries, err := parseDictionary(text)
	if err != nil {
		return err.Error()
	}
	return entries
}
