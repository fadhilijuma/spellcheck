package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/fadhilijuma/spellcheck/internal/spell"
)

// checkRequest is the body of POST /v1/check.
type checkRequest struct {
	Words    []string `json:"words"`
	Language string   `json:"language"`
}

// decodeCheck reads exactly one size-bounded JSON object with no
// unknown fields. Decoder errors retain their original cause with context.
func decodeCheck(w http.ResponseWriter, r *http.Request, maxBodyBytes int64) (checkRequest, error) {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()

	var req checkRequest
	if err := dec.Decode(&req); err != nil {
		return checkRequest{}, fmt.Errorf("decode request: %w", err)
	}

	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err != nil {
			return checkRequest{}, fmt.Errorf("decode trailing data: %w", err)
		}
		return checkRequest{}, errors.New("decode request: body must hold one JSON object")
	}

	return req, nil
}

// toWords validates the request and parses its words.
func toWords(req checkRequest, maxWordLen, maxWords int) ([]spell.Word, error) {
	switch {
	case len(req.Words) == 0:
		return nil, errors.New("words: at least 1 required")
	case len(req.Words) > maxWords:
		return nil, fmt.Errorf("words: max %d per request", maxWords)
	}

	words := make([]spell.Word, 0, len(req.Words))
	for i, s := range req.Words {
		w, err := spell.ParseWord(s, maxWordLen)
		if err != nil {
			return nil, fmt.Errorf("words[%d]: %w", i, err)
		}
		words = append(words, w)
	}

	return words, nil
}
