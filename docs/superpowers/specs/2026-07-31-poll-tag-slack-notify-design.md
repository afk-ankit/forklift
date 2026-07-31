# Poll Tag Reliability + Slack Notify Design

**Date:** 2026-07-31  
**Status:** Approved for implementation planning  
**Scope:** Fix `forklift poll tag` hanging on “Waiting for workflow to start…”, and add optional Slack notifications (with user tagging) configured at `forklift init`.

## Problem

1. **Poll hang:** `CheckWorkflowStatusForTag` fetches the last 10 `event=push` workflow runs and matches `head_branch == tag`. On busy repos the tag run is often missing. Any GitHub API error is treated as “workflow not started yet,” so auth/rate-limit failures look like an endless wait until timeout.
2. **Notifications:** Today only desktop notifications fire on terminal status. Users want Slack messages that tag the person who ran Forklift when a tag workflow starts or finishes (success / failure / cancelled).

## Goals

- Reliably find and track the GitHub Actions run for a given tag.
- Surface real GitHub API errors instead of masking them as “not started.”
- Notify on **started**, **success**, **failure**, and **cancelled**.
- At `init`, optionally collect Slack bot token + channel (+ optional member ID).
- Tag the runner: prefer configured Slack Member ID; fall back to `users.lookupByEmail` using git `user.email`.
- If Slack is configured, it replaces desktop notifications; otherwise keep desktop.
- `--no-notify` disables all notifications.

## Non-goals

- Slack Incoming Webhooks as the primary path.
- Tagging multiple people or sheet-driven assignee lists.
- Full Slack Go SDK / requiring the `gh` CLI.
- Changing merge/build/tag creation behavior.

## Approach

**Minimal GitHub query fix + Slack Web API (HTTP).**

Chosen over webhooks (weak tagging / no email lookup) and `gh`+SDK (heavier deps and tooling).

---

## Poll reliability

### Current behavior

```
GET /repos/{owner}/{repo}/actions/runs?event=push&per_page=10
→ scan for head_branch == tag
→ any error ⇒ “Waiting for workflow to start…”
```

### New behavior

1. Query with tag filter:
   ```
   GET /repos/{owner}/{repo}/actions/runs?branch={tag}&per_page=5
   ```
   GitHub’s `branch` query param also matches tag names for tag-push runs.
2. Classify outcomes:
   - **No runs yet** → keep polling until timeout (same UX message).
   - **Auth / rate-limit / unexpected API errors** → print the error and exit (do not pretend the workflow hasn’t started).
   - **Run found** → track by `run_id` for subsequent polls (prefer status-by-id once known).
3. **Started event:** the first time a run is seen with status `queued` or `in_progress`, emit a single “started” notification for that poll session (Slack or desktop, per delivery rules).
4. **Terminal events:** when `status == completed`, notify for `conclusion` in `{success, failure, cancelled}` only. Other conclusions (e.g. `skipped`) print to the console with the run URL and do **not** send a notification.

### Poll loop sketch

```
startedNotified = false
runID = 0
loop:
  status, err = check(tag, runID)
  if err is fatal API → exit with message
  if err is not-found → wait; continue
  if runID == 0 → runID = status.RunID
  if !startedNotified && status in {queued, in_progress}:
    notify(started); startedNotified = true
  if status == completed:
    notify(conclusion); return
  wait interval
```

---

## Init & config

### New `forklift init` prompts (after GitHub polling section)

```
--- Slack Notifications (Optional) ---
Enter Slack Bot Token (press Enter to skip/keep):
Enter Slack Channel (ID or #name):
Enter Slack Member ID (optional, press Enter to use email lookup):
```

### Validation

- If **either** bot token or channel is non-empty after the prompt merge with existing config, **both** are required; otherwise refuse to save with a clear error.
- Member ID is always optional.
- Empty Enter keeps the existing value (same pattern as GitHub token).

### Config fields (`structures.Config` / `config.json`)

| Field | JSON key | Required when Slack used |
|-------|----------|---------------------------|
| Bot token | `slack_bot_token` | yes |
| Channel | `slack_channel` | yes |
| Member ID | `slack_member_id` | no |

Existing fields (`sheet_id`, `credentials_path`, `github_token`, poll interval/timeout) unchanged.

### Slack app prerequisites (documented in README)

- Bot token (`xoxb-...`)
- Scopes: `chat:write`, `users:read.email` (for email lookup fallback)
- Bot invited to the target channel

---

## Slack notifications

### Package

New `internal/slack` using `net/http` against Slack Web API (no SDK).

### Resolve mention target

1. If `slack_member_id` set → use it.
2. Else read git `user.email` and call `users.lookupByEmail`.
3. If unresolved → post without `<@…>` and print a one-line warning.

### Message content

Include at least: event kind, tag name, and workflow run HTML URL when available.

Examples:

- `🚧 Forklift: workflow started for tag v-dev-0.0.5 — <@U…> {url}`
- `✅ Forklift: tag v-dev-0.0.5 built successfully — <@U…> {url}`
- `❌ Forklift: tag v-dev-0.0.5 build failed — <@U…> {url}`
- `⚠️ Forklift: tag v-dev-0.0.5 build cancelled — <@U…> {url}`

### Delivery rules in `poll tag`

| Slack configured? | `--no-notify` | Behavior |
|-------------------|---------------|----------|
| yes | false | Slack only for started / success / failure / cancelled (no desktop) |
| no | false | Desktop for the same four events |
| any | true | No notifications |

“Slack configured” means both `slack_bot_token` and `slack_channel` are non-empty.

### Failure handling

Slack API failures print a warning and do **not** fail the poll command after status is known.

---

## Components & data flow

```
forklift init
  → prompts → structures.Config → config.json

forklift poll tag [tag]
  → load config
  → resolve tag (arg or sheet latest)
  → github.Client: poll run for tag
  → on started / terminal:
       if slack configured → slack.Notify(...)
       else → notification.Send(...)  // unless --no-notify
```

| Unit | Responsibility |
|------|----------------|
| `internal/github` | Tag-scoped run lookup, run-by-id status, typed errors (`ErrNotFound` vs fatal) |
| `internal/slack` | User resolve + `chat.postMessage` |
| `internal/notification` | Desktop only (unchanged API) |
| `cmd/init` | Collect + validate Slack fields |
| `cmd/poll` | Orchestrate poll loop + notify routing |

---

## Testing

- **GitHub client:** URL includes `branch=<tag>`; not-found vs fatal error classification; started vs completed handling.
- **Slack:** mention precedence (member ID > email > none); message formatting.
- **Init validation:** token-only / channel-only rejected; both or neither accepted; member ID optional.
- Manual: poll a real tag with/without Slack config; confirm single “started” message then one terminal message.

---

## Docs

Update README poll/init sections:

- Slack optional setup and both-or-neither rule
- Required bot scopes and inviting the bot to the channel
- Notify events: started / success / failure / cancelled
- Slack replaces desktop when configured; `--no-notify` disables all

## Out of scope follow-ups

- Caching resolved Slack user ID back into config after email lookup
- Per-repo Slack channels
- Notifying on non-tag workflow events
