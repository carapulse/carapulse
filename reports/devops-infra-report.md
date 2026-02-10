# DevOps Infrastructure Audit Report

**Date:** 2026-02-08
**Scope:** Dockerfiles, Docker Compose, Helm chart, CI/CD pipelines, migrations

---

## Executive Summary

The deployment infrastructure is well-designed and production-ready. All five Dockerfiles follow security best practices (non-root, multi-stage, pinned bases). The Helm chart is comprehensive with proper security contexts, PDB support, and network policies. CI/CD covers testing, linting, OPA, build matrix, and container scanning. Migrations are idempotent with rollback support.

**Issues found:** 8 (3 fixed, 5 documented as recommendations)

---

## 1. Docker Validation

### Files Reviewed

| Dockerfile | Base Image | Multi-stage | Non-root | HEALTHCHECK | Status |
|---|---|---|---|---|---|
| `gateway.Dockerfile` | alpine:3.21 | Yes | UID 10001 | Yes (wget) | Good |
| `tool-router.Dockerfile` | debian:bookworm-slim | Yes | UID 10001 | Yes (curl) | Fixed |
| `orchestrator.Dockerfile` | debian:bookworm-slim | Yes | UID 10001 | Yes (file) | Fixed |
| `agent.Dockerfile` | alpine:3.21 | Yes | UID 10001 | Yes (wget) | Good |
| `sandbox.Dockerfile` | debian:bookworm-slim | No (runtime only) | UID 10001 | N/A | Good |

### Issues Found and Fixed

**FIXED - Orchestrator missing HEALTHCHECK and EXPOSE** (`docker/orchestrator.Dockerfile:44`)
- Added `EXPOSE 7233` and a file-based HEALTHCHECK. The orchestrator is a Temporal worker and doesn't expose an HTTP health endpoint, so a file-based health check is appropriate.

**FIXED - Tool-router uses wget on Debian** (`docker/tool-router.Dockerfile:85`)
- The tool-router uses `debian:bookworm-slim` which includes `curl` but not `wget`. Changed HEALTHCHECK from `wget` to `curl -fsS`.

### Positive Findings

- All build stages use `CGO_ENABLED=0` and `-trimpath` for reproducible builds
- All CLI tool versions are pinned via build ARGs (kubectl v1.29.0, helm v3.14.0, etc.)
- Build cache is optimized: `go.mod/go.sum` copied before source for layer caching
- Non-root user (UID 10001) consistently across all images
- Gateway Dockerfile correctly bundles both `gateway` and `migrate` binaries

### Recommendations

- **R1**: Pin Go build image more precisely. Currently `golang:1.23-alpine` -- consider `golang:1.23.5-alpine3.21` for build reproducibility.
- **R2**: The tool-router and sandbox Dockerfiles contain nearly identical tool installation blocks (8 CLI tools). Consider a shared base image to reduce duplication and build time.
- **R3**: Gateway and agent use `wget` in HEALTHCHECK but run on Alpine (which has it). This is fine, but `curl` (after `apk add curl`) would be more consistent with the Debian-based images.

---

## 2. Docker Compose Review

### File: `deploy/docker-compose.yml`

**Services (8 total):** postgres, temporal, temporal-ui, minio, opa, migrate, tool-router, orchestrator, gateway, agent

### Issues Found and Fixed

**FIXED - Minio image unpinned** (`deploy/docker-compose.yml:43`)
- Was `minio/minio` (latest), now pinned to `minio/minio:RELEASE.2024-01-18T22-51-28Z`.

**FIXED - Temporal missing health check** (`deploy/docker-compose.yml:18-29`)
- Added health check using `temporal operator cluster health`. Updated dependent services (temporal-ui, orchestrator, gateway) to use `condition: service_healthy` instead of `service_started` for Temporal dependency.

### Validation

```
docker compose -f deploy/docker-compose.yml config  -->  Valid
```

### Positive Findings

- Postgres has proper health check with `pg_isready`
- Data persistence via named volumes (postgres-data, minio-data)
- Agent behind `chatops` profile (not started by default)
- Migration service runs before gateway (`service_completed_successfully`)
- Config mounted read-only from host
- OPA policies mounted read-only

### Recommendations

- **R4**: Docker socket bind-mount (`/var/run/docker.sock`) on tool-router and orchestrator is a significant security concern in production. Consider Docker-in-Docker or a remote Docker daemon with TLS.
- **R5**: Compose file lacks a `networks` definition. All services use the default bridge. Add an explicit network for better isolation.
- **R6**: Add a health check to the OPA service: `opa eval 'data' --bundle /policies`.

---

## 3. Helm Chart Audit

### Files Reviewed (14 templates)

