package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/fadhilijuma/spellcheck/internal/httpapi"
	"github.com/fadhilijuma/spellcheck/internal/spell"
	"github.com/google/go-cmp/cmp"
)

// Table describes a unit test's execution and expected response.
type Table struct {
	Name    string
	ExpResp any
	ExcFunc func(ctx context.Context) any
	CmpFunc func(got any, exp any) string
}

// response is what the server wrote.
type response struct {
	Status      int
	Body        string
	Allow       string
	AllowOrigin string
}

// requestLog is the stable part of one structured request log.
type requestLog struct {
	Level   string `json:"level"`
	Message string `json:"msg"`
	Status  int    `json:"status"`
	Error   string `json:"err"`
	Records int    `json:"-"`
}

func Test_API(t *testing.T) {
	t.Parallel()

	srv := newServer(t, spell.EnglishUS)

	run(t, health(srv), "health")
	run(t, routing(srv), "routing")
	run(t, check200(srv), "check-200")
	run(t, check400(srv), "check-400")
	run(t, check413(srv), "check-413")
	run(t, checkCORS(srv), "check-cors")
	run(t, internalFailures(), "internal-failures")
}

func Test_PrototypeScope(t *testing.T) {
	t.Parallel()
	table := []Table{
		{
			Name:    "repeated checks are accepted without a client budget",
			ExpResp: true, CmpFunc: cmpDiff,
			ExcFunc: func(ctx context.Context) any {
				srv := newServer(t, spell.EnglishUS)
				for range 25 {
					r := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/check", strings.NewReader(`{"words":["the"]}`))
					w := httptest.NewRecorder()
					srv.ServeHTTP(w, r)
					if w.Code != http.StatusOK {
						return false
					}
				}
				return true
			},
		},
		{
			Name: "metrics endpoint is not part of the prototype", ExpResp: response{Status: http.StatusNotFound, Body: "404 page not found"}, CmpFunc: cmpDiff,
			ExcFunc: func(ctx context.Context) any {
				srv := newServer(t, spell.EnglishUS)
				return serve(ctx, srv, http.MethodGet, "/v1/metrics", "", nil)
			},
		},
	}
	run(t, table, "prototype")
}

func Test_DefaultLanguage(t *testing.T) {
	t.Parallel()
	table := []Table{
		{
			Name:    "configuration",
			ExpResp: response{Status: http.StatusOK, Body: `{"defaultLanguage":"en-GB","maxWordLen":64,"maxWords":500,"maxBodyBytes":385564}`},
			ExcFunc: func(ctx context.Context) any {
				return serve(ctx, newServer(t, spell.EnglishGB), http.MethodGet, "/v1/config", "", nil)
			},
			CmpFunc: cmpDiff,
		},
		{
			Name:    "omitted-language",
			ExpResp: response{Status: http.StatusOK, Body: `{"misspelled":[{"word":"color","suggestions":["colour"]}]}`},
			ExcFunc: func(ctx context.Context) any {
				return post(ctx, newServer(t, spell.EnglishGB), `{"words":["color","colour"]}`)
			},
			CmpFunc: cmpDiff,
		},
		{
			Name:    "empty-language",
			ExpResp: response{Status: http.StatusOK, Body: `{"misspelled":[{"word":"color","suggestions":["colour"]}]}`},
			ExcFunc: func(ctx context.Context) any {
				return post(ctx, newServer(t, spell.EnglishGB), `{"language":"","words":["color","colour"]}`)
			},
			CmpFunc: cmpDiff,
		},
		{
			Name:    "explicit-language-overrides-default",
			ExpResp: response{Status: http.StatusOK, Body: `{"misspelled":[{"word":"colour","suggestions":["color"]}]}`},
			ExcFunc: func(ctx context.Context) any {
				return post(ctx, newServer(t, spell.EnglishGB), `{"language":"en-US","words":["color","colour"]}`)
			},
			CmpFunc: cmpDiff,
		},
	}
	run(t, table, "default-language")
}

