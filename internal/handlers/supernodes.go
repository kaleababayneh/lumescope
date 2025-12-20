package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"lumescope/internal/db"
	"lumescope/internal/util"
)

// SingleSupernodeMetricsResponse represents the metrics for a specific supernode
type SingleSupernodeMetricsResponse struct {
	SupernodeAccount       string                 `json:"supernode_account"`
	ValidatorAddress       string                 `json:"validator_address,omitempty"`
	ValidatorMoniker       string                 `json:"validator_moniker,omitempty"`
	CurrentState           string                 `json:"current_state"`
	IPAddress              string                 `json:"ip_address,omitempty"`
	P2PPort                int32                  `json:"p2p_port,omitempty"`
	ProtocolVersion        string                 `json:"protocol_version"`
	ActualVersion          string                 `json:"actual_version,omitempty"`
	CPUUsagePercent        *float64               `json:"cpu_usage_percent,omitempty"`
	CPUCores               *int32                 `json:"cpu_cores,omitempty"`
	MemoryTotalGb          *float64               `json:"memory_total_gb,omitempty"`
	MemoryUsedGb           *float64               `json:"memory_used_gb,omitempty"`
	MemoryUsagePercent     *float64               `json:"memory_usage_percent,omitempty"`
	StorageTotalBytes      *int64                 `json:"storage_total_bytes,omitempty"`
	StorageUsedBytes       *int64                 `json:"storage_used_bytes,omitempty"`
	StorageUsagePercent    *float64               `json:"storage_usage_percent,omitempty"`
	HardwareSummary        *string                `json:"hardware_summary,omitempty"`
	PeersCount             *int32                 `json:"peers_count,omitempty"`
	UptimeSeconds          *int64                 `json:"uptime_seconds,omitempty"`
	Rank                   *int32                 `json:"rank,omitempty"`
	LastStatusCheck        *time.Time             `json:"last_status_check,omitempty"`
	IsStatusAPIAvailable   bool                   `json:"is_status_api_available"`
	MetricsReport          map[string]interface{} `json:"metrics_report,omitempty"`
	SchemaVersion          string                 `json:"schema_version"`
	LastSuccessfulProbe    *time.Time             `json:"last_successful_probe,omitempty"`
	FailedProbeCounter     int32                  `json:"failed_probe_counter"`
	LastKnownActualVersion string                 `json:"last_known_actual_version,omitempty"`
}

type SupernodeMetricsListResponse struct {
	Total         int                              `json:"total"`
	Nodes         []SingleSupernodeMetricsResponse `json:"nodes"`
	NextCursor    string                           `json:"next_cursor,omitempty"`
	SchemaVersion string                           `json:"schema_version"`
}

// SyncTrigger defines the interface for triggering sync+probe operations
type SyncTrigger interface {
	TriggerSyncAndProbe(ctx context.Context) bool
}

// TriggerSupernodeSync triggers a manual sync+probe of all supernodes
func TriggerSupernodeSync(trigger SyncTrigger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		started := trigger.TriggerSyncAndProbe(r.Context())

		if started {
			// Return 202 Accepted with JSON body
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			json.NewEncoder(w).Encode(map[string]string{"status": "started"})
		} else {
			// Return 204 No Content
			w.WriteHeader(http.StatusNoContent)
		}
	}
}

// validChainStates defines the allowed values for the currentState query parameter
var validChainStates = map[string]bool{
	"SUPERNODE_STATE_UNSPECIFIED": true,
	"SUPERNODE_STATE_ACTIVE":      true,
	"SUPERNODE_STATE_DISABLED":    true,
	"SUPERNODE_STATE_STOPPED":     true,
	"SUPERNODE_STATE_PENALIZED":   true,
}

