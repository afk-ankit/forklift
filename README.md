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
**What it does:**
1.  **Stashes** local changes.
2.  **Switches** to the configured merge branch.
3.  **Pulls** the latest changes.
4.  **Merges** your original branch into it.
5.  **Calculates** the next version tag.
6.  **Pushes** the merge commit and the new tag.
7.  **Updates** the Google Sheet with the new tag and your identity.
8.  **Restores** your original branch and stash.

**Conflict Handling:**
If a conflict occurs, Forklift stops and tells you to resolve it. 
1.  Fix conflicts manually.
2.  `git add .` and `git commit`.
3.  Run `forklift build merge` again to **resume** (it will detect the state and finish the tagging/pushing).

## Project Structure

This project follows a standard modular Go layout:

- `cmd/`: Contains Cobra CLI command definitions (`root`, `init`, `get`, `set`, `build`).
- `internal/`: Contains private application logic.
  - `config/`: Configuration management.
  - `git/`: Git command wrappers and helpers.
  - `sheets/`: Google Sheets API integration.
  - `build/`: Core build and merge workflow logic.
- `main.go`: Entry point.

## Contributing

1.  Clone the repo.
2.  Make changes.
3.  Run `go build` to verify.
4.  Submit a PR.
