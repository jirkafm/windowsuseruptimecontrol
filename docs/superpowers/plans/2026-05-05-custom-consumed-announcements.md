# Custom Consumed Announcements Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add configurable consumed-time percentage announcements that fire once per user without changing existing built-in warnings.

**Architecture:** Config loading validates and normalizes `custom_consumed_warning_percentages`. The policy engine evaluates those percentages against each user's consumed allowance and persists sent custom percentages in `UserDayState`.

**Tech Stack:** Go standard library, existing config/model/policy/service packages, `go test`.

---

### Task 1: Config Model and Validation

**Files:**
- Modify: `internal/model/types.go`
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write failing config tests**

Add tests that load `custom_consumed_warning_percentages`, expect duplicates removed and sorted, expect `50` ignored with a warning log callback, and expect invalid values or more than 10 values to fail.

- [ ] **Step 2: Run config tests to verify failure**

Run: `go test ./internal/config -run 'TestLoadConfigCustomConsumedWarnings|TestLoadConfigRejects' -v`
Expected: FAIL because the config field and warning logger do not exist yet.

- [ ] **Step 3: Implement config support**

Add `CustomConsumedWarningPercentages []int` to `model.Config`. Add an optional package-level warning logger in `internal/config` for tests and service startup. Validate the raw array length before dedupe, reject values outside `1..99`, ignore `50` with a warning, dedupe, sort, and store the normalized list.

- [ ] **Step 4: Run config tests to verify pass**

Run: `go test ./internal/config -run 'TestLoadConfigCustomConsumedWarnings|TestLoadConfigRejects' -v`
Expected: PASS.

### Task 2: Policy Evaluation and Per-User State

**Files:**
- Modify: `internal/model/types.go`
- Modify: `internal/policy/engine.go`
- Test: `internal/policy/engine_test.go`

- [ ] **Step 1: Write failing policy tests**

Add tests that configure custom percentages and verify a user receives a remaining-time message when crossing a configured consumed threshold, does not receive the same custom message twice, and another user still receives the same custom threshold independently.

- [ ] **Step 2: Run policy tests to verify failure**

Run: `go test ./internal/policy -run 'TestEvaluateEmitsCustomConsumed|TestEvaluateTracksCustomConsumedPerUser' -v`
Expected: FAIL because custom percentages are not evaluated yet.

- [ ] **Step 3: Implement policy support**

Add `CustomConsumedWarningsSent []int` to `model.UserDayState` and `CustomConsumedWarningPercentages []int` to `policy.Engine`. During evaluation, emit `You have N minutes remaining.` when a configured threshold is reached and mark that percentage as sent for that user.

- [ ] **Step 4: Run policy tests to verify pass**

Run: `go test ./internal/policy -run 'TestEvaluateEmitsCustomConsumed|TestEvaluateTracksCustomConsumedPerUser' -v`
Expected: PASS.

### Task 3: Service Wiring, Reset, API View, Installer, Docs

**Files:**
- Modify: `internal/service/runtime.go`
- Modify: `internal/service/runtime_test.go`
- Modify: `internal/runtime/app.go`
- Modify: `installer/install.ps1`
- Modify: `README.md`

- [ ] **Step 1: Write failing service test**

Add a test that verifies `ResetToday` clears custom warning sent state.

- [ ] **Step 2: Run service test to verify failure**

Run: `go test ./internal/service -run 'TestResetTodayClearsCustomConsumedWarnings' -v`
Expected: FAIL because reset does not clear custom warning state yet.

- [ ] **Step 3: Wire runtime and docs**

Pass configured custom percentages into `policy.Engine`, include the normalized list in `ConfigView`, set the config warning logger during service startup, clear custom sent state during reset, add the default empty config value in the installer, and document the config option in README.

- [ ] **Step 4: Run focused tests**

Run: `go test ./internal/config ./internal/policy ./internal/service -v`
Expected: PASS.

### Task 4: Final Verification

**Files:**
- All modified files.

- [ ] **Step 1: Run full test suite**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 2: Review diff and copyright headers**

Run: `GIT_CONFIG_GLOBAL=/Users/jikaplan/.jirkafm/my-env/.gitconfig git diff --check`
Expected: no whitespace errors. Check every modified file for a copyright header; if a header exists, ensure the latest year is `2026`.
