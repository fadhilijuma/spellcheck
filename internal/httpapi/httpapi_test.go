package httpapi_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
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

func Test_API(t *testing.T) {
	t.Parallel()

	srv := newServer(t, spell.EnglishUS)

	run(t, health(srv), "health")
	run(t, routing(srv), "routing")
	run(t, check200(srv), "check-200")
	run(t, check400(srv), "check-400")
	run(t, check413(srv), "check-413")
	run(t, checkCORS(srv), "check-cors")
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
			ExpResp: response{Status: http.StatusOK, Body: `{"defaultLanguage":"en-GB"}`},
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

func health(srv http.Handler) []Table {
	host, err := os.Hostname()
	if err != nil {
		host = "unavailable"
	}

	table := []Table{
		{
			Name:    "liveness",
			ExpResp: response{Status: http.StatusOK, Body: fmt.Sprintf(`{"status":"up","build":"test","host":%q,"GOMAXPROCS":%d}`, host, runtime.GOMAXPROCS(0))},
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
			ExpResp: badRequestBody(`language \"fr-FR\": expected en-US or en-GB`),
			ExcFunc: func(ctx context.Context) any { return post(ctx, srv, `{"language":"fr-FR","words":["the"]}`) },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "empty-body",
			ExpResp: badRequestBody("decode request: EOF"),
			ExcFunc: func(ctx context.Context) any { return post(ctx, srv, ``) },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "invalid-json",
			ExpResp: badRequestBody("decode request: unexpected EOF"),
			ExcFunc: func(ctx context.Context) any { return post(ctx, srv, `{"words":[`) },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "trailing-data",
			ExpResp: badRequestBody("decode request: body must hold one JSON object"),
			ExcFunc: func(ctx context.Context) any { return post(ctx, srv, `{"words":["the"]} {}`) },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "unknown-field",
			ExpResp: badRequestBody(`decode request: json: unknown field \"lang\"`),
			ExcFunc: func(ctx context.Context) any { return post(ctx, srv, `{"words":["the"],"lang":"en"}`) },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "no-words",
			ExpResp: badRequestBody("words: at least 1 required"),
			ExcFunc: func(ctx context.Context) any { return post(ctx, srv, `{"words":[]}`) },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "too-many-words",
			ExpResp: badRequestBody("words: max 500 per request"),
			ExcFunc: func(ctx context.Context) any { return post(ctx, srv, tooMany) },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "invalid-word",
			ExpResp: badRequestBody(`words[1]: invalid word \"abc1\": unexpected '1'`),
			ExcFunc: func(ctx context.Context) any { return post(ctx, srv, `{"words":["the","abc1"]}`) },
			CmpFunc: cmpDiff,
		},
	}

	return table
}

func check413(srv http.Handler) []Table {
	tooLarge := `{"words":["` + strings.Repeat("a", 130<<10) + `"]}`
	trailingTooLarge := `{"words":["the"]}` + strings.Repeat(" ", 130<<10)

	table := []Table{
		{
			Name:    "body",
			ExpResp: response{Status: http.StatusRequestEntityTooLarge, Body: `{"error":"request body is larger than 129564 bytes"}`},
			ExcFunc: func(ctx context.Context) any { return post(ctx, srv, tooLarge) },
			CmpFunc: cmpDiff,
		},
		{
			Name:    "trailing-data",
			ExpResp: response{Status: http.StatusRequestEntityTooLarge, Body: `{"error":"request body is larger than 129564 bytes"}`},
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
func newServer(t *testing.T, defaultLanguage spell.Language) http.Handler {
	t.Helper()

	checker, err := spell.New(seedDict)
	if err != nil {
		t.Fatalf("Should be able to load the seed dictionary : %s", err)
	}
	british, err := spell.New(strings.ReplaceAll(seedDict, "color", "colour"))
	if err != nil {
		t.Fatalf("Should be able to load the British seed dictionary : %s", err)
	}

	return httpapi.New(slog.New(slog.NewTextHandler(io.Discard, nil)), httpapi.Config{
		DefaultLanguage: defaultLanguage,
		Build:           "test",
		CORSOrigins:     []string{editorOrigin},
		Checkers:        map[spell.Language]*spell.Checker{spell.EnglishUS: checker, spell.EnglishGB: british},
	})
}

// serve sends one request to h and returns what h wrote.
func serve(ctx context.Context, h http.Handler, method string, target string, body string, header http.Header) response {
	r := httptest.NewRequestWithContext(ctx, method, target, strings.NewReader(body))
	for key, values := range header {
		r.Header[key] = values
	}

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
