# OPA Policy Fixes Report

## Summary

All 7 Rego policy files and 1 test file in `policies/policy/assistant/v1/` had syntax errors that caused OPA to reject them at load time. This meant **all policy evaluations failed**, effectively blocking every write operation (and potentially reads too, depending on error handling in the gateway).

The root cause: policies were written using a mix of Rego v1 `if` keyword syntax with invalid constructs (`and` keyword, parenthesized `or`, inline `else` chains, `input` variable shadowing). None of these are valid in any OPA version.

## Files Changed

### Policy Files (7 files)

| File | Errors Found | Fix Applied |
|------|-------------|-------------|
| `base.rego` | Invalid inline `else` chain; `default constraints := {}` conflicts with partial object rules | Split into multi-line `else` blocks; removed conflicting default |
| `read.rego` | `and` keyword (not valid Rego); single-line rule body | Converted to braced multi-line rule body with newline-separated conditions |
| `write.rego` | `and` keyword; parenthesized `(prod_env or blast_radius == "account")` | Split `or` into separate rules; converted all conditions to braced bodies |
| `environment.rego` | `and` keyword with `not` in rule bodies | Converted to braced multi-line rule bodies |
| `actors.rego` | `and not` in rule body | Converted to braced multi-line rule body |
| `break_glass.rego` | `and` keyword; `and not` in rule bodies | Converted to braced multi-line rule bodies |
| `helpers.rego` | No syntax errors | No changes needed |

### Test File (1 file)

| File | Errors Found | Fix Applied |
|------|-------------|-------------|
| `base_test.rego` | `input` variable shadowing (illegal in Rego v1); operator precedence bug in `with input as` assertion | Renamed local var to `inp`; fixed `constraints.break_glass_required` assertion to use intermediate variable |

### Infrastructure Files (4 files)

| File | Change |
|------|--------|
| `.github/workflows/ci.yml` | OPA version: `v0.62.1` -> `v1.4.2` |
| `.github/workflows/e2e.yml` | OPA version: `v0.62.1` -> `v1.4.2` |
| `deploy/docker-compose.yml` | OPA image: `openpolicyagent/opa:0.62.1` -> `openpolicyagent/opa:1.4.2` |
| `deploy/helm/carapulse/README.md` | OPA requirement: `0.62+` -> `1.0+` |

## Detailed Error Analysis

### Error 1: Invalid `and` keyword (5 files, ~12 occurrences)

**Before:**
```rego
allow_read if input.action.type == "read" and authenticated_actor
```

**Problem:** Rego does not have an `and` keyword. Conditions in rule bodies are implicitly ANDed by placing them on separate lines (or separated by `;`).

**After:**
```rego
allow_read if {
    input.action.type == "read"
    authenticated_actor
}
```

### Error 2: Parenthesized `or` (write.rego)

**Before:**
```rego
require_approval_write if write_action and (prod_env or blast_radius == "account") ...
```

**Problem:** Rego does not support `or` as an infix operator. Disjunctions are expressed as multiple rule definitions.

**After:**
```rego
require_approval_write if {
    write_action
    prod_env
    write_role_ok
    not deny_write
}

require_approval_write if {
    write_action
    blast_radius == "account"
    write_role_ok
    not deny_write
}
```

### Error 3: Inline `else` chain (base.rego)

**Before:**
```rego
decision := "allow" if allow_read else := "allow" if allow_low_write else := "require_approval" if require_approval_write else := "deny"
```

**Problem:** While Rego v1 supports `else` chains, they must use braced blocks on separate lines.

**After:**
```rego
decision := "allow" if {
    allow_read
} else := "allow" if {
    allow_low_write
} else := "require_approval" if {
    require_approval_write
}
```

### Error 4: `default constraints := {}` conflicts with partial object rules

**Before:** `base.rego` had `default constraints := {}` while `write.rego` and `break_glass.rego` defined `constraints["break_glass_required"] := true` and `constraints["break_glass_used"] := true`.

**Problem:** In Rego v1, a complete rule (`default constraints := {}`) cannot coexist with partial object rules (`constraints["key"] := value`). They have incompatible types.

**Fix:** Removed `default constraints := {}`. The `constraints` object is now defined solely by the partial object rules. When no constraints apply, the field is simply absent from the OPA response (the Go code already handles this via `map[string]any`).

### Error 5: `input` variable shadowing in tests

**Before:**
```rego
test_read_allows if {
    input := {"actor":{"id":"a",...},...}
    decision with input as input == "allow"
}
```

**Problem:** Rego v1 forbids local variables from shadowing the built-in `input`. All 8 tests used `input` as their local variable name.

**After:** Renamed to `inp` in all tests.

### Error 6: Operator precedence in `with` assertion

**Before:**
```rego
constraints.break_glass_required with input as inp == true
```

**Problem:** Parsed as `constraints.break_glass_required with input as (inp == true)`, setting input to `false`.

**After:**
```rego
bg := constraints.break_glass_required with input as inp
bg == true
```

### Error 7: OPA version mismatch

The policies use Rego v1 syntax (`if` keyword in rule heads, braced rule bodies). CI and Docker Compose referenced OPA v0.62.1, which does not support Rego v1. Updated all references to OPA v1.4.2.

## Policy Logic Review

The policy decision flow is correct and well-structured:

1. **Default deny** - `decision := "deny"` unless explicitly overridden
2. **Read path** - Authenticated actors (any `actor.id`) can read; TTL=300s for caching
3. **Low-risk write** - Allowed directly for non-prod, non-account-wide, low-risk writes by admin/operator
4. **Approval-required write** - Required for:
   - Production environment writes
   - Account-wide blast radius
   - Medium or high risk level
   - Break-glass tier actions
5. **Deny conditions** - Writes denied for:
   - Unknown environments
   - Missing operator/admin role
   - Production writes targeting >10 resources
6. **Constraints** - `break_glass_required` flag set for high-risk writes; `break_glass_used` annotated when break-glass is active

## Verification

```
$ opa check policies/
(no errors)

$ opa test policies/ -v
PASS: 8/8

$ go test ./internal/policy/...
ok  carapulse/internal/policy
```

## Impact

Before this fix: OPA could not load any policy files. Every policy evaluation returned an error, which depending on the gateway's error handling either:
- Denied all operations (fail-closed), or
- Allowed the `Evaluator.Check()` fallback (when `Checker == nil`, it returns `allow`)

After this fix: OPA correctly loads all policies. Read operations are allowed for authenticated actors. Write operations follow the intended risk-based decision tree with proper approval gates.
