// Package httpapi exposes the spellcheck service over HTTP: routes,
// middleware, JSON bodies and error responses. Spelling rules live in package
// spell; this package only translates between HTTP and Go values.
package httpapi

import (
	"log/slog"
	"net/http"
	"os"
	"runtime"

	"github.com/fadhilijuma/spellcheck/internal/spell"
)

// Config is what the handlers need.
type Config struct {
	Build       string
	CORSOrigins []string // browser origins allowed to call the API; empty turns CORS off
	Checker     *spell.Checker
}

// New returns the service's HTTP handler with all routes and middleware.
// Health probes are not logged: the container runtime calls them every few
// seconds.
func New(log *slog.Logger, cfg Config) http.Handler {
	a := app{
		log:     log,
		build:   cfg.Build,
		checker: cfg.Checker,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/liveness", a.liveness)
	mux.HandleFunc("GET /v1/readiness", a.readiness)
	mux.Handle("POST /v1/check", logRequests(log, http.HandlerFunc(a.check)))

	h := recoverPanics(log, mux)
	h = cors(cfg.CORSOrigins, h)
	h = cleanPath(h)

	return h
}

// =============================================================================

type app struct {
	log     *slog.Logger
	build   string
	checker *spell.Checker
}

// liveness reports that the process is up, which build it runs and where.
func (a *app) liveness(w http.ResponseWriter, _ *http.Request) {
	host, err := os.Hostname()
	if err != nil {
		host = "unavailable"
	}

	writeJSON(w, http.StatusOK, livenessResponse{
		Status:     "up",
		Build:      a.build,
		Host:       host,
		GOMAXPROCS: runtime.GOMAXPROCS(0),
	})
}

// readiness reports that the service can take traffic. The dictionary and its
// index are built before the server starts, so a running server is ready.
func (a *app) readiness(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

// check returns the misspelled words among those sent, each with its
// suggestions; valid words are omitted.
func (a *app) check(w http.ResponseWriter, r *http.Request) {
	req, err := decodeCheck(w, r)
	if err != nil {
		a.respondError(w, r, err)
		return
	}

	words, err := toWords(req)
	if err != nil {
		a.respondError(w, r, err)
		return
	}

	results, err := a.checker.Check(r.Context(), words)
	if err != nil {
		a.respondError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, toCheckResponse(results))
}
