# Installation

## Homebrew (macOS / Linux) — recommended

```bash
brew install gecko-iac/gecko/gecko
```

> Once the formula lands in homebrew-core, `brew install gecko` will work directly.

## curl (macOS / Linux)

```bash
curl -fsSL https://raw.githubusercontent.com/kicka5h/gecko-iac/main/scripts/install.sh | sh
```

The script detects your OS and architecture, downloads the correct binary from the latest GitHub release, verifies the SHA256 checksum, and installs it to `/usr/local/bin/gecko`.

## Windows (PowerShell)

```powershell
irm https://raw.githubusercontent.com/kicka5h/gecko-iac/main/scripts/install.ps1 | iex
```

Installs to `$env:LOCALAPPDATA\gecko\gecko.exe` and adds it to your `PATH`.

## Go install

```bash
go install github.com/gecko-iac/gecko@latest
```

Requires Go 1.22+.

## Download a binary

Download the right archive from [GitHub Releases](https://github.com/kicka5h/gecko-iac/releases/latest), extract it, and move the `gecko` binary to somewhere on your `PATH`.

| OS | Arch | File |
|---|---|---|
| macOS | Apple Silicon | `gecko_*_darwin_arm64.tar.gz` |
| macOS | Intel | `gecko_*_darwin_amd64.tar.gz` |
| Linux | x86_64 | `gecko_*_linux_amd64.tar.gz` |
| Linux | ARM64 | `gecko_*_linux_arm64.tar.gz` |
| Windows | x86_64 | `gecko_*_windows_amd64.zip` |

## Verify

```bash
gecko --version
# 🦎 gecko Amalosia — v0.1.0
```
