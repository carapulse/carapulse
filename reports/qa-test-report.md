# QA Test Coverage Report

**Date:** 2026-02-08
**Project:** Carapulse
**Go Version:** 1.25.5
**Overall Coverage:** 72.8% (before) -> improved after new tests

---

## 1. Coverage Analysis

### Package Coverage Summary

| Package | Coverage | Status |
|---------|----------|--------|
| `internal/approvals` | 100.0% | Excellent |
| `internal/audit` | 100.0% | Excellent |
| `internal/chatops` | 100.0% | Excellent |
| `internal/llm` | 100.0% | Excellent |
| `internal/metrics` | 100.0% | Excellent |
| `internal/policy` | 100.0% | Excellent |
| `internal/storage` | 100.0% | Excellent |
| `internal/db` | 94.9% | Excellent |
| `cmd/assistantctl` | 91.1% | Excellent |
| `internal/secrets` | 84.0% | Good |
| `internal/logging` | 81.0% | Good |
| `internal/context` | 79.6% | Good |
| `internal/web` | 78.5% | Good (improved from 70.7%) |
| `internal/config` | 75.7% | Good |
| `internal/tools` | 73.4% | Acceptable |
| `cmd/sandbox-exec` | 67.9% | Acceptable |
| `internal/workflows` | 67.2% | Acceptable (improved from 65.5%) |
| `internal/context/collectors` | 64.8% | Acceptable |
| `internal/auth` | 64.5% | Acceptable |
| `cmd/agent` | 63.8% | Acceptable |
| `cmd/migrate` | 56.7% | Below target |
| `cmd/orchestrator` | 40.9% | Below target |
| `cmd/tool-router` | 36.5% | Below target |
| `cmd/gateway` | 34.8% | Below target |
| `cmd/e2e` | 0.0% | Not unit tested (E2E runner) |

### Packages Below 60% Coverage

1. **`cmd/gateway`** (34.8%) - `run()` at 48.5%, `seedWorkflowCatalog()` at 28.0%. Main startup function with significant infrastructure wiring untested.
2. **`cmd/tool-router`** (36.5%) - `run()` at 36.2%. Same pattern: startup wiring.
3. **`cmd/orchestrator`** (40.9%) - `run()` at 38.6%, `Close()` at 0.0%. Temporal worker registration untested.
4. **`cmd/migrate`** (56.7%) - `run()` at 63.0%, `main()` at 0.0%. Database migration runner.

These are all main entry-point packages where the `run()` functions contain infrastructure wiring (server start, signal handling, DB connections). This is expected and acceptable -- these are integration concerns best tested via E2E.

---

## 2. Race Detection Results

### FOUND: 1 Data Race (FIXED)

**Location:** `internal/context/service_runtime_test.go`
**Test:** `TestContextServiceIngestLoop`
**Root Cause:** `storeStub` struct has unprotected `nodes` and `edges` slices. The `ingestLoop` goroutine writes to these slices via `UpsertContextNode`/`UpsertContextEdge`, while the test goroutine reads `len(store.nodes)` and `len(store.edges)` concurrently without synchronization.

**Fix Applied:** Added `sync.Mutex` to `storeStub` and protected all reads/writes to `nodes` and `edges` fields.

**File changed:** `internal/context/service_runtime_test.go`

After fix: `go test -race ./internal/context/...` passes cleanly.

---

## 3. Test Quality Assessment (5 Critical Packages)

### `internal/web` (78.5% coverage, 27 test files)

**Strengths:**
- Comprehensive handler testing for all major CRUD flows (plans, approvals, executions, schedules, playbooks, runbooks, sessions, workflows)
- Policy enforcement tested at allow/deny/require_approval/error decision paths
- Audit trail integration verified at plan creation
- Auth middleware tested with JWT parsing
- WebSocket/SSE handler coverage
- Constraint enforcement thoroughly tested

**Gaps (now partially addressed):**
- `handleOperatorMemory` / `handleOperatorMemoryByID` were 0% -- **now tested** (create, list, get, update, delete, policy deny, missing fields, store unavailable)
- `handleContextSnapshotByID` was 0% -- **now tested** (get, diff, missing base, missing tenant, policy deny, method not allowed)
- `EventGate.Accept` / `hashEvent` were 0% -- **now tested** (nil gate, nil store, allowed, denied, store error, severity filtering)
- `LoadOperatorMemory` / `fileMemoryID` were 0% -- **now tested**
- Still at 0%: `handleContextSnapshots` (list endpoint), `LoadRunbooks`, `UIHandler`, `NewScheduler.Run`, `DefaultWorkflowCatalog`, `WorkflowSpecFromTemplate`
- `handleSchedules` at 68.6% - PUT/DELETE paths partially untested
- `handleExecutionByID` at 61.0% - some error paths missing
- `requireSession` at 11.8% - most branches untested

