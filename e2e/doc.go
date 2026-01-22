// Package e2e provides end-to-end tests for Crush TUI using the trifle framework.
//
// Run all tests:
//
//	go test -v ./e2e/...
//
// Update snapshots:
//
//	go test ./e2e/... -update
//
// Run specific test file:
//
//	go test -v ./e2e/... -run TestStartup
//
// Build crush binary before running tests:
//
//	go build -o crush . && go test -v ./e2e/...
package e2e
