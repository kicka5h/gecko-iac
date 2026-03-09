#!/usr/bin/env bash
set -euo pipefail

echo "🦎 Gecko — packaging Scute VSCode extension"
echo "────────────────────────────────────────────"

cd "$(dirname "$0")/../editors/vscode-scute"

if ! command -v npm &> /dev/null; then
  echo "✗ npm not found — install Node.js from https://nodejs.org"
  exit 1
fi

echo "→ Installing dependencies..."
npm install

echo "→ Packaging extension..."
npm run package

VSIX=$(ls dist/*.vsix 2>/dev/null | head -1)
if [ -z "$VSIX" ]; then
  echo "✗ Package failed — no .vsix found in dist/"
  exit 1
fi

echo ""
echo "✓ Packaged: $VSIX"
echo ""

# Install locally if editors are present
if command -v code &> /dev/null; then
  echo "→ Installing in VSCode..."
  code --install-extension "$VSIX"
fi

if command -v cursor &> /dev/null; then
  echo "→ Installing in Cursor..."
  cursor --install-extension "$VSIX"
fi

echo ""
echo "✓ Done. Open any .scute file to see highlighting."
