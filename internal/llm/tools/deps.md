# Dependency Management Tool (deps)

The `deps` tool allows you to manage project dependencies across different package managers. It provides a unified interface for installing, updating, removing, and listing dependencies.

## Usage

```json
{
  "action": "install",
  "manager": "npm",
  "package": "express",
  "version": "4.18.2"
}
```

## Parameters

- `action` (required): The action to perform
  - `install`: Install a new dependency
  - `update`: Update an existing dependency
  - `remove`: Remove a dependency
  - `list`: List all dependencies
- `manager` (optional): The package manager to use
  - `npm`: Node.js package manager
  - `yarn`: Yarn package manager
  - `pnpm`: PNPM package manager
  - `pip`: Python package manager
  - `pipenv`: Pipenv package manager
  - `poetry`: Poetry package manager
  - `go`: Go module manager
  - If not specified, the tool will auto-detect based on project files
- `package` (optional): The package name for install/remove actions
- `version` (optional): Specific version of the package
- `working_dir` (optional): Custom working directory (defaults to current project)
- `options` (optional): Additional options for the package manager

## Examples

### Install a package

```json
{
  "action": "install",
  "manager": "npm",
  "package": "lodash",
  "version": "4.17.21"
}
```

### Update all dependencies

```json
{
  "action": "update",
  "manager": "npm"
}
```

### Remove a package

```json
{
  "action": "remove",
  "manager": "pip",
  "package": "requests"
}
```

### List dependencies

```json
{
  "action": "list",
  "manager": "go"
}
```

## Auto-detection

If no manager is specified, the tool will automatically detect the package manager based on the following files:

- `package.json` → npm
- `yarn.lock` → yarn
- `pnpm-lock.yaml` → pnpm
- `requirements.txt` → pip
- `Pipfile` → pipenv
- `pyproject.toml` → poetry
- `go.mod` → go

## Notes

- The tool requires appropriate permissions to modify the project directory
- Internet connection may be required for remote package operations
- Some actions may take time depending on the package manager and network speed