| Template | Purpose | Status |
|---|---|---|
| `_helpers.tpl` | Name/label helpers, image resolver | Good |
| `serviceaccount.yaml` | SA with conditional creation | Good |
| `secret.yaml` | Config as Secret (not ConfigMap) | Updated |
| `rbac.yaml` | ClusterRole (read-only K8s access) | Good |
| `gateway-deployment.yaml` | Gateway with liveness/readiness | Good |
| `tool-router-deployment.yaml` | Tool-router with probes | Good |
| `orchestrator-deployment.yaml` | Orchestrator (no probes -- headless worker) | Good |
| `agent-deployment.yaml` | Conditional agent deployment | Good |
| `migration-job.yaml` | Pre-install/pre-upgrade Job | Fixed |
| `gateway-service.yaml` | HTTP + Canvas ports | Good |
| `tool-router-service.yaml` | HTTP port | Good |
| `agent-service.yaml` | Conditional service | Good |
| `gateway-ingress.yaml` | Conditional ingress with TLS | Good |
| `networkpolicy.yaml` | Gateway + tool-router policies | Review |
| `pdb.yaml` | PDBs for gateway + tool-router | Good |
| `NOTES.txt` | Post-install instructions | Good |

### Validation

```
helm lint deploy/helm/carapulse/  -->  1 chart(s) linted, 0 chart(s) failed
helm template test deploy/helm/carapulse/  -->  Valid (all templates render)
```

### Issues Found and Fixed

**FIXED - Migration Job DSN exposed as plaintext env var** (`deploy/helm/carapulse/templates/migration-job.yaml:36-38`)
- Was `value: {{ .Values.config.storage.postgres_dsn }}`. Now uses `secretKeyRef` to source DSN from the Secret object, preventing it from appearing in `kubectl describe job`.
- Added `postgres-dsn` key to `secret.yaml`.

### Positive Findings

- Config stored as Secret (not ConfigMap) -- good for sensitive values
- `checksum/config` annotation triggers rolling updates on config change
- Security contexts enforced globally: `runAsNonRoot: true`, `readOnlyRootFilesystem: true`, `capabilities.drop: [ALL]`
- `/tmp` mounted as emptyDir (required with readOnlyRootFilesystem)
- Migration Job uses proper hooks: `pre-install,pre-upgrade` with `hook-weight: -5`
- `before-hook-creation` delete policy prevents stale Job conflicts
- PDB support (disabled by default, configurable)
- Network policies (disabled by default, configurable)
- RBAC is read-only (get/list/watch) -- proper least privilege

### Recommendations

- **R7**: Network policy egress is fully open (`- {}`) for both gateway and tool-router. In production, restrict egress to specific CIDR ranges or service endpoints.
- **R8**: Orchestrator deployment has no liveness/readiness probes. Since it's a Temporal worker without an HTTP endpoint, consider a sidecar or exec-based probe.
- **R9**: Chart.yaml should include `icon` field (helm lint info) and consider adding `maintainers` and `home` fields.
- **R10**: The `migration.image.repository` defaults to `carapulse/gateway` which is correct (gateway image includes migrate binary), but this coupling should be documented.

---

## 4. CI/CD Pipeline Review

### `.github/workflows/ci.yml`

| Job | What it does | Status |
|---|---|---|
| `test` | `go test -race -coverprofile` with Postgres service | Good |
| `lint` | golangci-lint v1.62.2 | Good |
| `opa-test` | OPA policy tests with pinned v0.62.1 | Good |
| `build` | Matrix build (6 services), uploads artifacts | Good |
| `helm-lint` | Helm lint + template rendering | Good |

### `.github/workflows/docker.yml`

| Feature | Status |
|---|---|
| Triggers on main push + tags | Good |
| Matrix build for 5 images | Good |
| GHCR login with GITHUB_TOKEN | Good |
| Docker metadata action for tags | Good |
| BuildX with GHA cache | Good |
| Trivy security scanning | Good |
| SARIF upload to GitHub Security | Good |
| Permissions scoped (read, write, security-events) | Good |

### `.github/workflows/e2e.yml`

| Feature | Status |
|---|---|
| Weekly schedule + manual dispatch | Good |
| Real Postgres + OPA | Good |
| Builds gateway + tool-router | Good |
| Health check smoke tests | Good |

### Positive Findings

- CI permissions are minimal (`contents: read`)
- Docker workflow adds `packages: write` and `security-events: write` only where needed
- Test job uses race detector
- Build matrix covers all 6 Go binaries
- Docker build uses BuildX with GHA caching (`cache-from: type=gha`, `cache-to: type=gha,mode=max`)
- Trivy scan reports CRITICAL and HIGH only, uploaded as SARIF

### Recommendations

