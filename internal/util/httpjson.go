package util

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"
)

// WriteJSON marshals v to JSON, sets headers, computes a weak ETag, and writes the response.
// If the request has If-None-Match/If-Modified-Since and matches, it returns 304.
func WriteJSON(w http.ResponseWriter, r *http.Request, status int, v interface{}, lastModified *time.Time) {
	w.Header().Set("Content-Type", "application/json")

	b, err := json.Marshal(v)
	if err != nil {
		http.Error(w, `{"error":"internal_error"}`, http.StatusInternalServerError)
		return
	}

	etag := makeWeakETag(b)
	w.Header().Set("ETag", etag)

	if lastModified != nil {
		w.Header().Set("Last-Modified", lastModified.UTC().Format(http.TimeFormat))
	}

	// Conditional requests
	if inm := r.Header.Get("If-None-Match"); inm != "" && inm == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	if ims := r.Header.Get("If-Modified-Since"); ims != "" && lastModified != nil {
		if t, err := time.Parse(http.TimeFormat, ims); err == nil {
			if !lastModified.After(t) {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}
	}

	w.WriteHeader(status)
	w.Write(b)
}

func makeWeakETag(b []byte) string {
	sum := sha256.Sum256(b)
	return "W/\"" + hex.EncodeToString(sum[:8]) + "\""
}
