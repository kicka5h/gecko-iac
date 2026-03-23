#!/usr/bin/env bash
set -euo pipefail

echo "🦎 Gecko — bootstrapping dependencies"
echo "────────────────────────────────────────"

cd "$(dirname "$0")/.."

echo "→ Fetching Proxmox client..."
go get github.com/luthermonson/go-proxmox@v0.3.2

echo "→ Tidying module graph..."
go mod tidy

echo "→ Building gecko..."
go build -o gecko .

echo ""
echo "✓ Done. Run ./gecko to get started."
