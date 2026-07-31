# Poll Tag Reliability + Slack Notify Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix `forklift poll tag` so it reliably finds tag workflow runs and surfaces real API errors, and add optional Slack notifications (with user tagging) configured at `forklift init`.

**Architecture:** Tighten `internal/github` to query runs by tag (`branch=` param), return typed not-found vs fatal errors, and support follow-up by run ID. Add a small `internal/slack` HTTP client for mention resolution + `chat.postMessage`. Extend config/`init` with Slack fields (both-or-neither). `cmd/poll` emits started + terminal notifications via Slack when configured, else desktop.

**Tech Stack:** Go 1.25, Cobra, `net/http` + `httptest` for GitHub/Slack (no Slack SDK), existing `beeep` desktop notifications.

**Spec:** `docs/superpowers/specs/2026-07-31-poll-tag-slack-notify-design.md`

## Global Constraints

- No Slack SDK; use `net/http` against Slack Web API only.
- Slack bot token + channel are both-or-neither; member ID optional.
- Notify events: started, success, failure, cancelled only (not skipped/other).
- When Slack is configured, it replaces desktop; `--no-notify` disables all.
- Do not change merge/build/tag creation behavior.
- Prefer co-located `*_test.go` tests; project currently has none — introduce them with this work.

## File Structure

| File | Responsibility |
|------|----------------|
| `internal/structures/structures.go` | Add Slack config fields |
| `internal/config/config.go` | `SlackConfigured`, `ValidateSlack` helpers |
| `internal/config/config_test.go` | Validation + configured tests |
| `internal/github/github.go` | Tag-scoped run lookup, run-by-id, typed errors |
| `internal/github/github_test.go` | httptest tests for query/errors |
| `internal/slack/slack.go` | Resolve user + post message |
| `internal/slack/slack_test.go` | httptest tests for mention + post |
| `internal/git/git.go` | `UserEmail()` helper for Slack lookup |
| `cmd/init.go` | Slack prompts + validation before save |
| `cmd/poll.go` | Poll loop with started notify + Slack/desktop routing |
| `README.md` | Document Slack setup and notify behavior |

---

### Task 1: Config fields + Slack validation helpers

**Files:**
- Modify: `internal/structures/structures.go`
- Modify: `internal/config/config.go`
- Create: `internal/config/config_test.go`

**Interfaces:**
- Consumes: existing `structures.Config`, `config.Load`/`Save`
- Produces:
  - `structures.Config` fields: `SlackBotToken`, `SlackChannel`, `SlackMemberID` (json: `slack_bot_token`, `slack_channel`, `slack_member_id`)
  - `func ValidateSlack(token, channel string) error`
  - `func SlackConfigured(cfg structures.Config) bool` — true iff both token and channel non-empty

- [ ] **Step 1: Write the failing tests**

Create `internal/config/config_test.go`:

```go
package config

import (
	"forklift/internal/structures"
	"strings"
	"testing"
)

func TestValidateSlack_bothOrNeither(t *testing.T) {
	if err := ValidateSlack("", ""); err != nil {
		t.Fatalf("empty pair should be ok, got %v", err)
	}
	if err := ValidateSlack("xoxb-1", "#builds"); err != nil {
		t.Fatalf("both set should be ok, got %v", err)
	}
	if err := ValidateSlack("xoxb-1", ""); err == nil {
		t.Fatal("token without channel should error")
	}
	if err := ValidateSlack("", "#builds"); err == nil {
		t.Fatal("channel without token should error")
	}
}

func TestSlackConfigured(t *testing.T) {
	if SlackConfigured(structures.Config{}) {
		t.Fatal("empty config should not be configured")
	}
	if SlackConfigured(structures.Config{SlackBotToken: "xoxb-1"}) {
		t.Fatal("token only should not be configured")
	}
	if !SlackConfigured(structures.Config{SlackBotToken: "xoxb-1", SlackChannel: "#builds"}) {
		t.Fatal("token+channel should be configured")
	}
}

func TestValidateSlack_errorMentionsBoth(t *testing.T) {
	err := ValidateSlack("xoxb-1", "")
	if err == nil || !strings.Contains(err.Error(), "both") {
		t.Fatalf("expected error mentioning both fields, got %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/ -v`

