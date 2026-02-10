# Carapulse Live Test Results

**Date:** 2026-02-10 14:01:46 UTC
**Test Mode:** In-process with httptest (no external deps)

## Summary

- **Total endpoints tested:** 34
- **Passed:** 34
- **Failed:** 0

## Detailed Results

| Method | Endpoint | Status | Result | Notes |
|--------|----------|--------|--------|-------|
| GET | /healthz | 200 | PASS |  |
| GET | /readyz | 200 | PASS |  |
| GET | /metrics | 200 | PASS |  |
| POST | /v1/plans | 200 | PASS |  |
| GET | /v1/plans/plan_1 | 200 | PASS |  |
| GET | /v1/plans/plan_1/diff | 200 | PASS |  |
| GET | /v1/plans/plan_1/risk | 200 | PASS |  |
| GET | /v1/plans/nonexistent | 404 | PASS |  |
| POST | /v1/approvals | 200 | PASS |  |
| POST | /v1/plans/plan_1:execute | 200 | PASS |  |
| GET | /v1/schedules | 200 | PASS |  |
| POST | /v1/schedules | 200 | PASS |  |
| POST | /v1/context/refresh | 200 | PASS |  |
| GET | /v1/context/services | 200 | PASS |  |
| GET | /v1/context/snapshots | 200 | PASS |  |
| GET | /v1/context/graph?service=api-gateway | 200 | PASS |  |
| POST | /v1/hooks/alertmanager | 200 | PASS |  |
| POST | /v1/hooks/argocd | 200 | PASS |  |
| POST | /v1/hooks/git | 200 | PASS |  |
| POST | /v1/hooks/k8s | 200 | PASS |  |
| POST | /v1/playbooks | 200 | PASS |  |
| GET | /v1/playbooks | 200 | PASS |  |
| POST | /v1/runbooks | 200 | PASS |  |
| GET | /v1/runbooks | 200 | PASS |  |
| GET | /v1/workflows | 200 | PASS |  |
| POST | /v1/sessions | 400 | PASS |  |
| GET | /v1/sessions | 200 | PASS |  |
| GET | /v1/audit/events | 200 | PASS |  |
| POST | /v1/plans (no auth) | 401 | PASS |  |
| POST | /v1/memory | 503 | PASS |  |
| GET | /v1/memory | 503 | PASS |  |
| DELETE | /v1/plans | 405 | PASS |  |
| POST | /v1/plans (invalid json) | 400 | PASS |  |
| GET | /v1/executions/exec_1/logs | 200 | PASS |  |

## Findings

- All endpoints passed.