func Test_WordLimit(t *testing.T) {
	t.Parallel()
	table := []Table{
		{
			Name: "configuration", ExpResp: response{Status: http.StatusOK, Body: `{"defaultLanguage":"en-US","maxWordLen":3,"maxWords":500,"maxBodyBytes":385564}`}, CmpFunc: cmpDiff,
			ExcFunc: func(ctx context.Context) any {
				return serve(ctx, newServer(t, spell.EnglishUS, 3), http.MethodGet, "/v1/config", "", nil)
			},
		},
		{
			Name: "at-limit", ExpResp: response{Status: http.StatusOK, Body: `{"misspelled":[]}`}, CmpFunc: cmpDiff,
			ExcFunc: func(ctx context.Context) any { return post(ctx, newServer(t, spell.EnglishUS, 3), `{"words":["the"]}`) },
		},
		{
			Name: "over-limit", ExpResp: badRequestBody(`parse check words: words[0]: invalid word \"quick\": length must be 1 to 3`), CmpFunc: cmpDiff,
			ExcFunc: func(ctx context.Context) any {
				return post(ctx, newServer(t, spell.EnglishUS, 3), `{"words":["quick"]}`)
			},
		},
		{
			Name: "above-old-limit", ExpResp: http.StatusOK, CmpFunc: cmpDiff,
			ExcFunc: func(ctx context.Context) any {
				return post(ctx, newServer(t, spell.EnglishUS, 80), `{"words":["`+strings.Repeat("a", 65)+`"]}`).Status
			},
		},
		{
			Name: "escaped-Unicode-full-batch", ExpResp: http.StatusOK, CmpFunc: cmpDiff,
			ExcFunc: func(ctx context.Context) any {
				word := `"` + strings.Repeat(`\ud801\udc00`, 8) + `"`
				body := `{"words":[` + strings.Repeat(word+",", 499) + word + `]}`
				return post(ctx, newServer(t, spell.EnglishUS, 8), body).Status
			},
		},
		{
			Name: "configured-body-limit", ExpResp: response{Status: http.StatusRequestEntityTooLarge, Body: `{"error":"request body is larger than 19564 bytes"}`}, CmpFunc: cmpDiff,
			ExcFunc: func(ctx context.Context) any {
				return post(ctx, newServer(t, spell.EnglishUS, 3, 500, 19564), `{"words":["the"]}`+strings.Repeat(" ", 20000))
			},
		},
	}
	run(t, table, "word-limit")
}

