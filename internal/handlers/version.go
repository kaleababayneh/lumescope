package handlers

import (
	"net/http"
	"time"

	"lumescope/internal/util"
)

type VersionVerdict string

const (
	VerdictFull         VersionVerdict = "full"
	VerdictPartial      VersionVerdict = "partial"
	VerdictIncompatible VersionVerdict = "incompatible"
)

type ChainActionVersion struct {
	ActionType string `json:"action_type"`
	Version    int    `json:"version"`
}

type SupernodeCapability struct {
	NodeID   string `json:"node_id"`
	Version  string `json:"version"`
	Supports map[string]bool `json:"supports"`
}

type Compatibility struct {
	NodeID   string         `json:"node_id"`
	Verdict  VersionVerdict `json:"verdict"`
	Rationale string        `json:"rationale"`
}

type VersionMatrixResponse struct {
	ChainActionVersions []ChainActionVersion `json:"chain_action_versions"`
	Supernodes          []SupernodeCapability `json:"supernodes"`
	Compatibility       []Compatibility       `json:"compatibility"`
	SchemaVersion       string                `json:"schema_version"`
}

func VersionMatrix(w http.ResponseWriter, r *http.Request) {
	now := time.Now().UTC()
	resp := VersionMatrixResponse{
		ChainActionVersions: []ChainActionVersion{
			{ActionType: "cascade", Version: 1},
			{ActionType: "transfer", Version: 2},
		},
		Supernodes: []SupernodeCapability{
			{NodeID: "sn-1", Version: "v2.3.0", Supports: map[string]bool{"LEP1": true, "LEP2": true}},
			{NodeID: "sn-2", Version: "v2.2.1", Supports: map[string]bool{"LEP1": true, "LEP2": false}},
		},
		Compatibility: []Compatibility{
			{NodeID: "sn-1", Verdict: VerdictFull, Rationale: "All required capabilities present"},
			{NodeID: "sn-2", Verdict: VerdictPartial, Rationale: "LEP2 missing; partial support"},
		},
		SchemaVersion: "v1.0",
	}

	lm := now
	util.WriteJSON(w, r, http.StatusOK, resp, &lm)
}
