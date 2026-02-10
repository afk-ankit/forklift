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
- Go 1.25+
- Git
- Google Cloud Service Account Credentials (`credentials.json`)

### Quick Install
```bash
./install.sh
```
This will build the binary and move it to `/usr/local/bin/forklift`.

## Usage

### 1. Initialize
First, set up your configuration. You'll need your Google Sheet URL/ID and the path to your `credentials.json`.
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

### 4. Get Latest Tag
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

### 5. Build & Merge (The Magic Command)
Run this command to start the automated workflow:
```bash
forklift build merge
```

This will:
1. Stash your changes
2. Switch to the merge branch and pull latest
3. Merge your current branch
4. Create and push a new tag (auto-incremented)
5. Update the Google Sheet with the new tag
6. Return you to your original branch

If merge conflicts occur, resolve them manually, commit, and run `forklift build merge` again to finish.

### 6. Poll GitHub Actions Workflow (NEW! 🚀)
Monitor your GitHub Actions build in real-time and get notified when it completes:

```bash
# Poll a specific tag
forklift poll tag v-dev-0.0.5

# Poll the latest tag from the sheet (auto-detect)
forklift poll tag
# or explicitly
forklift poll tag --latest

# Customize polling interval and timeout
forklift poll tag --interval 10 --timeout 60

# Disable desktop notifications
forklift poll tag --no-notify
```

**What it does:**
- 🔄 Monitors GitHub Actions workflow status in real-time
- 🔔 Sends desktop notification when build completes
- ⏱️ Configurable polling interval (default: 30s)
- ⏰ Configurable timeout (default: 30m)
- 🏷️ Auto-detects latest tag if not specified

**Setup:**
Run `forklift init` and provide your GitHub Personal Access Token when prompted (optional but recommended to avoid rate limits).

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
