#!/usr/bin/env bash
set -euo pipefail

echo "🦎 Gecko — bootstrapping dependencies"
echo "────────────────────────────────────────"

cd "$(dirname "$0")/.."

echo "→ Fetching Kubernetes client-go..."
go get k8s.io/client-go@v0.31.0
go get k8s.io/apimachinery@v0.31.0

echo "→ Fetching Vault API..."
go get github.com/hashicorp/vault/api@v1.14.0

echo "→ Fetching Nomad API..."
go get github.com/hashicorp/nomad/api@latest || go get github.com/hashicorp/nomad@latest

echo "→ Tidying module graph..."
go mod tidy

echo "→ Building gecko..."
go build -o gecko .

echo ""
echo "✓ Done. Run ./gecko to get started."
