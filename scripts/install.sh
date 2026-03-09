#!/usr/bin/env sh
set -e

# 🦎 Gecko installer
# Usage: curl -fsSL https://raw.githubusercontent.com/gecko-iac/gecko/main/scripts/install.sh | sh

REPO="gecko-iac/gecko"
BINARY="gecko"
INSTALL_DIR="${GECKO_INSTALL_DIR:-/usr/local/bin}"

# ── Helpers ───────────────────────────────────────────────────────────────────

info()  { printf "  \033[36m→\033[0m %s\n" "$1"; }
ok()    { printf "  \033[32m✓\033[0m %s\n" "$1"; }
err()   { printf "  \033[31m✗\033[0m %s\n" "$1" >&2; exit 1; }

need() {
  command -v "$1" > /dev/null 2>&1 || err "Required tool not found: $1"
}

# ── Detect OS and architecture ────────────────────────────────────────────────

detect_os() {
  case "$(uname -s)" in
    Darwin) echo "darwin" ;;
    Linux)  echo "linux"  ;;
    MINGW*|MSYS*|CYGWIN*) echo "windows" ;;
    *) err "Unsupported OS: $(uname -s)" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo "amd64" ;;
    arm64|aarch64) echo "arm64" ;;
    *) err "Unsupported architecture: $(uname -m)" ;;
  esac
}

# ── Fetch latest release version ──────────────────────────────────────────────

latest_version() {
  need curl
  curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep '"tag_name"' \
    | sed 's/.*"tag_name": *"v\([^"]*\)".*/\1/'
}

# ── Main ──────────────────────────────────────────────────────────────────────

main() {
  printf "\n  \033[32;1m🦎 gecko installer\033[0m\n"
  printf "  \033[90m────────────────────────────────────────\033[0m\n\n"

  need curl
  need tar

  OS=$(detect_os)
  ARCH=$(detect_arch)

  VERSION="${GECKO_VERSION:-$(latest_version)}"
  [ -z "$VERSION" ] && err "Could not determine latest version. Set GECKO_VERSION to install a specific version."

  info "Installing gecko v${VERSION} (${OS}/${ARCH})"

  # Build download URL
  EXT="tar.gz"
  [ "$OS" = "windows" ] && EXT="zip"
  FILENAME="gecko_${VERSION}_${OS}_${ARCH}.${EXT}"
  URL="https://github.com/${REPO}/releases/download/v${VERSION}/${FILENAME}"
  CHECKSUM_URL="https://github.com/${REPO}/releases/download/v${VERSION}/gecko_${VERSION}_checksums.txt"

  # Download to temp dir
  TMP="$(mktemp -d)"
  trap 'rm -rf "$TMP"' EXIT

  info "Downloading ${FILENAME}..."
  curl -fsSL "$URL" -o "${TMP}/${FILENAME}" || err "Download failed: ${URL}"

  # Verify checksum
  info "Verifying checksum..."
  curl -fsSL "$CHECKSUM_URL" -o "${TMP}/checksums.txt" 2>/dev/null || true
  if [ -f "${TMP}/checksums.txt" ]; then
    if command -v sha256sum > /dev/null 2>&1; then
      cd "$TMP" && grep "${FILENAME}" checksums.txt | sha256sum -c - > /dev/null 2>&1 \
        && ok "Checksum verified" || err "Checksum mismatch — aborting"
      cd - > /dev/null
    elif command -v shasum > /dev/null 2>&1; then
      cd "$TMP" && grep "${FILENAME}" checksums.txt | shasum -a 256 -c - > /dev/null 2>&1 \
        && ok "Checksum verified" || err "Checksum mismatch — aborting"
      cd - > /dev/null
    fi
  fi

  # Extract
  info "Extracting..."
  if [ "$EXT" = "tar.gz" ]; then
    tar -xzf "${TMP}/${FILENAME}" -C "$TMP"
  else
    need unzip
    unzip -q "${TMP}/${FILENAME}" -d "$TMP"
  fi

  # Install binary
  BIN="${TMP}/${BINARY}"
  [ "$OS" = "windows" ] && BIN="${BIN}.exe"
  [ -f "$BIN" ] || err "Binary not found in archive"
  chmod +x "$BIN"

  # Try to install to INSTALL_DIR, fall back to ~/.local/bin
  if [ -w "$INSTALL_DIR" ]; then
    mv "$BIN" "${INSTALL_DIR}/${BINARY}"
  elif [ "$(id -u)" = "0" ]; then
    mv "$BIN" "${INSTALL_DIR}/${BINARY}"
  else
    info "No write access to ${INSTALL_DIR}, trying sudo..."
    sudo mv "$BIN" "${INSTALL_DIR}/${BINARY}" 2>/dev/null || {
      LOCAL_BIN="${HOME}/.local/bin"
      mkdir -p "$LOCAL_BIN"
      mv "$BIN" "${LOCAL_BIN}/${BINARY}"
      INSTALL_DIR="$LOCAL_BIN"
      info "Installed to ${LOCAL_BIN} — make sure it's in your PATH"
    }
  fi

  printf "\n"
  ok "gecko v${VERSION} installed to ${INSTALL_DIR}/${BINARY}"
  printf "\n"
  printf "  Run \033[36mgecko --version\033[0m to confirm.\n"
  printf "  Run \033[36mgecko help\033[0m to get started.\n\n"
}

main "$@"
