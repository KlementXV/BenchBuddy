# BenchBuddy

One-shot Kubernetes cluster performance diagnostics.

## Quick Start

```bash
# 1. Create a namespace for benchmarks
kubectl create namespace benchbuddy

# 2. Run the quick profile
benchbuddy run --namespace benchbuddy --profile quick

# 3. Run with JSON and Markdown output
benchbuddy run --namespace benchbuddy --profile standard \
  --output json=report.json --output md=report.md
```

## Profiles

| Profile  | Approx. Duration | Use Case                     |
|----------|-----------------|------------------------------|
| `quick`  | ~2 min          | Fast smoke test              |
| `standard` | ~10 min       | Regular validation           |
| `deep`   | ~30 min         | Pre-production thoroughness  |

## Exit Codes

| Code | Meaning                                                  |
|------|----------------------------------------------------------|
| 0    | All benches passed, no HIGH or CRITICAL findings         |
| 1    | Benches completed, but ≥1 HIGH or CRITICAL finding found |
| 2    | Setup error (nothing was deployed)                       |
| 130  | Interrupted (Ctrl+C), cleanup completed                  |

## Output Formats

BenchBuddy always writes a terminal report. Additional formats:

```bash
--output json=path.json   # machine-readable JSON
--output md=report.md     # Markdown for sharing
```

## Utility Commands

```bash
# List orphaned runs (from crashed/interrupted sessions)
benchbuddy list-runs --namespace benchbuddy

# Delete a specific orphaned run
benchbuddy clean --namespace benchbuddy --run-id <id>

# Delete all orphaned runs older than 2 hours
benchbuddy clean --namespace benchbuddy --older-than 2h

# Show which images will be used
benchbuddy images list --profile standard
```

## Airgap Installation

BenchBuddy uses a single container image (`runner`) that bundles all bench tools.

### 1. Mirror the image

```bash
# Generate the skopeo mirror command
benchbuddy images list --format script --source-registry ghcr.io/clementlevoux/benchbuddy \
  --profile standard

# Example output:
# skopeo copy docker://ghcr.io/clementlevoux/benchbuddy/runner:v0.1.0 \
#             docker://registry.corp.internal/benchbuddy/runner:v0.1.0
```

### 2. Configure BenchBuddy to use your registry

```yaml
# benchbuddy.yaml
images:
  registry: registry.corp.internal/benchbuddy
  runner:
    repository: runner
    tag: v0.1.0
    pullPolicy: IfNotPresent
  pullSecrets:
    - regcred
```

```bash
benchbuddy run --namespace benchbuddy --config benchbuddy.yaml --profile quick
```

### 3. Disable external advisor refs (optional)

Advisor findings include documentation links. For strict airgap environments where you
do not want those URLs logged, simply ignore the `refs` fields — BenchBuddy never
fetches them.

## RBAC

BenchBuddy requires the following permissions:

```yaml
# Apply with: kubectl apply -f rbac.yaml
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: benchbuddy-runner
  namespace: benchbuddy  # change to your namespace
rules:
- apiGroups: [""]
  resources: ["pods", "pods/log", "persistentvolumeclaims", "configmaps", "services"]
  verbs: ["create", "get", "list", "watch", "delete"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: benchbuddy-reader
rules:
- apiGroups: [""]
  resources: ["nodes", "namespaces"]
  verbs: ["get", "list"]
- apiGroups: ["storage.k8s.io"]
  resources: ["storageclasses"]
  verbs: ["get", "list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: benchbuddy-runner
  namespace: benchbuddy
subjects:
- kind: ServiceAccount
  name: default
  namespace: benchbuddy
roleRef:
  kind: Role
  name: benchbuddy-runner
  apiGroup: rbac.authorization.k8s.io
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: benchbuddy-reader
subjects:
- kind: ServiceAccount
  name: default
  namespace: benchbuddy
roleRef:
  kind: ClusterRole
  name: benchbuddy-reader
  apiGroup: rbac.authorization.k8s.io
```

## Building

```bash
make build        # build binary
make test         # unit + integration tests
make docker-build # build runner image
```
