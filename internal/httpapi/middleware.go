package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path"
	"runtime/debug"
	"slices"
	"time"
)

// statusRecorder remembers the status code written, for request logging.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

// handleRequest adapts a returning handler to net/http and handles its error once.
// Internal details stay in the request log; canceled clients get no response.
func handleRequest(log *slog.Logger, next func(http.ResponseWriter, *http.Request) (int, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}

		attrs := []any{"method", r.Method, "path", r.URL.Path}
		level := slog.LevelInfo
		status, err := next(rec, r)
		if err != nil && !errors.Is(err, context.Canceled) {
			message := err.Error()
			limit, oversized := errors.AsType[*http.MaxBytesError](err)
			switch {
			case status >= http.StatusInternalServerError:
				level = slog.LevelError
				attrs = append(attrs, "err", err)
				message = "internal server error"
			case oversized:
				status = http.StatusRequestEntityTooLarge
				message = fmt.Sprintf("request body is larger than %d bytes", limit.Limit)
			}
			writeJSON(rec, status, errorResponse{Error: message})
		}
		attrs = append(attrs, "status", rec.status, "took", time.Since(start).String())
		log.Log(r.Context(), level, "request", attrs...)
	})
}

// recoverPanics turns a handler panic into a logged stack trace and a 500.
// http.ErrAbortHandler is re-panicked: net/http uses it to abort a response
// on purpose.
func recoverPanics(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			switch rec {
			case nil:
				return
			case http.ErrAbortHandler:
				panic(rec)
			}

			log.ErrorContext(r.Context(), "panic", "method", r.Method, "path", r.URL.Path, "panic", rec, "stack", string(debug.Stack()))
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal server error"})
		}()

		next.ServeHTTP(w, r)
	})
}

// cors lets browsers on the allowed origins call the API. A preflight from an
// allowed origin is answered here; every other request, OPTIONS included, is
// routed as usual. With no origins CORS is off: the editor reaches the API
// through a same-origin proxy. Max-Age lets the browser cache the preflight
// for a day.
func cors(origins []string, next http.Handler) http.Handler {
	if len(origins) == 0 {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Origin")

		origin := r.Header.Get("Origin")
		if !slices.Contains(origins, origin) {
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Access-Control-Allow-Origin", origin)

		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			w.Header().Set("Access-Control-Allow-Methods", "POST")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Max-Age", "86400")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// cleanPath normalises the path before routing. Without it ServeMux answers
// an unclean path such as //v1/check with a 301, and clients follow that
// redirect as a GET, which turns a POST into a 405.
func cleanPath(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = path.Clean(r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

// =============================================================================

func (s *statusRecorder) WriteHeader(status int) {
	if s.status == 0 {
		s.status = status
	}
	s.ResponseWriter.WriteHeader(status)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	return s.ResponseWriter.Write(b)
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (s *statusRecorder) Unwrap() http.ResponseWriter {
	return s.ResponseWriter
}
