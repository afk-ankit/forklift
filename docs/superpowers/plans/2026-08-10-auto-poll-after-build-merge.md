# Auto-Poll After Build Merge Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** After a successful `forklift build merge`, automatically start a detached background `forklift poll tag <exact-tag>` so the terminal is free and existing notifications still fire.

**Architecture:** `build.Finish` (and `Run`/`Resume`) return the created tag string. `cmd/build` adds `--no-poll` and, on success, calls `internal/poller.StartBackground(tag)` which spawns the same binary with `poll tag <tag>`, discards stdio, and detaches via `Setsid`. Foreground poll/notify logic is reused unchanged.

**Tech Stack:** Go 1.25, Cobra, `os/exec` + `syscall.SysProcAttr{Setsid: true}` (Unix/macOS).

**Spec:** `docs/superpowers/specs/2026-08-10-auto-poll-after-build-merge-design.md`

## Global Constraints

- Auto-poll is **on by default** after successful merge; opt out with `--no-poll`.
- Poll the **exact tag** returned by Finish — never `--latest`.
- Background poll is **silent** (discard stdout/stderr); notifications still fire via existing `poll tag`.
- Spawn failure must **warn**, not fail the merge.
- Concurrent background polls are allowed (no PID tracking).
- Conflict early-exit (Finish not reached) → empty tag → no poll.
- Prefer co-located `*_test.go` tests; TDD for new packages.

## File Structure

| File | Responsibility |
|------|----------------|
| `internal/poller/poller.go` | `ShouldStart`, `CommandArgs`, `newBackgroundCmd`, `StartBackground` |
| `internal/poller/poller_test.go` | Decision, args, and cmd construction tests |
| `internal/build/build.go` | `Run`/`Resume`/`Finish` return `(tag string, err error)` |
| `cmd/build.go` | `--no-poll` flag; spawn after successful merge |
| `README.md` | Document auto-poll + `--no-poll` |

---

### Task 1: Background poller helper

**Files:**
- Create: `internal/poller/poller.go`
- Create: `internal/poller/poller_test.go`

**Interfaces:**
- Consumes: `os.Executable` / `os.Args[0]`, `os/exec`, `syscall`
- Produces:
  - `func ShouldStart(noPoll bool, tag string) bool` — true iff `!noPoll && tag != ""`
  - `func CommandArgs(tag string) []string` — `[]string{"poll", "tag", tag}`
  - `func StartBackground(tag string) error` — resolve executable, build detached cmd, `Start()` only

- [ ] **Step 1: Write the failing tests**

Create `internal/poller/poller_test.go`:

```go
package poller

import (
	"reflect"
	"testing"
)

func TestShouldStart(t *testing.T) {
	tests := []struct {
		name   string
		noPoll bool
		tag    string
		want   bool
	}{
		{"default with tag", false, "v-dev-0.0.5", true},
		{"no-poll with tag", true, "v-dev-0.0.5", false},
		{"empty tag", false, "", false},
		{"no-poll empty tag", true, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShouldStart(tt.noPoll, tt.tag); got != tt.want {
				t.Fatalf("ShouldStart(%v, %q) = %v, want %v", tt.noPoll, tt.tag, got, tt.want)
			}
		})
	}
}

func TestCommandArgs(t *testing.T) {
	got := CommandArgs("v-dev-0.0.5")
	want := []string{"poll", "tag", "v-dev-0.0.5"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CommandArgs = %#v, want %#v", got, want)
	}
}

func TestNewBackgroundCmd(t *testing.T) {
	cmd := newBackgroundCmd("/usr/local/bin/forklift", "v-dev-0.0.5")
	if cmd.Path != "/usr/local/bin/forklift" {
		t.Fatalf("Path = %q", cmd.Path)
	}
	wantArgs := []string{"/usr/local/bin/forklift", "poll", "tag", "v-dev-0.0.5"}
	if !reflect.DeepEqual(cmd.Args, wantArgs) {
		t.Fatalf("Args = %#v, want %#v", cmd.Args, wantArgs)
	}
	if cmd.Stdout != nil || cmd.Stderr != nil || cmd.Stdin != nil {
		t.Fatal("stdio should be nil (discarded)")
	}
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.Setsid {
		t.Fatal("expected SysProcAttr.Setsid=true for detach")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/poller/ -v`

Expected: FAIL — package or symbols undefined.

- [ ] **Step 3: Write minimal implementation**

Create `internal/poller/poller.go`:

