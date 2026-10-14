// Package provider holds tools that delegate work to external AI CLIs
// (Claude Code, Codex) already signed in on this machine. Each CLI manages
// its own credentials; these tools never read or copy tokens.
//
// Every provider implements the strategy interface in delegate.go: name,
// description, binary, argument assembly, and output parsing. The shared
// runner owns everything common — prompt and resume validation, the
// permission prompt (the only gate: the nested CLI runs without asking
// anyone), binary availability, timeout, execution, and error precedence (a
// failure the CLI reported in its own output beats the process exit code).
// NewTools registers a provider only when its CLI is installed and the agent
// may use it. Adding a provider means implementing the strategy and listing
// it in NewTools.
package provider
