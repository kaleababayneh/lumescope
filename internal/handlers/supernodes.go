package handlers

import (
	"net/http"
	"time"

	"lumescope/internal/util"
)

type SupernodeMetric struct {
	NodeID        string    `json:"node_id"`
	Address       string    `json:"address"`
	LastScrape    time.Time `json:"last_scrape"`
	Availability  float64   `json:"availability"`
	RTTms         int       `json:"rtt_ms"`
	QueueDepth    int       `json:"queue_depth"`
	LastStatus    string    `json:"last_status"`
}

type MetricsRollup struct {
	SampledAt     time.Time `json:"sampled_at"`
	NodeCount     int       `json:"node_count"`
	AvailablePct  float64   `json:"available_pct"`
	AvgRTTms      int       `json:"avg_rtt_ms"`
}

type SupernodeMetricsResponse struct {
	Nodes         []SupernodeMetric `json:"nodes"`
	Rollup        MetricsRollup     `json:"rollup"`
	SchemaVersion string            `json:"schema_version"`
}

func SupernodeMetrics(w http.ResponseWriter, r *http.Request) {
	now := time.Now().UTC()
	nodes := []SupernodeMetric{
		{
			NodeID:       "sn-1",
			Address:      "https://sn1.example.com",
			LastScrape:   now.Add(-10 * time.Second),
			Availability: 1.0,
			RTTms:        120,
			QueueDepth:   2,
			LastStatus:   "ok",
		},
		{
			NodeID:       "sn-2",
			Address:      "https://sn2.example.com",
			LastScrape:   now.Add(-25 * time.Second),
			Availability: 0.98,
			RTTms:        160,
			QueueDepth:   4,
			LastStatus:   "ok",
		},
	}

	roll := MetricsRollup{
		SampledAt:    now,
		NodeCount:    len(nodes),
		AvailablePct: 99.0,
		AvgRTTms:     140,
	}

	resp := SupernodeMetricsResponse{
		Nodes:         nodes,
		Rollup:        roll,
		SchemaVersion: "v1.0",
	}

	lm := now
	util.WriteJSON(w, r, http.StatusOK, resp, &lm)
}