func Test_RequestLimits(t *testing.T) {
	t.Parallel()
	body := `{"words":["the","ten"]}`
	table := []Table{
		{
			Name: "configuration", ExpResp: response{Status: http.StatusOK, Body: `{"defaultLanguage":"en-US","maxWordLen":64,"maxWords":2,"maxBodyBytes":64}`}, CmpFunc: cmpDiff,
			ExcFunc: func(ctx context.Context) any {
				return serve(ctx, newServer(t, spell.EnglishUS, 64, 2, 64), http.MethodGet, "/v1/config", "", nil)
			},
		},
		{
			Name: "at-count-and-byte-limit", ExpResp: response{Status: http.StatusOK, Body: `{"misspelled":[]}`}, CmpFunc: cmpDiff,
			ExcFunc: func(ctx context.Context) any {
				return post(ctx, newServer(t, spell.EnglishUS, 64, 2, len(body)), body)
			},
		},
		{
			Name: "over-count-limit", ExpResp: badRequestBody("parse check words: words: max 2 per request"), CmpFunc: cmpDiff,
			ExcFunc: func(ctx context.Context) any {
				return post(ctx, newServer(t, spell.EnglishUS, 64, 2, 64), `{"words":["the","ten","tea"]}`)
			},
		},
		{
			Name: "over-byte-limit", ExpResp: response{Status: http.StatusRequestEntityTooLarge, Body: fmt.Sprintf(`{"error":"request body is larger than %d bytes"}`, len(body))}, CmpFunc: cmpDiff,
			ExcFunc: func(ctx context.Context) any {
				return post(ctx, newServer(t, spell.EnglishUS, 64, 2, len(body)), body+" ")
			},
		},
	}
	for _, row := range []struct {
		name           string
		language       spell.Language
		wordLen, words int
		bytes          int64
		err            string
	}{
		{name: "invalid-language", language: "fr-FR", wordLen: 64, words: 500, bytes: 385564, err: `default language: language "fr-FR": expected en-US or en-GB`},
		{name: "zero-word-length", wordLen: 0, words: 500, bytes: 385564, err: "max word length: must be positive"},
		{name: "negative-word-length", wordLen: -1, words: 500, bytes: 385564, err: "max word length: must be positive"},
		{name: "zero-count", wordLen: 64, words: 0, bytes: 385564, err: "max words: must be positive"},
		{name: "negative-count", wordLen: 64, words: -1, bytes: 385564, err: "max words: must be positive"},
		{name: "zero-body", wordLen: 64, words: 500, bytes: 0, err: "max body bytes: must be positive"},
		{name: "negative-body", wordLen: 64, words: 500, bytes: -1, err: "max body bytes: must be positive"},
	} {
		table = append(table, Table{
			Name: row.name, ExpResp: row.err, CmpFunc: cmpDiff,
			ExcFunc: func(context.Context) any {
				_, err := httpapi.New(slog.New(slog.DiscardHandler), row.language, "test", nil, row.wordLen, row.words, row.bytes, nil)
				if err != nil {
					return err.Error()
				}
				return ""
			},
		})
	}
	run(t, table, "request-limits")
}

func health(srv http.Handler) []Table {
	host, err := os.Hostname()
	if err != nil {
		host = "unavailable"
	}

	table := []Table{
		{
			Name:    "liveness",
			ExpResp: response{Status: http.StatusOK, Body: fmt.Sprintf(`{"status":"up","build":"test","host":%q}`, host)},
			ExcFunc: func(ctx context.Context) any { return serve(ctx, srv, http.MethodGet, "/v1/liveness", "", nil) },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "readiness",
			ExpResp: response{Status: http.StatusNoContent},
			ExcFunc: func(ctx context.Context) any { return serve(ctx, srv, http.MethodGet, "/v1/readiness", "", nil) },
			CmpFunc: cmpDiff,
		},
	}

	return table
}

func routing(srv http.Handler) []Table {
	table := []Table{
		{
			Name:    "readiness-wrong-method",
			ExpResp: response{Status: http.StatusMethodNotAllowed, Body: "Method Not Allowed", Allow: "GET, HEAD"},
			ExcFunc: func(ctx context.Context) any { return serve(ctx, srv, http.MethodPost, "/v1/readiness", "", nil) },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "check-wrong-method",
			ExpResp: response{Status: http.StatusMethodNotAllowed, Body: "Method Not Allowed", Allow: "POST"},
			ExcFunc: func(ctx context.Context) any { return serve(ctx, srv, http.MethodGet, "/v1/check", "", nil) },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "unknown-path",
			ExpResp: response{Status: http.StatusNotFound, Body: "404 page not found"},
			ExcFunc: func(ctx context.Context) any { return serve(ctx, srv, http.MethodGet, "/v1/nope", "", nil) },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "unclean-path-keeps-method",
			ExpResp: response{Status: http.StatusOK, Body: `{"misspelled":[]}`},
			ExcFunc: func(ctx context.Context) any {
				return serve(ctx, srv, http.MethodPost, "//v1/./check", `{"words":["the"]}`, nil)
			},
			CmpFunc: cmpDiff,
		},
	}

	return table
}