Expected: FAIL — `ValidateSlack` / `SlackConfigured` undefined; struct fields missing if referenced later.

- [ ] **Step 3: Implement minimal code**

In `internal/structures/structures.go`, add to `Config`:

```go
SlackBotToken string `json:"slack_bot_token,omitempty"`
SlackChannel  string `json:"slack_channel,omitempty"`
SlackMemberID string `json:"slack_member_id,omitempty"`
```

In `internal/config/config.go`, add:

```go
import "fmt"

func ValidateSlack(token, channel string) error {
	token = strings.TrimSpace(token)
	channel = strings.TrimSpace(channel)
	if (token == "") == (channel == "") {
		return nil
	}
	return fmt.Errorf("slack bot token and channel must both be set, or both left empty")
}

func SlackConfigured(cfg structures.Config) bool {
	return strings.TrimSpace(cfg.SlackBotToken) != "" && strings.TrimSpace(cfg.SlackChannel) != ""
}
```

Add `"strings"` to imports in `config.go`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/config/ -v`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/structures/structures.go internal/config/config.go internal/config/config_test.go
git commit -m "Add Slack config fields and both-or-neither validation."
```

---

### Task 2: Fix GitHub tag workflow lookup + typed errors

**Files:**
- Modify: `internal/github/github.go`
- Create: `internal/github/github_test.go`

**Interfaces:**
- Consumes: existing `Client`, `WorkflowStatus`
- Produces:
  - `var ErrNotFound = errors.New("no workflow runs found for tag")`
  - `func (c *Client) CheckWorkflowStatusForTag(tag string) (*WorkflowStatus, error)` — uses `branch={tag}` query; returns `ErrNotFound` when empty; fatal errors otherwise
  - `func (c *Client) GetWorkflowRun(runID int64) (*WorkflowStatus, error)` — `GET /repos/{owner}/{repo}/actions/runs/{run_id}`

- [ ] **Step 1: Write the failing tests**

Create `internal/github/github_test.go`:

```go
package github

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestCheckWorkflowStatusForTag_usesBranchQuery(t *testing.T) {
	var gotPath, gotRawQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotRawQuery = r.URL.RawQuery
		_ = json.NewEncoder(w).Encode(map[string]any{
			"workflow_runs": []map[string]any{
				{
					"id":          int64(42),
					"status":      "in_progress",
					"conclusion":  nil,
					"html_url":    "https://example.com/run/42",
					"head_branch": "v-dev-0.0.5",
					"created_at":  "2026-07-31T10:00:00Z",
				},
			},
		})
	}))
	defer srv.Close()

	c := NewClient("token", "org", "repo")
	c.baseURL = srv.URL // see Step 3 — add baseURL field for tests

	st, err := c.CheckWorkflowStatusForTag("v-dev-0.0.5")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if st.RunID != 42 || st.Status != "in_progress" {
		t.Fatalf("unexpected status: %+v", st)
	}
	if !strings.Contains(gotPath, "/repos/org/repo/actions/runs") {
		t.Fatalf("path = %s", gotPath)
	}
	q, _ := url.ParseQuery(gotRawQuery)
	if q.Get("branch") != "v-dev-0.0.5" {
		t.Fatalf("branch query = %q", q.Get("branch"))
	}
}

func TestCheckWorkflowStatusForTag_notFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"workflow_runs": []any{}})
	}))
	defer srv.Close()

	c := NewClient("", "org", "repo")
	c.baseURL = srv.URL

	_, err := c.CheckWorkflowStatusForTag("missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestCheckWorkflowStatusForTag_apiErrorIsFatal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Bad credentials"}`))
	}))
	defer srv.Close()

	c := NewClient("bad", "org", "repo")
	c.baseURL = srv.URL

	_, err := c.CheckWorkflowStatusForTag("v-1")
	if err == nil || errors.Is(err, ErrNotFound) {
		t.Fatalf("want fatal API error, got %v", err)
	}
	if !strings.Contains(err.Error(), "401") && !strings.Contains(err.Error(), "GitHub API error") {
		t.Fatalf("want API error message, got %v", err)
	}
}

func TestGetWorkflowRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/org/repo/actions/runs/99" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":         int64(99),
			"status":     "completed",
			"conclusion": "success",
			"html_url":   "https://example.com/run/99",
			"created_at": "2026-07-31T10:00:00Z",
		})
	}))
	defer srv.Close()

	c := NewClient("t", "org", "repo")
	c.baseURL = srv.URL

	st, err := c.GetWorkflowRun(99)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if st.Status != "completed" || st.Conclusion != "success" || st.RunID != 99 {
		t.Fatalf("unexpected: %+v", st)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/github/ -v`

Expected: FAIL — `baseURL` / `ErrNotFound` / `GetWorkflowRun` missing or query still wrong.

- [ ] **Step 3: Implement**

Replace `internal/github/github.go` with this shape (keep package/types; add `baseURL`, `ErrNotFound`, refactor request helper):

```go
package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

var ErrNotFound = errors.New("no workflow runs found for tag")

type WorkflowStatus struct {
	Status     string
	Conclusion string
	HTMLURL    string
	RunID      int64
	CreatedAt  time.Time
}

type Client struct {
	token   string
	owner   string
	repo    string
	baseURL string // default https://api.github.com; overridable in tests
	http    *http.Client
}

func NewClient(token, owner, repo string) *Client {
	return &Client{
		token:   token,
		owner:   owner,
		repo:    repo,
		baseURL: "https://api.github.com",
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) CheckWorkflowStatusForTag(tag string) (*WorkflowStatus, error) {
	u := fmt.Sprintf("%s/repos/%s/%s/actions/runs?branch=%s&per_page=5",
		c.baseURL, c.owner, c.repo, url.QueryEscape(tag))

	var result struct {
		WorkflowRuns []workflowRun `json:"workflow_runs"`
	}
	if err := c.getJSON(u, &result); err != nil {
		return nil, err
	}
	if len(result.WorkflowRuns) == 0 {
		return nil, ErrNotFound
	}
	return result.WorkflowRuns[0].toStatus(), nil
}

func (c *Client) GetWorkflowRun(runID int64) (*WorkflowStatus, error) {
	u := fmt.Sprintf("%s/repos/%s/%s/actions/runs/%d", c.baseURL, c.owner, c.repo, runID)
	var run workflowRun
	if err := c.getJSON(u, &run); err != nil {
		return nil, err
	}
	return run.toStatus(), nil
}

type workflowRun struct {
	ID         int64     `json:"id"`
	Status     string    `json:"status"`
	Conclusion *string   `json:"conclusion"`
	HTMLURL    string    `json:"html_url"`
	HeadBranch string    `json:"head_branch"`
	CreatedAt  time.Time `json:"created_at"`
}

func (r workflowRun) toStatus() *WorkflowStatus {
	conclusion := ""
	if r.Conclusion != nil {
		conclusion = *r.Conclusion
	}
	return &WorkflowStatus{
		Status:     r.Status,
		Conclusion: conclusion,
		HTMLURL:    r.HTMLURL,
		RunID:      r.ID,
		CreatedAt:  r.CreatedAt,
	}
}

func (c *Client) getJSON(rawURL string, dest any) error {
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch workflow runs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub API error (status %d): %s", resp.StatusCode, string(body))
	}
	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}
	return nil
}
```

Note: `baseURL` is unexported but same-package tests can set `c.baseURL = srv.URL`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/github/ -v`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/github/github.go internal/github/github_test.go
git commit -m "Fix tag workflow polling with branch filter and typed errors."
```

---

### Task 3: Slack client (resolve user + post message)

**Files:**
- Create: `internal/slack/slack.go`
- Create: `internal/slack/slack_test.go`
- Modify: `internal/git/git.go` (add `UserEmail`)

**Interfaces:**
- Consumes: bot token, channel, optional member ID, git email
- Produces:
  - `type Client struct` with `NewClient(token, channel, memberID string) *Client`
  - `func (c *Client) Notify(event, tag, htmlURL, gitEmail string) (mentioned bool, err error)`
  - Events: `"started"`, `"success"`, `"failure"`, `"cancelled"`
  - `func UserEmail() string` in `internal/git` — returns trimmed `git config user.email` or `""`

- [ ] **Step 1: Write failing Slack tests**

Create `internal/slack/slack_test.go`:

```go
package slack

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNotify_usesMemberIDWithoutLookup(t *testing.T) {
	var posted map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "lookupByEmail") {
			t.Fatal("should not lookup when member ID set")
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &posted)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	c := NewClient("xoxb-test", "#builds", "U123")
	c.baseURL = srv.URL

	mentioned, err := c.Notify("success", "v-dev-0.0.5", "https://example.com/run", "me@example.com")
	if err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if !mentioned {
		t.Fatal("expected mentioned=true")
	}
	if !strings.Contains(posted["text"], "<@U123>") {
		t.Fatalf("text missing mention: %q", posted["text"])
	}
	if !strings.Contains(posted["text"], "v-dev-0.0.5") {
		t.Fatalf("text missing tag: %q", posted["text"])
	}
	if posted["channel"] != "#builds" {
		t.Fatalf("channel = %q", posted["channel"])
	}
}

