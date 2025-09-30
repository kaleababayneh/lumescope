# LumeScope (Lumera InfoServer)

LumeScope is a read‑only aggregation and indexing service for the Lumera network. It centralizes heavy parsing and high‑fanout reads into one neutral service so client apps stay lightweight and responsive.

Current status: preview skeleton with working HTTP server, configuration, DB bootstrap, Lumera REST client, and background sync/probing.


## What it does (today)
- Stdlib‑only HTTP API server (net/http + ServeMux), no third‑party web frameworks.
- Health endpoints: `/healthz` (liveness) and `/readyz` (readiness stub).
- Stubbed v1 endpoints (wired for future DB reads):
  - `GET /v1/actions` (filters/pagination placeholders, ETag/Last‑Modified support)
  - `GET /v1/actions/{id}` (joined view placeholder)
  - `GET /v1/supernodes/metrics` (rollup placeholder)
  - `GET /v1/version-matrix` (LEP2 compatibility placeholder)
- Database support (PostgreSQL via pgx). Bootstrap creates the core tables/indexes automatically at startup.
- Lumera REST client (Cosmos SDK API) for:
  - Validators list (active/inactive/jailed)
  - Supernodes list
  - Actions list (with metadata decoding to JSON using Lumera protobuf types)
- Background workers:
  - Periodically fetch validators/supernodes/actions from Lumera and upsert into DB.
  - Probe supernode ports 4444/4445 and call status API on 8002, then persist metrics.


## Prerequisites
- Go 1.24+
- PostgreSQL 13+ (tested with 14+)
- Network access to a Lumera REST (LCD) endpoint (default `http://localhost:1317`)
- Optional: outbound access to supernodes on ports 4444, 4445, and 8002 for probing


## Quick start (TL;DR)
1) Start PostgreSQL and create a database:

```bash
createdb lumescope || true
# or via psql:
# psql -U postgres -c 'CREATE DATABASE lumescope;'
```

2) Build and run:

```bash
# From repo root
go build -o bin/lumescope ./cmd/lumescope

# Minimal run with defaults
./bin/lumescope
```

3) Verify:

```bash
curl -i http://localhost:18080/healthz
curl -i http://localhost:18080/readyz
curl -s http://localhost:18080/v1/actions | jq .
```

The server bootstraps DB tables automatically on first start.


## Configuration
All settings are provided via environment variables. Defaults are shown in parentheses.

- PORT (18080) — HTTP listen port.
- CORS_ALLOW_ORIGINS (*) — Comma‑separated list of allowed origins for CORS; use `*` to allow all.

- READ_HEADER_TIMEOUT (5s)
- READ_TIMEOUT (30s)
- WRITE_TIMEOUT (30s)
- IDLE_TIMEOUT (120s)
- REQUEST_TIMEOUT (10s) — Per‑request server‑side timeout.

Database:
- DB_DSN (postgres://postgres:postgres@localhost:5432/lumescope?sslmode=disable)
- DB_MAX_CONNS (10)

Lumera chain REST (LCD):
- LUMERA_API_BASE (http://localhost:1317)
- HTTP_TIMEOUT (10s) — Outbound HTTP timeout for Lumera calls.

Background workers:
- VALIDATORS_SYNC_INTERVAL (5m)
- SUPERNODES_SYNC_INTERVAL (2m)
- ACTIONS_SYNC_INTERVAL (30s)
- PROBE_INTERVAL (1m) — Port/status probing cadence for supernodes.
- DIAL_TIMEOUT (2s) — TCP dial timeout for port probes.

Example full run:

```bash
export PORT=18080
export DB_DSN="postgres://postgres:postgres@127.0.0.1:5432/lumescope?sslmode=disable"
export DB_MAX_CONNS=20
export LUMERA_API_BASE="http://localhost:1317"
export HTTP_TIMEOUT=10s
export VALIDATORS_SYNC_INTERVAL=5m
export SUPERNODES_SYNC_INTERVAL=2m
export ACTIONS_SYNC_INTERVAL=30s
export PROBE_INTERVAL=1m
export DIAL_TIMEOUT=2s
export CORS_ALLOW_ORIGINS="*"

./bin/lumescope
```


## Build and run
- Build:

```bash
go build -o bin/lumescope ./cmd/lumescope
```

- Run:

```bash
./bin/lumescope
```

- Or run directly:

```bash
go run ./cmd/lumescope
```


## Database notes
- The application automatically creates required tables and indexes on startup (bootstrap step).
- If using a non‑default Postgres setup, adjust `DB_DSN` accordingly. Examples:
  - Local trust auth: `postgres://localhost/lumescope?sslmode=disable`
  - With password: `postgres://user:pass@localhost:5432/lumescope?sslmode=disable`
- Ensure the configured user has privileges to create tables/indexes in the database.


## Endpoints (preview)
- `GET /healthz` — liveness (always 200 while process is healthy)
- `GET /readyz` — readiness (currently returns 200; replace with real checks later)
- `GET /v1/actions` — list (stubbed data; supports ETag/Last‑Modified headers)
- `GET /v1/actions/{id}` — details (stubbed data)
- `GET /v1/supernodes/metrics` — aggregated metrics (stubbed data)
- `GET /v1/version-matrix` — chain action versions, SN capabilities, compatibility (stubbed data)

Note: Background workers already fetch data into the DB. API handlers are currently placeholders and will be wired to the DB in subsequent work.


## Background tasks
The scheduler runs multiple jobs on independent intervals:
- Validators sync — pulls validators from Lumera LCD and caches monikers.
- Supernodes sync — pulls supernode list, joins validator info, persists state/history/metrics.
- Actions sync — pulls actions and decodes the Protobuf metadata (Sense/Cascade) into JSON for storage.
- Supernode probe — TCP checks ports 4444 and 4445; fetches status JSON from `http://<ip>:8002/api/v1/status`.

Tune intervals with the environment variables listed above.


## CORS and security
- Set `CORS_ALLOW_ORIGINS` to a comma‑separated allowlist of your web origins for browsers. Use `*` for permissive development.
- This service is read‑only by design. Add a reverse proxy and rate limiting as needed for public exposure.


## Troubleshooting
- Cannot connect to Postgres:
  - Verify `DB_DSN` and that the database exists: `psql <dsn> -c "\dt"`.
  - Check firewall/network for port 5432.
- Lumera LCD errors (4xx/5xx):
  - Confirm `LUMERA_API_BASE` is reachable and points to a valid Lumera REST endpoint.
- Supernode probe failures:
  - Ports 4444/4445 may be closed by operators; status API may be unavailable on 8002.
- CORS errors in browser:
  - Set `CORS_ALLOW_ORIGINS` appropriately (e.g., `https://your.site`), or `*` during development.


## Roadmap (next)
- Wire API handlers to DB queries (replace stubs with real data).
- Add ETag/Last‑Modified based on DB timestamps.
- Expose Prometheus metrics (optional, behind build tag or separate binary) without external framework.
- Helm chart and Docker packaging.
