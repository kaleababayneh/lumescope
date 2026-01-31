# LumeScope Developer Tutorial

> A comprehensive guide to using Lumescope – a blockchain indexer and query API for the Lumera network. 

---

## Table of Contents

1. [What is Lumescope?](#what-is-lumescope)
2. [Installation & Setup](#installation--setup)
3. [Running Lumescope](#running-lumescope)
4. [Core Functionality](#core-functionality)
5. [Example Use Case](#example-use-case)
6. [Why Use Lumescope?](#why-use-lumescope)
7. [API Reference Quick Guide](#api-reference-quick-guide)
8. [Troubleshooting](#troubleshooting)

---

## What is Lumescope?

**Lumescope** is a blockchain indexer and query API for the Lumera network. It continuously monitors the blockchain, extracts action data, and provides a searchable database.

### Analogy

Think of it like **Google for the Lumera blockchain**:

| Traditional Web | Lumera Blockchain |
|----------------|-------------------|
| Websites exist on servers | Actions exist on blockchain |
| Google crawls and indexes websites | Lumescope indexes blockchain actions |
| You search Google to find websites | You query Lumescope to find actions |
| Google returns search results | Lumescope returns action metadata |

### Architecture

```
┌─────────────────────────────────────────────────────────────┐ 
│                    LUMESCOPE SYSTEM                         │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────────┐         ┌──────────────┐                  │
│  │  Blockchain  │────────▶│  Lumescope   │                  │
│  │   (Lumera)   │ Monitor │   Indexer    │                  │
│  └──────────────┘         └──────┬───────┘                  │
│                                   │                         │
│                                   │ Index                   │
│                                   ▼                         │
│                          ┌─────────────────┐                │
│                          │    Database     │                │
│                          │  (PostgreSQL)   │                │
│                          └────────┬────────┘                │
│                                   │                         │
│                                   │ Query                   │
│                                   ▼                         │
│                          ┌─────────────────┐                │
│                          │   REST API      │                │
│                          │ (Port 18080)    │                │
│                          └────────┬────────┘                │
│                                   │                         │
│                                   │ HTTP                    │
│                                   ▼                         │
│                          ┌─────────────────┐                │
│                          │   Your dApp     │                │
│                          │ (Browser)       │                │
│                          └─────────────────┘                │
└─────────────────────────────────────────────────────────────┘
```

### What Lumescope Does

Lumescope solves several problems faced by developers working with the Lumera network by providing five core capabilities:

#### 1. Monitors the Blockchain

**Problem:** Complex on-chain data is encoded in binary protobuf format (non-human readable), making it difficult to parse and consume.

**Solution:** Lumescope acts as an active listener on the Lumera network. It continuously polls the LCD endpoint to watch for new block commits. When a new block is detected, it parses the transactions to identify relevant message types (like `MsgRegisterAction`). It interprets the raw protobuf-encoded metadata from these messages and uses custom decoders to convert it into structured JSON data.

#### 2. Indexes Data for Fast Retrieval

**Problem:** Querying blockchain data directly is slow and inefficient, especially for complex queries.

**Solution:** Once data is extracted, it is persisted into a normalized PostgreSQL database. Lumescope creates specific indexes on frequently queried fields such as `creator`, `action_type`, `state`, and `block_height`. This indexing strategy ensures that complex queries—which would be slow or impossible to perform directly against the chain—return results in milliseconds.

#### 3. Provides a Unified REST API

**Problem:** No standardized API exists for querying Lumera network data.

**Solution:** Lumescope exposes the indexed data through a set of 16 RESTful endpoints. This API layer abstracts away the complexity of the underlying blockchain data structures. Developers can perform rich queries (e.g., "find all Cascade actions created by specific address in the last 24 hours") and receive standard JSON responses, complete with pagination and error handling.

#### 4. Aggregates SuperNode Metrics

**Problem:** SuperNode health information is scattered across multiple sources, making monitoring difficult.

**Solution:** Beyond simple action tracking, Lumescope actively monitors the health of the SuperNode network. It aggregates hardware statistics (CPU, memory, storage), tracks software version distribution for compatibility analysis, and compiles payment acceptance data. This provides a holistic view of the network's infrastructure layer.

#### 5. Tracks Transaction Lifecycles

**Problem:** Actions on Lumera involve multi-step processes (register/finalize/approve) that are hard to track.

**Solution:** Lumescope understands that an "action" on Lumera is a multi-step process, not a single event. It correlates related transactions to track the complete lifecycle of an action: from `Registration`, through `Finalization`, to `Approval`. It automatically enriches the action record with timestamps and transaction hashes for each of these state transitions, providing a complete audit trail.

---
## Installation & Setup

### Prerequisites

Before you begin, ensure you have:

- **Docker** (recommended) or
- **Go 1.24+** and **PostgreSQL 13+** (for local development)

### Option 1: Docker Setup (Recommended)

Docker provides the simplest setup with an embedded PostgreSQL database.

#### Step 1: Clone the Repository

```bash
git clone https://github.com/LumeraProtocol/lumescope.git
cd lumescope
```

#### Step 2: Build the Docker Image

```bash
docker build -t lumescope .
```

This creates a ~358 MB image containing:
- The compiled Go binary
- Embedded PostgreSQL 14
- Startup scripts for initialization

#### Step 3: Configure Environment (Optional)

Copy and customize the example environment file:

```bash
cp .env.example .env
```

Key configuration options in `.env`:

```bash
# HTTP Server
PORT=18080
CORS_ALLOW_ORIGINS=*

# Lumera Chain REST API (required)
LUMERA_API_BASE=https://lcd.lumera.io

# Database (embedded PostgreSQL is used by default)
DB_DSN=postgres://postgres:postgres@localhost:5432/lumescope?sslmode=disable
DB_MAX_CONNS=10

# Sync Intervals
VALIDATORS_SYNC_INTERVAL=5m    # How often to sync validators
SUPERNODES_SYNC_INTERVAL=2m    # How often to sync supernodes
ACTIONS_SYNC_INTERVAL=30s      # How often to sync actions
PROBE_INTERVAL=1m              # How often to probe supernode ports
```

### Option 2: Local Development Setup

For contributors or advanced users who prefer running outside Docker:

#### Step 1: Install Dependencies

```bash
# Ensure Go 1.24+ is installed
go version

# Install and start PostgreSQL
# macOS:
brew install postgresql@14
brew services start postgresql@14

# Create the database
createdb lumescope
```

#### Step 2: Build the Binary

```bash
# Using Make
make build

# Or directly with Go
go build -o bin/lumescope ./cmd/lumescope
```

#### Step 3: Configure Environment

Set required environment variables:

```bash
export LUMERA_API_BASE=https://lcd.lumera.io
export DB_DSN="postgres://YOUR_USERNAME:postgres@localhost:5432/lumescope?sslmode=disable"
export PORT=18080
```

---

## Running Lumescope

### Running with Docker

#### Quick Start (Ephemeral)

For testing and development:

```bash
docker run -d -p 18080:18080 \
  -e LUMERA_API_BASE=https://lcd.lumera.io \
  --name lumescope lumescope
```

#### Production Mode (With Persistent Storage)

For production environments with data persistence:

```bash
docker run -d -p 18080:18080 \
  -e LUMERA_API_BASE=https://lcd.lumera.io \
  -e POSTGRES_PASSWORD=your-secure-password \
  -v lumescope_data:/var/lib/postgresql/data \
  --name lumescope lumescope
```

#### Using Makefile Shortcuts

```bash
# Mainnet with persistent volume
make docker-run-mainnet

# Testnet with persistent volume
make docker-run-testnet

# Ephemeral local development
make docker-run-local
```

### Running Locally (Without Docker)

```bash
./bin/lumescope
```

Or without building:

```bash
go run ./cmd/lumescope
```

### Verify Deployment

Once running, verify Lumescope is healthy:

```bash
# Health check (liveness)
curl -i http://localhost:18080/healthz
# Expected: HTTP/1.1 200 OK

# Readiness check
curl -i http://localhost:18080/readyz
# Expected: HTTP/1.1 200 OK

# Fetch some actions
curl -s 'http://localhost:18080/v1/actions?limit=3' | jq .

# Open Swagger UI in browser
open http://localhost:18080/docs
```

### Stopping Lumescope

```bash
# Stop the container
docker stop lumescope

# Remove the container
docker rm lumescope

# Or using Make
make docker-stop
make docker-rm
```

---

## Core Functionality

Lumescope provides three main categories of functionality:

### 1. Actions API

Actions represent on-chain activities in the Lumera network. Lumescope decodes and tracks two types:

| Action Type | Description |
|-------------|-------------|
| **Cascade** | File storage actions with MIME types, sizes, and file metadata |
| **Sense** | AI/ML sensing operations for content analysis |

#### List Actions

```bash
curl 'http://localhost:18080/v1/actions?type=cascade&limit=5'
```

**Query Parameters:**
- `type` — Filter by action type (`cascade` or `sense`)
- `creator` — Filter by creator address
- `state` — Filter by action state
- `supernode` — Filter by supernode address
- `fromHeight` / `toHeight` — Block height range
- `limit` — Maximum results (default: 50)
- `cursor` — Pagination cursor for next page
- `include_transactions` — Include transaction history

#### Get Action Details

```bash
curl http://localhost:18080/v1/actions/{action_id}
```

Returns complete action data including:
- Decoded metadata (JSON)
- MIME type and file size
- Price information
- SuperNode assignments
- Transaction lifecycle (register, finalize, approve)

#### Action Statistics

```bash
curl 'http://localhost:18080/v1/actions/stats?type=cascade&from=2024-01-01T00:00:00Z'
```

Returns aggregated statistics:
- Total action count by state
- MIME type breakdown with counts and average sizes
- Time-filtered results using RFC3339 format

### 2. SuperNode Metrics API

SuperNodes are the backbone of the Lumera network. Lumescope aggregates their status, hardware metrics, and availability.

#### List SuperNode Metrics

```bash
curl 'http://localhost:18080/v1/supernodes/metrics?status=available&limit=10'
```

**Query Parameters:**
- `currentState` — Filter by chain state (e.g., `SUPERNODE_STATE_ACTIVE`)
- `status` — Filter by availability (`available`, `unavailable`, `any`)
- `version` — Filter by software version
- `minFailedProbeCounter` — Minimum failed probe count
- `limit` / `cursor` — Pagination controls

#### Single SuperNode Metrics

```bash
curl http://localhost:18080/v1/supernodes/{supernode_address}/metrics
```

Returns detailed metrics:
- CPU, memory, storage stats
- Peer count and uptime
- Last successful probe timestamp
- Failed probe counter

#### Payment Information

```bash
curl http://localhost:18080/v1/supernodes/{supernode_address}/paymentInfo
```

Returns payment statistics grouped by denomination.

#### Aggregated Statistics

```bash
# Hardware statistics across all supernodes
curl http://localhost:18080/v1/supernodes/stats

# Action statistics per supernode
curl http://localhost:18080/v1/supernodes/action-stats

# List unavailable supernodes
curl http://localhost:18080/v1/supernodes/unavailable
```

### 3. Version & Compatibility

```bash
curl http://localhost:18080/v1/version/matrix
```

Returns the version compatibility matrix showing:
- Software versions in use
- Total/available/unavailable counts per version
- Latest version detection

---

## Example Use Case

### Building a Lumera Network Dashboard

Let's build a simple monitoring script that fetches network health data:

```javascript
#!/usr/bin/env node
/**
 * Lumera Network Dashboard - Using Lumescope API
 * 
 * Requirements: Node.js 18+ (for built-in fetch API)
 * For older Node.js versions, install node-fetch: npm install node-fetch
 */

const LUMESCOPE_BASE = 'http://localhost:18080';

async function getNetworkHealth() {
  try {
    // 1. Check Lumescope health
    const healthRes = await fetch(`${LUMESCOPE_BASE}/healthz`);
    const isHealthy = healthRes.status === 200;
    console.log(`✓ Lumescope Status: ${isHealthy ? 'Healthy' : 'Unhealthy'}`);
    
    // 2. Get SuperNode statistics
    const statsRes = await fetch(`${LUMESCOPE_BASE}/v1/supernodes/stats`);
    const stats = await statsRes.json();
    console.log('\n📊 SuperNode Hardware Stats:');
    console.log(`   - Available SuperNodes: ${stats.available_supernodes ?? 'N/A'}`);
    console.log(`   - Total CPU Cores: ${stats.total_cpu_cores ?? 'N/A'}`);
    console.log(`   - Total Memory: ${stats.total_memory_gb?.toFixed(2) ?? 'N/A'} GB`);
    console.log(`   - Storage Used: ${stats.storage_used_percent?.toFixed(1) ?? 'N/A'}%`);
    
    // 3. Get version distribution
    const versionsRes = await fetch(`${LUMESCOPE_BASE}/v1/version/matrix`);
    const versions = await versionsRes.json();
    console.log('\n🔄 Version Distribution:');
    console.log(`   Latest Version: ${versions.latest_version}`);
    versions.versions?.slice(0, 5).forEach(v => {
      console.log(`   - ${v.version}: ${v.nodes_total} nodes ` +
                  `(${v.nodes_available} available)`);
    });
    
    // 4. Get recent Cascade actions
    const actionsRes = await fetch(
      `${LUMESCOPE_BASE}/v1/actions?type=cascade&limit=5`
    );
    const actions = await actionsRes.json();
    
    console.log('\n📁 Recent Cascade Actions:');
    if (actions.items?.length > 0) {
      actions.items.forEach(action => {
        console.log(`   - ${action.id.substring(0, 20)}... [${action.state}] ` +
                    `MIME: ${action.mime_type ?? 'unknown'}`);
      });
    } else {
      console.log('   - No actions found');
    }
    
    // 5. Get action statistics
    const actionStatsRes = await fetch(`${LUMESCOPE_BASE}/v1/actions/stats`);
    const actionStats = await actionStatsRes.json();
    console.log('\n📈 Action Statistics:');
    console.log(`   - Total Actions: ${actionStats.total ?? 0}`);
    if (actionStats.states && Object.keys(actionStats.states).length > 0) {
      Object.entries(actionStats.states).forEach(([state, count]) => {
        console.log(`   - ${state}: ${count}`);
      });
    }
    
  } catch (error) {
    console.error('❌ Error fetching network health:', error.message);
  }
}

// Run the script
getNetworkHealth();
```

### Sample Output

> **Note:** Sample output captured from Lumera testnet as of January 31, 2026 at 21:05 UTC+3


```
✓ Lumescope Status: Healthy

📊 SuperNode Hardware Stats:
   - Available SuperNodes: 29
   - Total CPU Cores: 421
   - Total Memory: 2150.75 GB
   - Storage Used: 60.7%

🔄 Version Distribution:
   Latest Version: v2.4.26-testnet
   - v2.4.26-testnet: 27 nodes (27 available)
   - v2.4.10: 2 nodes (2 available)
   - v2.4.18: 1 nodes (1 available)
   - v2.4.21-testnet: 1 nodes (1 available)
   - v2.4.27: 1 nodes (1 available)

📁 Recent Cascade Actions:
   - No actions found

📈 Action Statistics:
   - Total Actions: 12673
   - ACTION_STATE_APPROVED: 6
   - ACTION_STATE_DONE: 11782
   - ACTION_STATE_EXPIRED: 885
```

---

## Why Use Lumescope?

### For dApp Developers

| Benefit | Description |
|---------|-------------|
| **Simplified Data Access** | No need to decode protobuf or query multiple chain endpoints |
| **Ready-to-use REST API** | Standard JSON responses with OpenAPI documentation |
| **Real-time Updates** | Background sync keeps data fresh (30s for actions) |
| **Pagination Built-in** | Cursor-based pagination for large datasets |

### For Network Operators

| Benefit | Description |
|---------|-------------|
| **SuperNode Monitoring** | Track availability, hardware metrics, and version compliance |
| **Probe Status** | Automatic port checking with failure tracking |
| **Payment Analytics** | View payment statistics per supernode |

### For Analytics Platforms

| Benefit | Description |
|---------|-------------|
| **Aggregated Statistics** | Pre-computed stats for dashboards |
| **Historical Data** | PostgreSQL storage for trend analysis |
| **Time-based Filtering** | RFC3339 timestamp support for date ranges |

### Technical Advantages

- **No external dependencies** — Uses Go's standard library for HTTP
- **Single container deployment** — Embedded PostgreSQL, no separate database setup
- **Automatic schema migration** — Tables and indexes created on startup
- **Interactive documentation** — Swagger UI at `/docs`

---

## API Reference Quick Guide

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/healthz` | GET | Liveness probe |
| `/readyz` | GET | Readiness probe |
| `/v1/actions` | GET | List actions with filters |
| `/v1/actions/{id}` | GET | Get action details |
| `/v1/actions/stats` | GET | Action statistics |
| `/v1/supernodes/metrics` | GET | List supernode metrics |
| `/v1/supernodes/{id}/metrics` | GET | Single supernode metrics |
| `/v1/supernodes/{id}/paymentInfo` | GET | Payment statistics |
| `/v1/supernodes/stats` | GET | Aggregated hardware stats |
| `/v1/supernodes/action-stats` | GET | Actions per supernode |
| `/v1/supernodes/unavailable` | GET | Unavailable supernodes |
| `/v1/supernodes/sync` | POST | Trigger manual sync |
| `/v1/version/matrix` | GET | Version compatibility |
| `/openapi.json` | GET | OpenAPI 3.0 spec |
| `/docs` | GET | Swagger UI |
| `/metrics` | GET | Prometheus metrics (stub) |

> 💡 **Tip**: Visit `http://localhost:18080/docs` for interactive API documentation with try-it-out functionality.

---

## Troubleshooting

### Common Issues

#### LCD Endpoint Unreachable

**Symptoms:** Actions/supernodes not syncing, connection errors in logs

**Solutions:**
```bash
# Verify LCD endpoint is accessible
curl https://lcd.lumera.io/cosmos/base/tendermint/v1beta1/node_info

# Check container network connectivity
docker exec lumescope wget -qO- https://lcd.lumera.io/healthz

# Verify LUMERA_API_BASE is set correctly
docker inspect lumescope | grep LUMERA_API_BASE
```

#### Database Connection Failures

**Symptoms:** Startup fails, queries timeout

**Solutions:**
```bash
# Check container logs
docker logs lumescope

# For external PostgreSQL, verify connection
psql "postgres://user:pass@host:5432/lumescope" -c "\dt"

# Ensure database user has CREATE TABLE privileges
```

#### SuperNode Probe Failures

**Symptoms:** Supernodes show as unavailable

**Note:** This is often expected behavior. Some supernodes have firewalled ports (4444, 4445, 8002) or disabled status APIs.

```bash
# Check which nodes are unavailable
curl http://localhost:18080/v1/supernodes/unavailable

# Failed probe count is tracked per node
curl 'http://localhost:18080/v1/supernodes/metrics?minFailedProbeCounter=5'
```

#### CORS Errors in Browser

**Symptoms:** Browser console shows CORS policy errors

**Solution:**
```bash
docker run -d -p 18080:18080 \
  -e LUMERA_API_BASE=https://lcd.lumera.io \
  -e CORS_ALLOW_ORIGINS="https://your-app.com" \
  --name lumescope lumescope
```

---

## Next Steps

1. **Explore the Swagger UI** — Visit `http://localhost:18080/docs` to interactively test all endpoints
2. **Read the OpenAPI Spec** — Download from `/openapi.json` for client code generation
3. **Join the Lumera Community** — Get help and share your projects
4. **Contribute** — Found a bug or have a feature request? Open an issue on GitHub!

---

## Additional Resources

- [GitHub Repository](https://github.com/LumeraProtocol/lumescope)
- [OpenAPI Specification](http://localhost:18080/openapi.json)
- [Swagger UI Documentation](http://localhost:18080/docs)

---