func ListSupernodesMetrics(pool *db.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()

		// Parse currentState parameter - now accepts exact chain state enum values
		var chainState *string
		if val := query.Get("currentState"); val != "" {
			if !validChainStates[val] {
				util.WriteJSONError(w, http.StatusBadRequest, "invalid currentState parameter: must be one of 'SUPERNODE_STATE_UNSPECIFIED', 'SUPERNODE_STATE_ACTIVE', 'SUPERNODE_STATE_DISABLED', 'SUPERNODE_STATE_STOPPED', 'SUPERNODE_STATE_PENALIZED'")
				return
			}
			chainState = &val
		}

		// Parse status parameter - "available" means all 3 ports are open
		status := query.Get("status")
		if status == "" {
			status = "any"
		}
		switch status {
		case "available", "unavailable", "any":
		default:
			util.WriteJSONError(w, http.StatusBadRequest, "invalid status parameter: must be 'available', 'unavailable', or 'any'")
			return
		}

		var version *string
		if versionParam := strings.TrimSpace(query.Get("version")); versionParam != "" {
			version = &versionParam
		}

		minFailed := 0
		if val := query.Get("minFailedProbeCounter"); val != "" {
			parsed, err := strconv.Atoi(val)
			if err != nil || parsed < 0 {
				util.WriteJSONError(w, http.StatusBadRequest, "invalid minFailedProbeCounter parameter: must be a non-negative integer")
				return
			}
			minFailed = parsed
		}

		limit := 100
		if val := query.Get("limit"); val != "" {
			parsed, err := strconv.Atoi(val)
			if err != nil {
				util.WriteJSONError(w, http.StatusBadRequest, "invalid limit parameter: must be an integer between 1 and 200")
				return
			}
			if parsed < 1 || parsed > 200 {
				util.WriteJSONError(w, http.StatusBadRequest, "invalid limit parameter: must be between 1 and 200")
				return
			}
			limit = parsed
		}

		var cursorAccount *string
		if val := query.Get("cursor"); val != "" {
			decoded, err := base64.StdEncoding.DecodeString(val)
			if err != nil {
				util.WriteJSONError(w, http.StatusBadRequest, "invalid cursor parameter: must be base64 encoded JSON")
				return
			}
			var payload struct {
				Account string `json:"account"`
			}
			if err := json.Unmarshal(decoded, &payload); err != nil || payload.Account == "" {
				util.WriteJSONError(w, http.StatusBadRequest, "invalid cursor parameter: must be base64 encoded JSON with account")
				return
			}
			cursorAccount = &payload.Account
		}

		filter := db.SupernodeMetricsFilter{
			CurrentState:  "any", // Use "any" for legacy filter since we're using ChainState now
			ChainState:    chainState,
			Status:        status,
			Version:       version,
			MinFailed:     minFailed,
			Limit:         limit,
			CursorAccount: cursorAccount,
		}

		supernodes, hasMore, err := db.ListSupernodeMetricsFiltered(r.Context(), pool, filter)
		if err != nil {
			util.WriteJSONError(w, http.StatusInternalServerError, "failed to fetch supernode metrics")
			return
		}

		nodes := make([]SingleSupernodeMetricsResponse, 0, len(supernodes))
		var maxTimestamp *time.Time

		for _, sn := range supernodes {
			node := SingleSupernodeMetricsResponse{
				SchemaVersion:          "v1.0",
				SupernodeAccount:       sn.SupernodeAccount,
				ValidatorAddress:       sn.ValidatorAddress,
				ValidatorMoniker:       sn.ValidatorMoniker,
				CurrentState:           sn.CurrentState,
				IPAddress:              sn.IPAddress,
				P2PPort:                sn.P2PPort,
				ProtocolVersion:        sn.ProtocolVersion,
				ActualVersion:          sn.ActualVersion,
				CPUUsagePercent:        sn.CPUUsagePercent,
				CPUCores:               sn.CPUCores,
				MemoryTotalGb:          sn.MemoryTotalGb,
				MemoryUsedGb:           sn.MemoryUsedGb,
				MemoryUsagePercent:     sn.MemoryUsagePercent,
				StorageTotalBytes:      sn.StorageTotalBytes,
				StorageUsedBytes:       sn.StorageUsedBytes,
				StorageUsagePercent:    sn.StorageUsagePercent,
				HardwareSummary:        sn.HardwareSummary,
				PeersCount:             sn.PeersCount,
				UptimeSeconds:          sn.UptimeSeconds,
				Rank:                   sn.Rank,
				LastStatusCheck:        sn.LastStatusCheck,
				IsStatusAPIAvailable:   sn.IsStatusAPIAvailable,
				LastSuccessfulProbe:    sn.LastSuccessfulProbe,
				FailedProbeCounter:     sn.FailedProbeCounter,
				LastKnownActualVersion: sn.LastKnownActualVersion,
			}

			if sn.MetricsReport != nil {
				if metricsMap, ok := sn.MetricsReport.(map[string]interface{}); ok {
					node.MetricsReport = metricsMap
				}
			}

			nodes = append(nodes, node)

			var candidate *time.Time
			if sn.LastStatusCheck != nil {
				candidate = sn.LastStatusCheck
			} else if sn.LastSuccessfulProbe != nil {
				candidate = sn.LastSuccessfulProbe
			}
			if candidate != nil {
				candidateTime := candidate.UTC()
				if maxTimestamp == nil || candidateTime.After(*maxTimestamp) {
					candidateCopy := candidateTime
					maxTimestamp = &candidateCopy
				}
			}
		}

		response := SupernodeMetricsListResponse{
			Total:         len(nodes),
			Nodes:         nodes,
			SchemaVersion: "v1.0",
		}

		if hasMore && len(supernodes) > 0 {
			cursorPayload := struct {
				Account string `json:"account"`
			}{
				Account: supernodes[len(supernodes)-1].SupernodeAccount,
			}
			buf, err := json.Marshal(cursorPayload)
			if err != nil {
				util.WriteJSONError(w, http.StatusInternalServerError, "failed to encode pagination cursor")
				return
			}
			response.NextCursor = base64.StdEncoding.EncodeToString(buf)
		}

		lastModified := time.Now().UTC()
		if maxTimestamp != nil {
			lastModified = *maxTimestamp
		}

		util.WriteJSON(w, r, http.StatusOK, response, &lastModified)
	}
}

