# Auto-Poll Tag After Build Merge Design

**Date:** 2026-08-10  
**Status:** Approved for implementation planning  
**Issue:** #3  
**Scope:** After a successful `forklift build merge`, automatically start `forklift poll tag` for the newly created tag in the background so the terminal is not blocked and existing notifications still fire.

## Problem

Today, after `forklift build merge` creates and pushes a tag, the user must manually run:

```bash
forklift poll tag --latest
```

That is an extra step, and foreground poll blocks the terminal until the workflow completes or times out.

## Goals

- After successful build merge (tag created and pushed), start polling automatically without a second manual command.
- Do not block `forklift build merge` on poll completion.
- Poll the **exact tag** just created (not `--latest`), to avoid races if another tag lands first.
- Keep existing notification behavior (desktop and/or Slack: started / success / failure / cancelled).
- Respect existing poll interval/timeout config defaults.
- Document the behavior in the README.

## Non-goals

- Log files for background poller output.
- PID files, process registries, or stopping a previous background poll.
- Config key such as `auto_poll` (opt-out is a CLI flag only for this iteration).
- Changing poll/notify semantics beyond reuse of the existing `poll tag` path.
- In-process goroutine polling (dies when the merge process exits).

## Decisions

| Question | Choice |
|----------|--------|
| Default | Auto-poll **on** after successful merge |
| Opt-out | `forklift build merge --no-poll` |
| Background output | Silent except notifications |
| Concurrent polls | Allow multiple independent background polls |
| Implementation | Detached child process running `poll tag <exact-tag>` |

## Approach

**Detach a child `forklift poll tag <exact-tag>` after merge succeeds.**

Chosen over an in-process goroutine (cannot outlive the CLI process) and over a dedicated daemon/launchd service (overkill for a local CLI).

---

## Behavior

1. On successful `build merge` Finish (tag created, pushed, sheet updated): if `--no-poll` is **not** set, start a background poll for that tag.
2. Print a single line: `Polling tag <tag> in the background…`
3. Merge command returns immediately (exit 0).
4. Background poll:
   - Uses the exact tag from Finish.
   - Discards stdout/stderr (silent terminal).
   - Uses existing `poll tag` logic for GitHub polling and notifications.
   - Uses existing interval/timeout from config/defaults (same as foreground `poll tag`).
5. Merge conflict pause (Finish not reached): **no** poll.
6. `--no-poll`: skip spawn; no polling message.

---

## Architecture

### Flow

```
build merge succeeds → Finish returns newTag
  → cmd/build: if !noPoll && newTag != ""
       → spawn detached: <same-binary> poll tag <newTag>
       → print “Polling tag … in the background…”
  → exit 0
```

### API change

- `build.Run` / `Resume` / `Finish` return the created tag (empty string when no tag was created, e.g. conflict early-exit that returns nil error).

### Components

| Area | Change |
|------|--------|
| `internal/build/build.go` | Return new tag from `Run` / `Resume` / `Finish` |
| `cmd/build.go` | Add `--no-poll`; on success spawn background poll |
| `internal/poller` | `StartBackground(tag string) error` — build and start detached process: `os.Args[0] poll tag <tag>`; discard stdio; Unix `Setsid` so the child survives terminal close; `Start()` only, no `Wait()` |
| `README.md` | Document auto-poll after merge and `--no-poll` |

### Child process

- Args: `poll tag <exactTag>` (not `--latest`).
- Do not pass `--no-notify` (notifications remain on unless the user later adds such a flag to build).
- Stdin closed; stdout/stderr discarded.
- Detached from the controlling terminal/session so closing the shell does not kill the poller.

---

## Error handling

- If spawn fails: print a short **warning**, do **not** fail the merge (tag already pushed). Suggest `forklift poll tag <tag>` manually.
- Later background poll failures (timeout, API errors): same as today’s foreground `poll tag`; no effect on the finished merge.
- Concurrent merges: each successful merge may start its own background poll; no bookkeeping.

---

## Testing

- Unit-test that successful Finish surfaces the created tag (or equivalent return-path coverage).
- Unit-test spawn decision: given tag + `--no-poll` / empty tag → skip; otherwise construct args `poll tag <exactTag>`.
- No integration test against a real GitHub Actions run.

---

## Documentation

Update README “Build & Merge” (and/or Poll) sections to state:

- Successful `forklift build merge` starts background polling for the new tag by default.
- Use `--no-poll` to disable.
- Notifications still come from the existing poll path (Slack if configured, else desktop).

## Acceptance criteria (from #3)

- [ ] After `forklift build merge` succeeds, poll starts without a second manual command
- [ ] Terminal is not blocked by the poller
- [ ] User still receives completion notifications (and started, if configured)
- [ ] Documented in README
