package background

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"

	"lumescope/internal/config"
	"lumescope/internal/db"
	"lumescope/internal/decoder"
	lclient "lumescope/internal/lumera"
)

// Runner holds dependencies for background syncs.
type Runner struct {
	Cfg      config.Config
	DB       *db.Pool
	Lumera   *lclient.Client

	validatorMonikers map[string]string
}

func NewRunner(cfg config.Config, pool *db.Pool, lumera *lclient.Client) *Runner {
	return &Runner{Cfg: cfg, DB: pool, Lumera: lumera}
}

func (r *Runner) Start(ctx context.Context) {
	go r.loopValidators(ctx)
	go r.loopSupernodes(ctx)
	go r.loopActions(ctx)
	go r.loopProbes(ctx)
}

func (r *Runner) loopValidators(ctx context.Context) {
	t := time.NewTicker(r.Cfg.ValidatorsSyncInterval)
	defer t.Stop()
	for {
		if err := r.syncValidators(ctx); err != nil {
			log.Printf("validators sync error: %v", err)
		}
		select { case <-ctx.Done(): return; case <-t.C: }
	}
}

func (r *Runner) loopSupernodes(ctx context.Context) {
	t := time.NewTicker(r.Cfg.SupernodesSyncInterval)
	defer t.Stop()
	for {
		if err := r.syncSupernodes(ctx); err != nil {
			log.Printf("supernodes sync error: %v", err)
		}
		select { case <-ctx.Done(): return; case <-t.C: }
	}
}

func (r *Runner) loopActions(ctx context.Context) {
	t := time.NewTicker(r.Cfg.ActionsSyncInterval)
	defer t.Stop()
	for {
		if err := r.syncActions(ctx); err != nil {
			log.Printf("actions sync error: %v", err)
		}
		select { case <-ctx.Done(): return; case <-t.C: }
	}
}

func (r *Runner) loopProbes(ctx context.Context) {
	t := time.NewTicker(r.Cfg.ProbeInterval)
	defer t.Stop()
	for {
		if err := r.probeSupernodes(ctx); err != nil {
			log.Printf("probe error: %v", err)
		}
		select { case <-ctx.Done(): return; case <-t.C: }
	}
}

// syncValidators returns a map of valoper -> moniker to be used in supernode join.
func (r *Runner) syncValidators(ctx context.Context) error {
	var (
		next string
		limit = 200
		monikers = map[string]string{}
	)
	for {
		vals, n, err := r.Lumera.GetValidators(ctx, next, limit)
		if err != nil { return err }
		for _, v := range vals {
			monikers[v.OperatorAddress] = v.Description.Moniker
		}
		if n == "" { break }
		next = n
	}
	// Store in memory for this run only; returned to syncSupernodes via closure
	// For simplicity, we attach to Runner for reuse across loops.
	r.validatorMonikers = monikers
	return nil
}

// validatorMonikers caches last fetched valoper->moniker mapping.
func (r *Runner) getMonikerFor(valoper string) string {
	if r.validatorMonikers == nil { return "" }
	return r.validatorMonikers[valoper]
}

var _monikers map[string]string

func (r *Runner) syncSupernodes(ctx context.Context) error {
	var next string
	limit := 200
	for {
		sns, n, err := r.Lumera.GetSupernodes(ctx, next, limit)
		if err != nil { return err }
		for _, sn := range sns {
			var ip string
			if len(sn.PrevIPAddresses) > 0 {
				ip = sn.PrevIPAddresses[len(sn.PrevIPAddresses)-1].Address
			}
			p2p := parseP2PPort(sn.P2PPortStr)
			state, height := currentState(sn.States)
			mon := r.getMonikerFor(sn.ValidatorAddress)
			rec := db.SupernodeDB{
				SupernodeAccount: sn.SupernodeAccount,
				ValidatorAddress: sn.ValidatorAddress,
				ValidatorMoniker: mon,
				CurrentState: state,
				CurrentStateHeight: height,
				IPAddress: ip,
				P2PPort: int32(p2p),
				ProtocolVersion: chooseProtocol(sn.Note),
				PrevIPAddresses: toJSONB(sn.PrevIPAddresses),
				Evidence: toJSONB(sn.Evidence),
				StateHistory: toJSONB(sn.States),
				MetricsReport: toJSONB(sn.Metrics),
			}
			if err := db.UpsertSupernode(ctx, r.DB, rec); err != nil { log.Printf("upsert supernode %s: %v", sn.SupernodeAccount, err) }
		}
		if n == "" { break }
		next = n
	}
	return nil
}

