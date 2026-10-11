package spell

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func Test_Check(t *testing.T) {
	t.Parallel()

	c, err := New(testDict)
	if err != nil {
		t.Fatalf("Should be able to load the test dictionary : %s", err)
	}

	run(t, check(c), "check")
	run(t, suggest(), "suggest")
	run(t, checkCancel(c), "check-cancel")
}

func Test_CheckEnglish(t *testing.T) {
	t.Parallel()

	c, err := NewEnglish()
	if err != nil {
		t.Fatalf("Should be able to load the default dictionary : %s", err)
	}

	run(t, checkEnglish(c), "check-english")
}

func checkEnglish(c *Checker) []Table {
	table := []Table{
		{
			Name:    "current-technical-word",
			ExpResp: []result{{Word: "ChatGPT", Valid: true}},
			ExcFunc: func(ctx context.Context) any { return checkWords(ctx, c, "ChatGPT") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "current-general-word",
			ExpResp: []result{{Word: "codebase", Valid: true}},
			ExcFunc: func(ctx context.Context) any { return checkWords(ctx, c, "codebase") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "misspelling-not-in-dictionary",
			ExpResp: []result{{Word: "calender", Suggestions: []string{"calendar"}}},
			ExcFunc: func(ctx context.Context) any { return checkWords(ctx, c, "calender") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "US-spelling",
			ExpResp: []result{{Word: "colour", Suggestions: []string{"color"}}},
			ExcFunc: func(ctx context.Context) any { return checkWords(ctx, c, "colour") },
			CmpFunc: cmpDiff,
		},
	}

	return table
}

func check(c *Checker) []Table {
	table := []Table{
		{
			Name:    "known-word",
			ExpResp: []result{{Word: "the", Valid: true}},
			ExcFunc: func(ctx context.Context) any { return checkWords(ctx, c, "the") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "known-word-capitalised",
			ExpResp: []result{{Word: "The", Valid: true}},
			ExcFunc: func(ctx context.Context) any { return checkWords(ctx, c, "The") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "typographic-contraction",
			ExpResp: []result{{Word: "don’t", Valid: true}},
			ExcFunc: func(ctx context.Context) any { return checkWords(ctx, c, "don’t") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "dictionary-apostrophes-normalized",
			ExpResp: []result{{Word: "don't", Valid: true}},
			ExcFunc: func(ctx context.Context) any { return checkIn(ctx, "don’t 10\n", "don't") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "dictionary-acronym",
			ExpResp: []result{{Word: "NASA", Valid: true}},
			ExcFunc: func(ctx context.Context) any { return checkIn(ctx, "nasa 10\n", "NASA") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "uppercase-typo",
			ExpResp: []result{{Word: "TEH", Suggestions: []string{"THE", "TEN", "TEA"}}},
			ExcFunc: func(ctx context.Context) any { return checkWords(ctx, c, "TEH") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "uppercase-unknown-is-not-an-acronym",
			ExpResp: []result{{Word: "ZZZZZZ"}},
			ExcFunc: func(ctx context.Context) any { return checkWords(ctx, c, "ZZZZZZ") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "uppercase-contraction",
			ExpResp: []result{{Word: "DON’T", Valid: true}},
			ExcFunc: func(ctx context.Context) any { return checkWords(ctx, c, "DON’T") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "transposition",
			ExpResp: []result{{Word: "teh", Suggestions: []string{"the", "ten", "tea"}}},
			ExcFunc: func(ctx context.Context) any { return checkWords(ctx, c, "teh") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "leading-capital-kept",
			ExpResp: []result{{Word: "Teh", Suggestions: []string{"The", "Ten", "Tea"}}},
			ExcFunc: func(ctx context.Context) any { return checkWords(ctx, c, "Teh") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "distance-two-ranked-by-count",
			ExpResp: []result{{Word: "recieev", Suggestions: []string{"receive", "relieve"}}},
			ExcFunc: func(ctx context.Context) any { return checkWords(ctx, c, "recieev") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "missing-letter",
			ExpResp: []result{{Word: "speling", Suggestions: []string{"spelling"}}},
			ExcFunc: func(ctx context.Context) any { return checkWords(ctx, c, "speling") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "receive-transposition",
			ExpResp: []result{{Word: "recieve", Suggestions: []string{"receive", "relieve"}}},
			ExcFunc: func(ctx context.Context) any { return checkWords(ctx, c, "recieve") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "non-ascii-dictionary-word-valid",
			ExpResp: []result{{Word: "café", Valid: true}},
			ExcFunc: func(ctx context.Context) any { return checkIn(ctx, "café 10\ncafe 5\n", "café") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "non-ascii-dictionary-word-suggested",
			ExpResp: []result{{Word: "cafs", Suggestions: []string{"café", "cafe"}}},
			ExcFunc: func(ctx context.Context) any { return checkIn(ctx, "café 10\ncafe 5\n", "cafs") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "non-ascii-typo",
			ExpResp: []result{{Word: "thé", Suggestions: []string{"the"}}},
			ExcFunc: func(ctx context.Context) any { return checkWords(ctx, c, "thé") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "no-candidate",
			ExpResp: []result{{Word: "zzzzzz"}},
			ExcFunc: func(ctx context.Context) any { return checkWords(ctx, c, "zzzzzz") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "input-order",
			ExpResp: []result{{Word: "teh", Suggestions: []string{"the", "ten", "tea"}}, {Word: "the", Valid: true}},
			ExcFunc: func(ctx context.Context) any { return checkWords(ctx, c, "teh", "the") },
			CmpFunc: cmpDiff,
		},
		{
			// Seven words are one edit from "cax": by count, ties alphabetically, at most five.
			Name:    "ranked-and-limited",
			ExpResp: []result{{Word: "cax", Suggestions: []string{"cap", "car", "cat", "caw", "cab"}}},
			ExcFunc: func(ctx context.Context) any {
				return checkIn(ctx, "cat 5\ncar 9\ncap 9\ncan 1\ncab 3\ncad 2\ncaw 4\n", "cax")
			},
			CmpFunc: cmpDiff,
		},
	}

	return table
}

// suggest pins down the edit distance the suggestions use: SymSpell's
// optimal string alignment, up to two edits, closest distance only. Every
// word has the same count, so ties are alphabetical.
func suggest() []Table {
	table := []Table{
		{
			Name:    "each-edit-kind",
			ExpResp: []result{{Word: "cat", Suggestions: []string{"cart", "ct", "cta", "cut"}}},
			ExcFunc: func(ctx context.Context) any { return checkIn(ctx, "cart 1\nct 1\ncta 1\ncut 1\ndog 1\n", "cat") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "closest-distance-only",
			ExpResp: []result{{Word: "cat", Suggestions: []string{"cats"}}},
			ExcFunc: func(ctx context.Context) any { return checkIn(ctx, "cats 1\ncoats 1\n", "cat") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "next-distance-when-none-closer",
			ExpResp: []result{{Word: "cat", Suggestions: []string{"coats"}}},
			ExcFunc: func(ctx context.Context) any { return checkIn(ctx, "coats 1\ncatsup 1\n", "cat") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "beyond-max-distance",
			ExpResp: []result{{Word: "cat"}},
			ExcFunc: func(ctx context.Context) any { return checkIn(ctx, "catsup 1\n", "cat") },
			CmpFunc: cmpDiff,
		},
		{
			// Optimal string alignment edits a substring once: "ca" → "ac" →
			// "abc" would edit "ac" twice, so the distance is 3, not 2.
			Name:    "restricted-transposition",
			ExpResp: []result{{Word: "ca"}},
			ExcFunc: func(ctx context.Context) any { return checkIn(ctx, "abc 1\n", "ca") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "two-transpositions",
			ExpResp: []result{{Word: "badc", Suggestions: []string{"abcd"}}},
			ExcFunc: func(ctx context.Context) any { return checkIn(ctx, "abcd 1\n", "badc") },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "edit-after-indexed-prefix",
			ExpResp: []result{{Word: "abcdefhgij", Suggestions: []string{"abcdefghij"}}},
			ExcFunc: func(ctx context.Context) any { return checkIn(ctx, "abcdefghij 1\n", "abcdefhgij") },
			CmpFunc: cmpDiff,
		},
	}

	return table
}

// checkCancel proves Check returns the context's error, not results, once
// its context has ended.
func checkCancel(c *Checker) []Table {
	table := []Table{
		{
			Name:    "canceled",
			ExpResp: "context canceled",
			ExcFunc: func(ctx context.Context) any {
				ctx, cancel := context.WithCancel(ctx)
				cancel()
				return checkWords(ctx, c, "teh")
			},
			CmpFunc: cmpDiff,
		},
		{
			Name:    "deadline",
			ExpResp: "context deadline exceeded",
			ExcFunc: func(ctx context.Context) any {
				ctx, cancel := context.WithDeadline(ctx, time.Now().Add(-time.Second))
				defer cancel()
				return checkWords(ctx, c, "teh")
			},
			CmpFunc: cmpDiff,
		},
	}

	return table
}

func BenchmarkCheck(b *testing.B) {
	c, err := NewEnglish()
	if err != nil {
		b.Fatalf("Should be able to load the default dictionary : %s", err)
	}

	tests := []struct {
		name string
		word string
	}{
		{name: "valid-word", word: "accommodation"},
		{name: "distance-1", word: "acommodation"},
		{name: "distance-2", word: "acommodatoin"},
	}

	for _, tt := range tests {
		w, err := ParseWord(tt.word)
		if err != nil {
			b.Fatalf("Should be able to parse %q : %s", tt.word, err)
		}

		b.Run(tt.name, func(b *testing.B) {
			for b.Loop() {
				if _, err := c.Check(b.Context(), []Word{w}); err != nil {
					b.Fatalf("Should be able to check : %s", err)
				}
			}
		})
	}
}

// =============================================================================

// testDict is a small dictionary so the tests do not depend on words.txt.
const testDict = `
the 1000
then 300
they 250
ten 200
tea 150
spelling 80
receive 60
relieve 40
don't 70
`

// result is a Result with plain values.
type result struct {
	Word        string
	Valid       bool
	Suggestions []string
}

// Table describes a unit test's execution and expected response.
type Table struct {
	Name    string
	ExpResp any
	ExcFunc func(ctx context.Context) any
	CmpFunc func(got any, exp any) string
}

// run executes the named cases, each with its subtest's context.
func run(t *testing.T, table []Table, testName string) {
	t.Helper()

	for _, tt := range table {
		t.Run(testName+"-"+tt.Name, func(t *testing.T) {
			got := tt.ExcFunc(t.Context())
			if diff := tt.CmpFunc(got, tt.ExpResp); diff != "" {
				t.Logf("DIFF\n%s", diff)
				t.Logf("GOT\n%#v", got)
				t.Logf("EXP\n%#v", tt.ExpResp)
				t.Fatal("Should get the expected response")
			}
		})
	}
}

func cmpDiff(got any, exp any) string {
	return cmp.Diff(got, exp)
}

// checkIn builds a Checker over dict and checks words with it; it returns
// the results, or the error text.
func checkIn(ctx context.Context, dict string, words ...string) any {
	c, err := New(dict)
	if err != nil {
		return err.Error()
	}
	return checkWords(ctx, c, words...)
}

// checkWords parses and checks words; it returns the results, or the error
// text.
func checkWords(ctx context.Context, c *Checker, words ...string) any {
	parsed := make([]Word, len(words))
	for i, s := range words {
		w, err := ParseWord(s)
		if err != nil {
			return err.Error()
		}
		parsed[i] = w
	}

	rs, err := c.Check(ctx, parsed)
	if err != nil {
		return err.Error()
	}

	results := make([]result, len(rs))
	for i, r := range rs {
		results[i] = result{Word: r.Word.String(), Valid: r.Valid}
		for _, s := range r.Suggestions {
			results[i].Suggestions = append(results[i].Suggestions, s.String())
		}
	}
	return results
}
