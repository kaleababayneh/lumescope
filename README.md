# LumeScope

![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go&logoColor=white)
![Docker](https://img.shields.io/badge/Docker-ready-2496ED?logo=docker&logoColor=white)
![License](https://img.shields.io/badge/License-MIT-green.svg)

**LumeScope** is a neutral, read-only API aggregator for the Lumera network. It decodes Action metadata (Cascade/Sense), provides Cascade previews (MIME types, file sizes), and aggregates SuperNode metrics, probes, version compatibility, and payment statistics—so Lumera clients stay lightweight and responsive.

## Features

- **Actions API** — List and detail endpoints for decoded Action metadata (Cascade/Sense types) with transaction lifecycle tracking (register, finalize, approve)
- **SuperNode Metrics** — Aggregated hardware stats, version matrix, payment info, and availability probes
- **Background Scheduler** — Automatic sync loops:
  - Validators sync (default: 5m)
  - SuperNodes sync (default: 2m)
  - Actions sync (default: 30s)
  - SuperNode port probes (default: 1m)
  - Action transaction enricher (background)
- **Embedded PostgreSQL 14** — Single-container deployment; no external database required
- **Swagger UI** — Interactive API docs at `/docs`
- **OpenAPI 3.0** — Machine-readable spec at `/openapi.json`
- **stdlib-only HTTP** — No third-party web frameworks; uses Go's `net/http`

## API Reference

LumeScope exposes **16 endpoints**. All data is read-only.

| Endpoint | Method | Description | Key Params | Example |
|----------|--------|-------------|------------|---------|
| `/healthz` | GET | Liveness probe (always 200 if running) | — | `curl http://localhost:18080/healthz` |
| `/readyz` | GET | Readiness probe | — | `curl http://localhost:18080/readyz` |
| `/v1/actions` | GET | List actions with decoded metadata | `type`, `creator`, `state`, `supernode`, `fromHeight`, `toHeight`, `limit`, `cursor`, `include_transactions` | `curl 'http://localhost:18080/v1/actions?type=cascade&limit=5'` |
| `/v1/actions/{id}` | GET | Action details with transactions | — | `curl http://localhost:18080/v1/actions/action123` |
| `/v1/actions/stats` | GET | Aggregated action statistics | `from`, `to` (RFC3339), `type` | `curl 'http://localhost:18080/v1/actions/stats?type=cascade'` |
| `/v1/supernodes/metrics` | GET | List supernode metrics | `currentState`, `status`, `version`, `minFailedProbeCounter`, `limit`, `cursor` | `curl 'http://localhost:18080/v1/supernodes/metrics?status=available&limit=10'` |
| `/v1/supernodes/{id}/metrics` | GET | Single supernode metrics | — | `curl http://localhost:18080/v1/supernodes/lumera1abc.../metrics` |
| `/v1/supernodes/{id}/paymentInfo` | GET | Payment statistics by denomination | — | `curl http://localhost:18080/v1/supernodes/lumera1abc.../paymentInfo` |
| `/v1/supernodes/stats` | GET | Aggregated hardware statistics | — | `curl http://localhost:18080/v1/supernodes/stats` |
| `/v1/supernodes/action-stats` | GET | Action statistics per supernode | — | `curl http://localhost:18080/v1/supernodes/action-stats` |
| `/v1/supernodes/unavailable` | GET | Supernodes with unavailable status API | `currentState` | `curl http://localhost:18080/v1/supernodes/unavailable` |
| `/v1/supernodes/sync` | POST | Trigger manual sync+probe (if enabled) | — | `curl -X POST http://localhost:18080/v1/supernodes/sync` |
| `/v1/version/matrix` | GET | Version compatibility matrix (partial LEP2) | — | `curl http://localhost:18080/v1/version/matrix` |
| `/openapi.json` | GET | OpenAPI 3.0 specification | — | `curl http://localhost:18080/openapi.json` |
| `/docs` | GET | Swagger UI documentation | — | Open in browser: `http://localhost:18080/docs` |
| `/metrics` | GET | Prometheus metrics (**stub**) | — | `curl http://localhost:18080/metrics` |

> **Note:** `/metrics` currently returns stub data. Rate limiting is planned for future releases.

See also: [`docs/openapi.json`](docs/openapi.json) and [`docs/context.json`](docs/context.json) for implementation details.

## Quickstart

### 1. Build the Docker image

```bash
docker build -t lumescope .
```

Build output is ~358 MB.

### 2. Run the container

**Minimal (ephemeral, for testing):**

```bash
docker run -d -p 18080:18080 -e LUMERA_API_BASE=https://lcd.lumera.io --name lumescope lumescope
```

**With custom environment file:**

```bash
cp .env.example .env
# Edit .env with your settings (see Configuration below)

docker run -d -p 18080:18080 \
  -v $(pwd)/.env:/app/.env \
  --name lumescope lumescope
```

### 3. Verify deployment

```bash
# Health check
curl -i http://localhost:18080/healthz

# Readiness check
curl -i http://localhost:18080/readyz

# Fetch recent actions
curl -s 'http://localhost:18080/v1/actions?limit=3' | jq .

# Open Swagger UI in browser
open http://localhost:18080/docs
```

### 4. Stop and remove container

```bash
docker stop lumescope
docker rm lumescope
```

## Configuration

Copy [`.env.example`](.env.example) to `.env` and customize as needed. The container reads environment variables at startup.

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `LUMERA_API_BASE` | **Yes** | `http://localhost:1317` | Lumera REST (LCD) endpoint URL |
| `PORT` | No | `18080` | HTTP server listen port |
| `CORS_ALLOW_ORIGINS` | No | `*` | Comma-separated CORS origins |
| `DB_DSN` | No | `postgres://postgres:postgres@localhost:5432/lumescope?sslmode=disable` | PostgreSQL connection string (for external DB) |
| `DB_MAX_CONNS` | No | `10` | Max database connections |
| `HTTP_TIMEOUT` | No | `10s` | Outbound HTTP timeout for Lumera calls |
| `READ_HEADER_TIMEOUT` | No | `5s` | Server read header timeout |
| `READ_TIMEOUT` | No | `30s` | Server read timeout |
| `WRITE_TIMEOUT` | No | `30s` | Server write timeout |
| `IDLE_TIMEOUT` | No | `120s` | Server idle timeout |
| `REQUEST_TIMEOUT` | No | `10s` | Per-request server timeout |
| `VALIDATORS_SYNC_INTERVAL` | No | `5m` | Validators sync frequency |
| `SUPERNODES_SYNC_INTERVAL` | No | `2m` | SuperNodes sync frequency |
| `ACTIONS_SYNC_INTERVAL` | No | `30s` | Actions sync frequency |
| `PROBE_INTERVAL` | No | `1m` | SuperNode probe frequency |
| `DIAL_TIMEOUT` | No | `2s` | TCP dial timeout for probes |

### Embedded PostgreSQL Variables (Docker only)

| Variable | Default | Description |
|----------|---------|-------------|
| `POSTGRES_DB` | `lumescope` | Database name |
| `POSTGRES_USER` | `postgres` | Database user |
| `POSTGRES_PASSWORD` | `postgres` | Database password (set for security) |
| `PGDATA` | `/var/lib/postgresql/data` | Data directory |

**Recommendation:** Set `POSTGRES_PASSWORD` to a strong value in production:

```bash
docker run -d -p 18080:18080 \
  -e LUMERA_API_BASE=https://lcd.lumera.io \
  -e POSTGRES_PASSWORD=your-secure-password \
  --name lumescope lumescope
```

## Production Deployment

### Persistent Data Volume

Mount a volume to retain PostgreSQL data across container restarts:

```bash
docker run -d -p 18080:18080 \
  -e LUMERA_API_BASE=https://lcd.lumera.io \
  -e POSTGRES_PASSWORD=your-secure-password \
  -v lumescope_data:/var/lib/postgresql/data \
  --name lumescope lumescope
```

### Using Makefile Targets

The [`Makefile`](Makefile) provides convenience targets:

```bash
# Mainnet with persistent volume
make docker-run-mainnet

# Testnet with persistent volume
make docker-run-testnet

# Ephemeral local development
make docker-run-local
```

### External PostgreSQL Database

To use an external PostgreSQL 13+ database instead of the embedded one:

```bash
docker run -d -p 18080:18080 \
  -e LUMERA_API_BASE=https://lcd.lumera.io \
  -e DB_DSN="postgres://user:pass@your-pg-host:5432/lumescope?sslmode=require" \
  --name lumescope lumescope
```

The application auto-creates required tables and indexes on startup.

### Running Multiple Replicas

For high availability, run multiple LumeScope containers pointing to a shared external PostgreSQL:

```bash
# Instance 1
docker run -d -p 18081:18080 \
  -e LUMERA_API_BASE=https://lcd.lumera.io \
  -e DB_DSN="postgres://user:pass@pg-host:5432/lumescope?sslmode=require" \
  --name lumescope-1 lumescope

# Instance 2
docker run -d -p 18082:18080 \
  -e LUMERA_API_BASE=https://lcd.lumera.io \
  -e DB_DSN="postgres://user:pass@pg-host:5432/lumescope?sslmode=require" \
  --name lumescope-2 lumescope
```

Place a load balancer (nginx, HAProxy, cloud LB) in front of the instances.

### Monitoring

- **Health endpoint:** `GET /healthz` (liveness)
- **Readiness endpoint:** `GET /readyz`
- **Metrics endpoint:** `GET /metrics` (currently returns stub data; Prometheus integration planned)

The Docker image includes a built-in `HEALTHCHECK` that polls `/healthz` every 30 seconds.

### Future Enhancements

- Full Prometheus metrics export
- Rate limiting per client
- Redis caching layer for sub-200ms p95 latency

## Publishing to Public Registry (For Repo Owners)

### Docker Hub

```bash
# Build with a version tag
docker build -t username/lumescope:v1.0.0 .
docker build -t username/lumescope:latest .

# Push to Docker Hub
docker login
docker push username/lumescope:v1.0.0
docker push username/lumescope:latest
```

### GitHub Container Registry (GHCR)

```bash
# Login to GHCR
echo $GITHUB_TOKEN | docker login ghcr.io -u USERNAME --password-stdin

# Tag for GHCR
docker tag lumescope ghcr.io/org/lumescope:v1.0.0
docker tag lumescope ghcr.io/org/lumescope:latest

# Push to GHCR
docker push ghcr.io/org/lumescope:v1.0.0
docker push ghcr.io/org/lumescope:latest
```

### Multi-Architecture Builds

For ARM64 and AMD64 support:

```bash
docker buildx create --use
docker buildx build --platform linux/amd64,linux/arm64 \
  -t username/lumescope:v1.0.0 \
  --push .
```

## Development

### Prerequisites

- Go 1.24+
- PostgreSQL 13+ (or use Docker)

### Building Locally

```bash
# Build binary
make build
# or: go build -o bin/lumescope ./cmd/lumescope

# Run directly
./bin/lumescope

# Or run without building
go run ./cmd/lumescope
```

### Makefile Targets

| Target | Description |
|--------|-------------|
| `make build` | Build the Go binary to `bin/lumescope` |
| `make docker-build` | Build the Docker image |
| `make docker-run` | Run ephemeral container (local dev) |
| `make docker-run-local` | Alias for `docker-run` |
| `make docker-run-mainnet` | Run with mainnet LCD + persistent volume |
| `make docker-run-testnet` | Run with testnet LCD + persistent volume |
| `make docker-stop` | Stop the lumescope container |
| `make docker-rm` | Remove the lumescope container |
| `make docker-rebuild` | Stop, remove, rebuild, and run |

### Project Structure

```
├── cmd/lumescope/       # Application entrypoint
├── internal/
│   ├── background/      # Scheduler and sync loops
│   ├── config/          # Environment configuration
│   ├── db/              # PostgreSQL operations
│   ├── decoder/         # Protobuf metadata decoder
│   ├── handlers/        # HTTP route handlers
│   ├── lumera/          # Lumera LCD client
│   ├── server/          # HTTP router setup
│   └── util/            # JSON helpers
├── docs/
│   ├── context.json     # Implementation status reference
│   ├── requirements.json # Project requirements
│   ├── openapi.json     # OpenAPI 3.0 spec (JSON)
│   └── openapi.yaml     # OpenAPI 3.0 spec (YAML)
├── Dockerfile           # Multi-stage build with embedded Postgres
├── Makefile             # Build and run targets
└── .env.example         # Environment variable template
```

## Troubleshooting

### LCD endpoint unreachable

**Symptom:** Actions/supernodes not syncing; logs show connection errors.

**Solution:**
1. Verify `LUMERA_API_BASE` is set correctly:
   ```bash
   curl https://lcd.lumera.io/cosmos/base/tendermint/v1beta1/node_info
   ```
2. Check network connectivity from the container:
   ```bash
   docker exec lumescope wget -qO- https://lcd.lumera.io/healthz
   ```
3. If using a local node, ensure the LCD port (1317) is exposed.

### Database connection failures

**Symptom:** Startup fails or queries timeout.

**Solution:**
1. For embedded Postgres, check container logs:
   ```bash
   docker logs lumescope
   ```
2. For external Postgres, verify `DB_DSN`:
   ```bash
   psql "postgres://user:pass@host:5432/lumescope?sslmode=disable" -c "\dt"
   ```
3. Ensure the database user has CREATE TABLE privileges.

### SuperNode probe failures

**Symptom:** Supernodes show as unavailable; probe metrics missing.

**Cause:** SuperNode ports 4444, 4445, or 8002 may be firewalled or the status API is disabled by operators.

**Solution:**
1. This is expected for some supernodes.
2. Check `/v1/supernodes/unavailable` to see affected nodes.
3. The `failedProbeCounter` field tracks consecutive failures.

### CORS errors in browser

**Symptom:** Browser console shows CORS policy errors.

**Solution:**
Set `CORS_ALLOW_ORIGINS` to your frontend origin:
```bash
docker run -d -p 18080:18080 \
  -e LUMERA_API_BASE=https://lcd.lumera.io \
  -e CORS_ALLOW_ORIGINS="https://your-app.com" \
  --name lumescope lumescope
```

Use `*` only for development.

---

## License

MIT License. See [LICENSE](LICENSE) for details.
