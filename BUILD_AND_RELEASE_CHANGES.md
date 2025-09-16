# Build and Release Changes for Blush Rebranding

## 1. Go Module Changes

### go.mod
- Module declaration: `module github.com/charmbracelet/crush` → `module github.com/nom-nom-hub/blush`

## 2. Taskfile Changes

### Taskfile.yaml
- Project name references
- Environment variables: `CRUSH_PROFILE` → `BLUSH_PROFILE`
- Generated file names: `crush` → `blush`
- Task descriptions
- Completion script generation commands
- Man page generation commands

## 3. GoReleaser Configuration

### .goreleaser.yml
This is the most critical file that needs comprehensive updates:

#### Project Metadata
- `project_name`: `crush` → `blush`
- `homepage`: `"https://charm.sh/crush"` → `"https://charm.sh/blush"`
- `description` updates
- `full_description` updates

#### Build Configuration
- `ldflags`: Update package path references
- Output binary names

#### Before Hooks
- Completion script generation commands:
  - `go run . completion bash >./completions/crush.bash` → `go run . completion bash >./completions/blush.bash`
  - `go run . completion zsh >./completions/crush.zsh` → `go run . completion zsh >./completions/blush.zsh`
  - `go run . completion fish >./completions/crush.fish` → `go run . completion fish >./completions/blush.fish`
- Man page generation:
  - `go run . man | gzip -c >./manpages/crush.1.gz` → `go run . man | gzip -c >./manpages/blush.1.gz`

#### Archive Configuration
- `name_template`: Update references from `crush` to `blush`
- File inclusions for completions and man pages

#### AUR Sources
- `git_url`: `"ssh://aur@aur.archlinux.org/crush.git"` → `"ssh://aur@aur.archlinux.org/blush.git"`
- Build commands referencing `./crush` → `./blush`
- Installation paths: `/usr/bin/crush` → `/usr/bin/blush`
- License directory paths
- Completion file paths
- Man page paths
- Documentation paths

#### AUR Binary Packages
- Similar changes as AUR sources but for binary packages

#### Homebrew Configuration
- Repository paths
- Installation paths
- Completion script references

#### Scoop Configuration
- Repository paths
- Installation paths

#### NPM Configuration
- Package name: `"@charmland/crush"` → `"@charmland/blush"`
- Repository URL updates

#### NFPM (Package Formats)
- File destination paths for completions
- File destination paths for man pages

#### Nix Configuration
- Repository paths
- Installation commands

#### Winget Configuration
- Publisher information
- Package name references
- Repository paths

#### Changelog Configuration
- Filters and exclusions

#### Release Configuration
- Footer URLs

## 4. GitHub Actions Workflows

### .github/workflows/
All workflow files need updates:
- Workflow names
- Job names
- Environment variables
- File paths
- Artifact names
- Release configurations

## 5. Installation Scripts

### Package Installation Commands
- Homebrew: `brew install charmbracelet/tap/crush` → `brew install charmbracelet/tap/blush`
- AUR: Package names
- Scoop: `scoop install crush` → `scoop install blush`
- NPM: `npm install -g @charmland/crush` → `npm install -g @charmland/blush`
- Winget: `winget install charmbracelet.crush` → `winget install charmbracelet.blush`

## 6. Docker Configuration (if applicable)

### Dockerfile
- Binary copy paths
- Entry point commands

## 7. Documentation Updates

### README.md
- Installation commands
- Usage examples
- All command references
- URLs and links

### Wiki Documentation
- All pages referencing "crush"

## 8. Website Changes

### Project Website
- URLs from `/crush` to `/blush`
- Content updates
- Download links
- Documentation pages

## 9. Package Registry Updates

### Package.json (NPM)
- Package name
- Repository URLs
- Bugs URL
- Homepage URL

### AUR PKGBUILD
- Package name
- Source URLs
- Installation scripts

### Homebrew Formula
- Formula name
- URL references
- Installation paths

### Scoop Manifest
- Manifest name
- URL references
- Installation paths

## 10. CI/CD Environment Variables

### GitHub Secrets
- Update secret names and values where appropriate
- API keys and tokens for package registries

### Build Environment
- Environment variable names
- Path references

## 11. Release Process

### Versioning
- Ensure version tags follow the new naming convention

### Release Notes
- Update template to reference "Blush" instead of "Crush"

### Announcement Templates
- Social media posts
- Blog posts
- Email announcements

## 12. Migration Strategy

### Backward Compatibility
- Option to read existing configuration files
- Data migration scripts
- Warning messages for users migrating from Crush

### Deprecation Timeline
- Plan for deprecating the old "crush" name
- Communication strategy for existing users