### `internal/workflows` (67.2% coverage, 10 test files)

**Strengths:**
- Executor `RunOnce` success and failure paths well tested
- Every error path in `executeStep` tested (store errors, blob errors, tool errors, evidence errors)
- `tryRollback` error paths comprehensively covered
- Temporal workflow test via `testsuite` for success and failure
- All 5 workflow error scenarios tested (ArgoSync, ArgoWait, HelmUpgrade, HelmRollback, ScaleScale, ScaleRollout)
- Workflow template builder functions tested

**Gaps (now partially addressed):**
- `rollbackSteps` was 0% -- **now tested** (success, partial failure, empty steps)
- Verify-stage failure triggering multi-step rollback -- **now tested**
- Still at 0%: All Temporal activity implementations (`ExecuteStep`, `RollbackStep`, `UpdateExecutionStatus`, `CompleteExecution`, `CheckApproval`, `CreateExecution`, `contextToTools`)
- Still at 0%: All 5 Temporal workflow catalog functions (`GitOpsDeployWorkflowTemporal`, `HelmReleaseWorkflowTemporal`, etc.)
- Still at 0%: `StartExecution`, `convertPlanSteps`, `ReplayHistoryFromJSONFile`
- `extractExternalIDs` at 17.1%, `extractExternalIDsFromText` at 53.8%
- `PlanExecutionWorkflow` at 58.3%

### `internal/tools` (73.4% coverage, 25 test files)

**Strengths:**
- Router execution tested end-to-end
- CLI command builders well tested (kubectl, helm, argocd, aws, boundary)
- JWT/OIDC authentication tested
- Schema validation, redaction, artifact resolution all covered
- Sandbox execution tested with custom RunFunc injection
- Egress proxy tested including connection handling

**Gaps:**
- `BuildVaultCmd` at 23.1%, `BuildGitCmd` at 66.0%, `BuildGhCmd` at 43.6%, `BuildGlabCmd` at 40.0% - complex CLI builder branches
- `ExecuteAPI` at 61.2% - API fallback paths for various tools
- `Sandbox.Run` at 40.5%, `runContainer` at 63.0% - container execution paths
- `authenticate` (tool router server) at 38.5% - JWT verification paths
- Resource/prompt loader functions all at 0% (`ListResources`, `ListPrompts`, `loadRunbookResources`, etc.)
- `validateArgo` at 48.1%, `validateGit` at 24.0% - input validation branches

### `internal/db` (94.9% coverage, 16 test files)

**Strengths:**
- Near-complete coverage with comprehensive CRUD testing
- Uses `database/sql` driver-level fakes for isolation
- Error paths tested for most operations
- All table operations covered: plans, executions, approvals, schedules, playbooks, runbooks, sessions, workflow catalog, operator memory, context, audit

**Gaps:**
- `CreatePlaybook` at 64.7% - some error branches
- `Conn()` at 66.7% - error path
- Minor gaps in `CreateRunbook` (82.1%), `ListPlaybooks` (83.3%), `GetPlaybook`/`GetRunbook` (91.7%)

### `internal/policy` (100.0% coverage, 3 test files)

**Strengths:**
- Complete coverage of OPA evaluation
- Error paths tested (HTTP failures, invalid responses)
- Middleware `Evaluate` and `Check` both covered
- All policy decision types handled

**No gaps.**

---

## 4. E2E Tests

### What Exists

The E2E test runner lives in `cmd/e2e/main.go` (507 lines). It is a CLI tool, not a Go test file, so it does not contribute to `go test` coverage.

**Workflow coverage:**
- `gitops_deploy` - ArgoCD sync -> wait -> verify
- `helm_release` - Helm upgrade/rollback -> rollout status -> verify
- `scale_service` - kubectl scale -> rollout status -> verify
- `incident_remediation` - Prometheus/Tempo query -> annotation -> linear issue
- `secret_rotation` - Vault rotate -> ArgoCD sync -> verify

**Flow tested per workflow:**
1. `POST /v1/workflows/{name}/start` (creates plan)
2. `GET /v1/plans/{id}/diff` (plan diff view)
3. `GET /v1/plans/{id}/risk` (risk assessment)
4. `POST /v1/approvals` (approve plan)
5. `POST /v1/plans/{id}:execute` (start execution)
6. `GET /v1/executions/{id}` (poll until complete)

**Configuration:**
- Boot scripts: `scripts/e2e-up.sh`, `scripts/e2e-ports.sh`, `scripts/e2e-config.sh`
- Requires: Docker, kind, kubectl, helm
- Config: `e2e/config.sample.json`

### What's Not Covered by E2E

- Session management (create, join, leave)
- Operator memory CRUD
- Context snapshot diff
- Scheduler/cron execution
- Playbook/runbook CRUD
- WebSocket/SSE streaming
- ChatOps (Slack commands)
- Multi-tenant isolation
- Auth/OIDC flow

