package db

import (
	"context"
	"errors"
	"fmt"
	"time"

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
func Close(pool *pgxpool.Pool) { if pool != nil { pool.Close() } }

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
			"protocolVersion"      VARCHAR(32) NOT NULL DEFAULT '1.0.0',
			"actualVersion"        VARCHAR(32),
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
			"createdAt"            TIMESTAMP NOT NULL DEFAULT now(),
			"updatedAt"            TIMESTAMP NOT NULL DEFAULT now()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_supernodes_validator_address ON supernodes ("validatorAddress")`,
		`CREATE INDEX IF NOT EXISTS idx_supernodes_supernode_account ON supernodes ("supernodeAccount")`,
		`CREATE INDEX IF NOT EXISTS idx_supernodes_current_state ON supernodes ("currentState")`,
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
			"createdAt"     TIMESTAMP NOT NULL DEFAULT now(),
			"updatedAt"     TIMESTAMP NOT NULL DEFAULT now()
		)`,
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
		"validatorMoniker"=EXCLUDED."validatorMoniker",
		"currentState"=EXCLUDED."currentState",
		"currentStateHeight"=EXCLUDED."currentStateHeight",
		"ipAddress"=EXCLUDED."ipAddress",
		"p2pPort"=EXCLUDED."p2pPort",
		"protocolVersion"=EXCLUDED."protocolVersion",
		"actualVersion"=EXCLUDED."actualVersion",
		"cpuUsagePercent"=EXCLUDED."cpuUsagePercent",
		"cpuCores"=EXCLUDED."cpuCores",
		"memoryTotalGb"=EXCLUDED."memoryTotalGb",
		"memoryUsedGb"=EXCLUDED."memoryUsedGb",
		"memoryUsagePercent"=EXCLUDED."memoryUsagePercent",
		"storageTotalBytes"=EXCLUDED."storageTotalBytes",
		"storageUsedBytes"=EXCLUDED."storageUsedBytes",
		"storageUsagePercent"=EXCLUDED."storageUsagePercent",
		"hardwareSummary"=EXCLUDED."hardwareSummary",
		"peersCount"=EXCLUDED."peersCount",
		"uptimeSeconds"=EXCLUDED."uptimeSeconds",
		rank=EXCLUDED.rank,
		"registeredServices"=EXCLUDED."registeredServices",
		"runningTasks"=EXCLUDED."runningTasks",
		"stateHistory"=EXCLUDED."stateHistory",
		evidence=EXCLUDED.evidence,
		"prevIpAddresses"=EXCLUDED."prevIpAddresses",
		"lastStatusCheck"=EXCLUDED."lastStatusCheck",
		"isStatusApiAvailable"=EXCLUDED."isStatusApiAvailable",
		"metricsReport"=EXCLUDED."metricsReport",
		"updatedAt"=now()`
	_, err := pool.Exec(ctx, sql,
		sn.SupernodeAccount, sn.ValidatorAddress, sn.ValidatorMoniker, sn.CurrentState, sn.CurrentStateHeight, sn.IPAddress, sn.P2PPort, sn.ProtocolVersion, sn.ActualVersion, sn.CPUUsagePercent, sn.CPUCores, sn.MemoryTotalGb, sn.MemoryUsedGb, sn.MemoryUsagePercent, sn.StorageTotalBytes, sn.StorageUsedBytes, sn.StorageUsagePercent, sn.HardwareSummary, sn.PeersCount, sn.UptimeSeconds, sn.Rank, sn.RegisteredServices, sn.RunningTasks, sn.StateHistory, sn.Evidence, sn.PrevIPAddresses, sn.LastStatusCheck, sn.IsStatusAPIAvailable, sn.MetricsReport,
	)
	return err
}

// UpsertAction inserts/updates an action record.
func UpsertAction(ctx context.Context, pool *pgxpool.Pool, a ActionDB) error {
	sql := `INSERT INTO actions ("actionID","creator","actionType","state","blockHeight","priceDenom","priceAmount","expirationTime","metadataRaw","metadataJSON","createdAt","updatedAt")
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::jsonb,now(),now())
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
		"updatedAt"=now()`
	_, err := pool.Exec(ctx, sql,
		a.ActionID, a.Creator, a.ActionType, a.State, a.BlockHeight, a.PriceDenom, a.PriceAmount, a.ExpirationTime, a.MetadataRaw, a.MetadataJSON,
	)
	return err
}

// ListKnownSupernodes returns supernode accounts and last known IP/port to probe.
func ListKnownSupernodes(ctx context.Context, pool *pgxpool.Pool) ([]ProbeTarget, error) {
	rows, err := pool.Query(ctx, `SELECT "supernodeAccount","ipAddress","p2pPort" FROM supernodes`)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []ProbeTarget
	for rows.Next() {
		var t ProbeTarget
		if err := rows.Scan(&t.SupernodeAccount, &t.IPAddress, &t.P2PPort); err != nil { return nil, err }
		out = append(out, t)
	}
	return out, rows.Err()
}

// Data structs used by DB helpers

type SupernodeDB struct {
	SupernodeAccount   string
	ValidatorAddress   string
	ValidatorMoniker   string
	CurrentState       string
	CurrentStateHeight string
	IPAddress          string
	P2PPort            int32
	ProtocolVersion    string
	ActualVersion      string
	CPUUsagePercent    *float64
	CPUCores           *int32
	MemoryTotalGb      *float64
	MemoryUsedGb       *float64
	MemoryUsagePercent *float64
	StorageTotalBytes  *int64
	StorageUsedBytes   *int64
	StorageUsagePercent *float64
	HardwareSummary    *string
	PeersCount         *int32
	UptimeSeconds      *int64
	Rank               *int32
	RegisteredServices any
	RunningTasks       any
	StateHistory       any
	Evidence           any
	PrevIPAddresses    any
	LastStatusCheck    *time.Time
	IsStatusAPIAvailable bool
	MetricsReport      any
}

type ActionDB struct {
	ActionID      string
	Creator       string
	ActionType    string
	State         string
	BlockHeight   int64
	PriceDenom    string
	PriceAmount   string
	ExpirationTime int64
	MetadataRaw   []byte
	MetadataJSON  any
}

type ProbeTarget struct {
	SupernodeAccount string
	IPAddress        string
	P2PPort          int32
}

// ErrNotFound sentinel
var ErrNotFound = errors.New("not found")
