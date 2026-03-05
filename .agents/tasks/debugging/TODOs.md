1. [x] Confirm the bug with a unit test that reproduces the UNIQUE constraint failure
2. [x] Fix `createNewFile()` in edit.go to check history before inserting version 0
3. [x] Scope `CreateVersion()` to query only the current session (not all sessions)
4. [x] Harden the retry loop by making `Create()` delegate to `CreateVersion()`
5. [x] Audit write.go and multiedit.go for the same patterns; add maintainer comments
6. [x] Run full test suite and verify no regressions
7. [x] Fix data race in `db.Connect()` and verify with `go test -race ./...`
