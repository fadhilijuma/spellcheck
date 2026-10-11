package spell

import (
	"context"
	"testing"
)

func Test_ParseDictionary(t *testing.T) {
	t.Parallel()

	run(t, dictionary(), "dictionary")
}

func dictionary() []Table {
	table := []Table{
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
			ExpResp: `line 1: invalid word "'the": unexpected '\''`,
			ExcFunc: func(context.Context) any { return parseDict("'the 10\n") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "missing-count",
			ExpResp: `line 2: want "word count", got "broken"`,
			ExcFunc: func(context.Context) any { return parseDict("the 10\nbroken\n") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "negative-count",
			ExpResp: `line 1: invalid count "-1"`,
			ExcFunc: func(context.Context) any { return parseDict("the -1\n") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "bad-count",
			ExpResp: `line 1: invalid count "ten"`,
			ExcFunc: func(context.Context) any { return parseDict("the ten\n") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "default-list",
			ExpResp: 107901,
			ExcFunc: func(context.Context) any {
				entries, err := parseDictionary(english)
				if err != nil {
					return err.Error()
				}
				return len(entries)
			},
			CmpFunc: cmpDiff,
		},
	}

	return table
}

// =============================================================================

// plainEntry is an entry with plain values.
type plainEntry struct {
	Word  string
	Count uint64
}

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