func (r *Runner) syncActions(ctx context.Context) error {
	var next string
	limit := 100
	for {
		actions, n, err := r.Lumera.GetActions(ctx, "ACTION_TYPE_UNSPECIFIED", "ACTION_STATE_UNSPECIFIED", next, limit)
		if err != nil { return err }
		for _, a := range actions {
			raw, decoded, derr := decoder.DecodeActionMetadata(a.ActionType, a.MetadataB64)
			if derr != nil { log.Printf("decode action %s: %v", a.ActionID, derr) }
			var bh int64
			if a.BlockHeight != "" { if v, err := strconv.ParseInt(a.BlockHeight, 10, 64); err == nil { bh = v } }
			var exp int64
			if a.ExpirationTime != "" { if v, err := strconv.ParseInt(a.ExpirationTime, 10, 64); err == nil { exp = v } }
			rec := db.ActionDB{
				ActionID: a.ActionID,
				Creator: a.Creator,
				ActionType: a.ActionType,
				State: a.State,
				BlockHeight: bh,
				PriceDenom: a.Price.Denom,
				PriceAmount: a.Price.Amount,
				ExpirationTime: exp,
				MetadataRaw: raw,
				MetadataJSON: toJSONB(decoded),
			}
			if err := db.UpsertAction(ctx, r.DB, rec); err != nil { log.Printf("upsert action %s: %v", a.ActionID, err) }
		}
		if n == "" { break }
		next = n
	}
	return nil
}

func (r *Runner) probeSupernodes(ctx context.Context) error {
	targets, err := db.ListKnownSupernodes(ctx, r.DB)
	if err != nil { return err }
	for _, t := range targets {
		if t.IPAddress == "" { continue }
		host := t.IPAddress
		open4444 := tcpOpen(ctx, host, 4444, r.Cfg.DialTimeout)
		open4445 := tcpOpen(ctx, host, 4445, r.Cfg.DialTimeout)
		status := fetchStatus(ctx, host)
		// Update DB with probe results (merge into metricsReport and status fields)
		now := time.Now().UTC()
		report := map[string]any{
			"ports": map[string]bool{"4444": open4444, "4445": open4445},
			"status": status,
		}
		sn := db.SupernodeDB{
			SupernodeAccount: t.SupernodeAccount,
			MetricsReport: toJSONB(report),
			ActualVersion: status.Version,
			UptimeSeconds: ptrI64(status.UptimeSeconds),
			CPUUsagePercent: ptrF64(status.CPUUsagePercent),
			CPUCores: ptrI32(status.CPUCores),
			MemoryTotalGb: ptrF64(status.MemoryTotalGb),
			MemoryUsedGb: ptrF64(status.MemoryUsedGb),
			MemoryUsagePercent: ptrF64(status.MemoryUsagePercent),
			StorageTotalBytes: ptrI64(status.StorageTotalBytes),
			StorageUsedBytes: ptrI64(status.StorageUsedBytes),
			StorageUsagePercent: ptrF64(status.StorageUsagePercent),
			HardwareSummary: ptrStr(status.HardwareSummary),
			PeersCount: ptrI32(status.PeersCount),
			Rank: ptrI32(status.Rank),
			LastStatusCheck: &now,
			IsStatusAPIAvailable: status.Available,
		}
		if err := db.UpsertSupernode(ctx, r.DB, sn); err != nil { log.Printf("probe upsert %s: %v", t.SupernodeAccount, err) }
	}
	return nil
}

// Helpers

func currentState(states []lclient.SupernodeState) (string, string) {
	if len(states) == 0 { return "SUPERNODE_STATE_UNKNOWN", "" }
	s := states[len(states)-1]
	return s.State, s.Height
}

