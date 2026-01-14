# CI/CD Documentation

Crush uses GitHub Actions for its continuous integration and delivery pipelines.

## Workflows

### 1. Build & Test (`build.yml`)
- **Trigger:** Pull requests and pushes to `main`.
- **Steps:**
  - Setup Go environment.
  - Run `go test ./...` across multiple OS platforms (macOS, Linux, Windows).
  - Verify the project builds successfully.

### 2. Linting (`lint.yml`)
- **Trigger:** Pull requests and pushes to `main`.
- **Steps:**
  - Runs `golangci-lint` to ensure code quality and adherence to style guides.
  - Checks for common errors, security issues, and formatting.

### 3. Release (`release.yml`)
- **Trigger:** Push of a semantic version tag (e.g., `v*`).
- **Steps:**
  - Runs GoReleaser to build, package, and upload artifacts to GitHub Releases.
  - Updates package repositories.

### 4. Nightly Builds (`nightly.yml`)
- **Trigger:** Scheduled (daily).
- **Purpose:** Ensures the codebase remains stable against the latest dependencies.

### 5. Dependency Management (`dependabot.yml`)
- **Action:** Automatically opens PRs to update Go modules and GitHub Actions.

## Style Enforcement
- **Format:** `gofumpt` is used for strict formatting.
- **Commits:** Semantic commits (`feat:`, `fix:`, etc.) are encouraged for better changelog generation.
