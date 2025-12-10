package handlers

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"lumescope/internal/db"
	"lumescope/internal/util"
)

type ActionItem struct {
	ID          string      `json:"id"`
	Type        string      `json:"type"`
	Creator     string      `json:"creator"`
	State       string      `json:"state"`
	BlockHeight int64       `json:"block_height"`
	Decoded     interface{} `json:"decoded,omitempty"`
	Raw         string      `json:"raw,omitempty"` // base64 of raw bytes if unknown type
}

type ActionsListResponse struct {
	Items         []ActionItem `json:"items"`
	NextCursor    string       `json:"next_cursor,omitempty"`
	SchemaVersion string       `json:"schema_version"`
}

func ListActions(pool *db.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		queryValues := r.URL.Query()

		filter := db.ActionsFilter{}

		if typeStr := queryValues.Get("type"); typeStr != "" {
			filterType := typeStr
			filter.Type = &filterType
		}
		if creatorStr := queryValues.Get("creator"); creatorStr != "" {
			filterCreator := creatorStr
			filter.Creator = &filterCreator
		}
		if stateStr := queryValues.Get("state"); stateStr != "" {
			filterState := stateStr
			filter.State = &filterState
		}

		limit := 50
		if limitStr := queryValues.Get("limit"); limitStr != "" {
			parsedLimit, err := strconv.Atoi(limitStr)
			if err != nil {
				util.WriteJSONError(w, http.StatusBadRequest, "invalid limit parameter")
				return
			}
			if parsedLimit < 1 {
				parsedLimit = 1
			} else if parsedLimit > 200 {
				parsedLimit = 200
			}
			limit = parsedLimit
		}
		filter.Limit = limit

		if fromStr := queryValues.Get("from"); fromStr != "" {
			parsedFrom, err := strconv.ParseInt(fromStr, 10, 64)
			if err != nil {
				util.WriteJSONError(w, http.StatusBadRequest, "invalid from parameter: must be a block height")
				return
			}
			filterFrom := parsedFrom
			filter.FromHeight = &filterFrom
		}

		if toStr := queryValues.Get("to"); toStr != "" {
			parsedTo, err := strconv.ParseInt(toStr, 10, 64)
			if err != nil {
				util.WriteJSONError(w, http.StatusBadRequest, "invalid to parameter: must be a block height")
				return
			}
			filterTo := parsedTo
			filter.ToHeight = &filterTo
		}

		if cursorStr := queryValues.Get("cursor"); cursorStr != "" {
			decodedCursor, err := base64.StdEncoding.DecodeString(cursorStr)
			if err != nil {
				util.WriteJSONError(w, http.StatusBadRequest, "invalid cursor encoding")
				return
			}
			var payload struct {
				TS string `json:"ts"`
				ID string `json:"id"`
			}
			if err := json.Unmarshal(decodedCursor, &payload); err != nil || payload.TS == "" || payload.ID == "" {
				util.WriteJSONError(w, http.StatusBadRequest, "invalid cursor format")
				return
			}
			parsedCursorTS, err := time.Parse(time.RFC3339, payload.TS)
			if err != nil {
				util.WriteJSONError(w, http.StatusBadRequest, "invalid cursor timestamp")
				return
			}
			parsedCursorTS = parsedCursorTS.UTC()
			cursorTime := parsedCursorTS
			cursorID := payload.ID
			filter.CursorTS = &cursorTime
			filter.CursorID = &cursorID
		}

		actions, hasMore, err := db.ListActionsFiltered(r.Context(), pool, filter)
		if err != nil {
			util.WriteJSONError(w, http.StatusInternalServerError, "failed to fetch actions")
			return
		}

		items := make([]ActionItem, 0, len(actions))
		for _, a := range actions {
			item := ActionItem{
				ID:          a.ActionID,
				Type:        a.ActionType,
				Creator:     a.Creator,
				State:       a.State,
				BlockHeight: a.BlockHeight,
			}

			// Add decoded metadata if available
			if a.MetadataJSON != nil {
				item.Decoded = a.MetadataJSON
			} else if len(a.MetadataRaw) > 0 {
				item.Raw = base64.StdEncoding.EncodeToString(a.MetadataRaw)
			}

			items = append(items, item)
		}

		resp := ActionsListResponse{
			Items:         items,
			SchemaVersion: "v1.0",
		}

		if hasMore && len(actions) > 0 {
			last := actions[len(actions)-1]
			cursorPayload := struct {
				TS string `json:"ts"`
				ID string `json:"id"`
			}{
				TS: last.CreatedAt.UTC().Format(time.RFC3339),
				ID: last.ActionID,
			}
			cursorJSON, err := json.Marshal(cursorPayload)
			if err != nil {
				util.WriteJSONError(w, http.StatusInternalServerError, "failed to encode cursor")
				return
			}
			resp.NextCursor = base64.StdEncoding.EncodeToString(cursorJSON)
		}

		lm := time.Now().UTC()
		util.WriteJSON(w, r, http.StatusOK, resp, &lm)
	}
}

