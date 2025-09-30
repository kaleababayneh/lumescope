package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"lumescope/internal/util"
)

type ActionItem struct {
	ID        string      `json:"id"`
	Type      string      `json:"type"`
	Creator   string      `json:"creator"`
	State     string      `json:"state"`
	Timestamp time.Time   `json:"timestamp"`
	Decoded   interface{} `json:"decoded,omitempty"`
	Raw       string      `json:"raw,omitempty"` // base64 of raw bytes if unknown type
}

type ActionsListResponse struct {
	Items         []ActionItem `json:"items"`
	NextCursor    string       `json:"next_cursor,omitempty"`
	SchemaVersion string       `json:"schema_version"`
}

func ListActions(w http.ResponseWriter, r *http.Request) {
	// Parse filters (not used in stub)
	_ = r.URL.Query().Get("type")
	_ = r.URL.Query().Get("creator")
	_ = r.URL.Query().Get("state")
	_ = r.URL.Query().Get("from")
	_ = r.URL.Query().Get("to")

	// Pagination
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 200 {
			limit = v
		}
	}
	cursor := r.URL.Query().Get("cursor")
	_ = cursor

	now := time.Now().UTC()
	items := make([]ActionItem, 0, limit)
	for i := 0; i < limit; i++ {
		items = append(items, ActionItem{
			ID:        "act_" + strconv.Itoa(i+1),
			Type:      "cascade", // example type
			Creator:   "lumera1example...",
			State:     "committed",
			Timestamp: now.Add(-time.Duration(i) * time.Minute),
			Decoded: map[string]interface{}{
				"lep1": map[string]interface{}{
					"root": "bafy...",
					"pieces": 12,
				},
			},
		})
	}

	resp := ActionsListResponse{
		Items:         items,
		NextCursor:    "", // empty in stub
		SchemaVersion: "v1.0",
	}

	lm := time.Now().UTC()
	util.WriteJSON(w, r, http.StatusOK, resp, &lm)
}

func GetAction(w http.ResponseWriter, r *http.Request) {
	id := actionIDFromPath(r.URL.Path)
	if id == "" {
		http.Error(w, `{"error":"bad_request"}`, http.StatusBadRequest)
		return
	}

	now := time.Now().UTC()
	type CascadeLEP1 struct {
		RootCID      string `json:"root_cid"`
		PieceCount   int    `json:"piece_count"`
		TotalSize    int64  `json:"total_size_bytes"`
		PreviewMIME  string `json:"preview_mime"`
	}
	resp := struct {
		ID            string       `json:"id"`
		Type          string       `json:"type"`
		Creator       string       `json:"creator"`
		State         string       `json:"state"`
		Timestamp     time.Time    `json:"timestamp"`
		Decoded       interface{}  `json:"decoded"`
		CascadeLayout CascadeLEP1  `json:"cascade_layout"`
		SchemaVersion string       `json:"schema_version"`
	}{
		ID:        id,
		Type:      "cascade",
		Creator:   "lumera1example...",
		State:     "committed",
		Timestamp: now,
		Decoded: map[string]interface{}{
			"some_field": "value",
		},
		CascadeLayout: CascadeLEP1{
			RootCID:     "bafy...",
			PieceCount:  12,
			TotalSize:   1_048_576,
			PreviewMIME: "image/png",
		},
		SchemaVersion: "v1.0",
	}

	lm := time.Now().UTC()
	util.WriteJSON(w, r, http.StatusOK, resp, &lm)
}

func actionIDFromPath(path string) string {
	const prefix = "/v1/actions/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	s := strings.TrimPrefix(path, prefix)
	if s == "" || strings.Contains(s, "/") {
		return ""
	}
	return s
}
