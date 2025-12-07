package db

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool exposes a subset of pgxpool.Pool we need. Wrap for easier testing later.
// In this base, we simply export the pgxpool.Pool pointer.
type Pool = pgxpool.Pool

// Connect opens a connection pool to Postgres using pgxpool.
func Connect(ctx context.Context, dsn string, maxConns int32) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	if maxConns > 0 {
		cfg.MaxConns = maxConns
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect db: %w", err)
	}
	ctxPing, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(ctxPing); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db ping: %w", err)
	}
	return pool, nil
}

// Close closes the pool.
func Close(pool *pgxpool.Pool) {
	if pool != nil {
		pool.Close()
	}
}

// Bootstrap creates required tables and indexes if they do not exist.
func Bootstrap(ctx context.Context, pool *pgxpool.Pool) error {
	// We intentionally avoid custom enum types for portability; use TEXT with defaults.
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS supernodes (
			"supernodeAccount"     VARCHAR(255) PRIMARY KEY,
			"validatorAddress"     VARCHAR(255),
			"validatorMoniker"     VARCHAR(255),
			"currentState"         TEXT NOT NULL DEFAULT 'SUPERNODE_STATE_UNKNOWN',
			"currentStateHeight"   VARCHAR(255),
			"ipAddress"            VARCHAR(64),
			"p2pPort"              INTEGER,
			"protocolVersion"      VARCHAR(255) NOT NULL DEFAULT '1.0.0',
			"actualVersion"        VARCHAR(255),
			"cpuUsagePercent"      DOUBLE PRECISION,
			"cpuCores"             INTEGER,
			"memoryTotalGb"        DOUBLE PRECISION,
			"memoryUsedGb"         DOUBLE PRECISION,
			"memoryUsagePercent"   DOUBLE PRECISION,
			"storageTotalBytes"    BIGINT,
			"storageUsedBytes"     BIGINT,
			"storageUsagePercent"  DOUBLE PRECISION,
			"hardwareSummary"      TEXT,
			"peersCount"           INTEGER,
			"uptimeSeconds"        BIGINT,
			rank                   INTEGER,
			"registeredServices"   JSONB,
			"runningTasks"         JSONB,
			"stateHistory"         JSONB,
			evidence               JSONB,
			"prevIpAddresses"      JSONB,
			"lastStatusCheck"      TIMESTAMP,
			"isStatusApiAvailable" BOOLEAN NOT NULL DEFAULT FALSE,
			"metricsReport"        JSONB,
			"lastSuccessfulProbe"  TIMESTAMP,
			"failedProbeCounter"   INTEGER NOT NULL DEFAULT 0,
			"lastKnownActualVersion" VARCHAR(255),
			"createdAt"            TIMESTAMP NOT NULL DEFAULT now(),
			"updatedAt"            TIMESTAMP NOT NULL DEFAULT now()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_supernodes_validator_address ON supernodes ("validatorAddress")`,
		`CREATE INDEX IF NOT EXISTS idx_supernodes_supernode_account ON supernodes ("supernodeAccount")`,
		`CREATE INDEX IF NOT EXISTS idx_supernodes_current_state ON supernodes ("currentState")`,
		// Migration for existing tables: add new columns if they don't exist
		`ALTER TABLE supernodes ADD COLUMN IF NOT EXISTS "lastSuccessfulProbe" TIMESTAMP`,
		`ALTER TABLE supernodes ADD COLUMN IF NOT EXISTS "failedProbeCounter" INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE supernodes ADD COLUMN IF NOT EXISTS "lastKnownActualVersion" VARCHAR(255)`,
		`CREATE TABLE IF NOT EXISTS actions (
			"actionID"      VARCHAR(64) PRIMARY KEY,
			"creator"       VARCHAR(255),
			"actionType"    TEXT,
			"state"         TEXT,
			"blockHeight"   BIGINT,
			"priceDenom"    TEXT,
			"priceAmount"   TEXT,
			"expirationTime" BIGINT,
			"metadataRaw"   BYTEA,
			"metadataJSON"  JSONB,
			"superNodes"    JSONB,
			"createdAt"     TIMESTAMP NOT NULL DEFAULT now(),
			"updatedAt"     TIMESTAMP NOT NULL DEFAULT now()
		)`,
		// Migration for existing actions table: add superNodes column if it doesn't exist
		`ALTER TABLE actions ADD COLUMN IF NOT EXISTS "superNodes" JSONB`,
	}
	for _, s := range stmts {
		if _, err := pool.Exec(ctx, s); err != nil {
			return fmt.Errorf("bootstrap exec: %w", err)
		}
	}
	return nil
}

// UpsertSupernode stores or updates a supernode record.
func UpsertSupernode(ctx context.Context, pool *pgxpool.Pool, sn SupernodeDB) error {
	sql := `INSERT INTO supernodes (
		"supernodeAccount","validatorAddress","validatorMoniker","currentState","currentStateHeight","ipAddress","p2pPort","protocolVersion","actualVersion","cpuUsagePercent","cpuCores","memoryTotalGb","memoryUsedGb","memoryUsagePercent","storageTotalBytes","storageUsedBytes","storageUsagePercent","hardwareSummary","peersCount","uptimeSeconds",rank,"registeredServices","runningTasks","stateHistory",evidence,"prevIpAddresses","lastStatusCheck","isStatusApiAvailable","metricsReport","createdAt","updatedAt"
	) VALUES (
		$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22::jsonb,$23::jsonb,$24::jsonb,$25::jsonb,$26::jsonb,$27,$28,$29::jsonb,now(),now()
	) ON CONFLICT ("supernodeAccount") DO UPDATE SET
		"validatorAddress"=EXCLUDED."validatorAddress",
		"validatorMoniker"=COALESCE(NULLIF(EXCLUDED."validatorMoniker",''),supernodes."validatorMoniker"),
		"currentState"=EXCLUDED."currentState",
		"currentStateHeight"=EXCLUDED."currentStateHeight",
		"ipAddress"=EXCLUDED."ipAddress",
		"p2pPort"=EXCLUDED."p2pPort",
		"protocolVersion"=EXCLUDED."protocolVersion",
		"stateHistory"=EXCLUDED."stateHistory",
		evidence=EXCLUDED.evidence,
		"prevIpAddresses"=EXCLUDED."prevIpAddresses",
		"metricsReport"=COALESCE(EXCLUDED."metricsReport",supernodes."metricsReport"),
		"registeredServices"=COALESCE(EXCLUDED."registeredServices",supernodes."registeredServices"),
		"runningTasks"=COALESCE(EXCLUDED."runningTasks",supernodes."runningTasks"),
		"updatedAt"=now()`
	_, err := pool.Exec(ctx, sql,
		sn.SupernodeAccount, sn.ValidatorAddress, sn.ValidatorMoniker, sn.CurrentState, sn.CurrentStateHeight, sn.IPAddress, sn.P2PPort, sn.ProtocolVersion, sn.ActualVersion, sn.CPUUsagePercent, sn.CPUCores, sn.MemoryTotalGb, sn.MemoryUsedGb, sn.MemoryUsagePercent, sn.StorageTotalBytes, sn.StorageUsedBytes, sn.StorageUsagePercent, sn.HardwareSummary, sn.PeersCount, sn.UptimeSeconds, sn.Rank, sn.RegisteredServices, sn.RunningTasks, sn.StateHistory, sn.Evidence, sn.PrevIPAddresses, sn.LastStatusCheck, sn.IsStatusAPIAvailable, sn.MetricsReport,
	)
	return err
}

// UpdateSupernodeProbeData updates only probe-related fields for a supernode.
// This is used by the probe loop to avoid overwriting other fields like ValidatorAddress, CurrentState, etc.
func UpdateSupernodeProbeData(ctx context.Context, pool *pgxpool.Pool, sn SupernodeProbeUpdate) error {
	// Try to update with new columns first
	var sql string
	var args []any

	if sn.IsStatusAPIAvailable {
		// Successful probe: set lastSuccessfulProbe, reset failedProbeCounter, update lastKnownActualVersion
		sql = `UPDATE supernodes SET
			"actualVersion"=COALESCE(NULLIF($2,''),"actualVersion"),
			"cpuUsagePercent"=$3,
			"cpuCores"=$4,
			"memoryTotalGb"=$5,
			"memoryUsedGb"=$6,
			"memoryUsagePercent"=$7,
			"storageTotalBytes"=$8,
			"storageUsedBytes"=$9,
			"storageUsagePercent"=$10,
			"hardwareSummary"=$11,
			"peersCount"=$12,
			"uptimeSeconds"=$13,
			rank=$14,
			"lastStatusCheck"=$15,
			"isStatusApiAvailable"=$16,
			"metricsReport"=$17::jsonb,
			"lastSuccessfulProbe"=$18,
			"failedProbeCounter"=0,
			"lastKnownActualVersion"=COALESCE(NULLIF($2,''),"lastKnownActualVersion"),
			"updatedAt"=now()
		WHERE "supernodeAccount"=$1`
		args = []any{
			sn.SupernodeAccount,
			sn.ActualVersion,
			sn.CPUUsagePercent,
			sn.CPUCores,
			sn.MemoryTotalGb,
			sn.MemoryUsedGb,
			sn.MemoryUsagePercent,
			sn.StorageTotalBytes,
			sn.StorageUsedBytes,
			sn.StorageUsagePercent,
			sn.HardwareSummary,
			sn.PeersCount,
			sn.UptimeSeconds,
			sn.Rank,
			sn.LastStatusCheck,
			sn.IsStatusAPIAvailable,
			sn.MetricsReport,
			sn.ProbeTimeUTC,
		}
	} else {
		// Failed probe: increment failedProbeCounter, do NOT change lastSuccessfulProbe or lastKnownActualVersion
		sql = `UPDATE supernodes SET
			"actualVersion"=COALESCE(NULLIF($2,''),"actualVersion"),
			"cpuUsagePercent"=$3,
			"cpuCores"=$4,
			"memoryTotalGb"=$5,
			"memoryUsedGb"=$6,
			"memoryUsagePercent"=$7,
			"storageTotalBytes"=$8,
			"storageUsedBytes"=$9,
			"storageUsagePercent"=$10,
			"hardwareSummary"=$11,
			"peersCount"=$12,
			"uptimeSeconds"=$13,
			rank=$14,
			"lastStatusCheck"=$15,
			"isStatusApiAvailable"=$16,
			"metricsReport"=$17::jsonb,
			"failedProbeCounter"=COALESCE("failedProbeCounter",0)+1,
			"updatedAt"=now()
		WHERE "supernodeAccount"=$1`
		args = []any{
			sn.SupernodeAccount,
			sn.ActualVersion,
			sn.CPUUsagePercent,
			sn.CPUCores,
			sn.MemoryTotalGb,
			sn.MemoryUsedGb,
			sn.MemoryUsagePercent,
			sn.StorageTotalBytes,
			sn.StorageUsedBytes,
			sn.StorageUsagePercent,
			sn.HardwareSummary,
			sn.PeersCount,
			sn.UptimeSeconds,
			sn.Rank,
			sn.LastStatusCheck,
			sn.IsStatusAPIAvailable,
			sn.MetricsReport,
		}
	}

	_, err := pool.Exec(ctx, sql, args...)
	if err != nil {
		// Check if error is due to missing columns (graceful degradation during rollout)
		errMsg := err.Error()
		if strings.Contains(errMsg, "lastSuccessfulProbe") ||
			strings.Contains(errMsg, "failedProbeCounter") ||
			strings.Contains(errMsg, "lastKnownActualVersion") ||
			strings.Contains(errMsg, "column") && (strings.Contains(errMsg, "does not exist") || strings.Contains(errMsg, "unknown")) {
			log.Printf("Warning: New probe columns not yet available in DB (supernode %s), falling back to old behavior: %v", sn.SupernodeAccount, err)

			// Fallback to old behavior without new columns
			sqlFallback := `UPDATE supernodes SET
				"actualVersion"=COALESCE(NULLIF($2,''),"actualVersion"),
				"cpuUsagePercent"=$3,
				"cpuCores"=$4,
				"memoryTotalGb"=$5,
				"memoryUsedGb"=$6,
				"memoryUsagePercent"=$7,
				"storageTotalBytes"=$8,
				"storageUsedBytes"=$9,
				"storageUsagePercent"=$10,
				"hardwareSummary"=$11,
				"peersCount"=$12,
				"uptimeSeconds"=$13,
				rank=$14,
				"lastStatusCheck"=$15,
				"isStatusApiAvailable"=$16,
				"metricsReport"=$17::jsonb,
				"updatedAt"=now()
			WHERE "supernodeAccount"=$1`
			_, fallbackErr := pool.Exec(ctx, sqlFallback,
				sn.SupernodeAccount,
				sn.ActualVersion,
				sn.CPUUsagePercent,
				sn.CPUCores,
				sn.MemoryTotalGb,
				sn.MemoryUsedGb,
				sn.MemoryUsagePercent,
				sn.StorageTotalBytes,
				sn.StorageUsedBytes,
				sn.StorageUsagePercent,
				sn.HardwareSummary,
				sn.PeersCount,
				sn.UptimeSeconds,
				sn.Rank,
				sn.LastStatusCheck,
				sn.IsStatusAPIAvailable,
				sn.MetricsReport,
			)
			return fallbackErr
		}
		// Return other errors as-is
		return err
	}
	return nil
}

// UpsertAction inserts/updates an action record.
func UpsertAction(ctx context.Context, pool *pgxpool.Pool, a ActionDB) error {
	sql := `INSERT INTO actions ("actionID","creator","actionType","state","blockHeight","priceDenom","priceAmount","expirationTime","metadataRaw","metadataJSON","superNodes","createdAt","updatedAt")
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::jsonb,$11::jsonb,now(),now())
	ON CONFLICT ("actionID") DO UPDATE SET
		"creator"=EXCLUDED."creator",
		"actionType"=EXCLUDED."actionType",
		"state"=EXCLUDED."state",
		"blockHeight"=EXCLUDED."blockHeight",
		"priceDenom"=EXCLUDED."priceDenom",
		"priceAmount"=EXCLUDED."priceAmount",
		"expirationTime"=EXCLUDED."expirationTime",
		"metadataRaw"=EXCLUDED."metadataRaw",
		"metadataJSON"=EXCLUDED."metadataJSON",
		"superNodes"=EXCLUDED."superNodes",
		"updatedAt"=now()`
	_, err := pool.Exec(ctx, sql,
		a.ActionID, a.Creator, a.ActionType, a.State, a.BlockHeight, a.PriceDenom, a.PriceAmount, a.ExpirationTime, a.MetadataRaw, a.MetadataJSON, a.SuperNodes,
	)
	return err
}

// ListKnownSupernodes returns supernode accounts and last known IP/port to probe.
func ListKnownSupernodes(ctx context.Context, pool *pgxpool.Pool) ([]ProbeTarget, error) {
	rows, err := pool.Query(ctx, `SELECT "supernodeAccount","ipAddress","p2pPort" FROM supernodes`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ProbeTarget
	for rows.Next() {
		var t ProbeTarget
		if err := rows.Scan(&t.SupernodeAccount, &t.IPAddress, &t.P2PPort); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func ListSupernodeMetricsFiltered(ctx context.Context, pool *pgxpool.Pool, f SupernodeMetricsFilter) ([]SupernodeDB, bool, error) {
	return listSupernodeMetricsFiltered(ctx, pool, f, true)
}

func listSupernodeMetricsFiltered(ctx context.Context, pool *pgxpool.Pool, f SupernodeMetricsFilter, includeMinFailed bool) ([]SupernodeDB, bool, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = 1
	}

	var (
		sb         strings.Builder
		conditions []string
		args       []any
		argPos     = 1
	)

	sb.WriteString(`SELECT "supernodeAccount","validatorAddress","validatorMoniker","currentState","currentStateHeight","ipAddress","p2pPort","protocolVersion","actualVersion","cpuUsagePercent","cpuCores","memoryTotalGb","memoryUsedGb","memoryUsagePercent","storageTotalBytes","storageUsedBytes","storageUsagePercent","hardwareSummary","peersCount","uptimeSeconds",rank,"registeredServices","runningTasks","stateHistory",evidence,"prevIpAddresses","lastStatusCheck","isStatusApiAvailable","metricsReport","lastSuccessfulProbe","failedProbeCounter",COALESCE("lastKnownActualVersion",'')
		FROM supernodes`)

	// Legacy CurrentState filter for "running"/"stopped"/"any"
	switch f.CurrentState {
	case "running":
		conditions = append(conditions, `"currentState" != 'SUPERNODE_STATE_STOPPED'`)
	case "stopped":
		conditions = append(conditions, `"currentState" = 'SUPERNODE_STATE_STOPPED'`)
	}

	// New ChainState filter for exact currentState enum values
	if f.ChainState != nil {
		conditions = append(conditions, fmt.Sprintf(`"currentState" = $%d`, argPos))
		args = append(args, *f.ChainState)
		argPos++
	}

	// Status filter: "available" now means all 3 ports are open
	switch f.Status {
	case "available":
		// Filter for supernodes where all 3 ports are available:
		// 1. status API (8002) is available - stored in isStatusApiAvailable column
		// 2. port1 (from ipAddress) is open - stored in metricsReport->'ports'->>'port1'
		// 3. p2p port (4445) is open - stored in metricsReport->'ports'->>'p2p'
		conditions = append(conditions, `"isStatusApiAvailable" = true`)
		conditions = append(conditions, `"metricsReport"->'ports'->>'port1' = 'true'`)
		conditions = append(conditions, `"metricsReport"->'ports'->>'p2p' = 'true'`)
	case "unavailable":
		// Unavailable means at least one of the 3 ports is not open
		conditions = append(conditions, `("isStatusApiAvailable" = false OR "metricsReport"->'ports'->>'port1' != 'true' OR "metricsReport"->'ports'->>'p2p' != 'true')`)
	}

	if f.Version != nil {
		conditions = append(conditions, fmt.Sprintf(`COALESCE(NULLIF("lastKnownActualVersion", ''), NULLIF("actualVersion", '')) = $%d`, argPos))
		args = append(args, *f.Version)
		argPos++
	}

	if includeMinFailed {
		conditions = append(conditions, fmt.Sprintf(`"failedProbeCounter" >= $%d`, argPos))
		args = append(args, f.MinFailed)
		argPos++
	}

	if f.CursorAccount != nil {
		conditions = append(conditions, fmt.Sprintf(`"supernodeAccount" > $%d`, argPos))
		args = append(args, *f.CursorAccount)
		argPos++
	}

	if len(conditions) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(conditions, " AND "))
	}

	sb.WriteString(` ORDER BY "supernodeAccount" ASC`)
	sb.WriteString(fmt.Sprintf(" LIMIT $%d", argPos))
	args = append(args, limit+1)

	query := sb.String()
	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		if includeMinFailed {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.SQLState() == "42703" {
				log.Printf("Warning: failedProbeCounter column not available, retrying without filter: %v", err)
				return listSupernodeMetricsFiltered(ctx, pool, f, false)
			}
		}
		return nil, false, err
	}
	defer rows.Close()

	results := make([]SupernodeDB, 0, limit+1)
	for rows.Next() {
		var sn SupernodeDB
		if err := rows.Scan(
			&sn.SupernodeAccount,
			&sn.ValidatorAddress,
			&sn.ValidatorMoniker,
			&sn.CurrentState,
			&sn.CurrentStateHeight,
			&sn.IPAddress,
			&sn.P2PPort,
			&sn.ProtocolVersion,
			&sn.ActualVersion,
			&sn.CPUUsagePercent,
			&sn.CPUCores,
			&sn.MemoryTotalGb,
			&sn.MemoryUsedGb,
			&sn.MemoryUsagePercent,
			&sn.StorageTotalBytes,
			&sn.StorageUsedBytes,
			&sn.StorageUsagePercent,
			&sn.HardwareSummary,
			&sn.PeersCount,
			&sn.UptimeSeconds,
			&sn.Rank,
			&sn.RegisteredServices,
			&sn.RunningTasks,
			&sn.StateHistory,
			&sn.Evidence,
			&sn.PrevIPAddresses,
			&sn.LastStatusCheck,
			&sn.IsStatusAPIAvailable,
			&sn.MetricsReport,
			&sn.LastSuccessfulProbe,
			&sn.FailedProbeCounter,
			&sn.LastKnownActualVersion,
		); err != nil {
			return nil, false, err
		}
		results = append(results, sn)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}

	hasMore := false
	if len(results) > limit {
		hasMore = true
		results = results[:limit]
	}

	return results, hasMore, nil
}

// ListUnavailableSupernodes returns supernodes where isStatusApiAvailable=false,
// optionally filtered by currentState: "running" (default, excludes STOPPED),
// "stopped" (only STOPPED), or "any" (no state filter).
func ListUnavailableSupernodes(ctx context.Context, pool *pgxpool.Pool, stateFilter string) ([]SupernodeDB, error) {
	var query string
	switch stateFilter {
	case "stopped":
		query = `SELECT "supernodeAccount","validatorAddress","validatorMoniker","currentState","currentStateHeight","ipAddress","p2pPort","protocolVersion","actualVersion","cpuUsagePercent","cpuCores","memoryTotalGb","memoryUsedGb","memoryUsagePercent","storageTotalBytes","storageUsedBytes","storageUsagePercent","hardwareSummary","peersCount","uptimeSeconds",rank,"registeredServices","runningTasks","stateHistory",evidence,"prevIpAddresses","lastStatusCheck","isStatusApiAvailable","metricsReport"
			FROM supernodes
			WHERE "isStatusApiAvailable" = false AND "currentState" = 'SUPERNODE_STATE_STOPPED'`
	case "any":
		query = `SELECT "supernodeAccount","validatorAddress","validatorMoniker","currentState","currentStateHeight","ipAddress","p2pPort","protocolVersion","actualVersion","cpuUsagePercent","cpuCores","memoryTotalGb","memoryUsedGb","memoryUsagePercent","storageTotalBytes","storageUsedBytes","storageUsagePercent","hardwareSummary","peersCount","uptimeSeconds",rank,"registeredServices","runningTasks","stateHistory",evidence,"prevIpAddresses","lastStatusCheck","isStatusApiAvailable","metricsReport"
			FROM supernodes
			WHERE "isStatusApiAvailable" = false`
	default: // "running" is default
		query = `SELECT "supernodeAccount","validatorAddress","validatorMoniker","currentState","currentStateHeight","ipAddress","p2pPort","protocolVersion","actualVersion","cpuUsagePercent","cpuCores","memoryTotalGb","memoryUsedGb","memoryUsagePercent","storageTotalBytes","storageUsedBytes","storageUsagePercent","hardwareSummary","peersCount","uptimeSeconds",rank,"registeredServices","runningTasks","stateHistory",evidence,"prevIpAddresses","lastStatusCheck","isStatusApiAvailable","metricsReport"
			FROM supernodes
			WHERE "isStatusApiAvailable" = false AND "currentState" != 'SUPERNODE_STATE_STOPPED'`
	}

	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SupernodeDB
	for rows.Next() {
		var sn SupernodeDB
		if err := rows.Scan(
			&sn.SupernodeAccount,
			&sn.ValidatorAddress,
			&sn.ValidatorMoniker,
			&sn.CurrentState,
			&sn.CurrentStateHeight,
			&sn.IPAddress,
			&sn.P2PPort,
			&sn.ProtocolVersion,
			&sn.ActualVersion,
			&sn.CPUUsagePercent,
			&sn.CPUCores,
			&sn.MemoryTotalGb,
			&sn.MemoryUsedGb,
			&sn.MemoryUsagePercent,
			&sn.StorageTotalBytes,
			&sn.StorageUsedBytes,
			&sn.StorageUsagePercent,
			&sn.HardwareSummary,
			&sn.PeersCount,
			&sn.UptimeSeconds,
			&sn.Rank,
			&sn.RegisteredServices,
			&sn.RunningTasks,
			&sn.StateHistory,
			&sn.Evidence,
			&sn.PrevIPAddresses,
			&sn.LastStatusCheck,
			&sn.IsStatusAPIAvailable,
			&sn.MetricsReport,
		); err != nil {
			return nil, err
		}
		out = append(out, sn)
	}
	return out, rows.Err()
}

// Data structs used by DB helpers

type SupernodeDB struct {
	SupernodeAccount       string
	ValidatorAddress       string
	ValidatorMoniker       string
	CurrentState           string
	CurrentStateHeight     string
	IPAddress              string
	P2PPort                int32
	ProtocolVersion        string
	ActualVersion          string
	CPUUsagePercent        *float64
	CPUCores               *int32
	MemoryTotalGb          *float64
	MemoryUsedGb           *float64
	MemoryUsagePercent     *float64
	StorageTotalBytes      *int64
	StorageUsedBytes       *int64
	StorageUsagePercent    *float64
	HardwareSummary        *string
	PeersCount             *int32
	UptimeSeconds          *int64
	Rank                   *int32
	RegisteredServices     any
	RunningTasks           any
	StateHistory           any
	Evidence               any
	PrevIPAddresses        any
	LastStatusCheck        *time.Time
	IsStatusAPIAvailable   bool
	MetricsReport          any
	LastSuccessfulProbe    *time.Time
	FailedProbeCounter     int32
	LastKnownActualVersion string
}

type SupernodeMetricsFilter struct {
	CurrentState  string   // "running", "stopped", "any" - legacy filter on running state
	ChainState    *string  // New: exact match on currentState column (e.g., "SUPERNODE_STATE_ACTIVE")
	Status        string   // "available" (all 3 ports), "unavailable", "any"
	Version       *string
	MinFailed     int
	Limit         int
	CursorAccount *string
}

type ActionDB struct {
	ActionID       string
	Creator        string
	ActionType     string
	State          string
	BlockHeight    int64
	PriceDenom     string
	PriceAmount    string
	ExpirationTime int64
	MetadataRaw    []byte
	MetadataJSON   any
	SuperNodes     any
	CreatedAt      time.Time
}

type ActionsFilter struct {
	Type       *string
	Creator    *string
	State      *string
	FromHeight *int64
	ToHeight   *int64
	Limit      int
	CursorTS   *time.Time
	CursorID   *string
}

type ProbeTarget struct {
	SupernodeAccount string
	IPAddress        string
	P2PPort          int32
}

type SupernodeProbeUpdate struct {
	SupernodeAccount     string
	ActualVersion        string
	CPUUsagePercent      *float64
	CPUCores             *int32
	MemoryTotalGb        *float64
	MemoryUsedGb         *float64
	MemoryUsagePercent   *float64
	StorageTotalBytes    *int64
	StorageUsedBytes     *int64
	StorageUsagePercent  *float64
	HardwareSummary      *string
	PeersCount           *int32
	UptimeSeconds        *int64
	Rank                 *int32
	LastStatusCheck      *time.Time
	IsStatusAPIAvailable bool
	MetricsReport        any
	ProbeTimeUTC         time.Time // Used for lastSuccessfulProbe when successful
}

// ListAllActions fetches all actions from the database ordered by block height descending
func ListAllActions(ctx context.Context, pool *pgxpool.Pool) ([]ActionDB, error) {
	query := `SELECT "actionID","creator","actionType","state","blockHeight","priceDenom","priceAmount","expirationTime","metadataRaw","metadataJSON","superNodes","createdAt"
 	FROM actions
 	ORDER BY "blockHeight" DESC`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var actions []ActionDB
	for rows.Next() {
		var a ActionDB
		if err := rows.Scan(
			&a.ActionID,
			&a.Creator,
			&a.ActionType,
			&a.State,
			&a.BlockHeight,
			&a.PriceDenom,
			&a.PriceAmount,
			&a.ExpirationTime,
			&a.MetadataRaw,
			&a.MetadataJSON,
			&a.SuperNodes,
			&a.CreatedAt,
		); err != nil {
			return nil, err
		}
		actions = append(actions, a)
	}
	return actions, rows.Err()
}

// ListActionsFiltered applies filters and keyset pagination to list actions.
func ListActionsFiltered(ctx context.Context, pool *pgxpool.Pool, f ActionsFilter) ([]ActionDB, bool, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = 1
	}

	var (
		sb         strings.Builder
		conditions []string
		args       []any
		argPos     = 1
	)

	sb.WriteString(`SELECT 
						"actionID","creator","actionType","state","blockHeight",
						"priceDenom","priceAmount","expirationTime","metadataRaw","metadataJSON",
						"superNodes","createdAt" 
					FROM actions`)

	if f.Type != nil {
		conditions = append(conditions, fmt.Sprintf(`"actionType" = $%d`, argPos))
		args = append(args, *f.Type)
		argPos++
	}
	if f.Creator != nil {
		conditions = append(conditions, fmt.Sprintf(`"creator" = $%d`, argPos))
		args = append(args, *f.Creator)
		argPos++
	}
	if f.State != nil {
		conditions = append(conditions, fmt.Sprintf(`"state" = $%d`, argPos))
		args = append(args, *f.State)
		argPos++
	}
	if f.FromHeight != nil {
		conditions = append(conditions, fmt.Sprintf(`"blockHeight" >= $%d`, argPos))
		args = append(args, *f.FromHeight)
		argPos++
	}
	if f.ToHeight != nil {
		conditions = append(conditions, fmt.Sprintf(`"blockHeight" <= $%d`, argPos))
		args = append(args, *f.ToHeight)
		argPos++
	}
	if f.CursorTS != nil && f.CursorID != nil {
		conditions = append(conditions, fmt.Sprintf(`("createdAt" < $%d OR ("createdAt" = $%d AND "actionID" < $%d))`, argPos, argPos, argPos+1))
		args = append(args, *f.CursorTS, *f.CursorID)
		argPos += 2
	}

	if len(conditions) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(conditions, " AND "))
	}

	sb.WriteString(` ORDER BY "createdAt" DESC, "actionID" DESC`)
	sb.WriteString(fmt.Sprintf(" LIMIT $%d", argPos))
	args = append(args, limit+1)

	rows, err := pool.Query(ctx, sb.String(), args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	actions := make([]ActionDB, 0, limit+1)
	for rows.Next() {
		var a ActionDB
		if err := rows.Scan(
			&a.ActionID,
			&a.Creator,
			&a.ActionType,
			&a.State,
			&a.BlockHeight,
			&a.PriceDenom,
			&a.PriceAmount,
			&a.ExpirationTime,
			&a.MetadataRaw,
			&a.MetadataJSON,
			&a.SuperNodes,
			&a.CreatedAt,
		); err != nil {
			return nil, false, err
		}
		actions = append(actions, a)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}

	hasMore := false
	if len(actions) > limit {
		hasMore = true
		actions = actions[:limit]
	}

	return actions, hasMore, nil
}

// GetActionByID fetches a single action by ID from the database
func GetActionByID(ctx context.Context, pool *pgxpool.Pool, actionID string) (ActionDB, error) {
	query := `SELECT "actionID","creator","actionType","state","blockHeight","priceDenom","priceAmount","expirationTime","metadataRaw","metadataJSON","superNodes","createdAt"
		FROM actions
		WHERE "actionID" = $1`

	var a ActionDB
	err := pool.QueryRow(ctx, query, actionID).Scan(
		&a.ActionID,
		&a.Creator,
		&a.ActionType,
		&a.State,
		&a.BlockHeight,
		&a.PriceDenom,
		&a.PriceAmount,
		&a.ExpirationTime,
		&a.MetadataRaw,
		&a.MetadataJSON,
		&a.SuperNodes,
		&a.CreatedAt,
	)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return ActionDB{}, ErrNotFound
		}
		return ActionDB{}, err
	}
	return a, nil
}

// GetSupernodeByID fetches a single supernode by supernodeAccount from the database
func GetSupernodeByID(ctx context.Context, pool *pgxpool.Pool, supernodeAccount string) (SupernodeDB, error) {
	query := `SELECT "supernodeAccount","validatorAddress","validatorMoniker","currentState","currentStateHeight",
			"ipAddress","p2pPort","protocolVersion","actualVersion",
			"cpuUsagePercent","cpuCores","memoryTotalGb","memoryUsedGb","memoryUsagePercent",
			"storageTotalBytes","storageUsedBytes","storageUsagePercent","hardwareSummary",
			"peersCount","uptimeSeconds",
			rank,"registeredServices","runningTasks",
			"stateHistory",evidence,
			"prevIpAddresses",
			"lastStatusCheck","isStatusApiAvailable",
			"metricsReport",
			"lastSuccessfulProbe","failedProbeCounter",COALESCE("lastKnownActualVersion",'')

		FROM supernodes
		WHERE "supernodeAccount" = $1`

	var sn SupernodeDB
	err := pool.QueryRow(ctx, query, supernodeAccount).Scan(
		&sn.SupernodeAccount,
		&sn.ValidatorAddress,
		&sn.ValidatorMoniker,
		&sn.CurrentState,
		&sn.CurrentStateHeight,
		&sn.IPAddress,
		&sn.P2PPort,
		&sn.ProtocolVersion,
		&sn.ActualVersion,
		&sn.CPUUsagePercent,
		&sn.CPUCores,
		&sn.MemoryTotalGb,
		&sn.MemoryUsedGb,
		&sn.MemoryUsagePercent,
		&sn.StorageTotalBytes,
		&sn.StorageUsedBytes,
		&sn.StorageUsagePercent,
		&sn.HardwareSummary,
		&sn.PeersCount,
		&sn.UptimeSeconds,
		&sn.Rank,
		&sn.RegisteredServices,
		&sn.RunningTasks,
		&sn.StateHistory,
		&sn.Evidence,
		&sn.PrevIPAddresses,
		&sn.LastStatusCheck,
		&sn.IsStatusAPIAvailable,
		&sn.MetricsReport,
		&sn.LastSuccessfulProbe,
		&sn.FailedProbeCounter,
		&sn.LastKnownActualVersion,
	)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return SupernodeDB{}, ErrNotFound
		}
		return SupernodeDB{}, err
	}
	return sn, nil
}

// VersionRow represents aggregated version statistics
type VersionRow struct {
	Version     string
	Total       int
	Available   int
	Unavailable int
}

// ListVersionMatrix returns aggregated version statistics from supernodes.
// It groups by COALESCE(lastKnownActualVersion, actualVersion) when available,
// falling back to actualVersion only if lastKnownActualVersion column doesn't exist.
func ListVersionMatrix(ctx context.Context, pool *pgxpool.Pool) ([]VersionRow, error) {
	// Try query with new columns first (lastKnownActualVersion)
	query := `SELECT
		COALESCE(NULLIF("lastKnownActualVersion", ''), NULLIF("actualVersion", ''), 'unknown') AS version,
		COUNT(*) AS total,
		COUNT(*) FILTER (WHERE "isStatusApiAvailable" = true) AS available,
		COUNT(*) FILTER (WHERE "isStatusApiAvailable" = false) AS unavailable
	FROM supernodes
	WHERE COALESCE(NULLIF("lastKnownActualVersion", ''), NULLIF("actualVersion", ''), 'unknown') != 'unknown'
	GROUP BY version
	ORDER BY total DESC`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		// Check if error is due to missing lastKnownActualVersion column
		errMsg := err.Error()
		if strings.Contains(errMsg, "lastKnownActualVersion") ||
			(strings.Contains(errMsg, "column") && strings.Contains(errMsg, "does not exist")) {
			// Fallback to query without lastKnownActualVersion
			queryFallback := `SELECT
				COALESCE(NULLIF("actualVersion", ''), 'unknown') AS version,
				COUNT(*) AS total,
				COUNT(*) FILTER (WHERE "isStatusApiAvailable" = true) AS available,
				COUNT(*) FILTER (WHERE "isStatusApiAvailable" = false) AS unavailable
			FROM supernodes
			WHERE COALESCE(NULLIF("actualVersion", ''), 'unknown') != 'unknown'
			GROUP BY version
			ORDER BY total DESC`

			rows, err = pool.Query(ctx, queryFallback)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	defer rows.Close()

	var result []VersionRow
	for rows.Next() {
		var vr VersionRow
		if err := rows.Scan(&vr.Version, &vr.Total, &vr.Available, &vr.Unavailable); err != nil {
			return nil, err
		}
		result = append(result, vr)
	}
	return result, rows.Err()
}

// ErrNotFound sentinel
var ErrNotFound = errors.New("not found")

// HardwareStats holds aggregated hardware statistics for available supernodes
type HardwareStats struct {
	TotalCPUCores       int64 `json:"total_cpu_cores"`
	TotalMemoryGb       float64 `json:"total_memory_gb"`
	TotalStorageBytes   int64 `json:"total_storage_bytes"`
	UsedStorageBytes    int64 `json:"used_storage_bytes"`
	AvailableSupernodes int64 `json:"available_supernodes"`
}

// GetAggregatedHardwareStats returns aggregated hardware statistics for fully available supernodes.
// A supernode is considered "fully available" when:
// 1. isStatusApiAvailable = true (status API port 8002 is open)
// 2. metricsReport->'ports'->>'port1' = 'true' (port1 from ipAddress is open)
// 3. metricsReport->'ports'->>'p2p' = 'true' (P2P port 4445 is open)
// 4. currentState != 'SUPERNODE_STATE_STOPPED' (node is not stopped on-chain)
func GetAggregatedHardwareStats(ctx context.Context, pool *pgxpool.Pool) (*HardwareStats, error) {
	query := `SELECT
		COALESCE(SUM("cpuCores"), 0) AS total_cpu_cores,
		COALESCE(SUM("memoryTotalGb"), 0) AS total_memory_gb,
		COALESCE(SUM("storageTotalBytes"), 0) AS total_storage_bytes,
		COALESCE(SUM("storageUsedBytes"), 0) AS used_storage_bytes,
		COUNT(*) AS available_supernodes
	FROM supernodes
	WHERE "isStatusApiAvailable" = true
		AND "metricsReport"->'ports'->>'port1' = 'true'
		AND "metricsReport"->'ports'->>'p2p' = 'true'
		AND "currentState" != 'SUPERNODE_STATE_STOPPED'`

	var stats HardwareStats
	err := pool.QueryRow(ctx, query).Scan(
		&stats.TotalCPUCores,
		&stats.TotalMemoryGb,
		&stats.TotalStorageBytes,
		&stats.UsedStorageBytes,
		&stats.AvailableSupernodes,
	)
	if err != nil {
		return nil, err
	}
	return &stats, nil
}
