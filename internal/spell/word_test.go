package spell

import (
	"context"
	"strings"
	"testing"
)

func Test_ParseWord(t *testing.T) {
	t.Parallel()

	run(t, parseWords(), "parse")
}

// BenchmarkParseWord measures input validation, without dictionary lookup.
func BenchmarkParseWord(b *testing.B) {
	for _, row := range []struct {
		name string
		word string
	}{
		{name: "ASCII", word: "hello"},
		{name: "apostrophe", word: "don’t"},
		{name: "Unicode", word: "𐐀café"},
		{name: "long", word: strings.Repeat("a", 80)},
	} {
		b.Run(row.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_, _ = ParseWord(row.word, 80)
			}
		})
	}
}

func parseWords() []Table {
	long := strings.Repeat("a", 65)

	table := []Table{
		{Name: "plain", ExpResp: "hello", ExcFunc: func(context.Context) any { return parseOne("hello") }, CmpFunc: cmpDiff},
		{Name: "contraction", ExpResp: "don't", ExcFunc: func(context.Context) any { return parseOne("don't") }, CmpFunc: cmpDiff},
		{Name: "typographic-apostrophe", ExpResp: "don’t", ExcFunc: func(context.Context) any { return parseOne("don’t") }, CmpFunc: cmpDiff},
		{Name: "unicode-letters", ExpResp: "café", ExcFunc: func(context.Context) any { return parseOne("café") }, CmpFunc: cmpDiff},
		{Name: "configured-unicode-boundary", ExpResp: "𐐀é", ExcFunc: func(context.Context) any { return parseOne("𐐀é", 2) }, CmpFunc: cmpDiff},
		{Name: "configured-small-limit", ExpResp: `invalid word "café": length must be 1 to 3`, ExcFunc: func(context.Context) any { return parseOne("café", 3) }, CmpFunc: cmpDiff},
		{Name: "configured-large-limit", ExpResp: strings.Repeat("a", 65), ExcFunc: func(context.Context) any { return parseOne(strings.Repeat("a", 65), 80) }, CmpFunc: cmpDiff},
		{Name: "configured-unicode-over-limit", ExpResp: `invalid word "𐐀éa": length must be 1 to 2`, ExcFunc: func(context.Context) any { return parseOne("𐐀éa", 2) }, CmpFunc: cmpDiff},
		{Name: "zero-limit", ExpResp: `invalid word "a": length must be 1 to 0`, ExcFunc: func(context.Context) any { return parseOne("a", 0) }, CmpFunc: cmpDiff},
		{Name: "negative-limit", ExpResp: `invalid word "a": length must be 1 to -1`, ExcFunc: func(context.Context) any { return parseOne("a", -1) }, CmpFunc: cmpDiff},
		{Name: "empty", ExpResp: `invalid word "": length must be 1 to 64`, ExcFunc: func(context.Context) any { return parseOne("") }, CmpFunc: cmpDiff},
		{Name: "too-long", ExpResp: `invalid word "` + long + `": length must be 1 to 64`, ExcFunc: func(context.Context) any { return parseOne(long) }, CmpFunc: cmpDiff},
		{Name: "digit", ExpResp: `invalid word "abc1": unexpected '1'`, ExcFunc: func(context.Context) any { return parseOne("abc1") }, CmpFunc: cmpDiff},
		{Name: "leading-apostrophe", ExpResp: `invalid word "'twas": unexpected '\''`, ExcFunc: func(context.Context) any { return parseOne("'twas") }, CmpFunc: cmpDiff},
		{Name: "trailing-apostrophe", ExpResp: `invalid word "word’": unexpected '’'`, ExcFunc: func(context.Context) any { return parseOne("word’") }, CmpFunc: cmpDiff},
		{Name: "doubled-apostrophe", ExpResp: `invalid word "don''t": unexpected '\''`, ExcFunc: func(context.Context) any { return parseOne("don''t") }, CmpFunc: cmpDiff},
		{Name: "multiple-contractions", ExpResp: "rock'n'roll", ExcFunc: func(context.Context) any { return parseOne("rock'n'roll") }, CmpFunc: cmpDiff},
		{Name: "invalid-UTF-8", ExpResp: `invalid word "a\xff": unexpected '�'`, ExcFunc: func(context.Context) any { return parseOne("a\xff") }, CmpFunc: cmpDiff},
		{Name: "hyphen", ExpResp: `invalid word "well-known": unexpected '-'`, ExcFunc: func(context.Context) any { return parseOne("well-known") }, CmpFunc: cmpDiff},
	}

	return table
}

// =============================================================================

// parseOne returns the word as typed, or the error text.
func parseOne(s string, limits ...int) string {
	maxWordLen := 64
	if len(limits) > 0 {
		maxWordLen = limits[0]
	}
	w, err := ParseWord(s, maxWordLen)
	if err != nil {
		return err.Error()
	}
	return w.String()
}
