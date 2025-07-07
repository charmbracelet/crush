# Contributing

Contributions are welcome!

Please submit a pull request for minor changes and submit issues for major changes for discussions.

## Testing

When providing a new feature or bug fix, please provide tests that demonstrate the issue along with your fix.

### Golden Files

If golden files need to be updated, you can do so with `-update`.

Usage: `go test ./pkg/... -update`.

## Themes

New styles need to be implemented in `styles/<style-name>.go`, and then `go generate
./...` will create the JSON files from it.

You can look up all references of another theme (e.g. Dracula), and add your
theme accordingly.
