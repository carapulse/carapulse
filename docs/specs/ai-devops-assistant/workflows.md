# Workflows and playbooks

## Shared lifecycle
- Trigger -> Diagnose -> Plan -> Approval (required for writes by default) -> Execute -> Verify -> Annotate -> Close
- All workflows produce evidence and audit events

## Workflow catalog

### GitOpsDeployWorkflow
Input:
```yaml
DeployInput:
  service: string
  argocd_app: string
  context: ContextRef
  revision: string|null
  strategy: enum[sync,rollback]
```
Steps:
- Validate context
- Argo dry-run/preview
- Approval if medium/high
- Argo sync
- Argo wait healthy
- Verify with PromQL SLO query
- Grafana annotation
Rollback:
- Argo rollback to previous revision

### HelmReleaseWorkflow
Input:
```yaml
HelmInput:
  release: string
  chart: string
  version: string|null
  values_ref: ArtifactRef|null
  namespace: string
  context: ContextRef
  strategy: enum[upgrade,rollback]
```
Steps:
- Validate context
- Resolve values_ref (git path or object store); default chart values if null
- Helm status to capture current revision
- Approval (writes default)
- Helm upgrade --install OR Helm rollback
- Helm status + kubectl rollout status (if workloads changed)
- Verify with PromQL SLO query
Rollback:
- Helm rollback to previous revision

### ScaleServiceWorkflow
Input:
```yaml
ScaleInput:
  service: string
  context: ContextRef
  replicas: int
  max_delta: int
```
Steps:
- Check current replicas
- Evaluate policy (limit max_delta)
- kubectl scale with preconditions
- kubectl rollout status
- Verify with PromQL latency/err rate
Rollback:
- Scale back to previous replicas

### IncidentRemediationWorkflow
Input:
```yaml
IncidentInput:
  alert_id: string
  service: string
  context: ContextRef
```
Steps:
- Pull alert + rules
- Query PromQL range
- Query exemplars -> trace_id
- Query Tempo trace/TraceQL
- Propose plan (restart/scale/sync)
- Approval if medium/high
- Execute remediation
- Verify metrics + traces
- PagerDuty resolve + Grafana annotation
Rollback:
- If remediation failed, revert change and page human

### SecretRotationWorkflow
Input:
```yaml
SecretRotationInput:
  secret_path: string
  context: ContextRef
  target: string
```
Steps:
- Rotate in Vault
- Update dependent workloads via GitOps
- Verify app health
Rollback:
- Revert GitOps + previous secret version

## Activities (Temporal)
- `QueryPrometheusActivity`
- `QueryTempoActivity`
- `ArgoSyncActivity`
- `ArgoWaitActivity`
- `HelmUpgradeActivity`
- `HelmRollbackActivity`
- `HelmStatusActivity`
- `KubectlScaleActivity`
- `KubectlRolloutStatusActivity`
- `CreateGrafanaAnnotationActivity`
- `CreateLinearIssueActivity`
- `CreatePagerDutyIncidentActivity`
- `CreateGitPullRequestActivity`

## Retry policy
- Activities retry with exponential backoff
- Workflow deterministic, no external time source without Temporal timers


## Evidence templates
- Argo sync evidence: app status, revision, sync ID
- K8s rollout evidence: deployment status, replicas, events
- PromQL evidence: error rate, latency p95, saturation
- Tempo evidence: trace count, error spans
