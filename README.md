# Forklift

**Forklift** is a powerful CLI tool designed to simplify and automate the Git workflow for managing merge branches and release tagging across multiple repositories. It uses a **Google Sheet** as a central source of truth for branch configuration and deployment history.

## Features

- **Centralized Configuration**: Manage merge branches for multiple repos in a single Google Sheet.
- **Automated Merging**: Fetches merge branch, pulls latest, merges your current branch, and handles conflicts intelligently.
- **Auto-Tagging**: Automatically increments semantic version tags (e.g., `v-dev-0.0.1` -> `v-dev-0.0.2`).
- **Conflict Handling**: Pauses on merge conflicts, allowing manual resolution, and resumes exactly where it left off.
- **Audit Trail**: Tracks who initiated the build and when, directly in the Google Sheet.
- **Safe Stashing**: Automatically stashes and restores your local changes.

## Installation

### Prerequisites
- Git
- Google Cloud Service Account Credentials (`credentials.json`)
- Go 1.25+ (only if building from source)

### Homebrew (recommended)
```bash
brew tap afk-ankit/tap
brew install forklift
```

Upgrade later with:
```bash
brew update
brew upgrade forklift
```

After install, run `forklift init` to get started.

Print the installed version:

```bash
forklift version
```

### Build from source
```bash
./install.sh
```
This builds the binary and installs it to `/usr/local/bin/forklift`.

## Usage

### 1. Initialize
First, set up your configuration. You'll need your Google Sheet URL/ID and the path to your `credentials.json`.

`forklift init` can also collect optional Slack settings: bot token, channel, and member ID. Token and channel must be provided together; member ID is optional.

It also asks whether `build merge` should create and push a tag. The default is yes. Answer `n` to only merge, push the merge branch, and switch back.

```bash
forklift init
```

### 2. Configure a Repo
Tell Forklift which branch this repository should merge into.
```bash
forklift set branch <branch-name>
# Example:
forklift set branch dev
```
*Note: If a branch is already set, Forklift will ask if you want to override it. Overriding resets the tag sequence.*

### 3. Check Configuration
See what the current merge branch is for the current repository.
```bash
forklift get branch
```

### 4. Check whether this branch is merged
Compares your current commits (and uncommitted files) with `origin/<merge-branch>` after a fetch. Exits 0 when everything is already on that branch, 1 otherwise.

```bash
forklift get merged
```

### 5. Get Latest Tag
View the latest tag recorded in the Google Sheet.
```bash
forklift get tag
```

**Copy tag to clipboard:**
```bash
forklift get tag --copy
# or
forklift get tag -c
```

### 6. Build & Merge (The Magic Command)
Run this command to start the automated workflow:
```bash
forklift build merge
```

This will:
1. Stash your changes
2. Switch to the merge branch and pull latest
3. Merge your current branch
4. Push the merge branch
5. Create and push a new tag, then update the Google Sheet (skipped when tagging is off in `forklift init`)
6. Return you to your original branch

If merge conflicts occur, resolve them manually, commit, and run `forklift build merge` again to finish.

After a successful merge that created a tag, Forklift automatically polls GitHub Actions for that tag in the background and sends the usual notifications (Slack if configured, otherwise desktop). The merge command returns immediately. With tagging off, it pushes the merge and switches back, and does not poll.

To skip background polling:

```bash
forklift build merge --no-poll
```

### 7. Poll GitHub Actions Workflow (NEW! 🚀)
Monitor your GitHub Actions build in real-time and get notified when it completes.

`forklift build merge` starts this automatically for the tag it just created (unless you pass `--no-poll`). You can also poll manually:

```bash
# Poll a specific tag
forklift poll tag v-dev-0.0.5

# Poll the latest tag from the sheet (auto-detect)
forklift poll tag
# or explicitly
forklift poll tag --latest

# Customize polling interval and timeout
forklift poll tag --interval 10 --timeout 60

# Disable all notifications
forklift poll tag --no-notify
```

**What it does:**
- Monitors GitHub Actions for the tag (filtered by tag name)
- Notifies on **started**, **success**, **failure**, and **cancelled**
- Slack (if configured) replaces desktop notifications
- `--no-notify` disables all notifications
- Configurable polling interval (default: 30s)
- Configurable timeout (default: 30m)
- Auto-detects latest tag if not specified

**Setup:**
Run `forklift init` and provide your GitHub Personal Access Token when prompted (optional but recommended to avoid rate limits).

#### Slack setup
1. Create a Slack app with bot token scopes: `chat:write`, `users:read.email`
2. Install to workspace; copy bot token (`xoxb-...`)
3. Invite the bot to the target channel
4. Run `forklift init` and provide token + channel; optionally Member ID (`U…`). If Member ID omitted, Forklift looks up Slack user by `git config user.email`.

#### How to get a GitHub Token
1. Go to **GitHub Settings** -> **Developer settings** -> **Personal access tokens** -> **Tokens (classic)**.
2. Click **Generate new token (classic)**.
3. Give it a name (e.g., "Forklift CLI").
4. Select the following scopes:
   - `repo` (Full control of private repositories)
   - `workflow` (Update GitHub Action workflows)
5. Click **Generate token** and copy the string starting with `ghp_`.

---

## Project Structure

This project follows a standard modular Go layout:

- `cmd/`: Contains Cobra CLI command definitions (`root`, `init`, `get`, `set`, `build`, `poll`).
- `internal/`: Contains private application logic.
  - `config/`: Configuration management.
  - `git/`: Git command wrappers and helpers.
  - `sheets/`: Google Sheets API integration.
  - `build/`: Core build and merge workflow logic.
  - `github/`: GitHub Actions API integration for workflow polling.
  - `notification/`: Cross-platform desktop notification system.
  - `clipboard/`: Cross-platform clipboard operations.
  - `structures/`: Shared data structures and types.
- `main.go`: Entry point.

## Contributing

1.  Clone the repo.
2.  Make changes.
3.  Run `go build` to verify.
4.  Submit a PR.