---

## 5. Critical Path Analysis

The critical user flow is: **create plan -> approve plan -> execute plan -> verify results**

### Step 1: Create Plan (`POST /v1/plans`)

**Test coverage:** Well tested in `http_test.go`:
- `TestHandlePlansCreateFlow` - happy path
- `TestHandlePlansReadRisk` - read risk classification
- `TestHandlePlansPlannerSuccess/Error` - LLM planner integration
- `TestHandlePlansAuditAllow/Deny` - audit trail
- `TestHandlePlansPolicyError` - policy evaluation failure
- `TestHandlePlansAutoApprove*` - auto-approval for low-risk
- `TestHandlePlansConstraints*` - constraint enforcement
- DB error paths tested via `errorDB`

### Step 2: Approve Plan (`POST /v1/approvals`)

**Test coverage:** Tested in `http_test.go`:
- `TestHandleApprovalsCreate` - approval creation
- `TestHandleApprovalsBadJSON/MissingPlanID` - input validation
- `TestHandleApprovalsDBError` - error handling
- `TestHandleApprovalsLinear*` - Linear integration
- Approval watcher tested in `internal/approvals/` (100% coverage)

### Step 3: Execute Plan (`POST /v1/plans/{id}:execute`)

**Test coverage:** Tested in multiple places:
- `TestHandlePlanByIDExecute` - HTTP handler
- `TestHandlePlanByIDExecuteDBError` - DB failure
- `TestHandlePlanByIDExecuteNotApproved` - approval gate
- `TestExecutorRunOnceSuccess` - executor happy path
- `TestExecutorRunOnceFailureRollback` - failure + rollback
- `TestExecutorExecutePlanUpdateError` - status update failure
- Temporal workflow: `TestPlanExecutionWorkflowSuccess/Failure`

### Step 4: Verify Results

**Test coverage:**
- Verify stage splitting tested in `TestSplitStepsByStage*`
- Verify step execution failure triggering rollback -- **now tested** (`TestExecutorVerifyStepFailureTrigersRollback`)
- Evidence recording tested (`TestExecutorRecordEvidence*`)

### Critical Path Gaps

1. **Temporal StartExecution** (0%) - The function that bridges HTTP handler to Temporal workflow has no test coverage. Currently only the direct `Executor` path is tested.
2. **Multi-step rollback** via `rollbackSteps` -- **now tested**
3. **End-to-end plan status transitions** - No test verifies the full state machine: `pending -> running -> succeeded/failed/rolled_back`

---

## 6. New Tests Written

### Race Condition Fix

**File:** `internal/context/service_runtime_test.go`
- Added `sync.Mutex` to `storeStub` to protect concurrent access to `nodes` and `edges` slices
- Updated `TestContextServiceIngestLoop` to use mutex when reading stub fields

### New Test Files

**`internal/web/operator_memory_test.go`** (18 tests):
- `TestHandleOperatorMemoryCreate` - happy path
- `TestHandleOperatorMemoryCreateMissingFields` - validation
- `TestHandleOperatorMemoryCreateInvalidJSON` - parse error
- `TestHandleOperatorMemoryCreatePolicyDeny` - policy enforcement
- `TestHandleOperatorMemoryList` - list endpoint
- `TestHandleOperatorMemoryListMissingTenant` - tenant required
- `TestHandleOperatorMemoryStoreUnavailable` - DB interface check
- `TestHandleOperatorMemoryMethodNotAllowed` - PATCH rejected
- `TestHandleOperatorMemoryByIDGet` - get by ID
- `TestHandleOperatorMemoryByIDGetNotFound` - 404
- `TestHandleOperatorMemoryByIDGetMissingTenant` - validation
- `TestHandleOperatorMemoryByIDUpdate` - PUT update
- `TestHandleOperatorMemoryByIDUpdateMissingFields` - validation
- `TestHandleOperatorMemoryByIDDelete` - DELETE
- `TestHandleOperatorMemoryByIDDeleteMissingTenant` - validation
- `TestHandleOperatorMemoryByIDDeletePolicyDeny` - policy
- `TestHandleOperatorMemoryByIDMethodNotAllowed` - PATCH rejected
- `TestHandleOperatorMemoryByIDStoreUnavailable` - DB check
- `TestHandleOperatorMemoryByIDMissingID` - empty ID

**`internal/web/operator_memory_files_test.go`** (4 tests):
- `TestLoadOperatorMemoryEmpty` - empty workspace
- `TestLoadOperatorMemoryMissingFile` - file not found
- `TestLoadOperatorMemoryValid` - valid file loading
- `TestLoadOperatorMemoryInvalidJSON` - parse error
- `TestFileMemoryID` - deterministic hashing