```go
package poller

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func ShouldStart(noPoll bool, tag string) bool {
	return !noPoll && tag != ""
}

func CommandArgs(tag string) []string {
	return []string{"poll", "tag", tag}
}

func newBackgroundCmd(executable, tag string) *exec.Cmd {
	args := CommandArgs(tag)
	cmd := exec.Command(executable, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd
}

func StartBackground(tag string) error {
	executable, err := os.Executable()
	if err != nil || executable == "" {
		executable = os.Args[0]
	}
	if executable == "" {
		return fmt.Errorf("could not resolve forklift executable path")
	}
	cmd := newBackgroundCmd(executable, tag)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start background poll: %w", err)
	}
	return nil
}
```

Note: `Setsid` is Unix/macOS. This project targets Homebrew/macOS; do not add Windows stubs in this task unless `go test` requires it on the agent’s OS.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/poller/ -v`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/poller/poller.go internal/poller/poller_test.go
git commit -m "$(cat <<'EOF'
Add detached background poller helper for post-merge tag poll.

EOF
)"
```

---

### Task 2: Return created tag from build

**Files:**
- Modify: `internal/build/build.go`
- Modify: `cmd/build.go` (signature call site only — flag/spawn wiring is Task 3; temporarily ignore the returned tag or use `_`)

**Interfaces:**
- Consumes: existing `Finish` tag creation logic
- Produces:
  - `func Run(ctx context.Context, s *sheets.Service, sheetID, sheetName, repoName string) (string, error)`
  - `func Resume(ctx context.Context, s *sheets.Service, sheetID, sheetName, statePath string) (string, error)`
  - `func Finish(ctx context.Context, s *sheets.Service, sheetID, sheetName string, state structures.BuildState, lastTag string) (string, error)`
  - On conflict early-exit in `Run`: return `("", nil)`
  - On success: return `(newTag, nil)`
  - On error: return `("", err)`

- [ ] **Step 1: Write the failing test**

There is no lightweight way to drive full `Finish` without git/sheets. Add a focused test for the tag-increment helper already used by Finish (documents the tag string Finish will return), in `internal/build/build_tag_test.go`:

```go
package build

import "testing"

func TestIncrementTag_devSequence(t *testing.T) {
	got, err := IncrementTag("v-dev-0.0.4", "dev")
	if err != nil {
		t.Fatal(err)
	}
	if got != "v-dev-0.0.5" {
		t.Fatalf("got %q, want v-dev-0.0.5", got)
	}
}

func TestIncrementTag_emptyStartsAtOne(t *testing.T) {
	got, err := IncrementTag("", "dev")
	if err != nil {
		t.Fatal(err)
	}
	if got != "v-dev-0.0.1" {
		t.Fatalf("got %q, want v-dev-0.0.1", got)
	}
}
```

(If `IncrementTag` tests already exist elsewhere, skip creating duplicates and instead add a one-line compile assertion comment in Step 3 — but currently they do not exist.)

- [ ] **Step 2: Run test to verify it fails or package compiles**

Run: `go test ./internal/build/ -v`

Expected: PASS for IncrementTag (already implemented) — this locks the tag-naming contract Finish returns. Proceed to signature change next.

- [ ] **Step 3: Change Run / Resume / Finish signatures**

In `internal/build/build.go`:

1. Change `Run` to return `(string, error)`.
2. Conflict early-exit (merge in progress path that currently `return nil`): change to `return "", nil`.
3. All other error returns: `return "", fmt.Errorf(...)` (or `return "", err`).
4. Successful path: `return Finish(...)`.
5. Change `Resume` to return `(string, error)`; on success `return Finish(...)`; errors return `("", err)`; after successful Finish keep calling `Cleanup(state)` as today.
6. Change `Finish` to return `(string, error)`:
   - On any error before/during tag create/push/sheet update: `return "", err`
   - On success after sheet update and success print: `return newTag, nil`

Exact success tail of `Finish`:

```go
	fmt.Println("🏗️  Build merge completed successfully! 🎉")
	return newTag, nil
```

Exact conflict early-exit in `Run` (preserve messages; only change return):

```go
			return "", nil
```

- [ ] **Step 4: Update `cmd/build.go` call site so it compiles**

Replace:

```go
		if err := build.Run(ctx, service, cfg.SheetID, cfg.SheetName, repoName); err != nil {
			fatalf("build failed: %v", err)
		}
```

with:

```go
		if _, err := build.Run(ctx, service, cfg.SheetID, cfg.SheetName, repoName); err != nil {
			fatalf("build failed: %v", err)
		}
```

(Task 3 will use the tag.)