func parseP2PPort(s string) int {
	if s == "" { return 0 }
	v, _ := strconv.Atoi(s)
	return v
}

func chooseProtocol(note string) string {
	if note == "" { return "1.0.0" }
	return note
}

func toJSONB(v any) any {
	if v == nil { return nil }
	b, err := json.Marshal(v)
	if err != nil { return nil }
	return string(b)
}

func tcpOpen(ctx context.Context, host string, port int, timeout time.Duration) bool {
	d := net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil { return false }
	conn.Close()
	return true
}

// status fetch

type statusResponse struct {
	Version string `json:"version"`
	UptimeSecondsStr string `json:"uptime_seconds"`
	Resources struct {
		CPU struct {
			UsagePercent float64 `json:"usage_percent"`
			Cores int `json:"cores"`
		} `json:"cpu"`
		Memory struct {
			TotalGb float64 `json:"total_gb"`
			UsedGb float64 `json:"used_gb"`
			UsagePercent float64 `json:"usage_percent"`
		} `json:"memory"`
		StorageVolumes []struct {
			Path string `json:"path"`
			TotalBytesStr string `json:"total_bytes"`
			UsedBytesStr string `json:"used_bytes"`
			UsagePercent float64 `json:"usage_percent"`
		} `json:"storage_volumes"`
		HardwareSummary string `json:"hardware_summary"`
	} `json:"resources"`
	RunningTasks any `json:"running_tasks"`
	RegisteredServices any `json:"registered_services"`
	Network struct {
		PeersCount int `json:"peers_count"`
	} `json:"network"`
	Rank int `json:"rank"`
	IPAddress string `json:"ip_address"`
}

type statusSummary struct {
	Available bool
	Version string
	UptimeSeconds int64
	CPUUsagePercent float64
	CPUCores int32
	MemoryTotalGb float64
	MemoryUsedGb float64
	MemoryUsagePercent float64
	StorageTotalBytes int64
	StorageUsedBytes int64
	StorageUsagePercent float64
	HardwareSummary string
	PeersCount int32
	Rank int32
}

func fetchStatus(ctx context.Context, host string) statusSummary {
	client := &http.Client{ Timeout: 6 * time.Second }
	url := "http://" + net.JoinHostPort(host, "8002") + "/api/v1/status"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := client.Do(req)
	if err != nil { return statusSummary{Available: false} }
	defer resp.Body.Close()
	if resp.StatusCode != 200 { return statusSummary{Available: false} }
	var sr statusResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil { return statusSummary{Available: false} }
	ss := statusSummary{ Available: true, Version: sr.Version, CPUUsagePercent: sr.Resources.CPU.UsagePercent, CPUCores: int32(sr.Resources.CPU.Cores), MemoryTotalGb: sr.Resources.Memory.TotalGb, MemoryUsedGb: sr.Resources.Memory.UsedGb, MemoryUsagePercent: sr.Resources.Memory.UsagePercent, HardwareSummary: sr.Resources.HardwareSummary, PeersCount: int32(sr.Network.PeersCount), Rank: int32(sr.Rank) }
	if sr.UptimeSecondsStr != "" { if v, err := strconv.ParseInt(sr.UptimeSecondsStr, 10, 64); err == nil { ss.UptimeSeconds = v } }
	// Sum storage volumes
	var total, used int64
	for _, vol := range sr.Resources.StorageVolumes {
		if vol.TotalBytesStr != "" { if v, err := strconv.ParseInt(vol.TotalBytesStr, 10, 64); err == nil { total += v } }
		if vol.UsedBytesStr != "" { if v, err := strconv.ParseInt(vol.UsedBytesStr, 10, 64); err == nil { used += v } }
		ss.StorageUsagePercent = vol.UsagePercent // last volume percent; approximate
	}
	ss.StorageTotalBytes = total
	ss.StorageUsedBytes = used
	return ss
}

func ptrF64(v float64) *float64 { return &v }
func ptrI64(v int64) *int64 { return &v }
func ptrI32(v int32) *int32 { return &v }
func ptrStr(v string) *string { return &v }
