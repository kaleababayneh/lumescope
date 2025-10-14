# Database Update Bug Fix Summary

## Problem Identified

**Critical Bug**: During database updates, many columns were being overridden with empty/null values.

### Root Cause

The issue was in the `probeSupernodes()` function in `internal/background/scheduler.go` (lines 286-310).

When probing supernodes, the code created a `SupernodeDB` struct with only probe-related fields:

```go
sn := db.SupernodeDB{
    SupernodeAccount:     t.SupernodeAccount,
    MetricsReport:        toJSONB(report),
    ActualVersion:        status.Version,
    // ... only probe fields, other fields left uninitialized
}
```

However, the `UpsertSupernode()` SQL statement performs a full `ON CONFLICT DO UPDATE SET` that updates **ALL columns**:

```sql
ON CONFLICT ("supernodeAccount") DO UPDATE SET
    "validatorAddress"=EXCLUDED."validatorAddress",
    "validatorMoniker"=EXCLUDED."validatorMoniker",
    "currentState"=EXCLUDED."currentState",
    -- ... ALL 27+ columns updated
```

### The Impact

Every time the probe loop ran (every few seconds/minutes), it would:
- Overwrite `validatorAddress` with `""` (empty string)
- Overwrite `validatorMoniker` with `""` (empty string)
- Overwrite `currentState` with `""` (empty string)
- Set all uninitialized pointer fields to `NULL`
- Reset integer fields to `0`

This meant that data fetched by `syncSupernodes()` was being immediately wiped out by `probeSupernodes()`.

## Solution Implemented

### Changes Made

1. **Created new struct `SupernodeProbeUpdate`** (`internal/db/db.go`)
   - Contains only probe-related fields
   - Separate from the full `SupernodeDB` struct

2. **Created new function `UpdateSupernodeProbeData()`** (`internal/db/db.go`)
   - Performs a targeted UPDATE only on probe-related fields
   - Uses `COALESCE(NULLIF($2,''),\"actualVersion\")` to prevent overwriting actualVersion with empty strings
   - Does NOT touch fields like `validatorAddress`, `validatorMoniker`, `currentState`, etc.

3. **Modified `probeSupernodes()`** (`internal/background/scheduler.go`)
   - Changed from using `SupernodeDB` to `SupernodeProbeUpdate`
   - Changed from calling `UpsertSupernode()` to `UpdateSupernodeProbeData()`

### SQL Query Comparison

**Before (Full Upsert - WRONG)**:
```sql
ON CONFLICT DO UPDATE SET
    "validatorAddress"=EXCLUDED."validatorAddress",  -- Overwrites with ""
    "validatorMoniker"=EXCLUDED."validatorMoniker",  -- Overwrites with ""
    "currentState"=EXCLUDED."currentState",          -- Overwrites with ""
    ... (all 27+ columns)
```

**After (Partial Update - CORRECT)**:
```sql
UPDATE supernodes SET
    "actualVersion"=COALESCE(NULLIF($2,''),"actualVersion"),
    "cpuUsagePercent"=$3,
    "cpuCores"=$4,
    ... (only 17 probe-related columns)
WHERE "supernodeAccount"=$1
```

## Benefits

1. **Data Integrity**: Probe updates no longer overwrite chain-synced data
2. **Performance**: UPDATE is slightly faster than UPSERT for existing records
3. **Maintainability**: Clear separation between full upserts and partial updates
4. **Safety**: Using COALESCE prevents empty strings from overwriting valid data

## Testing Recommendations

1. Verify that `validatorAddress`, `validatorMoniker`, and `currentState` persist after probe cycles
2. Confirm that probe metrics (CPU, memory, storage) are still being updated correctly
3. Monitor logs for any "probe update" errors
4. Check that new supernodes can still be created (initial insert) by `syncSupernodes()`

## Additional Fix: ValidatorMoniker Empty Issue

### Problem
After the initial fix, `validatorMoniker` was still appearing empty due to a race condition:
- `loopValidators` and `loopSupernodes` started simultaneously
- `syncSupernodes` could run before `syncValidators` completed its first fetch
- `getMonikerFor()` would return `""` for all validators
- Empty strings would overwrite existing monikers in the database