func check200(srv http.Handler) []Table {
	table := []Table{
		{
			Name:    "American-language",
			ExpResp: response{Status: http.StatusOK, Body: `{"misspelled":[{"word":"colour","suggestions":["color"]}]}`},
			ExcFunc: func(ctx context.Context) any {
				return post(ctx, srv, `{"language":"en-US","words":["color","colour"]}`)
			},
			CmpFunc: cmpDiff,
		},
		{
			Name:    "British-language",
			ExpResp: response{Status: http.StatusOK, Body: `{"misspelled":[{"word":"color","suggestions":["colour"]}]}`},
			ExcFunc: func(ctx context.Context) any {
				return post(ctx, srv, `{"language":"en-GB","words":["color","colour"]}`)
			},
			CmpFunc: cmpDiff,
		},
		{
			Name:    "omitted-language-defaults-to-American",
			ExpResp: response{Status: http.StatusOK, Body: `{"misspelled":[{"word":"colour","suggestions":["color"]}]}`},
			ExcFunc: func(ctx context.Context) any { return post(ctx, srv, `{"words":["color","colour"]}`) },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "uppercase-typo",
			ExpResp: response{Status: http.StatusOK, Body: `{"misspelled":[{"word":"TEH","suggestions":["THE","TEN","TEA"]}]}`},
			ExcFunc: func(ctx context.Context) any { return post(ctx, srv, `{"words":["TEH","NASA"]}`) },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "misspelled",
			ExpResp: response{Status: http.StatusOK, Body: `{"misspelled":[{"word":"Teh","suggestions":["The","Ten","Tea"]}]}`},
			ExcFunc: func(ctx context.Context) any { return post(ctx, srv, `{"words":["Teh","quick"]}`) },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "all-valid-is-empty-array",
			ExpResp: response{Status: http.StatusOK, Body: `{"misspelled":[]}`},
			ExcFunc: func(ctx context.Context) any { return post(ctx, srv, `{"words":["the","NASA"]}`) },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "no-suggestions-is-empty-array",
			ExpResp: response{Status: http.StatusOK, Body: `{"misspelled":[{"word":"zzzzzz","suggestions":[]}]}`},
			ExcFunc: func(ctx context.Context) any { return post(ctx, srv, `{"words":["zzzzzz"]}`) },
			CmpFunc: cmpDiff,
		},
		{
			// httptest reports 200 when nothing is written; the empty body
			// is what shows the handler wrote nothing.
			Name:    "client-gone-gets-nothing",
			ExpResp: response{Status: http.StatusOK},
			ExcFunc: func(ctx context.Context) any {
				ctx, cancel := context.WithCancel(ctx)
				cancel()
				return post(ctx, srv, `{"words":["zzzzzz"]}`)
			},
			CmpFunc: cmpDiff,
		},
	}

	return table
}

func check400(srv http.Handler) []Table {
	tooMany := `{"words":[` + strings.Repeat(`"the",`, 500) + `"the"]}`

	table := []Table{
		{
			Name:    "unsupported-language",
			ExpResp: badRequestBody(`parse check language: language \"fr-FR\": expected en-US or en-GB`),
			ExcFunc: func(ctx context.Context) any { return post(ctx, srv, `{"language":"fr-FR","words":["the"]}`) },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "empty-body",
			ExpResp: badRequestBody("decode check request: decode request: EOF"),
			ExcFunc: func(ctx context.Context) any { return post(ctx, srv, ``) },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "invalid-json",
			ExpResp: badRequestBody("decode check request: decode request: unexpected EOF"),
			ExcFunc: func(ctx context.Context) any { return post(ctx, srv, `{"words":[`) },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "trailing-data",
			ExpResp: badRequestBody("decode check request: decode request: body must hold one JSON object"),
			ExcFunc: func(ctx context.Context) any { return post(ctx, srv, `{"words":["the"]} {}`) },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "malformed-trailing-data",
			ExpResp: badRequestBody("decode check request: decode trailing data: unexpected EOF"),
			ExcFunc: func(ctx context.Context) any { return post(ctx, srv, `{"words":["the"]} {`) },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "unknown-field",
			ExpResp: badRequestBody(`decode check request: decode request: json: unknown field \"lang\"`),
			ExcFunc: func(ctx context.Context) any { return post(ctx, srv, `{"words":["the"],"lang":"en"}`) },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "no-words",
			ExpResp: badRequestBody("parse check words: words: at least 1 required"),
			ExcFunc: func(ctx context.Context) any { return post(ctx, srv, `{"words":[]}`) },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "too-many-words",
			ExpResp: badRequestBody("parse check words: words: max 500 per request"),
			ExcFunc: func(ctx context.Context) any { return post(ctx, srv, tooMany) },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "invalid-word",
			ExpResp: badRequestBody(`parse check words: words[1]: invalid word \"abc1\": unexpected '1'`),
			ExcFunc: func(ctx context.Context) any { return post(ctx, srv, `{"words":["the","abc1"]}`) },
			CmpFunc: cmpDiff,
		},
	}

	return table
}

