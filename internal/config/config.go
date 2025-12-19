package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds runtime configuration for the API server.
// Values are sourced from environment variables with sensible defaults.
// This base config intentionally keeps things minimal and read-only.
type Config struct {
	Port              string
	AllowOrigins      []string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	RequestTimeout    time.Duration

	// DB
	DB_DSN      string
	DB_MaxConns int32

	// Lumera chain REST API
	LumeraAPIBase string
	HTTPTimeout   time.Duration

	// Background intervals
	ValidatorsSyncInterval    time.Duration
	SupernodesSyncInterval    time.Duration
	ActionsSyncInterval       time.Duration
	ProbeInterval             time.Duration
	DialTimeout               time.Duration
	ActionTxEnricherInterval  time.Duration
	ActionEnricherStartID     uint64

	// Feature flags
	EnableSyncEndpoint bool
}

func Load() Config {
	// Load .env file if it exists (ignore error if file doesn't exist)
	if err := godotenv.Load(); err != nil {
		log.Printf("Note: .env file not found or could not be loaded (will use environment variables or defaults): %v", err)
	}

	port := getenv("PORT", "18080")
	origins := splitAndClean(getenv("CORS_ALLOW_ORIGINS", "*"))

	return Config{
		Port:              port,
		AllowOrigins:      origins,
		ReadHeaderTimeout: durationEnv("READ_HEADER_TIMEOUT", 5*time.Second),
		ReadTimeout:       durationEnv("READ_TIMEOUT", 30*time.Second),
		WriteTimeout:      durationEnv("WRITE_TIMEOUT", 30*time.Second),
		IdleTimeout:       durationEnv("IDLE_TIMEOUT", 120*time.Second),
		RequestTimeout:    durationEnv("REQUEST_TIMEOUT", 10*time.Second),

		DB_DSN:      getenv("DB_DSN", "postgres://postgres:postgres@localhost:5432/lumescope?sslmode=disable"),
		DB_MaxConns: int32Env("DB_MAX_CONNS", 10),

		LumeraAPIBase: getenv("LUMERA_API_BASE", "http://localhost:1317"),
		HTTPTimeout:   durationEnv("HTTP_TIMEOUT", 30*time.Second),

		ValidatorsSyncInterval:   durationEnv("VALIDATORS_SYNC_INTERVAL", 5*time.Minute),
		SupernodesSyncInterval:   durationEnv("SUPERNODES_SYNC_INTERVAL", 2*time.Minute),
		ActionsSyncInterval:      durationEnv("ACTIONS_SYNC_INTERVAL", 30*time.Second),
		ProbeInterval:            durationEnv("PROBE_INTERVAL", 1*time.Minute),
		DialTimeout:              durationEnv("DIAL_TIMEOUT", 2*time.Second),
		ActionTxEnricherInterval: durationEnv("ACTION_TX_ENRICHER_INTERVAL", 10*time.Second),
		ActionEnricherStartID:    uint64Env("ACTION_ENRICHER_START_ID", 0),

		EnableSyncEndpoint: boolEnv("ENABLE_SYNC_ENDPOINT", false),
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func int32Env(key string, def int32) int32 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return int32(n)
		}
	}
	return def
}

func durationEnv(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func boolEnv(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func uint64Env(key string, def uint64) uint64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			return n
		}
	}
	return def
}

func splitAndClean(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return []string{"*"}
	}
	return out
}
