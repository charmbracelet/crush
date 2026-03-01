# npm Distribution with optionalDependencies

This directory contains scripts for building npm packages using the **optionalDependencies pattern** - the same approach used by esbuild, SWC, Turbo, and OpenCode.

## Why This Approach?

The default goreleaser npm publisher uses a postinstall script that downloads binaries from GitHub releases. This fails in environments where:

- `registry.npmjs.org` is allowed (standard for development)
- `github.com` / `objects.githubusercontent.com` is blocked (common security policy)

The optionalDependencies pattern embeds binaries directly in platform-specific npm packages, so everything downloads from `registry.npmjs.org` only.

## How It Works

Instead of one package that downloads at install time:

```
@charmland/crush (postinstall downloads from GitHub) ❌
```

We create multiple packages with embedded binaries:

```
@charmland/crush (thin wrapper, no downloads)
  └── optionalDependencies:
      ├── @charmland/crush-linux-x64   (18MB binary embedded)
      ├── @charmland/crush-linux-arm64 (18MB binary embedded)
      ├── @charmland/crush-darwin-x64  (18MB binary embedded)
      ├── @charmland/crush-darwin-arm64(18MB binary embedded)
      ├── @charmland/crush-win32-x64   (18MB binary embedded)
      └── ...
```

npm automatically installs only the package matching the user's platform.

## Usage

### Building Packages

After running goreleaser (which creates the archives):

```bash
./scripts/npm-optional-deps/build-npm-packages.sh 0.43.0 ./dist
```

This creates:
- `dist/npm/charmland-crush-linux-x64-0.43.0.tgz`
- `dist/npm/charmland-crush-darwin-arm64-0.43.0.tgz`
- `dist/npm/charmland-crush-0.43.0.tgz` (main package)
- etc.

### Publishing

```bash
cd dist/npm

# Publish platform packages first
npm publish charmland-crush-linux-x64-0.43.0.tgz --access public
npm publish charmland-crush-darwin-arm64-0.43.0.tgz --access public
# ... etc

# Then publish main package
npm publish charmland-crush-0.43.0.tgz --access public
```

### GitHub Actions

The `.github/workflows/npm-publish.yml` workflow automates this process. It:
1. Triggers on new releases
2. Downloads release archives
3. Builds npm packages
4. Publishes to npm registry

## Files

- `build-npm-packages.sh` - Main build script
- `crush-wrapper.js` - JavaScript wrapper that finds and executes the platform binary

## Supported Platforms

| Platform | Package |
|----------|---------|
| Linux x64 | `@charmland/crush-linux-x64` |
| Linux ARM64 | `@charmland/crush-linux-arm64` |
| Linux x86 | `@charmland/crush-linux-ia32` |
| Linux ARM | `@charmland/crush-linux-arm` |
| macOS x64 | `@charmland/crush-darwin-x64` |
| macOS ARM64 | `@charmland/crush-darwin-arm64` |
| Windows x64 | `@charmland/crush-win32-x64` |
| Windows ARM64 | `@charmland/crush-win32-arm64` |
| Windows x86 | `@charmland/crush-win32-ia32` |

## References

- [esbuild npm distribution](https://github.com/evanw/esbuild/blob/main/npm/esbuild/package.json)
- [OpenCode npm distribution](https://github.com/anomalyco/opencode)
- [Issue #2230](https://github.com/charmbracelet/crush/issues/2230) - Original feature request
