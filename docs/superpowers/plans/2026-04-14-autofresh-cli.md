# Autofresh CLI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a small cross-platform Go CLI that schedules daily `codex` / `claude` keepalive pings using native macOS and Linux job systems.

**Architecture:** The binary stores one local JSON config, derives the current day's schedule from a start time plus a fixed `5h10m` interval, and installs platform-native scheduled jobs that invoke `autofresh run`. Provider execution stays isolated behind a small runner interface so scheduling, config, and platform logic remain testable without spawning real CLIs.

**Tech Stack:** Go, standard library, `launchd` on macOS, `crontab` on Linux

---

### Task 1: Bootstrap module and CLI skeleton

**Files:**
- Create: `go.mod`
- Create: `cmd/autofresh/main.go`
- Create: `internal/cli/cli.go`
- Test: `internal/cli/cli_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestParseTargetRejectsInvalidValue(t *testing.T) {
	_, err := ParseTarget("bad")
	if err == nil {
		t.Fatal("expected error")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli -run TestParseTargetRejectsInvalidValue -v`
Expected: FAIL because `ParseTarget` does not exist yet

- [ ] **Step 3: Write minimal implementation**

Implement:
- `main.go` entrypoint that calls `cli.Run(os.Args[1:], deps)`
- `cli.go` with command dispatch for `set`, `plan`, `trigger`, `delete`, `run`, `doctor`
- `ParseTarget` supporting `codex`, `claude`, `all`

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/cli -run TestParseTargetRejectsInvalidValue -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add go.mod cmd/autofresh/main.go internal/cli/cli.go internal/cli/cli_test.go
git commit -m "feat: add autofresh cli skeleton"
```

### Task 2: Add schedule and config core

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`
- Create: `internal/schedule/schedule.go`
- Create: `internal/schedule/schedule_test.go`
- Modify: `internal/cli/cli.go`

- [ ] **Step 1: Write the failing tests**

```go
func TestTimesForDay(t *testing.T) {
	got := TimesForDay("08:00")
	want := []string{"08:00", "13:10", "18:20", "23:30"}
	if diff := cmpStrings(got, want); diff != "" {
		t.Fatal(diff)
	}
}

func TestConfigRoundTrip(t *testing.T) {
	cfg := Config{StartTime: "08:00", Target: "all"}
	if err := Save(tempPath, cfg); err != nil { t.Fatal(err) }
	got, err := Load(tempPath)
	if err != nil { t.Fatal(err) }
	if got.Target != "all" { t.Fatalf("got %q", got.Target) }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/config ./internal/schedule -v`
Expected: FAIL because config and schedule packages are missing

- [ ] **Step 3: Write minimal implementation**

Implement:
- `Config` struct with start time, target, binary path, interval minutes
- config path resolution under `~/.config/autofresh/config.json`
- JSON `Load`, `Save`, `Delete`, `Exists`
- schedule generator using fixed `310` minute interval and same-day cutoff

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/config ./internal/schedule -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/config internal/schedule internal/cli/cli.go
git commit -m "feat: add config and scheduling core"
```

### Task 3: Add provider execution and logging

**Files:**
- Create: `internal/provider/provider.go`
- Create: `internal/provider/provider_test.go`
- Create: `internal/logging/logging.go`
- Modify: `internal/cli/cli.go`

- [ ] **Step 1: Write the failing tests**

```go
func TestBuildCommandForAll(t *testing.T) {
	cmds, err := BuildCommands("all")
	if err != nil { t.Fatal(err) }
	if len(cmds) != 2 { t.Fatalf("got %d commands", len(cmds)) }
}

