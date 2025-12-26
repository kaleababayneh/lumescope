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

// TransactionDTO represents transaction data in API responses
type TransactionDTO struct {
	TxType           string     `json:"tx_type"`
	TxHash           string     `json:"tx_hash"`
	Height           int64      `json:"height"`
	BlockTime        time.Time  `json:"block_time"`
	GasWanted        *int64     `json:"gas_wanted,omitempty"`
	GasUsed          *int64     `json:"gas_used,omitempty"`
	ActionPrice      *string    `json:"action_price,omitempty"`
	ActionPriceDenom *string    `json:"action_price_denom,omitempty"`
	FlowPayer        *string    `json:"flow_payer,omitempty"`
	FlowPayee        *string    `json:"flow_payee,omitempty"`
	TxFee            *string    `json:"tx_fee,omitempty"`
	TxFeeDenom       *string    `json:"tx_fee_denom,omitempty"`
}

// PlaceholderTxHash is used to mark actions that have been checked but have no
// transactions on chain. This allows the enricher to skip them in future runs.
const PlaceholderTxHash = "_NO_TX_FOUND_"

// isPlaceholderTransaction returns true if the transaction is a placeholder
// inserted by the enricher to mark "not found" cases.
func isPlaceholderTransaction(tx db.ActionTransaction) bool {
	return tx.TxHash == PlaceholderTxHash
}

// actionTransactionToDTO converts a db.ActionTransaction to TransactionDTO
func actionTransactionToDTO(tx db.ActionTransaction) TransactionDTO {
	return TransactionDTO{
		TxType:           tx.TxType,
		TxHash:           tx.TxHash,
		Height:           tx.Height,
		BlockTime:        tx.BlockTime,
		GasWanted:        tx.GasWanted,
		GasUsed:          tx.GasUsed,
		ActionPrice:      tx.ActionPrice,
		ActionPriceDenom: tx.ActionPriceDenom,
		FlowPayer:        tx.FlowPayer,
		FlowPayee:        tx.FlowPayee,
		TxFee:            tx.TxFee,
		TxFeeDenom:       tx.TxFeeDenom,
	}
}

