# Database Scanning Fixes Applied

## Overview

Fixed two critical database scanning issues that were preventing API tests from passing. These were application-level bugs discovered during API testing.

## Issue #1: JSONB Scanning Error

### Problem
```
sql: Scan error on column index 9, name "form_config":
unsupported Scan, storing driver.Value type []uint8 into type *store.JSONB
```

### Root Cause
The `JSONB.Scan()` method wasn't handling empty byte slices and null values properly, causing unmarshaling errors.

### Fix Applied
**File**: `internal/store/models.go`

Enhanced the `Scan` method to handle edge cases:

```go
// Scan implements the sql.Scanner interface for JSONB
func (j *JSONB) Scan(value interface{}) error {
    if value == nil {
        *j = nil
        return nil
    }

    var bytes []byte
    switch v := value.(type) {
    case []byte:
        bytes = v
    case string:
        bytes = []byte(v)
    default:
        return errors.New("incompatible type for JSONB")
    }

    // Handle empty or null JSON
    if len(bytes) == 0 || string(bytes) == "null" {
        *j = make(JSONB)
        return nil
    }

    result := make(JSONB)
    err := json.Unmarshal(bytes, &result)
    if err != nil {
        return err
    }
    *j = result
    return nil
}
```

**Changes**:
- Added check for empty byte slices
- Added check for "null" string
- Initialize empty JSONB map for these cases
- Improved error handling

## Issue #2: PostgreSQL Array Scanning Error

### Problem
```
sql: Scan error on column index 5, name "events":
unsupported Scan, storing driver.Value type string into type *[]string
```

### Root Cause
PostgreSQL text[] arrays are returned as strings in format `{item1,item2,item3}`, but Go's `[]string` type doesn't know how to scan this format automatically.

### Fix Applied

**File**: `internal/store/models.go`

Created a custom `StringArray` type with proper scanning logic:

```go
// StringArray is a custom type for PostgreSQL text[] arrays
type StringArray []string

// Value implements the driver.Valuer interface for StringArray
func (a StringArray) Value() (driver.Value, error) {
    if a == nil {
        return nil, nil
    }
    if len(a) == 0 {
        return "{}", nil
    }
    // PostgreSQL array format: {item1,item2,item3}
    return "{" + strings.Join(a, ",") + "}", nil
}

// Scan implements the sql.Scanner interface for StringArray
func (a *StringArray) Scan(value interface{}) error {
    if value == nil {
        *a = nil
        return nil
    }

    var str string
    switch v := value.(type) {
    case []byte:
        str = string(v)
    case string:
        str = v
    default:
        return fmt.Errorf("unsupported type for StringArray: %T", value)
    }

    // Handle empty array
    if str == "" || str == "{}" {
        *a = []string{}
        return nil
    }

    // Remove curly braces and split
    str = strings.Trim(str, "{}")
    if str == "" {
        *a = []string{}
        return nil
    }

    // Split by comma
    *a = strings.Split(str, ",")
    return nil
}
```

**Updated Structs**:

1. **Webhook** (`internal/store/models.go`):
   ```go
   Events StringArray `db:"events" json:"events"`
   ```

2. **APIKey** (`internal/store/models.go`):
   ```go
   Scopes StringArray `db:"scopes" json:"scopes"`
   ```

**Updated Store Methods**:

1. `internal/store/webhook.go`:
   - `CreateWebhook`: Changed from `pq.Array(params.Events)` to `StringArray(params.Events)`
   - `UpdateWebhook`: Changed from `pq.Array(params.Events)` to `StringArray(params.Events)`
   - Removed unused `github.com/lib/pq` import

2. `internal/store/api_key.go`:
   - `CreateAPIKey`: Changed from `pq.Array(params.Scopes)` to `StringArray(params.Scopes)`
   - Removed unused `github.com/lib/pq` import

## Impact

### Before Fixes
- ❌ Campaign tests: 0/45 passing (100% failure due to JSONB error)
- ❌ Webhook tests: 4/20 passing (80% failure due to array scanning error)
- ✅ Auth tests: 21/21 passing
- ✅ Health tests: 1/1 passing

**Total**: 26/86 tests passing (30%)

### After Fixes
- ✅ Campaign tests: Expected 45/45 passing
- ✅ Webhook tests: Expected 20/20 passing
- ✅ Auth tests: 21/21 passing
- ✅ Health tests: 1/1 passing

**Expected Total**: 86/86 tests passing (100%)

## Testing

To verify the fixes:

```bash
# Restart the server to load the fixed code
# Terminal 1:
make test-db-up
go run main.go

# Terminal 2:
make test-api
```

## Files Modified

1. `internal/store/models.go`
   - Enhanced JSONB.Scan() method
   - Added StringArray type with Value() and Scan() methods
   - Updated Webhook.Events type from []string to StringArray
   - Updated APIKey.Scopes type from []string to StringArray

2. `internal/store/webhook.go`
   - Updated CreateWebhook to use StringArray
   - Updated UpdateWebhook to use StringArray
   - Removed pq import

3. `internal/store/api_key.go`
   - Updated CreateAPIKey to use StringArray
   - Removed pq import

## Notes

- The `StringArray` type is a simple wrapper around `[]string` that implements the `sql.Scanner` and `driver.Valuer` interfaces
- Since `StringArray` is just an alias, it's JSON-compatible and will serialize/deserialize as a regular array
- The fixes are backward compatible - existing code using `[]string` can convert to `StringArray` with a simple type conversion: `StringArray(mySlice)`
- No database schema changes required
- No API contract changes - JSON responses remain identical

## Related

- See `tests/TEST_RESULTS.md` for detailed test results
- See `tests/README.md` for API testing documentation

---

**Date Applied**: 2025-11-07
**Issues Found By**: API integration tests in `tests/` directory
**Impact**: Critical - Fixes were blocking 60 of 86 API tests
