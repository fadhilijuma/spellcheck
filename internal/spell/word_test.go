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

func parseWords() []Table {
	long := strings.Repeat("a", MaxWordLen+1)

	table := []Table{
		{Name: "plain", ExpResp: "hello", ExcFunc: func(context.Context) any { return parseOne("hello") }, CmpFunc: cmpDiff},
		{Name: "contraction", ExpResp: "don't", ExcFunc: func(context.Context) any { return parseOne("don't") }, CmpFunc: cmpDiff},
		{Name: "typographic-apostrophe", ExpResp: "don’t", ExcFunc: func(context.Context) any { return parseOne("don’t") }, CmpFunc: cmpDiff},
		{Name: "unicode-letters", ExpResp: "café", ExcFunc: func(context.Context) any { return parseOne("café") }, CmpFunc: cmpDiff},
		{Name: "empty", ExpResp: `invalid word "": length must be 1 to 64`, ExcFunc: func(context.Context) any { return parseOne("") }, CmpFunc: cmpDiff},
		{Name: "too-long", ExpResp: `invalid word "` + long + `": length must be 1 to 64`, ExcFunc: func(context.Context) any { return parseOne(long) }, CmpFunc: cmpDiff},
		{Name: "digit", ExpResp: `invalid word "abc1": unexpected '1'`, ExcFunc: func(context.Context) any { return parseOne("abc1") }, CmpFunc: cmpDiff},
		{Name: "leading-apostrophe", ExpResp: `invalid word "'twas": unexpected '\''`, ExcFunc: func(context.Context) any { return parseOne("'twas") }, CmpFunc: cmpDiff},
		{Name: "hyphen", ExpResp: `invalid word "well-known": unexpected '-'`, ExcFunc: func(context.Context) any { return parseOne("well-known") }, CmpFunc: cmpDiff},
	}

	return table
}

// =============================================================================

// parseOne returns the word as typed, or the error text.
func parseOne(s string) string {
	w, err := ParseWord(s)
	if err != nil {
		return err.Error()
	}
	return w.String()
}
