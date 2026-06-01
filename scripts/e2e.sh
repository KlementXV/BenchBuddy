#!/usr/bin/env bash
set -euo pipefail

CLUSTER=${CLUSTER:-benchbuddy-e2e}
IMAGE=${IMAGE:-benchbuddy-runner:e2e}

cleanup() {
  kind delete cluster --name "$CLUSTER" >/dev/null 2>&1 || true
}
trap cleanup EXIT

# Build runner image and main binary.
docker build -t "$IMAGE" -f images/runner/Dockerfile .
go build -o benchbuddy ./cmd/benchbuddy

# Spin up kind and load the image.
kind create cluster --name "$CLUSTER" --config test/e2e/kind-config.yaml
kind load docker-image "$IMAGE" --name "$CLUSTER"

# Run the e2e test.
export KUBECONFIG="$(kind get kubeconfig-path --name "$CLUSTER" 2>/dev/null || echo "")"
if [ -z "$KUBECONFIG" ]; then
  KUBECONFIG=$(mktemp)
  kind get kubeconfig --name "$CLUSTER" > "$KUBECONFIG"
  export KUBECONFIG
fi

go test ./test/e2e/... -tags=e2e -timeout=15m -count=1 -v
