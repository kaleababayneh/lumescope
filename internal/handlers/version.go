package handlers

import (
	"net/http"
	"time"

	"lumescope/internal/db"
	"lumescope/internal/util"
)

// VersionMatrixRow represents a single version entry in the matrix
type VersionMatrixRow struct {
	Version         string `json:"version"`
	NodesTotal      int    `json:"nodes_total"`
	NodesAvailable  int    `json:"nodes_available"`
	NodesUnavailable int   `json:"nodes_unavailable"`
	IsLatest        bool   `json:"is_latest"`
}

// VersionMatrixResponse represents the version compatibility matrix response
type VersionMatrixResponse struct {
	LatestVersion string             `json:"latest_version,omitempty"`
	Versions      []VersionMatrixRow `json:"versions"`
}

// VersionMatrix returns the current version compatibility matrix from database
func VersionMatrix(pool *db.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Fetch version data from database
		versions, err := db.ListVersionMatrix(r.Context(), pool)
		if err != nil {
			util.WriteJSONError(w, http.StatusInternalServerError, "failed to fetch version data")
			return
		}

		// If no data, return empty response
		if len(versions) == 0 {
			now := time.Now().UTC()
			resp := VersionMatrixResponse{
				Versions: []VersionMatrixRow{},
			}
			util.WriteJSON(w, r, http.StatusOK, resp, &now)
			return
		}

		// Determine latest version (most common version as heuristic)
		latestVersion := versions[0].Version // Already sorted by total DESC
		
		// Build response
		rows := make([]VersionMatrixRow, 0, len(versions))
		for _, v := range versions {
			rows = append(rows, VersionMatrixRow{
				Version:          v.Version,
				NodesTotal:       v.Total,
				NodesAvailable:   v.Available,
				NodesUnavailable: v.Unavailable,
				IsLatest:         v.Version == latestVersion,
			})
		}

		resp := VersionMatrixResponse{
			LatestVersion: latestVersion,
			Versions:      rows,
		}

		now := time.Now().UTC()
		util.WriteJSON(w, r, http.StatusOK, resp, &now)
	}
}