// ListUnavailableSupernodes returns supernodes where isStatusApiAvailable=false,
// filtered by currentState query parameter (running|stopped|any, default: running)
func ListUnavailableSupernodes(pool *db.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Parse currentState query parameter
		stateFilter := r.URL.Query().Get("currentState")
		if stateFilter == "" {
			stateFilter = "running" // default
		}

		// Validate currentState parameter
		if stateFilter != "running" && stateFilter != "stopped" && stateFilter != "any" {
			util.WriteJSONError(w, http.StatusBadRequest, "invalid currentState parameter: must be 'running', 'stopped', or 'any'")
			return
		}

		// Query database
		supernodes, err := db.ListUnavailableSupernodes(r.Context(), pool, stateFilter)
		if err != nil {
			util.WriteJSONError(w, http.StatusInternalServerError, "database query failed")
			return
		}

		// Return response
		now := time.Now().UTC()
		util.WriteJSON(w, r, http.StatusOK, supernodes, &now)
	}
}

// GetSupernodeMetrics returns metrics for a specific supernode by ID
func GetSupernodeMetrics(pool *db.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := supernodeIDFromPath(r.URL.Path)
		if id == "" {
			util.WriteJSONError(w, http.StatusBadRequest, "invalid supernode ID")
			return
		}

		// Fetch supernode from database
		sn, err := db.GetSupernodeByID(r.Context(), pool, id)
		if err != nil {
			if err == db.ErrNotFound {
				util.WriteJSONError(w, http.StatusNotFound, "supernode not found")
				return
			}
			util.WriteJSONError(w, http.StatusInternalServerError, "failed to fetch supernode")
			return
		}

		// Build response with metrics
		resp := SingleSupernodeMetricsResponse{
			SchemaVersion:          "v1.0",
			SupernodeAccount:       sn.SupernodeAccount,
			ValidatorAddress:       sn.ValidatorAddress,
			ValidatorMoniker:       sn.ValidatorMoniker,
			CurrentState:           sn.CurrentState,
			IPAddress:              sn.IPAddress,
			P2PPort:                sn.P2PPort,
			ProtocolVersion:        sn.ProtocolVersion,
			ActualVersion:          sn.ActualVersion,
			CPUUsagePercent:        sn.CPUUsagePercent,
			CPUCores:               sn.CPUCores,
			MemoryTotalGb:          sn.MemoryTotalGb,
			MemoryUsedGb:           sn.MemoryUsedGb,
			MemoryUsagePercent:     sn.MemoryUsagePercent,
			StorageTotalBytes:      sn.StorageTotalBytes,
			StorageUsedBytes:       sn.StorageUsedBytes,
			StorageUsagePercent:    sn.StorageUsagePercent,
			HardwareSummary:        sn.HardwareSummary,
			PeersCount:             sn.PeersCount,
			UptimeSeconds:          sn.UptimeSeconds,
			Rank:                   sn.Rank,
			LastStatusCheck:        sn.LastStatusCheck,
			IsStatusAPIAvailable:   sn.IsStatusAPIAvailable,
			LastSuccessfulProbe:    sn.LastSuccessfulProbe,
			FailedProbeCounter:     sn.FailedProbeCounter,
			LastKnownActualVersion: sn.LastKnownActualVersion,
		}

		// Add metrics report if available
		if sn.MetricsReport != nil {
			if metricsMap, ok := sn.MetricsReport.(map[string]interface{}); ok {
				resp.MetricsReport = metricsMap
			}
		}

		lm := time.Now().UTC()
		if sn.LastStatusCheck != nil {
			lm = *sn.LastStatusCheck
		}
		util.WriteJSON(w, r, http.StatusOK, resp, &lm)
	}
}

func supernodeIDFromPath(path string) string {
	const prefix = "/v1/supernodes/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	s := strings.TrimPrefix(path, prefix)
	// Extract ID before "/metrics"
	if idx := strings.Index(s, "/"); idx != -1 {
		s = s[:idx]
	}
	if s == "" {
		return ""
	}
	return s
}

