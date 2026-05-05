# Helper HTTP Connection Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Use a live HTTP stream so each user has at most one active helper and helpers exit when disconnected from `activitysvc`.

**Architecture:** `activitysvc` exposes a private authenticated helper stream on its existing HTTP server. A helper registry keyed by user SID owns command channels and closes the previous channel when a duplicate helper connects. The service launches helpers with a private token and URL.

**Tech Stack:** Go standard library `net/http`, newline-delimited JSON, existing Windows launcher and runtime packages.

---

### Task 1: Active Helper Registry

**Files:**
- Modify: `internal/helperipc/server.go`
- Modify: `internal/helperipc/server_test.go`

- [ ] Write failing tests that registering a second helper for the same user closes the first helper channel and that `Connected(userSID)` reflects active registration.
- [ ] Run `go test ./internal/helperipc -run 'TestServer' -v` and confirm the new tests fail.
- [ ] Update `Server.Register` to return a receive-only command channel plus an unregister function, close the older channel on replacement, remove the active helper on unregister, and add `Connected`.
- [ ] Run `go test ./internal/helperipc -run 'TestServer' -v` and confirm it passes.

### Task 2: Private HTTP Stream Endpoint

**Files:**
- Modify: `internal/api/server.go`
- Modify: `internal/api/server_test.go`

- [ ] Write failing API tests for unauthorized helper stream rejection and authenticated command streaming.
- [ ] Run `go test ./internal/api -run 'TestHelper' -v` and confirm the tests fail.
- [ ] Add `HelperStream` support to `api.Server`, route `GET /internal/helper/stream`, validate `Authorization: Bearer <helper token>`, register the helper, encode commands as newline-delimited JSON, and flush after each command.
- [ ] Run `go test ./internal/api -run 'TestHelper|TestInfo|TestAdjust' -v` and confirm it passes.

### Task 3: Helper HTTP Client Runtime

**Files:**
- Modify: `internal/helper/runtime.go`
- Modify: `internal/helper/runtime_test.go`

- [ ] Write failing tests that the helper speaks streamed commands and returns when the stream closes or returns a non-OK status.
- [ ] Run `go test ./internal/helper -run 'TestRuntime' -v` and confirm the tests fail.
- [ ] Add an HTTP stream runner that sends user SID/session ID, decodes newline-delimited JSON commands, speaks `speak` commands, and returns on EOF or HTTP failure.
- [ ] Run `go test ./internal/helper -run 'TestRuntime' -v` and confirm it passes.

### Task 4: Service Wiring And Launcher Arguments

**Files:**
- Modify: `internal/runtime/app.go`
- Modify: `internal/runtime/app_test.go`
- Modify: `internal/windows/helper/launcher.go`
- Modify: `internal/windows/helper/launcher_windows.go`
- Modify: `internal/windows/helper/launcher_stub.go`
- Modify: `internal/windows/helper/launcher_test.go`

- [ ] Write failing tests for helper URL construction, helper argument parsing, and launcher command-line inclusion of `--helper-url` and `--helper-token`.
- [ ] Run `go test ./internal/runtime ./internal/windows/helper -run 'TestHelper|TestLauncher|TestBuild' -v` and confirm the tests fail.
- [ ] Generate a per-service helper token, create the helper registry, pass it to `api.New`, set `service.Runtime.Helper` to the registry, and launch helpers with URL/token arguments.
- [ ] Update helper startup to require URL/token arguments and use the HTTP stream runner; helpers without a live stream exit.
- [ ] Run `go test ./internal/runtime ./internal/windows/helper -run 'TestHelper|TestLauncher|TestBuild' -v` and confirm it passes.

### Task 5: Documentation And Verification

**Files:**
- Modify: `README.md`

- [ ] Update README helper architecture language to describe the HTTP stream connection.
- [ ] Run `gofmt` on modified Go files.
- [ ] Run `go test ./...`.
- [ ] Run `GOOS=windows GOARCH=amd64 go test ./...`.
