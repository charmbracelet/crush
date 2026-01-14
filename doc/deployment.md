# Deployment Guide

Crush is a single binary application. Deployment primarily refers to how it is built, released, and installed.

## Installation

### Package Managers
- **macOS (Homebrew):** `brew install charmbracelet/tap/crush`
- **Linux:** Available via `apt`, `yum`, `yay` (AUR), and `nix`.
- **Windows:** Available via `winget` and `scoop`.

### From Source
```bash
go install github.com/charmbracelet/crush@latest
```

## Build Process
Crush uses a `Taskfile.yaml` to manage build tasks.

- **Build Binary:** `task build` (or `go build .`)
- **Run Tests:** `task test`
- **Lint Code:** `task lint`

## Release Process
Releases are automated using **GoReleaser** via GitHub Actions.

1. **Trigger:** A new tag is pushed (e.g., `v1.2.3`).
2. **Action:** The `release.yml` workflow triggers GoReleaser.
3. **Artifacts:**
   - Binaries for Linux, macOS, Windows, and BSDs.
   - Debian and RPM packages.
   - Homebrew formula update.
   - Docker image (if configured).

## Deployment Manifests
- **`.goreleaser.yml`:** Defines the cross-platform build and release configuration.
- **`flake.nix`:** Provides a reproducible build environment and Nix package definition.
- **`.github/workflows/`:** Contains CI/CD definitions.