type ActionItem struct {
	ID           string           `json:"id"`
	Type         string           `json:"type"`
	Creator      string           `json:"creator"`
	State        string           `json:"state"`
	BlockHeight  int64            `json:"block_height"`
	MimeType     string           `json:"mime_type,omitempty"`
	Size         int64            `json:"size"`
	Price        Price            `json:"price"`
	Decoded      interface{}      `json:"decoded,omitempty"`
	Raw          string           `json:"raw,omitempty"` // base64 of raw bytes if unknown type
	// Flattened transaction fields for convenience
	RegisterTxID     *string    `json:"register_tx_id,omitempty"`
	RegisterTxTime   *time.Time `json:"register_tx_time,omitempty"`
	FinalizeTxID     *string    `json:"finalize_tx_id,omitempty"`
	FinalizeTxTime   *time.Time `json:"finalize_tx_time,omitempty"`
	ApproveTxID      *string    `json:"approve_tx_id,omitempty"`
	ApproveTxTime    *time.Time `json:"approve_tx_time,omitempty"`
	Transactions     []TransactionDTO `json:"transactions,omitempty"`
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
		if supernodeStr := queryValues.Get("supernode"); supernodeStr != "" {
			filterSupernode := supernodeStr
			filter.Supernode = &filterSupernode
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
			// Parse cursor ID as uint64
			cursorIDVal, err := strconv.ParseUint(payload.ID, 10, 64)
			if err != nil {
				util.WriteJSONError(w, http.StatusBadRequest, "invalid cursor ID: must be numeric")
				return
			}
			filter.CursorTS = &cursorTime
			filter.CursorID = &cursorIDVal
		}

		// Parse include_transactions parameter (default: false)
		includeTransactions := false
		if includeTxStr := queryValues.Get("include_transactions"); includeTxStr != "" {
			includeTransactions = includeTxStr == "true" || includeTxStr == "1"
		}

		actions, hasMore, err := db.ListActionsFiltered(r.Context(), pool, filter)
		if err != nil {
			util.WriteJSONError(w, http.StatusInternalServerError, "failed to fetch actions")
			return
		}

		// Always fetch transactions to populate flattened fields (register_tx_id, etc.)
		// Collect action IDs for bulk transaction fetch
		actionIDs := make([]uint64, 0, len(actions))
		for _, a := range actions {
			actionIDs = append(actionIDs, a.ActionID)
		}

		txMap, err := db.GetActionTransactionsByActionIDs(r.Context(), pool, actionIDs)
		if err != nil {
			util.WriteJSONError(w, http.StatusInternalServerError, "failed to fetch action transactions")
			return
		}

		items := make([]ActionItem, 0, len(actions))
		for _, a := range actions {
			// Convert uint64 ActionID to string for JSON response
			actionIDStr := strconv.FormatUint(a.ActionID, 10)
			item := ActionItem{
				ID:          actionIDStr,
				Type:        a.ActionType,
				Creator:     a.Creator,
				State:       a.State,
				BlockHeight: a.BlockHeight,
				MimeType:    a.MimeType,
				Size:        a.Size,
				Price: Price{
					Amount: a.PriceAmount,
					Denom:  a.PriceDenom,
				},
			}

			// Add decoded metadata if available
			if a.MetadataJSON != nil {
				item.Decoded = a.MetadataJSON
			} else if len(a.MetadataRaw) > 0 {
				item.Raw = base64.StdEncoding.EncodeToString(a.MetadataRaw)
			}

			// Always populate flattened fields from transactions
			// Filter out placeholder transactions (_NO_TX_FOUND_) from API responses
			if txs, ok := txMap[a.ActionID]; ok && len(txs) > 0 {
				var txDTOs []TransactionDTO
				if includeTransactions {
					txDTOs = make([]TransactionDTO, 0, len(txs))
				}
				for _, tx := range txs {
					// Skip placeholder transactions
					if isPlaceholderTransaction(tx) {
						continue
					}
					if includeTransactions {
						txDTOs = append(txDTOs, actionTransactionToDTO(tx))
					}
					// Always populate flattened transaction fields
					txHash := tx.TxHash
					txTime := tx.BlockTime
					switch tx.TxType {
					case "register":
						item.RegisterTxID = &txHash
						item.RegisterTxTime = &txTime
					case "finalize":
						item.FinalizeTxID = &txHash
						item.FinalizeTxTime = &txTime
					case "approve":
						item.ApproveTxID = &txHash
						item.ApproveTxTime = &txTime
					}
				}
				// Only include Transactions array if requested
				if includeTransactions {
					item.Transactions = txDTOs
				}
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
				ID: strconv.FormatUint(last.ActionID, 10),
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
		idStr := actionIDFromPath(r.URL.Path)
		if idStr == "" {
			util.WriteJSONError(w, http.StatusBadRequest, "invalid action ID")
			return
		}

		// Parse string ID to uint64
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			util.WriteJSONError(w, http.StatusBadRequest, "invalid action ID: must be numeric")
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

		// Fetch transactions for this action
		transactions, err := db.GetActionTransactions(r.Context(), pool, id)
		if err != nil {
			util.WriteJSONError(w, http.StatusInternalServerError, "failed to fetch action transactions")
			return
		}

		// Convert transactions to DTOs and extract flattened fields
		// Filter out placeholder transactions (_NO_TX_FOUND_) from API responses
		var txDTOs []TransactionDTO
		var registerTxID, finalizeTxID, approveTxID *string
		var registerTxTime, finalizeTxTime, approveTxTime *time.Time
		if len(transactions) > 0 {
			txDTOs = make([]TransactionDTO, 0, len(transactions))
			for _, tx := range transactions {
				// Skip placeholder transactions
				if isPlaceholderTransaction(tx) {
					continue
				}
				txDTOs = append(txDTOs, actionTransactionToDTO(tx))
				// Populate flattened transaction fields
				txHash := tx.TxHash
				txTime := tx.BlockTime
				switch tx.TxType {
				case "register":
					registerTxID = &txHash
					registerTxTime = &txTime
				case "finalize":
					finalizeTxID = &txHash
					finalizeTxTime = &txTime
				case "approve":
					approveTxID = &txHash
					approveTxTime = &txTime
				}
			}
		}

		// Build response with action details
		resp := struct {
			ID             string           `json:"id"`
			Type           string           `json:"type"`
			Creator        string           `json:"creator"`
			State          string           `json:"state"`
			BlockHeight    int64            `json:"block_height"`
			MimeType       string           `json:"mime_type,omitempty"`
			Size           int64            `json:"size"`
			Price          Price            `json:"price"`
			Decoded        interface{}      `json:"decoded,omitempty"`
			Raw            string           `json:"raw,omitempty"`
			SuperNodes     interface{}      `json:"super_nodes,omitempty"`
			RegisterTxID   *string          `json:"register_tx_id,omitempty"`
			RegisterTxTime *time.Time       `json:"register_tx_time,omitempty"`
			FinalizeTxID   *string          `json:"finalize_tx_id,omitempty"`
			FinalizeTxTime *time.Time       `json:"finalize_tx_time,omitempty"`
			ApproveTxID    *string          `json:"approve_tx_id,omitempty"`
			ApproveTxTime  *time.Time       `json:"approve_tx_time,omitempty"`
			Transactions   []TransactionDTO `json:"transactions,omitempty"`
			SchemaVersion  string           `json:"schema_version"`
		}{
			ID:             strconv.FormatUint(action.ActionID, 10),
			Type:           action.ActionType,
			Creator:        action.Creator,
			State:          action.State,
			BlockHeight:    action.BlockHeight,
			MimeType:       action.MimeType,
			Size:           action.Size,
			Price: Price{
				Denom:  action.PriceDenom,
				Amount: action.PriceAmount,
			},
			RegisterTxID:   registerTxID,
			RegisterTxTime: registerTxTime,
			FinalizeTxID:   finalizeTxID,
			FinalizeTxTime: finalizeTxTime,
			ApproveTxID:    approveTxID,
			ApproveTxTime:  approveTxTime,
			Transactions:   txDTOs,
			SchemaVersion:  "v1.0",
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

// MimeTypeStatResponse represents statistics for a single MIME type
type MimeTypeStatResponse struct {
	Type    string  `json:"type"`
	Count   int     `json:"count"`
	AvgSize float64 `json:"avg_size"`
}

// ActionStatsResponse represents aggregated action statistics for all actions
type ActionStatsResponse struct {
	Total         int                    `json:"total"`
	States        map[string]int         `json:"states"`
	MimeTypes     []MimeTypeStatResponse `json:"mime_types,omitempty"`
	SchemaVersion string                 `json:"schema_version"`
}

// GetActionStats returns aggregated action statistics for all actions (global)
func GetActionStats(pool *db.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()

		// Build filter from query parameters
		filter := db.ActionStatsFilter{}

		// Get optional type parameter
		if actionType := strings.TrimSpace(query.Get("type")); actionType != "" {
			filter.ActionType = &actionType
		}

		// Parse optional 'from' parameter (RFC3339 format)
		if fromStr := strings.TrimSpace(query.Get("from")); fromStr != "" {
			parsedFrom, err := time.Parse(time.RFC3339, fromStr)
			if err != nil {
				util.WriteJSONError(w, http.StatusBadRequest, "invalid 'from' parameter: must be RFC3339 format")
				return
			}
			filter.From = &parsedFrom
		}

		// Parse optional 'to' parameter (RFC3339 format)
		if toStr := strings.TrimSpace(query.Get("to")); toStr != "" {
			parsedTo, err := time.Parse(time.RFC3339, toStr)
			if err != nil {
				util.WriteJSONError(w, http.StatusBadRequest, "invalid 'to' parameter: must be RFC3339 format")
				return
			}
			filter.To = &parsedTo
		}

		// Query database with extended stats
		stats, err := db.GetActionStatsExtended(r.Context(), pool, filter)
		if err != nil {
			util.WriteJSONError(w, http.StatusInternalServerError, "failed to fetch action stats")
			return
		}

		// Build states map from state counts
		statesMap := make(map[string]int)
		for _, sc := range stats.StateCounts {
			statesMap[sc.State] = sc.Count
		}

		// Build MIME types list
		var mimeTypes []MimeTypeStatResponse
		for _, ms := range stats.MimeTypeStats {
			mimeTypes = append(mimeTypes, MimeTypeStatResponse{
				Type:    ms.MimeType,
				Count:   ms.Count,
				AvgSize: ms.AvgSize,
			})
		}

		response := ActionStatsResponse{
			Total:         stats.Total,
			States:        statesMap,
			MimeTypes:     mimeTypes,
			SchemaVersion: "v1.0",
		}

		now := time.Now().UTC()
		util.WriteJSON(w, r, http.StatusOK, response, &now)
	}
}