func TestRunnerContinuesAfterSingleFailure(t *testing.T) {
	// fake executor returns error for codex, nil for claude
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/provider -v`
Expected: FAIL because provider package is missing

- [ ] **Step 3: Write minimal implementation**

Implement:
- provider command definitions for `codex exec "ok" --max-tokens 5`
- provider command definitions for `claude -p "ok" --max-tokens 5`
- runner using `exec.CommandContext` with `15s` timeout and `io.Discard`
- sequential `all` execution with partial-failure reporting
- append-only lightweight logger under `~/.config/autofresh/autofresh.log`

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/provider -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/provider internal/logging internal/cli/cli.go
git commit -m "feat: add provider execution"
```

### Task 4: Add platform job installers

**Files:**
- Create: `internal/platform/platform.go`
- Create: `internal/platform/platform_test.go`
- Create: `internal/platform/darwin.go`
- Create: `internal/platform/linux.go`
- Modify: `internal/cli/cli.go`

- [ ] **Step 1: Write the failing tests**

```go
func TestLaunchdPlistContainsAllTimes(t *testing.T) {
	plist := BuildLaunchdPlist("/tmp/autofresh", []TimeOfDay{{Hour: 8, Minute: 0}})
	if !strings.Contains(plist, "StartCalendarInterval") {
		t.Fatal("missing intervals")
	}
}

func TestCronRewritePreservesForeignEntries(t *testing.T) {
	input := "MAILTO=test\n0 1 * * * /bin/echo hi\n"
	out := RewriteCron(input, "/tmp/autofresh", []TimeOfDay{{Hour: 8, Minute: 0}})
	if !strings.Contains(out, "MAILTO=test") {
		t.Fatal("dropped foreign entry")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/platform -v`
Expected: FAIL because platform package is missing

- [ ] **Step 3: Write minimal implementation**

Implement:
- platform abstraction with `Install`, `Remove`, `Status`
- macOS plist rendering and install/remove shell calls
- Linux cron block rendering and rewrite helpers
- PATH injection for scheduled environment

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/platform -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/platform internal/cli/cli.go
git commit -m "feat: add platform schedulers"
```

### Task 5: Wire application flows end-to-end

**Files:**
- Create: `internal/app/app.go`
- Create: `internal/app/app_test.go`
- Modify: `internal/cli/cli.go`
- Modify: `cmd/autofresh/main.go`

- [ ] **Step 1: Write the failing tests**

```go
func TestSetPersistsConfigAndInstallsJob(t *testing.T) {
	// fake config store + fake platform
}

func TestPlanPrintsTodaySchedule(t *testing.T) {
	// capture output and verify 08:00 13:10 18:20 23:30
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/app -v`
Expected: FAIL because app orchestration is missing

- [ ] **Step 3: Write minimal implementation**

Implement:
- `set` flow: validate input, persist config, install platform jobs, print summary
- `plan` flow: load config, derive schedule, print compact readable output
- `trigger` flow: run chosen target immediately
- `run` flow: load config and run configured target
- `delete` flow: remove jobs and config
- `doctor` flow: inspect PATH availability and job status

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/app -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/app internal/cli/cli.go cmd/autofresh/main.go
git commit -m "feat: wire autofresh commands"
```

### Task 6: Final verification and polish

**Files:**
- Modify: `README.md`
- Modify: any affected Go files for small cleanup

- [ ] **Step 1: Write the failing test or missing check**

Add any narrow regression tests discovered during integration, especially around:
- invalid time parsing
- `trigger --target` overriding config target
- `21:00` generating one daily slot only

- [ ] **Step 2: Run checks to verify current gaps**

Run:
- `go test ./...`
- `go test ./internal/schedule -run TestTimesForDay -v`

Expected: any failing regression surfaces before cleanup

- [ ] **Step 3: Write minimal implementation**

Implement:
- concise `README.md` usage examples
- any small fixes required by full test run
- help text cleanup

- [ ] **Step 4: Run checks to verify they pass**

Run:
- `go test ./...`
- `go build ./cmd/autofresh`

Expected:
- all tests PASS
- binary builds successfully

- [ ] **Step 5: Commit**

```bash
git add README.md .
git commit -m "docs: finalize autofresh usage"
```
