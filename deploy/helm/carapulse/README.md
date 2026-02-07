# Carapulse Helm Chart

Deploys Carapulse to Kubernetes.

## Prerequisites

- Kubernetes 1.27+
- Helm 3.14+
- PostgreSQL 15 (external)
- Temporal 1.22+ (external)
- OPA 0.62+ (external, optional)

## Installation

```bash
helm install carapulse ./deploy/helm/carapulse/ \
  --namespace carapulse \
  --create-namespace \
  -f my-values.yaml
```

## Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `global.image.registry` | Image registry | `ghcr.io` |
| `global.image.tag` | Image tag | Chart appVersion |
| `config` | Inline JSON config (stored as Secret) | See values.yaml |
| `gateway.replicaCount` | Gateway replicas | `1` |
| `gateway.resources` | Gateway resource requests/limits | 100m/128Mi - 500m/512Mi |
| `gateway.ingress.enabled` | Enable Ingress | `false` |
| `gateway.pdb.enabled` | Enable PodDisruptionBudget | `false` |
| `toolRouter.replicaCount` | Tool Router replicas | `1` |
| `toolRouter.pdb.enabled` | Enable PodDisruptionBudget | `false` |
| `orchestrator.replicaCount` | Orchestrator replicas | `1` |
| `agent.enabled` | Enable ChatOps agent | `false` |
| `migration.enabled` | Run DB migrations | `true` |
| `serviceAccount.create` | Create ServiceAccount | `true` |
| `networkPolicy.enabled` | Enable NetworkPolicies | `false` |

## External Dependencies

This chart does **not** include PostgreSQL, Temporal, or OPA. Deploy them separately:

```bash
# PostgreSQL
helm install postgres bitnami/postgresql --set auth.database=carapulse

# Temporal
helm install temporal temporalio/temporal

# OPA (optional)
helm install opa opa/opa
```

## Security

- All pods run as non-root (UID 10001)
- Read-only root filesystem
- All capabilities dropped
- Config stored as Kubernetes Secret
- RBAC with least-privilege ClusterRole (when serviceAccount.create=true)