**`internal/web/event_gate_test.go`** (7 tests):
- `TestEventGateAcceptNilGate` - nil safety
- `TestEventGateAcceptNilStore` - nil store safety
- `TestEventGateAcceptAllowed` - allowed through
- `TestEventGateAcceptDenied` - denied
- `TestEventGateAcceptStoreError` - error handling
- `TestEventGateAcceptSeverityFilter` - severity allowlist (3 sub-cases)
- `TestHashEvent` - fingerprint consistency

**`internal/web/context_snapshots_test.go`** (10 tests):
- `TestHandleContextSnapshotByIDGet` - happy path
- `TestHandleContextSnapshotByIDMethodNotAllowed` - POST rejected
- `TestHandleContextSnapshotByIDNoTenant` - missing tenant
- `TestHandleContextSnapshotByIDNoDB` - nil DB
- `TestHandleContextSnapshotByIDPolicyDeny` - policy
- `TestHandleContextSnapshotByIDMissingID` - empty ID
- `TestParseSnapshotPayload` - valid parse
- `TestParseSnapshotPayloadInvalid` - invalid JSON
- `TestDiffSnapshots` - node/edge diff logic
- `TestHandleContextSnapshotByIDDiff` - diff endpoint
- `TestHandleContextSnapshotByIDDiffMissingBase` - missing base
- `TestHandleContextSnapshotByIDInvalidPath` - bad path

**`internal/workflows/executor_rollback_test.go`** (5 tests):
- `TestExecutorRollbackStepsSuccess` - multi-step rollback
- `TestExecutorRollbackStepsPartialFailure` - partial failure
- `TestExecutorRollbackStepsEmpty` - empty steps
- `TestExecutorVerifyStepFailureTrigersRollback` - verify failure triggers rollback
- `TestExecutorCompleteExecutionError` - completion error path

### Coverage Impact

| Package | Before | After | Delta |
|---------|--------|-------|-------|
| `internal/web` | 70.7% | 78.5% | +7.8% |
| `internal/workflows` | 65.5% | 67.2% | +1.7% |

---

## 7. Remaining Test Gaps (Priority Order)

### High Priority

1. **Temporal activities** (`internal/workflows/temporal_activities.go`) - 0% coverage. These are the bridge between Temporal and the executor. Each activity wraps a single tool operation.
2. **Temporal workflow catalog** (`internal/workflows/temporal_catalog.go`) - 0% coverage. The 5 durable workflow implementations.
3. **`StartExecution`** (`internal/workflows/starter.go`) - 0% coverage. Creates Temporal workflow execution.
4. **`handleExecutionByID`** (61.0%) - Error paths for SSE log streaming, missing execution.
5. **`handleSchedules`** (68.6%) - PUT/DELETE error paths.
6. **`requireSession`** (11.8%) - Session enforcement logic largely untested.

### Medium Priority

7. **`BuildVaultCmd`** (23.1%), **`BuildGitCmd`** (66.0%), **`BuildGhCmd`** (43.6%), **`BuildGlabCmd`** (40.0%) - CLI command builder branches.
8. **`validateArgo`** (48.1%), **`validateGit`** (24.0%) - Input validation branches.
9. **`Sandbox.Run`** (40.5%) - Container execution path.
10. **`ExecuteAPI`** (61.2%) - API fallback paths.
11. **`handleContextSnapshots`** list endpoint (0%) - Listing snapshots.
12. **Context collectors** - `AlertmanagerPoller.Snapshot` (0%), `DefaultK8sResources` (0%), `PollingWatcher` (0%).

### Low Priority

13. **`cmd/gateway/main.go run()`** (48.5%) - Infrastructure wiring. Best tested via E2E.
14. **`cmd/tool-router/main.go run()`** (36.2%) - Same.
15. **`cmd/orchestrator/main.go run()`** (38.6%) - Same.
16. **UI handlers** - Template rendering (`UIHandler`, `handleUIPlaybooks`, etc.).
17. **Resource loaders** - `ListResources`, `ListPrompts`, `loadRunbookResources`.

---

## 8. Recommendations

1. **Fix the data race** -- DONE. The `storeStub` in `internal/context/service_runtime_test.go` now uses a mutex.

2. **Write Temporal activity tests** -- The 0% coverage on all Temporal activities is the biggest gap. These can be tested with `testsuite.TestActivityEnvironment`.

3. **Write `StartExecution` test** -- This is the only untested function in the critical plan execution path.

4. **Add E2E coverage for sessions and memory** -- The E2E runner only tests workflow execution. Session management and operator memory need E2E scenarios.

5. **Consider test helpers** -- The `fakeDB` types are duplicated across many test files with slight variations. A shared test helper file could reduce boilerplate.

6. **CI enforcement** -- Add a coverage threshold check to CI (e.g., fail if any internal package drops below 60%).
