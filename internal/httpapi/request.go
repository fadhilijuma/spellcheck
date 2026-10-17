package httpapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"
)

// maxBodyBytes caps each complete JSON request at 1 MiB, including whitespace.
const maxBodyBytes = 1 << 20

// checkRequest is the body of POST /v1/check.
type checkRequest struct {
	Words    []string `json:"words"`
	Language string   `json:"language"`
}

// decodeCheck reads one size-bounded JSON request, ignoring unknown fields.
func decodeCheck(w http.ResponseWriter, r *http.Request) (checkRequest, error) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		return checkRequest{}, fmt.Errorf("read request: %w", err)
	}

	var req checkRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return checkRequest{}, fmt.Errorf("decode request: %w", err)
	}

	return req, nil
}

// validateCheck enforces request limits without restricting word syntax.
func validateCheck(req checkRequest, maxWordLen, maxWords int) error {
	if len(req.Words) > maxWords {
		return fmt.Errorf("words: max %d per request", maxWords)
	}

	for i, word := range req.Words {
		if utf8.RuneCountInString(word) > maxWordLen && strings.TrimSpace(word) != "" {
			return fmt.Errorf("words[%d]: max %d characters", i, maxWordLen)
		}
	}

	return nil
}