func GetAction(pool *db.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := actionIDFromPath(r.URL.Path)
		if id == "" {
			util.WriteJSONError(w, http.StatusBadRequest, "invalid action ID")
			return
		}

		action, err := db.GetActionByID(r.Context(), pool, id)
		if err != nil {
			if err == db.ErrNotFound {
				util.WriteJSONError(w, http.StatusNotFound, "action not found")
				return
			}
			util.WriteJSONError(w, http.StatusInternalServerError, "failed to fetch action")
			return
		}

		// Build response with action details
		resp := struct {
			ID            string      `json:"id"`
			Type          string      `json:"type"`
			Creator       string      `json:"creator"`
			State         string      `json:"state"`
			BlockHeight   int64       `json:"block_height"`
			Price         Price       `json:"price"`
			Timestamp     time.Time   `json:"timestamp"`
			Decoded       interface{} `json:"decoded,omitempty"`
			Raw           string      `json:"raw,omitempty"`
			SuperNodes    interface{} `json:"super_nodes,omitempty"`
			SchemaVersion string      `json:"schema_version"`
		}{
			ID:          action.ActionID,
			Type:        action.ActionType,
			Creator:     action.Creator,
			State:       action.State,
			BlockHeight: action.BlockHeight,
			Price: Price{
				Denom:  action.PriceDenom,
				Amount: action.PriceAmount,
			},
			Timestamp:     time.Unix(action.BlockHeight, 0).UTC(),
			SchemaVersion: "v1.0",
		}

		// Add decoded metadata if available
		if action.MetadataJSON != nil {
			resp.Decoded = action.MetadataJSON
		} else if len(action.MetadataRaw) > 0 {
			// If no decoded JSON, include raw bytes as base64
			resp.Raw = base64.StdEncoding.EncodeToString(action.MetadataRaw)
		}

		// Add SuperNodes if available
		if action.SuperNodes != nil {
			resp.SuperNodes = action.SuperNodes
		}

		lm := time.Now().UTC()
		util.WriteJSON(w, r, http.StatusOK, resp, &lm)
	}
}

type Price struct {
	Denom  string `json:"denom"`
	Amount string `json:"amount"`
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

// ActionStatsResponse represents aggregated action statistics for all actions
type ActionStatsResponse struct {
	Total         int            `json:"total"`
	States        map[string]int `json:"states"`
	SchemaVersion string         `json:"schema_version"`
}

// GetActionStats returns aggregated action statistics for all actions (global)
func GetActionStats(pool *db.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()

		// Get optional type parameter
		actionType := strings.TrimSpace(query.Get("type"))

		// Query database
		stats, err := db.GetActionStats(r.Context(), pool, actionType)
		if err != nil {
			util.WriteJSONError(w, http.StatusInternalServerError, "failed to fetch action stats")
			return
		}

		// Build states map from state counts
		statesMap := make(map[string]int)
		for _, sc := range stats.StateCounts {
			statesMap[sc.State] = sc.Count
		}

		response := ActionStatsResponse{
			Total:         stats.Total,
			States:        statesMap,
			SchemaVersion: "v1.0",
		}

		now := time.Now().UTC()
		util.WriteJSON(w, r, http.StatusOK, response, &now)
	}
}