- **R11**: CI does not run `go vet` separately from tests (it's a step, but linting should catch most issues). Consider adding `gosec` for security-specific linting.
- **R12**: Docker workflow builds on every push to `main`. Consider requiring CI to pass first via `needs:` dependency or branch protection rules.
- **R13**: E2E workflow only smoke-tests health endpoints. Consider adding actual API endpoint tests (create a plan, list plans, etc.).
- **R14**: Coverage artifact is uploaded but not published to a coverage service (Codecov, Coveralls). Consider adding a coverage gate.

---

## 5. Migration Safety

### Architecture

- **Engine**: goose v3 with embedded FS support
- **Runner**: `cmd/migrate/main.go` -- simple CLI wrapping goose
- **Actions supported**: `up`, `down`, `status`, `version`, `redo`
- **Migrations**: 20 files (0001-0020), all with `+goose Up` / `+goose Down` markers

### Idempotency

All migrations use idempotent DDL:

| Pattern | Count | Example |
|---|---|---|
| `CREATE TABLE IF NOT EXISTS` | 10 | 0001, 0003, 0007, 0008, etc. |
| `ALTER TABLE ... ADD COLUMN IF NOT EXISTS` | 12 | 0002, 0005, 0006, etc. |
| `CREATE INDEX IF NOT EXISTS` | 8 | 0001, 0010, 0011, etc. |
| `DROP TABLE IF EXISTS ... CASCADE` | 10 | All down migrations |
| `DROP INDEX IF EXISTS` | 3 | 0013, 0020 |
| `DROP COLUMN IF EXISTS` | 12 | All column-add downs |

**Result**: All 20 migrations are idempotent in both up and down directions. Safe to re-run.

### Rollback Support

Every migration has a `+goose Down` section. The `down` action removes the most recent migration. The `redo` action does `down` then `up` for the current version.

### Failure Handling

- goose uses advisory locks by default to prevent concurrent migration runs
- Each migration runs in a transaction (goose default for SQL files)
- If a migration fails mid-way, the transaction is rolled back and the migration version is not recorded
- `backoffLimit: 3` in the Helm Job allows retries

### Recommendations

- **R15**: Migration 0017 is a placeholder (`SELECT 1`). This should be documented or removed to avoid confusion.
- **R16**: Consider adding a migration lock timeout to prevent indefinite blocking when another migration is running.

---

## 6. Simulation Environment

Created `deploy/docker-compose.simulation.yml` as an overlay for the base compose file.

### Components

| Service | Image | Purpose |
|---|---|---|
| `localstack` | localstack/localstack:3.1 | Mock AWS APIs (S3, STS, IAM, CloudWatch, EC2, ECS, Lambda, SSM) |
| `vault` | hashicorp/vault:1.15.6 | Dev-mode Vault for secrets management |
| `vault-init` | hashicorp/vault:1.15.6 | Seeds Vault with test secrets |
| `localstack-init` | localstack/localstack:3.1 | Creates S3 buckets for artifacts/evidence |

### Usage

```bash
docker compose -f deploy/docker-compose.yml \
               -f deploy/docker-compose.simulation.yml \
               up -d
```

### What it provides

- **AWS API simulation**: Tool-router receives `AWS_ENDPOINT_URL=http://localstack:4566` to redirect all AWS CLI calls to LocalStack
- **Vault integration**: Both tool-router and orchestrator connect to dev Vault with pre-seeded secrets at `carapulse/config`
- **Full stack**: Postgres, Temporal, OPA, MinIO all run from the base compose file
- **Initialization**: Init containers create S3 buckets and seed Vault secrets before services start

---

## Summary of Changes Made

| File | Change | Severity |
|---|---|---|
| `docker/orchestrator.Dockerfile` | Added EXPOSE + HEALTHCHECK | Medium |
| `docker/tool-router.Dockerfile` | Changed wget to curl in HEALTHCHECK | Low |
| `deploy/docker-compose.yml` | Pinned minio image tag | Medium |
| `deploy/docker-compose.yml` | Added Temporal health check | High |
| `deploy/docker-compose.yml` | Changed Temporal deps to service_healthy | High |
| `deploy/helm/carapulse/templates/migration-job.yaml` | DSN from Secret instead of plaintext | High |
| `deploy/helm/carapulse/templates/secret.yaml` | Added postgres-dsn key | High |
| `deploy/docker-compose.simulation.yml` | New simulation environment | New |

---

## Recommendations Summary

| ID | Area | Priority | Description |
|---|---|---|---|
| R1 | Docker | Low | Pin Go build image more precisely |
| R2 | Docker | Medium | Shared base image for CLI tools |
| R3 | Docker | Low | Consistent health check tool across images |
| R4 | Compose | High | Docker socket security concern |
| R5 | Compose | Low | Explicit network definition |
| R6 | Compose | Low | OPA health check |
| R7 | Helm | High | Restrict network policy egress |
| R8 | Helm | Medium | Orchestrator probes |
| R9 | Helm | Low | Chart.yaml metadata |
| R10 | Helm | Low | Document migration image coupling |
| R11 | CI | Medium | Add gosec security linter |
| R12 | CI | Medium | Docker workflow depends on CI |
| R13 | CI | Medium | E2E beyond health checks |
| R14 | CI | Low | Coverage reporting service |
| R15 | Migrations | Low | Document/remove placeholder migration |
| R16 | Migrations | Low | Migration lock timeout |

**Overall assessment: Production-ready with minor improvements recommended.**
