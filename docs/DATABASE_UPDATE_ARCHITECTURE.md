# Database Update Architecture - Final Solution

## Problem Overview

The application had **multiple overlapping update issues** where different sync loops were overwriting each other's data.

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Background Sync Loops                         │
└─────────────────────────────────────────────────────────────────────┘

┌──────────────────────┐  ┌──────────────────────┐  ┌──────────────────────┐
│  syncValidators()    │  │  syncSupernodes()    │  │  probeSupernodes()   │
│  Every N minutes     │  │  Every N minutes     │  │  Every N minutes     │
└──────────────────────┘  └──────────────────────┘  └──────────────────────┘
         │                          │                          │
         │                          │                          │
         ▼                          ▼                          ▼
┌──────────────────────┐  ┌──────────────────────┐  ┌──────────────────────┐
│ In-memory map:       │  │ Fetches from chain:  │  │ Probes nodes:        │
│ valoper → moniker    │  │ • SupernodeAccount   │  │ • Port connectivity  │
└──────────────────────┘  │ • ValidatorAddress   │  │ • Status API         │
                          │ • CurrentState       │  │ • CPU/Memory/Storage │
                          │ • StateHistory       │  │ • Version, Uptime    │
                          │ • PrevIPAddresses    │  │ • Peers, Rank        │
                          │ • Evidence           │  └──────────────────────┘
                          │ • ProtocolVersion    │             │
                          └──────────────────────┘             │
                                    │                          │
                                    ▼                          ▼
                          ┌──────────────────────┐  ┌──────────────────────┐
                          │ UpsertSupernode()    │  │UpdateSupernodeProbe()│
                          │ PARTIAL UPDATE       │  │ PARTIAL UPDATE       │
                          └──────────────────────┘  └──────────────────────┘
                                    │                          │
                                    └────────┬─────────────────┘
                                             ▼
                                ┌────────────────────────────┐
                                │   supernodes table         │
                                │   (Postgres)               │
                                └────────────────────────────┘
```

## Database Update Strategy

### UpsertSupernode() - Chain Data Only
**Used by**: `syncSupernodes()`
**Updates**: Only chain-related columns
```sql
ON CONFLICT DO UPDATE SET
    "validatorAddress" = EXCLUDED."validatorAddress",
    "currentState" = EXCLUDED."currentState",
    "stateHistory" = EXCLUDED."stateHistory",
    "prevIpAddresses" = EXCLUDED."prevIpAddresses",
    -- DOES NOT UPDATE: actualVersion, cpuUsagePercent, memoryTotalGb, etc.
```

### UpdateSupernodeProbeData() - Probe Data Only
**Used by**: `probeSupernodes()`7899
**Updates**: Only probe-related columns
```sql
UPDATE supernodes SET
    "actualVersion" = COALESCE(NULLIF($2,''),"actualVersion"),
    "cpuUsagePercent" = $3,
    "memoryTotalGb" = $5,
    "storageUsedBytes" = $9,
    -- DOES NOT UPDATE: validatorAddress, currentState, stateHistory, etc.
```

## Key Principles

### 1. Separation of Concerns
- **Chain sync** manages blockchain data
- **Probe sync** manages runtime metrics
- **Neither overwrites the other's data**

### 2. COALESCE for Safety
```sql
-- Preserve existing value if new value is empty
"validatorMoniker" = COALESCE(NULLIF(EXCLUDED."validatorMoniker",''), supernodes."validatorMoniker")
```

### 3. Height-Based Selection
```go
// Always select the entry with highest block height
func latestState(states []SupernodeState) (string, string)
func latestIPAddress(addresses []PrevIPAddress) string
```

### 4. Initial Sync Before Loops
```go
func (r *Runner) Start(ctx context.Context) {
    // Populate monikers BEFORE starting background loops
    r.syncValidators(ctx)
    
    go r.loopValidators(ctx)
    go r.loopSupernodes(ctx)
    go r.loopProbes(ctx)
}
```

## Data Flow Example

```
Time T0: New supernode appears on chain
  syncSupernodes() → UpsertSupernode() → INSERT all columns
  Result: validatorMoniker="", cpuUsagePercent=NULL (no probe data yet)

Time T1: Validator sync runs
  syncValidators() → Updates in-memory map
  Next syncSupernodes() → validatorMoniker now populated

Time T2: Probe runs for the first time
  probeSupernodes() → UpdateSupernodeProbeData()
  Result: cpuUsagePercent=45.2, memoryUsedGb=8.5, etc.
  Chain data unchanged!

Time T3: Chain sync runs again
  syncSupernodes() → UpsertSupernode()
  Result: Updates currentState, stateHistory
  Probe data (CPU, memory) PRESERVED!

Time T4: Probe runs again
  probeSupernodes() → UpdateSupernodeProbeData()
  Result: cpuUsagePercent=52.1 (updated)
  Chain data (currentState, stateHistory) PRESERVED!
```

## Summary of All Fixes

1. ✅ **Created `UpdateSupernodeProbeData()`** - Partial update for probe data
2. ✅ **Modified `UpsertSupernode()`** - Only updates chain data, preserves probe data
3. ✅ **Added `COALESCE` for monikers** - Prevents empty strings from overwriting
4. ✅ **Initial validator sync** - Populate monikers before other loops start
5. ✅ **Height-based selection** - Always select entries with highest block height
6. ✅ **Clear separation** - Each sync loop has its own update function

Result: **No more data loss!** Each sync operation updates only its domain without interfering with others.
