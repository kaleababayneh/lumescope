package background

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"lumescope/internal/config"
	"lumescope/internal/db"
	"lumescope/internal/decoder"
	lclient "lumescope/internal/lumera"
)

// Runner holds dependencies for background syncs.
type Runner struct {
	Cfg    config.Config
	DB     *db.Pool
	Lumera *lclient.Client

	validatorMonikers map[string]string
	syncRunning       bool
	syncMu            sync.Mutex
}

func NewRunner(cfg config.Config, pool *db.Pool, lumera *lclient.Client) *Runner {
	return &Runner{Cfg: cfg, DB: pool, Lumera: lumera}
}

func (r *Runner) Start(ctx context.Context) {
	// Run initial validator sync to populate monikers before starting other loops
	if err := r.syncValidators(ctx); err != nil {
		log.Printf("initial validators sync error: %v", err)
	}
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
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (r *Runner) loopSupernodes(ctx context.Context) {
	t := time.NewTicker(r.Cfg.SupernodesSyncInterval)
	defer t.Stop()
	for {
		if err := r.syncSupernodes(ctx); err != nil {
			log.Printf("supernodes sync error: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (r *Runner) loopActions(ctx context.Context) {
	t := time.NewTicker(r.Cfg.ActionsSyncInterval)
	defer t.Stop()
	for {
		if err := r.syncActions(ctx); err != nil {
			log.Printf("actions sync error: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (r *Runner) loopProbes(ctx context.Context) {
	t := time.NewTicker(r.Cfg.ProbeInterval)
	defer t.Stop()
	for {
		if err := r.probeSupernodes(ctx); err != nil {
			log.Printf("probe error: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// syncValidators returns a map of valoper -> moniker to be used in supernode join.
func (r *Runner) syncValidators(ctx context.Context) error {
	var (
		next     string
		limit    = 200
		monikers = map[string]string{}
	)
	for {
		vals, n, err := r.Lumera.GetValidators(ctx, next, limit)
		if err != nil {
			return err
		}
		for _, v := range vals {
			monikers[v.OperatorAddress] = v.Description.Moniker
		}
		if n == "" {
			break
		}
		next = n
	}
	// Store in memory for this run only; returned to syncSupernodes via closure
	// For simplicity, we attach to Runner for reuse across loops.
	r.validatorMonikers = monikers
	return nil
}

// validatorMonikers caches last fetched valoper->moniker mapping.
func (r *Runner) getMonikerFor(valoper string) string {
	if r.validatorMonikers == nil {
		return ""
	}
	return r.validatorMonikers[valoper]
}

func (r *Runner) syncSupernodes(ctx context.Context) error {
	var next string
	limit := 200
	for {
		sns, n, err := r.Lumera.GetSupernodes(ctx, next, limit)
		if err != nil {
			return err
		}
		for _, sn := range sns {
			ip := latestIPAddress(sn.PrevIPAddresses)
			p2p := parseP2PPort(sn.P2PPortStr)
			state, height := latestState(sn.States)
			mon := r.getMonikerFor(sn.ValidatorAddress)
			rec := db.SupernodeDB{
				SupernodeAccount:   sn.SupernodeAccount,
				ValidatorAddress:   sn.ValidatorAddress,
				ValidatorMoniker:   mon,
				CurrentState:       state,
				CurrentStateHeight: height,
				IPAddress:          ip,
				P2PPort:            int32(p2p),
				ProtocolVersion:    chooseProtocol(sn.Note),
				PrevIPAddresses:    toJSONB(sn.PrevIPAddresses),
				Evidence:           toJSONB(sn.Evidence),
				StateHistory:       toJSONB(sn.States),
				MetricsReport:      toJSONB(sn.Metrics),
			}
			if err := db.UpsertSupernode(ctx, r.DB, rec); err != nil {
				log.Printf("upsert supernode %s: %v", sn.SupernodeAccount, err)
			}
		}
		if n == "" {
			break
		}
		next = n
	}
	return nil
}

func (r *Runner) syncActions(ctx context.Context) error {
	var next string
	limit := 100
	for {
		actions, n, err := r.Lumera.GetActions(ctx, "ACTION_TYPE_UNSPECIFIED", "ACTION_STATE_UNSPECIFIED", next, limit)
		if err != nil {
			return err
		}
		for _, a := range actions {
			raw, decoded, derr := decoder.DecodeActionMetadata(a.ActionType, a.MetadataB64)
			if derr != nil {
				log.Printf("decode action %s: %v", a.ActionID, derr)
			}
			var bh int64
			if a.BlockHeight != "" {
				if v, err := strconv.ParseInt(a.BlockHeight, 10, 64); err == nil {
					bh = v
				}
			}
			var exp int64
			if a.ExpirationTime != "" {
				if v, err := strconv.ParseInt(a.ExpirationTime, 10, 64); err == nil {
					exp = v
				}
			}
			// Ensure SuperNodes is never nil to avoid null in DB
			superNodes := a.SuperNodes
			if superNodes == nil {
				superNodes = []string{}
			}
			rec := db.ActionDB{
				ActionID:       a.ActionID,
				Creator:        a.Creator,
				ActionType:     a.ActionType,
				State:          a.State,
				BlockHeight:    bh,
				PriceDenom:     a.Price.Denom,
				PriceAmount:    a.Price.Amount,
				ExpirationTime: exp,
				MetadataRaw:    raw,
				MetadataJSON:   toJSONB(decoded),
				SuperNodes:     toJSONB(superNodes),
			}
			if err := db.UpsertAction(ctx, r.DB, rec); err != nil {
				log.Printf("upsert action %s: %v", a.ActionID, err)
			}
		}
		if n == "" {
			break
		}
		next = n
	}
	return nil
}

// TriggerSyncAndProbe manually triggers a sync+probe run if not already in progress.
// Returns true if the run was started, false if already running.
func (r *Runner) TriggerSyncAndProbe(ctx context.Context) bool {
	r.syncMu.Lock()
	if r.syncRunning {
		r.syncMu.Unlock()
		return false
	}
	r.syncRunning = true
	r.syncMu.Unlock()

	// Run sync+probe asynchronously
	go func() {
		defer func() {
			r.syncMu.Lock()
			r.syncRunning = false
			r.syncMu.Unlock()
		}()

		if err := r.syncSupernodes(ctx); err != nil {
			log.Printf("manual sync error: %v", err)
		}
		if err := r.probeSupernodes(ctx); err != nil {
			log.Printf("manual probe error: %v", err)
		}
	}()

	return true
}

func (r *Runner) probeSupernodes(ctx context.Context) error {
	targets, err := db.ListKnownSupernodes(ctx, r.DB)
	if err != nil {
		return err
	}
	for _, t := range targets {
		// ipAddress MUST have host:port format, otherwise it's a bad supernode
		if t.IPAddress == "" {
			log.Printf("skipping supernode %s: empty IP address (bad supernode)", t.SupernodeAccount)
			continue
		}

		// Trim any whitespace from ipAddress
		ipAddress := strings.TrimSpace(t.IPAddress)

		// Split ipAddress into host and port1
		host, portStr, err := net.SplitHostPort(ipAddress)
		if err != nil {
			// No port in ipAddress - this is a bad supernode
			log.Printf("skipping supernode %s: ipAddress '%s' has no port (bad supernode)", t.SupernodeAccount, ipAddress)
			continue
		}

		// Trim whitespace from host and port (in case of malformed data like "host :port" or "host: port ")
		host = strings.TrimSpace(host)
		portStr = strings.TrimSpace(portStr)

		port1, err := strconv.Atoi(portStr)
		if err != nil || port1 == 0 {
			log.Printf("skipping supernode %s: invalid port '%s' in ipAddress (bad supernode)", t.SupernodeAccount, portStr)
			continue
		}

		// Validate that host is either a valid IP or valid hostname
		if !isValidHost(host) {
			log.Printf("skipping supernode %s: invalid host '%s' in ipAddress (bad supernode)", t.SupernodeAccount, host)
			continue
		}

		// Probe 1: use host and port1 (from ipAddress)
		openPort1 := tcpOpen(ctx, host, port1, r.Cfg.DialTimeout)

		// Probe 2: use host and p2pPort (or default 4445 if empty)
		p2pPort := t.P2PPort
		if p2pPort == 0 {
			p2pPort = 4445 // default
		}
		openP2P := tcpOpen(ctx, host, int(p2pPort), r.Cfg.DialTimeout)

		// Status check: use host and port 8002
		status := fetchStatus(ctx, host)

		// Update DB with probe results (merge into metricsReport and status fields)
		now := time.Now().UTC()
		report := map[string]any{
			"ports": map[string]any{
				"port1":    openPort1,
				"port1Num": port1,
				"p2p":      openP2P,
				"p2pPort":  p2pPort,
			},
			"status": status,
		}
		sn := db.SupernodeProbeUpdate{
			SupernodeAccount:     t.SupernodeAccount,
			MetricsReport:        toJSONB(report),
			ActualVersion:        status.Version,
			UptimeSeconds:        ptrI64(status.UptimeSeconds),
			CPUUsagePercent:      ptrF64(status.CPUUsagePercent),
			CPUCores:             ptrI32(status.CPUCores),
			MemoryTotalGb:        ptrF64(status.MemoryTotalGb),
			MemoryUsedGb:         ptrF64(status.MemoryUsedGb),
			MemoryUsagePercent:   ptrF64(status.MemoryUsagePercent),
			StorageTotalBytes:    ptrI64(status.StorageTotalBytes),
			StorageUsedBytes:     ptrI64(status.StorageUsedBytes),
			StorageUsagePercent:  ptrF64(status.StorageUsagePercent),
			HardwareSummary:      ptrStr(status.HardwareSummary),
			PeersCount:           ptrI32(status.PeersCount),
			Rank:                 ptrI32(status.Rank),
			LastStatusCheck:      &now,
			IsStatusAPIAvailable: status.Available,
			ProbeTimeUTC:         now,
		}
		if err := db.UpdateSupernodeProbeData(ctx, r.DB, sn); err != nil {
			log.Printf("probe update %s: %v", t.SupernodeAccount, err)
		}
	}
	return nil
}

// Helpers

// latestState finds the state entry with the highest height value.
func latestState(states []lclient.SupernodeState) (string, string) {
	if len(states) == 0 {
		return "SUPERNODE_STATE_UNKNOWN", ""
	}
	// Find the state with the maximum height
	maxIdx := 0
	maxHeight := int64(0)
	for i, s := range states {
		if h, err := strconv.ParseInt(s.Height, 10, 64); err == nil {
			if h > maxHeight {
				maxHeight = h
				maxIdx = i
			}
		}
	}
	return states[maxIdx].State, states[maxIdx].Height
}

// latestIPAddress finds the IP address entry with the highest height value.
func latestIPAddress(addresses []lclient.PrevIPAddress) string {
	if len(addresses) == 0 {
		return ""
	}
	// Find the address with the maximum height
	maxIdx := 0
	maxHeight := int64(0)
	for i, addr := range addresses {
		if h, err := strconv.ParseInt(addr.Height, 10, 64); err == nil {
			if h > maxHeight {
				maxHeight = h
				maxIdx = i
			}
		}
	}
	return addresses[maxIdx].Address
}

func parseP2PPort(s string) int {
	if s == "" {
		return 0
	}
	v, _ := strconv.Atoi(s)
	return v
}

func chooseProtocol(note string) string {
	if note == "" {
		return "1.0.0"
	}
	return note
}

func toJSONB(v any) any {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return string(b)
}

func tcpOpen(ctx context.Context, host string, port int, timeout time.Duration) bool {
	d := net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// stripPort removes the port from a host:port string, returning just the host.
// If no port is present, returns the original string.
// Examples: "1.2.3.4:8080" -> "1.2.3.4", "host.com" -> "host.com"
func stripPort(hostPort string) string {
	host, _, err := net.SplitHostPort(hostPort)
	if err != nil {
		// No port present or invalid format, return as-is
		return hostPort
	}
	return host
}

// isValidHost checks if a string is either a valid IP address or a valid hostname/FQDN.
// Returns false for clearly invalid values like "SUNUCUIP", random text, etc.
func isValidHost(host string) bool {
	// Check if it's a valid IP address (IPv4 or IPv6)
	if net.ParseIP(host) != nil {
		return true
	}

	// Check if it's a valid hostname/FQDN
	// Valid hostnames:
	// - Can contain letters, digits, hyphens, and dots
	// - Cannot start or end with hyphen or dot
	// - Labels (parts between dots) must be 1-63 characters
	// - Total length must be <= 253 characters
	// - Must contain at least one letter (to exclude things like "123" or pure numbers)

	if len(host) == 0 || len(host) > 253 {
		return false
	}

	// Check for valid hostname pattern
	hasLetter := false
	hasDot := false
	prevChar := byte(0)

	for i := 0; i < len(host); i++ {
		r := host[i]
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			hasLetter = true
		} else if r == '.' {
			hasDot = true
			// Cannot start or end with dot, or have consecutive dots
			if i == 0 || i == len(host)-1 || prevChar == '.' {
				return false
			}
		} else if r >= '0' && r <= '9' {
			// Digits are ok
		} else if r == '-' {
			// Hyphen is ok, but not at start or end
			if i == 0 || i == len(host)-1 {
				return false
			}
		} else {
			// Invalid character
			return false
		}
		prevChar = r
	}

	// Must have at least one letter to be a valid hostname
	// For production use, require FQDN (with dot) to exclude single-label placeholders
	// like "SUNUCUIP", "localhost", etc. Real supernodes should use proper domains or IPs.
	return hasLetter && hasDot
}

// status fetch

type statusResponse struct {
	Version          string `json:"version"`
	UptimeSecondsStr string `json:"uptime_seconds"`
	Resources        struct {
		CPU struct {
			UsagePercent float64 `json:"usage_percent"`
			Cores        int     `json:"cores"`
		} `json:"cpu"`
		Memory struct {
			TotalGb      float64 `json:"total_gb"`
			UsedGb       float64 `json:"used_gb"`
			UsagePercent float64 `json:"usage_percent"`
		} `json:"memory"`
		StorageVolumes []struct {
			Path          string  `json:"path"`
			TotalBytesStr string  `json:"total_bytes"`
			UsedBytesStr  string  `json:"used_bytes"`
			UsagePercent  float64 `json:"usage_percent"`
		} `json:"storage_volumes"`
		HardwareSummary string `json:"hardware_summary"`
	} `json:"resources"`
	RunningTasks       any `json:"running_tasks"`
	RegisteredServices any `json:"registered_services"`
	Network            struct {
		PeersCount int `json:"peers_count"`
	} `json:"network"`
	Rank      int    `json:"rank"`
	IPAddress string `json:"ip_address"`
}

type statusSummary struct {
	Available           bool
	Version             string
	UptimeSeconds       int64
	CPUUsagePercent     float64
	CPUCores            int32
	MemoryTotalGb       float64
	MemoryUsedGb        float64
	MemoryUsagePercent  float64
	StorageTotalBytes   int64
	StorageUsedBytes    int64
	StorageUsagePercent float64
	HardwareSummary     string
	PeersCount          int32
	Rank                int32
}

func fetchStatus(ctx context.Context, host string) statusSummary {
	client := &http.Client{Timeout: 6 * time.Second}
	url := "http://" + net.JoinHostPort(host, "8002") + "/api/v1/status"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return statusSummary{Available: false}
	}
	resp, err := client.Do(req)
	if err != nil {
		return statusSummary{Available: false}
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return statusSummary{Available: false}
	}
	var sr statusResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return statusSummary{Available: false}
	}
	ss := statusSummary{Available: true, Version: sr.Version, CPUUsagePercent: sr.Resources.CPU.UsagePercent, CPUCores: int32(sr.Resources.CPU.Cores), MemoryTotalGb: sr.Resources.Memory.TotalGb, MemoryUsedGb: sr.Resources.Memory.UsedGb, MemoryUsagePercent: sr.Resources.Memory.UsagePercent, HardwareSummary: sr.Resources.HardwareSummary, PeersCount: int32(sr.Network.PeersCount), Rank: int32(sr.Rank)}
	if sr.UptimeSecondsStr != "" {
		if v, err := strconv.ParseInt(sr.UptimeSecondsStr, 10, 64); err == nil {
			ss.UptimeSeconds = v
		}
	}
	// Sum storage volumes
	var total, used int64
	for _, vol := range sr.Resources.StorageVolumes {
		if vol.TotalBytesStr != "" {
			if v, err := strconv.ParseInt(vol.TotalBytesStr, 10, 64); err == nil {
				total += v
			}
		}
		if vol.UsedBytesStr != "" {
			if v, err := strconv.ParseInt(vol.UsedBytesStr, 10, 64); err == nil {
				used += v
			}
		}
		ss.StorageUsagePercent = vol.UsagePercent // last volume percent; approximate
	}
	ss.StorageTotalBytes = total
	ss.StorageUsedBytes = used
	return ss
}

func ptrF64(v float64) *float64 { return &v }
func ptrI64(v int64) *int64     { return &v }
func ptrI32(v int32) *int32     { return &v }
func ptrStr(v string) *string   { return &v }