func check413(srv http.Handler) []Table {
	tooLarge := `{"words":["` + strings.Repeat("a", 400<<10) + `"]}`
	trailingTooLarge := `{"words":["the"]}` + strings.Repeat(" ", 400<<10)

	table := []Table{
		{
			Name:    "body",
			ExpResp: response{Status: http.StatusRequestEntityTooLarge, Body: `{"error":"request body is larger than 385564 bytes"}`},
			ExcFunc: func(ctx context.Context) any { return post(ctx, srv, tooLarge) },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "trailing-data",
			ExpResp: response{Status: http.StatusRequestEntityTooLarge, Body: `{"error":"request body is larger than 385564 bytes"}`},
			ExcFunc: func(ctx context.Context) any { return post(ctx, srv, trailingTooLarge) },
			CmpFunc: cmpDiff,
		},
	}

	return table
}

func checkCORS(srv http.Handler) []Table {
	preflight := func(origin string) http.Header {
		return http.Header{"Origin": {origin}, "Access-Control-Request-Method": {http.MethodPost}}
	}

	table := []Table{
		{
			Name:    "preflight-allowed-origin",
			ExpResp: response{Status: http.StatusNoContent, AllowOrigin: editorOrigin},
			ExcFunc: func(ctx context.Context) any {
				return serve(ctx, srv, http.MethodOptions, "/v1/check", "", preflight(editorOrigin))
			},
			CmpFunc: cmpDiff,
		},
		{
			Name:    "preflight-other-origin-is-routed",
			ExpResp: response{Status: http.StatusMethodNotAllowed, Body: "Method Not Allowed", Allow: "POST"},
			ExcFunc: func(ctx context.Context) any {
				return serve(ctx, srv, http.MethodOptions, "/v1/check", "", preflight("http://other.example"))
			},
			CmpFunc: cmpDiff,
		},
		{
			Name:    "options-without-preflight-is-routed",
			ExpResp: response{Status: http.StatusNotFound, Body: "404 page not found", AllowOrigin: editorOrigin},
			ExcFunc: func(ctx context.Context) any {
				return serve(ctx, srv, http.MethodOptions, "/v1/nope", "", http.Header{"Origin": {editorOrigin}})
			},
			CmpFunc: cmpDiff,
		},
		{
			Name:    "check-from-allowed-origin",
			ExpResp: response{Status: http.StatusOK, Body: `{"misspelled":[]}`, AllowOrigin: editorOrigin},
			ExcFunc: func(ctx context.Context) any {
				return serve(ctx, srv, http.MethodPost, "/v1/check", `{"words":["the"]}`, http.Header{"Origin": {editorOrigin}})
			},
			CmpFunc: cmpDiff,
		},
	}

	return table
}

// =============================================================================