func TestNotify_emailLookupFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "lookupByEmail"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok":   true,
				"user": map[string]any{"id": "U999"},
			})
		case strings.Contains(r.URL.Path, "chat.postMessage"):
			body, _ := io.ReadAll(r.Body)
			var posted map[string]string
			_ = json.Unmarshal(body, &posted)
			if !strings.Contains(posted["text"], "<@U999>") {
				t.Errorf("expected <@U999>, got %q", posted["text"])
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	c := NewClient("xoxb-test", "C123", "")
	c.baseURL = srv.URL

	mentioned, err := c.Notify("started", "v-1", "https://example.com", "dev@example.com")
	if err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if !mentioned {
		t.Fatal("expected mentioned=true after email lookup")
	}
}

func TestNotify_noMentionWhenUnresolved(t *testing.T) {
	var posted map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "lookupByEmail") {
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "users_not_found"})
			return
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &posted)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	c := NewClient("xoxb-test", "#builds", "")
	c.baseURL = srv.URL

	mentioned, err := c.Notify("failure", "v-1", "https://example.com", "missing@example.com")
	if err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if mentioned {
		t.Fatal("expected mentioned=false")
	}
	if strings.Contains(posted["text"], "<@") {
		t.Fatalf("should not mention: %q", posted["text"])
	}
}

