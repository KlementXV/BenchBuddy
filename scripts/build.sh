#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

VERSION="${VERSION:-dev}"
LDFLAGS="-X github.com/clementlevoux/benchbuddy/internal/cli.version=${VERSION}"

echo "==> Building benchbuddy (version=${VERSION})"
go build -ldflags "${LDFLAGS}" -o benchbuddy ./cmd/benchbuddy

echo "==> Building runner"
go build -ldflags "${LDFLAGS}" -o runner ./cmd/runner

if [[ "${WITH_IMAGE:-0}" == "1" ]]; then
  echo "==> Building runner Docker image"
  docker build -t "benchbuddy/runner:${VERSION}" -f images/runner/Dockerfile .
fi

echo "==> Done"
ls -lh benchbuddy runner