// internalFailures checks that operational details do not reach the client.
func internalFailures() []Table {
	return []Table{
		{
			Name:    "unconfigured checker is redacted",
			ExpResp: response{Status: http.StatusInternalServerError, Body: `{"error":"internal server error"}`},
			ExcFunc: func(ctx context.Context) any {
				handler, err := httpapi.New(slog.New(slog.DiscardHandler), spell.EnglishUS, "test", nil, 64, 500, 385564, nil)
				if err != nil {
					return err.Error()
				}
				return post(ctx, handler, `{"words":["the"]}`)
			},
			CmpFunc: cmpDiff,
		},
		{
			Name:    "internal failure is logged once at the request boundary",
			ExpResp: requestLog{Level: "ERROR", Message: "request", Status: http.StatusInternalServerError, Error: "checker for en-US is not configured", Records: 1},
			ExcFunc: func(ctx context.Context) any {
				var output bytes.Buffer
				handler, err := httpapi.New(slog.New(slog.NewJSONHandler(&output, nil)), spell.EnglishUS, "test", nil, 64, 500, 385564, nil)
				if err != nil {
					return err.Error()
				}
				post(ctx, handler, `{"words":["the"]}`)
				records := bytes.Count(output.Bytes(), []byte("\n"))
				var entry requestLog
				if err := json.NewDecoder(&output).Decode(&entry); err != nil {
					return err.Error()
				}
				entry.Records = records
				return entry
			},
			CmpFunc: cmpDiff,
		},
	}
}

// =============================================================================

// editorOrigin is the browser origin the test server allows.
const editorOrigin = "http://editor.example"

// seedDict is the dictionary the test server checks against, so the tests do
// not depend on the embedded language lists.
const seedDict = `
the 100
ten 50
tea 40
quick 30
nasa 20
color 25
`

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

// newServer builds the service's handler over seedDict.
func newServer(t *testing.T, defaultLanguage spell.Language, limits ...int) http.Handler {
	t.Helper()

	maxWordLen := 64
	maxWords, maxBodyBytes := 500, 385564
	if len(limits) > 0 {
		maxWordLen = limits[0]
	}
	if len(limits) > 1 {
		maxWords = limits[1]
	}
	if len(limits) > 2 {
		maxBodyBytes = limits[2]
	}
	cfg := spell.Config{MaxSuggestions: 5, MaxDistance: 2, PrefixLength: 7}
	checker, err := spell.New(seedDict, cfg)
	if err != nil {
		t.Fatalf("Should be able to load the seed dictionary : %s", err)
	}
	british, err := spell.New(strings.ReplaceAll(seedDict, "color", "colour"), cfg)
	if err != nil {
		t.Fatalf("Should be able to load the British seed dictionary : %s", err)
	}

	handler, err := httpapi.New(
		slog.New(slog.DiscardHandler),
		defaultLanguage,
		"test",
		[]string{editorOrigin},
		maxWordLen,
		maxWords,
		int64(maxBodyBytes),
		map[spell.Language]*spell.Checker{spell.EnglishUS: checker, spell.EnglishGB: british},
	)
	if err != nil {
		t.Fatalf("Should be able to configure the API : %s", err)
	}
	return handler
}

// serve sends one request to h and returns what h wrote.
func serve(ctx context.Context, h http.Handler, method string, target string, body string, header http.Header) response {
	r := httptest.NewRequestWithContext(ctx, method, target, strings.NewReader(body))
	maps.Copy(r.Header, header)

	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	return response{
		Status:      w.Code,
		Body:        strings.TrimSpace(w.Body.String()),
		Allow:       w.Header().Get("Allow"),
		AllowOrigin: w.Header().Get("Access-Control-Allow-Origin"),
	}
}

func post(ctx context.Context, h http.Handler, body string) response {
	return serve(ctx, h, http.MethodPost, "/v1/check", body, nil)
}

func badRequestBody(msg string) response {
	return response{Status: http.StatusBadRequest, Body: `{"error":"` + msg + `"}`}
}