- [ ] **Step 5: Run tests / build**

Run: `go test ./internal/build/ ./cmd/ -v` and `go build -o /dev/null .`

Expected: PASS / build OK

- [ ] **Step 6: Commit**

```bash
git add internal/build/build.go internal/build/build_tag_test.go cmd/build.go
git commit -m "$(cat <<'EOF'
Return created tag from build merge Finish path.

EOF
)"
```

---

### Task 3: Wire `--no-poll`, spawn after merge, document

**Files:**
- Modify: `cmd/build.go`
- Modify: `README.md`
- Create: `cmd/build_test.go` (pure decision coverage via `poller.ShouldStart` / message helper if added)

**Interfaces:**
- Consumes: `poller.ShouldStart`, `poller.StartBackground`, `build.Run` → `(tag, err)`
- Produces: `--no-poll` flag on `build` command; post-success spawn + message

- [ ] **Step 1: Write the failing test for spawn messaging helper**

Add a tiny exported-from-cmd helper is awkward (cmd helpers are package-private). Prefer testing via `poller` (already done) plus a package-level helper in `cmd` for the user-facing line:

In `cmd/build.go` (will add in Step 3), add:

```go
func backgroundPollMessage(tag string) string {
	return fmt.Sprintf("Polling tag %s in the background…", tag)
}
```

Create `cmd/build_test.go`:

```go
package cmd

import "testing"

func TestBackgroundPollMessage(t *testing.T) {
	got := backgroundPollMessage("v-dev-0.0.5")
	want := "Polling tag v-dev-0.0.5 in the background…"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/ -run TestBackgroundPollMessage -v`

Expected: FAIL — `backgroundPollMessage` undefined.

- [ ] **Step 3: Implement flag + wiring in `cmd/build.go`**

Add imports: `"fmt"`, `"forklift/internal/poller"`.

Add package var:

```go
var buildNoPoll bool
```

In `init()`:

```go
	buildCmd.Flags().BoolVar(&buildNoPoll, "no-poll", false, "Do not start background workflow polling after a successful merge")
```

Replace the `build.Run` call block with:

```go
		tag, err := build.Run(ctx, service, cfg.SheetID, cfg.SheetName, repoName)
		if err != nil {
			fatalf("build failed: %v", err)
		}

		if poller.ShouldStart(buildNoPoll, tag) {
			if err := poller.StartBackground(tag); err != nil {
				fmt.Printf("⚠️  Failed to start background poll: %v\n", err)
				fmt.Printf("   Run manually: forklift poll tag %s\n", tag)
			} else {
				fmt.Println(backgroundPollMessage(tag))
			}
		}
```

Add helper:

```go
func backgroundPollMessage(tag string) string {
	return fmt.Sprintf("Polling tag %s in the background…", tag)
}
```

- [ ] **Step 4: Update README**

In the “Build & Merge” section (after the numbered list of what merge does), add:

```markdown
After a successful merge, Forklift automatically polls GitHub Actions for the **new tag** in the background and sends the usual notifications (Slack if configured, otherwise desktop). The merge command returns immediately.

To skip background polling:

```bash
forklift build merge --no-poll
```
```

Also add a short cross-reference under the Poll section noting that `build merge` starts this automatically unless `--no-poll` is passed.

- [ ] **Step 5: Run full verification**

Run:

```bash
go test ./...
go build -o /dev/null .
```

Expected: all tests PASS; build OK.

- [ ] **Step 6: Commit**

```bash
git add cmd/build.go cmd/build_test.go README.md
git commit -m "$(cat <<'EOF'
Auto-start background tag poll after successful build merge.

EOF
)"
```

---

## Spec coverage checklist

| Spec requirement | Task |
|------------------|------|
| Auto-poll on by default after successful merge | Task 3 |
| `--no-poll` opt-out | Task 3 |
| Exact tag (not `--latest`) | Task 1 `CommandArgs` + Task 3 |
| Non-blocking / detached child | Task 1 `Setsid` + `Start` |
| Silent except notifications | Task 1 nil stdio |
| Spawn failure warns, merge still succeeds | Task 3 |
| Conflict path does not poll | Task 2 empty tag + Task 3 `ShouldStart` |
| Concurrent polls allowed | (no PID logic — by omission) |
| README documented | Task 3 |
| Unit tests for decision/args | Task 1, Task 3 |

## Self-review notes

- No placeholders left in steps.
- Signatures consistent: `(string, error)` for Run/Resume/Finish; empty tag on conflict/error.
- `poller.ShouldStart` is the single gate used by cmd.