// SupernodeStatsResponse represents aggregated hardware statistics for available supernodes
type SupernodeStatsResponse struct {
	TotalCPUCores            int64   `json:"total_cpu_cores"`
	TotalMemoryGb            float64 `json:"total_memory_gb"`
	TotalStorageBytes        int64   `json:"total_storage_bytes"`
	UsedStorageBytes         int64   `json:"used_storage_bytes"`
	AvailableStorageBytes    int64   `json:"available_storage_bytes"`
	StorageUsedPercent       float64 `json:"storage_used_percent"`
	StorageAvailablePercent  float64 `json:"storage_available_percent"`
	AvailableSupernodes      int64   `json:"available_supernodes"`
	SchemaVersion            string  `json:"schema_version"`
}

// SupernodeActionStatsResponse represents aggregated action statistics for a supernode
type SupernodeActionStatsResponse struct {
	Total            int            `json:"total"`
	States           map[string]int `json:"states"`
	SupernodeAddress string         `json:"supernode_address"`
	SchemaVersion    string         `json:"schema_version"`
}

// GetSupernodeStats returns aggregated hardware statistics for fully available supernodes
func GetSupernodeStats(pool *db.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stats, err := db.GetAggregatedHardwareStats(r.Context(), pool)
		if err != nil {
			util.WriteJSONError(w, http.StatusInternalServerError, "failed to fetch hardware stats")
			return
		}

		// Calculate derived values
		availableStorageBytes := stats.TotalStorageBytes - stats.UsedStorageBytes
		var storageUsedPercent, storageAvailablePercent float64
		if stats.TotalStorageBytes > 0 {
			storageUsedPercent = float64(stats.UsedStorageBytes) / float64(stats.TotalStorageBytes) * 100
			storageAvailablePercent = float64(availableStorageBytes) / float64(stats.TotalStorageBytes) * 100
		}

		response := SupernodeStatsResponse{
			TotalCPUCores:           stats.TotalCPUCores,
			TotalMemoryGb:           stats.TotalMemoryGb,
			TotalStorageBytes:       stats.TotalStorageBytes,
			UsedStorageBytes:        stats.UsedStorageBytes,
			AvailableStorageBytes:   availableStorageBytes,
			StorageUsedPercent:      storageUsedPercent,
			StorageAvailablePercent: storageAvailablePercent,
			AvailableSupernodes:     stats.AvailableSupernodes,
			SchemaVersion:           "v1.0",
		}

		now := time.Now().UTC()
		util.WriteJSON(w, r, http.StatusOK, response, &now)
	}
}

// GetSupernodeActionStats returns aggregated action statistics for a specific supernode
func GetSupernodeActionStats(pool *db.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()

		// Get required address parameter
		address := strings.TrimSpace(query.Get("address"))
		if address == "" {
			util.WriteJSONError(w, http.StatusBadRequest, "address parameter is required")
			return
		}

		// Get optional type parameter
		actionType := strings.TrimSpace(query.Get("type"))

		// Query database
		stats, err := db.GetSupernodeActionStats(r.Context(), pool, address, actionType)
		if err != nil {
			util.WriteJSONError(w, http.StatusInternalServerError, "failed to fetch action stats")
			return
		}

		// Build states map from state counts
		statesMap := make(map[string]int)
		for _, sc := range stats.StateCounts {
			statesMap[sc.State] = sc.Count
		}

		response := SupernodeActionStatsResponse{
			Total:            stats.Total,
			States:           statesMap,
			SupernodeAddress: address,
			SchemaVersion:    "v1.0",
		}

		now := time.Now().UTC()
		util.WriteJSON(w, r, http.StatusOK, response, &now)
	}
}

// SupernodePaymentInfoResponse represents payment statistics for a supernode
type SupernodePaymentInfoResponse struct {
	Payments      []db.PaymentStat `json:"payments"`
	SchemaVersion string           `json:"schema_version"`
}

// GetPaymentInfo returns payment statistics for a specific supernode
func GetPaymentInfo(pool *db.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := supernodeIDFromPath(r.URL.Path)
		if id == "" {
			util.WriteJSONError(w, http.StatusBadRequest, "invalid supernode ID")
			return
		}

		// Query payment stats from database
		stats, err := db.GetSupernodePaymentStats(r.Context(), pool, id)
		if err != nil {
			util.WriteJSONError(w, http.StatusInternalServerError, "failed to fetch payment stats")
			return
		}

		// If no stats found, return empty array (not an error)
		if stats == nil {
			stats = []db.PaymentStat{}
		}

		response := SupernodePaymentInfoResponse{
			Payments:      stats,
			SchemaVersion: "v1.0",
		}

		now := time.Now().UTC()
		util.WriteJSON(w, r, http.StatusOK, response, &now)
	}
}
