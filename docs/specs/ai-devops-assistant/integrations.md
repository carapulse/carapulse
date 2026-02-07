# Integrations

## Tooling rule
- Use CLI as primary interface for every tool.
- If CLI unavailable in sandbox, fallback to HTTP API.

## AWS
- CLI: `aws` (primary)
- Auth: AssumeRole with session name
- Read: Resource Groups Tagging `GetResources`, CloudTrail Event History
- Write: EKS/EC2/IAM/CloudWatch actions per policy
- Evidence: CloudTrail event IDs, AWS API responses

## Vault
- CLI: `vault` (primary)
- Auth: Kubernetes auth or AppRole
- Read: KV, lease status
- Write: secret rotation, renew, revoke
- Evidence: audit device entries

## Kubernetes
- CLI: `kubectl` (primary)
- Auth: kubeconfig contexts, RBAC
- Read: list/watch, events, rollout status
- Write: scale, rollout restart, patch (break-glass only)
- Evidence: kubectl outputs, resource versions

## Helm
- CLI: `helm` (primary)
- Auth: kubeconfig context
- Read: `helm list`, `helm status`, `helm get values`
- Write: `helm upgrade --install`, `helm rollback`
- Evidence: release revision, status, diff (if enabled)
- Values source: `values_ref` from Git path or object store artifact
## Boundary
- CLI: `boundary` (primary)
- Auth: Boundary auth token
- Read: targets, sessions
- Write: session open/close
- Evidence: session IDs

## Argo CD
- CLI: `argocd` (primary)
- Auth: JWT
- Read: app status, health, sync state
- Write: app sync, rollback, wait
- Evidence: sync IDs, app status

## Prometheus / Thanos
- API only (no standard CLI)
- Read: `/api/v1/query`, `/api/v1/query_range`, `/api/v1/rules`
- Evidence: query results, timestamps

## Grafana
- API only (no standard CLI)
- Read: dashboards, data sources
- Write: annotations only by default
- Evidence: annotation IDs

## Tempo
- API only (no standard CLI)
- Read: traces by ID, TraceQL
- Evidence: trace IDs, span counts

## GitHub/GitLab
- CLI: `gh` / `glab` if available; API fallback
- Auth: app token or PAT scoped to repo
- Write: create branch, commit, open PR/MR
- Evidence: PR URL, commit SHA

## OpenAI API (API key)
- Auth: `OPENAI_API_KEY` (preferred) or config `llm.api_key` (if wired)
- LLM provider: `openai`

## OpenAI Codex (ChatGPT subscription)
- Auth: ChatGPT sign-in via Codex CLI (`codex --login`) with credentials stored in `~/.codex/auth.json`
- Import: `assistantctl llm auth import --codex-auth <path>` writes `~/.carapulse/auth-profiles.json`
- Login helper: `assistantctl llm auth login [--codex-auth <path>]` runs Codex CLI login then imports
- LLM provider: `openai-codex` (OAuth access token, not API key)
- Env fallback: `OPENAI_ACCESS_TOKEN`
- Profile selection: `CARAPULSE_AUTH_PROFILE`, `CARAPULSE_AUTH_PATH`
- If Codex CLI stores creds in OS keyring, set `cli_auth_credentials_store = "file"` so `auth.json` is written.
- Treat `auth-profiles.json` as the token sink to avoid refresh token churn from multiple consumers.
- OpenClaw reference: OAuth PKCE login for `openai-codex`, stores tokens in `auth-profiles.json` as a token sink.
- OpenClaw token path: `~/.openclaw/agents/<agentId>/agent/auth-profiles.json` (legacy `~/.openclaw/agent/auth-profiles.json`)
- Import from OpenClaw: `assistantctl llm auth import --openclaw-auth <path>` writes `~/.carapulse/auth-profiles.json`

## Linear
- API only (no standard CLI)
- Auth: API token
- Write: create approval issues, update labels
- Evidence: issue ID + label history

## PagerDuty
- API only (no standard CLI)
- Auth: Events v2 API key
- Write: trigger/resolve incidents
- Evidence: incident ID

## OIDC
- Auth: user login, JWT claims to roles
- Required claims: `sub`, `email`, `groups`


## Required permissions (minimum)
- AWS: sts:AssumeRole, tagging:GetResources, cloudwatch:GetMetricData, cloudtrail:LookupEvents
- Vault: auth/kubernetes/login or auth/approle/login, read/write on target paths
- Kubernetes: get/list/watch on core/apps, update on deployments for scale
- Helm: namespace-scoped releases, list/status/get/upgrade/rollback
- Argo CD: app get, app sync, app rollback
- Grafana: annotations:read, annotations:create
- GitHub/GitLab: repo write, pull_request create
- Linear: issues:write, comments:write, labels:write
- PagerDuty: events:write