### Solution
1. **Run initial validator sync** before starting background loops (in `Start()` method)
2. **Use COALESCE in UpsertSupernode**: `COALESCE(NULLIF(EXCLUDED."validatorMoniker",''),supernodes."validatorMoniker")`
   - If new moniker is empty, keep the existing value
   - If new moniker has a value, use it

This ensures monikers are populated on startup and never get overwritten with empty strings.

## Additional Fix: Height-Based Selection for State and IP Address

### Problem
The code was selecting the **last element** from arrays instead of the entry with the **highest height**:

1. **IP Address extraction**: `ip = sn.PrevIPAddresses[len(sn.PrevIPAddresses)-1].Address`
2. **Current state extraction**: `s := states[len(states)-1]`

This is incorrect because:
- Arrays are not guaranteed to be sorted by height
- The last element might not be the most recent (highest height)
- Example: `[{"height": "412540", ...}, {"height": "951118", ...}, {"height": "836657", ...}]`
  - Last element has height 836657
  - But highest height is 951118 (second element)

### Solution
Created two new helper functions that find entries by **numerically comparing heights**:

1. **`latestState()`** - Finds the state with the highest height value
   - Iterates through all states
   - Parses height strings as int64
   - Tracks the index with maximum height
   - Returns the state and height of the entry with max height

2. **`latestIPAddress()`** - Finds the IP address with the highest height value
   - Same logic as latestState
   - Returns the address string from the entry with max height

### Code Changes
```go
// Before (WRONG)
if len(sn.PrevIPAddresses) > 0 {
    ip = sn.PrevIPAddresses[len(sn.PrevIPAddresses)-1].Address
}
s := states[len(states)-1]

// After (CORRECT)
ip := latestIPAddress(sn.PrevIPAddresses)
state, height := latestState(sn.States)
```

This ensures the most recent data (by block height) is always used, regardless of array ordering.

## Critical Fix: UpsertSupernode Was Clearing Probe Data

### Problem
After fixing the probe update issue, there was still a problem: `syncSupernodes()` was clearing all probe data!

**Root cause**: The `UpsertSupernode()` function was updating **ALL columns** during the `ON CONFLICT DO UPDATE`, including probe-related fields that `syncSupernodes()` doesn't populate:

```go
// syncSupernodes only populates these fields:
rec := db.SupernodeDB{
    SupernodeAccount:   sn.SupernodeAccount,
    ValidatorAddress:   sn.ValidatorAddress,
    // ... chain data fields
    // NOTE: probe fields like ActualVersion, CPUUsagePercent, etc. are NOT set!
}
```

When `UpsertSupernode` executed with this partially populated struct:
- Probe fields (ActualVersion, CPUUsagePercent, CPUCores, Memory*, Storage*, etc.) were `nil` or `0`
- The UPDATE statement overwrote existing probe data with these empty values
- All probe data collected by `probeSupernodes()` was wiped out on every chain sync

### Solution
Modified `UpsertSupernode()` to **only update chain-related fields** and preserve probe data:

```sql
ON CONFLICT ("supernodeAccount") DO UPDATE SET
    -- Chain data fields (always update)
    "validatorAddress"=EXCLUDED."validatorAddress",
    "currentState"=EXCLUDED."currentState",
    "ipAddress"=EXCLUDED."ipAddress",
    "stateHistory"=EXCLUDED."stateHistory",
    "evidence"=EXCLUDED.evidence,
    "prevIpAddresses"=EXCLUDED."prevIpAddresses",
    
    -- Conditional updates (preserve existing if new is NULL)
    "validatorMoniker"=COALESCE(NULLIF(EXCLUDED."validatorMoniker",''),supernodes."validatorMoniker"),
    "metricsReport"=COALESCE(EXCLUDED."metricsReport",supernodes."metricsReport"),
    
    -- Probe fields NOT updated here (handled by UpdateSupernodeProbeData)
```

### Result
- ✅ Chain sync updates chain-related fields only
- ✅ Probe loop updates probe-related fields only
- ✅ No data loss between sync operations
- ✅ Clear separation of concerns

## Notes

- The `UpsertSupernode()` function is still used by `syncSupernodes()` for full record creation/updates
- The new `UpdateSupernodeProbeData()` is only used by the probe loop
- This pattern could be applied to other update scenarios if needed