func TestFormatMessage(t *testing.T) {
	cases := []struct {
		event string
		want  string
	}{
		{"started", "started"},
		{"success", "successfully"},
		{"failure", "failed"},
		{"cancelled", "cancelled"},
	}
	for _, tc := range cases {
		msg := formatMessage(tc.event, "v-1", "U1", "https://x")
		if !strings.Contains(msg, tc.want) || !strings.Contains(msg, "<@U1>") {
			t.Fatalf("event %s => %q", tc.event, msg)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/slack/ -v`

Expected: FAIL — package/types missing.

- [ ] **Step 3: Implement Slack client + UserEmail**

Add to `internal/git/git.go`:

```go
func UserEmail() string {
	out, err := exec.Command("git", "config", "user.email").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
```

Create `internal/slack/slack.go`:

```go
package slack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	token    string
	channel  string
	memberID string
	baseURL  string
	http     *http.Client
}

func NewClient(token, channel, memberID string) *Client {
	return &Client{
		token:    token,
		channel:  channel,
		memberID: memberID,
		baseURL:  "https://slack.com/api",
		http:     &http.Client{Timeout: 10 * time.Second},
	}
}

// Notify posts a channel message for a poll event. gitEmail is used only when memberID is empty.
func (c *Client) Notify(event, tag, htmlURL, gitEmail string) (bool, error) {
	userID := strings.TrimSpace(c.memberID)
	if userID == "" && gitEmail != "" {
		id, err := c.lookupByEmail(gitEmail)
		if err == nil {
			userID = id
		}
	}
	text := formatMessage(event, tag, userID, htmlURL)
	if err := c.postMessage(text); err != nil {
		return false, err
	}
	return userID != "", nil
}

func formatMessage(event, tag, userID, htmlURL string) string {
	mention := ""
	if userID != "" {
		mention = " — <@" + userID + ">"
	}
	link := ""
	if htmlURL != "" {
		link = " " + htmlURL
	}
	switch event {
	case "started":
		return fmt.Sprintf("🚧 Forklift: workflow started for tag %s%s%s", tag, mention, link)
	case "success":
		return fmt.Sprintf("✅ Forklift: tag %s built successfully%s%s", tag, mention, link)
	case "failure":
		return fmt.Sprintf("❌ Forklift: tag %s build failed%s%s", tag, mention, link)
	case "cancelled":
		return fmt.Sprintf("⚠️ Forklift: tag %s build cancelled%s%s", tag, mention, link)
	default:
		return fmt.Sprintf("Forklift: tag %s %s%s%s", tag, event, mention, link)
	}
}

func (c *Client) lookupByEmail(email string) (string, error) {
	u := c.baseURL + "/users.lookupByEmail?email=" + url.QueryEscape(email)
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
		User  struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if !result.OK {
		return "", fmt.Errorf("slack lookupByEmail: %s", result.Error)
	}
	return result.User.ID, nil
}

func (c *Client) postMessage(text string) error {
	payload, _ := json.Marshal(map[string]string{
		"channel": c.channel,
		"text":    text,
	})
	req, err := http.NewRequest("POST", c.baseURL+"/chat.postMessage", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var result struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return err
	}
	if !result.OK {
		return fmt.Errorf("slack chat.postMessage: %s", result.Error)
	}
	return nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/slack/ ./internal/git/ -v`  
(git package may have no tests; `go test ./internal/slack/ -v` is enough)

Expected: PASS for slack tests.

- [ ] **Step 5: Commit**

```bash
git add internal/slack/slack.go internal/slack/slack_test.go internal/git/git.go
git commit -m "Add Slack notify client with member ID and email lookup."
```

---

### Task 4: Extend `forklift init` with Slack prompts

**Files:**
- Modify: `cmd/init.go`

**Interfaces:**
- Consumes: `config.ValidateSlack`, existing prompt helper pattern
- Produces: saved config including Slack fields; refuses save when validation fails

- [ ] **Step 1: Update init to prompt and validate**

After the GitHub polling section in `cmd/init.go` (before building `cfg`), add:

```go
fmt.Print("\n--- Slack Notifications (Optional) ---\n")
slackToken := prompt("Enter Slack Bot Token (press Enter to skip/keep)", currentCfg.SlackBotToken, false)
slackChannel := prompt("Enter Slack Channel (ID or #name)", currentCfg.SlackChannel, false)
slackMemberID := prompt("Enter Slack Member ID (optional, press Enter to use email lookup)", currentCfg.SlackMemberID, false)

if err := config.ValidateSlack(slackToken, slackChannel); err != nil {
	fatalf("%v", err)
}
```

Include the new fields in the `structures.Config{...}` literal:

```go
cfg := structures.Config{
	SheetID:         sheetID,
	SheetName:       sheetName,
	CredentialsPath: absPath,
	GitHubToken:     githubToken,
	PollInterval:    pollInterval,
	PollTimeout:     pollTimeout,
	SlackBotToken:   slackToken,
	SlackChannel:    slackChannel,
	SlackMemberID:   slackMemberID,
}
```

Update `Long` on `initCmd` to mention Slack optionally.

- [ ] **Step 2: Build to verify compile**

Run: `go build -o /dev/null .`

Expected: success

- [ ] **Step 3: Manual smoke (optional)**

Run `forklift init`, skip Slack with Enter/Enter — should still save.  
Or set token only — should fatal with both-or-neither message.

- [ ] **Step 4: Commit**

```bash
git add cmd/init.go
git commit -m "Prompt for optional Slack token, channel, and member ID on init."
```

---

### Task 5: Wire poll loop to new GitHub API + notify routing

**Files:**
- Modify: `cmd/poll.go`

**Interfaces:**
- Consumes: `github.ErrNotFound`, `GetWorkflowRun`, `config.SlackConfigured`, `slack.Client.Notify`, `git.UserEmail`, `notification.Send`
- Produces: poll loop that fatal-exits on non-ErrNotFound GitHub errors; one started notify; terminal notify for success/failure/cancelled; Slack XOR desktop

- [ ] **Step 1: Replace the poll loop body**

Keep flag/setup/tag resolution. Replace the polling `for` loop and add helpers in `cmd/poll.go`:

```go
startedNotified := false
var runID int64

for {
	var status *github.WorkflowStatus
	var err error
	if runID != 0 {
		status, err = client.GetWorkflowRun(runID)
	} else {
		status, err = client.CheckWorkflowStatusForTag(tag)
	}

	if err != nil {
		if errors.Is(err, github.ErrNotFound) {
			elapsed := time.Since(startTime)
			fmt.Printf("⏳ Waiting for workflow to start... (%s elapsed)\n", formatDuration(elapsed))
			if elapsed > timeoutDuration {
				fmt.Println("\n⏰ Timeout reached. No workflow found.")
				return
			}
			<-ticker.C
			continue
		}
		fatalf("failed to check workflow status: %v", err)
	}

	if runID == 0 {
		runID = status.RunID
	}

	elapsed := time.Since(status.CreatedAt)

	switch status.Status {
	case "queued":
		fmt.Printf("⏳ Status: queued (waiting to start)\n")
		if !startedNotified {
			notifyPoll(cfg, "started", tag, status.HTMLURL)
			startedNotified = true
		}
	case "in_progress":
		fmt.Printf("⏳ Status: in_progress (running for %s)\n", formatDuration(elapsed))
		if !startedNotified {
			notifyPoll(cfg, "started", tag, status.HTMLURL)
			startedNotified = true
		}
	case "completed":
		fmt.Printf("\n")
		switch status.Conclusion {
		case "success":
			fmt.Println("✅ Workflow completed successfully! 🎉")
			notifyPoll(cfg, "success", tag, status.HTMLURL)
		case "failure":
			fmt.Println("❌ Workflow failed.")
			notifyPoll(cfg, "failure", tag, status.HTMLURL)
		case "cancelled":
			fmt.Println("⚠️  Workflow was cancelled.")
			notifyPoll(cfg, "cancelled", tag, status.HTMLURL)
		default:
			fmt.Printf("⚠️  Workflow completed with status: %s\n", status.Conclusion)
		}
		fmt.Printf("🔗 %s\n", status.HTMLURL)
		return
	}

	if time.Since(startTime) > timeoutDuration {
		fmt.Println("\n⏰ Timeout reached.")
		return
	}

	<-ticker.C
}
```

Add imports: `"errors"`, `"forklift/internal/slack"` (and keep notification).

Add helper (same file):

```go
func notifyPoll(cfg structures.Config, event, tag, htmlURL string) {
	if noNotify {
		return
	}
	title, message := desktopCopy(event, tag)
	if config.SlackConfigured(cfg) {
		client := slack.NewClient(cfg.SlackBotToken, cfg.SlackChannel, cfg.SlackMemberID)
		mentioned, err := client.Notify(event, tag, htmlURL, git.UserEmail())
		if err != nil {
			fmt.Printf("⚠️  Slack notify failed: %v\n", err)
			return
		}
		if !mentioned {
			fmt.Println("⚠️  Could not resolve Slack user to tag; posted without a mention.")
		}
		return
	}
	_ = notification.Send(title, message)
}

func desktopCopy(event, tag string) (string, string) {
	switch event {
	case "started":
		return "Forklift Build Started", fmt.Sprintf("Tag %s workflow started.", tag)
	case "success":
		return "Forklift Build Complete", fmt.Sprintf("Tag %s built successfully!", tag)
	case "failure":
		return "Forklift Build Failed", fmt.Sprintf("Tag %s build failed.", tag)
	case "cancelled":
		return "Forklift Build Cancelled", fmt.Sprintf("Tag %s build was cancelled.", tag)
	default:
		return "Forklift", fmt.Sprintf("Tag %s: %s", tag, event)
	}
}
```

Add `"forklift/internal/structures"` import if `notifyPoll` takes `structures.Config` (or keep using the type already returned by `config.Load`). `Notify` returns `(mentioned bool, err error)` from Task 3.

- [ ] **Step 2: Build + unit tests**

Run:

```bash
go test ./internal/...
go build -o /dev/null .
```

Expected: all PASS, build OK.

- [ ] **Step 3: Commit**

```bash
git add cmd/poll.go internal/slack/slack.go internal/slack/slack_test.go
git commit -m "Wire poll tag to reliable GitHub status and Slack/desktop notify."
```

---

### Task 6: README updates

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Update init + poll sections**

In the Initialize section, note that `forklift init` can also collect optional Slack bot token, channel, and member ID (both token and channel required together).

Replace/extend “### 6. Poll GitHub Actions Workflow” notify bullets:

- Monitors GitHub Actions for the tag (filtered by tag name)
- Notifies on **started**, **success**, **failure**, and **cancelled**
- Slack (if configured) replaces desktop notifications
- `--no-notify` disables all notifications

Add subsection **Slack setup**:

1. Create a Slack app with bot token scopes: `chat:write`, `users:read.email`
2. Install to workspace; copy bot token (`xoxb-...`)
3. Invite the bot to the target channel
4. Run `forklift init` and provide token + channel; optionally Member ID (`U…`). If Member ID omitted, Forklift looks up Slack user by `git config user.email`.

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "Document Slack notify setup and poll tag behavior."
```

---

### Task 7: Final verification

- [ ] **Step 1: Run full test + build**

```bash
go test ./...
go build -o /dev/null .
```

Expected: PASS, build success.

- [ ] **Step 2: Spec checklist (manual)**

Confirm against `docs/superpowers/specs/2026-07-31-poll-tag-slack-notify-design.md`:

- [ ] Tag query uses `branch=`
- [ ] Fatal API errors exit; ErrNotFound keeps waiting
- [ ] Started notify once; terminal for success/failure/cancelled only
- [ ] Init Slack both-or-neither + optional member ID
- [ ] Slack replaces desktop; `--no-notify` disables all
- [ ] README documents scopes and behavior

- [ ] **Step 3: No further commit unless something failed and was fixed**

---

## Self-review (plan vs spec)

| Spec requirement | Task |
|------------------|------|
| Query `branch={tag}` | Task 2 |
| ErrNotFound vs fatal API | Task 2, 5 |
| Track by run_id | Task 2 `GetWorkflowRun`, Task 5 |
| Started once + success/failure/cancelled | Task 5 |
| Init Slack prompts + both-or-neither | Task 1, 4 |
| Member ID > email > none + warning | Task 3, 5 |
| Slack replaces desktop; `--no-notify` | Task 5 |
| README | Task 6 |
| Unit tests | Tasks 1–3, 5 |
| No SDK / no build-merge changes | Global + file list |

No placeholders left after review. Types aligned: `ValidateSlack`, `SlackConfigured`, `ErrNotFound`, `GetWorkflowRun`, `Notify(event, tag, htmlURL, gitEmail) (mentioned bool, err error)`.
