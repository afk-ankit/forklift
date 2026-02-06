# Forklift

A CLI tool for managing merge branches across repositories using Google Sheets.

## Simple Installation

1. **Clone the repository:**
   ```bash
   git clone https://github.com/YOUR_USERNAME/forklift.git
   cd forklift
   ```

2. **Run the install script:**
   ```bash
   ./install.sh
   # This will build the binary and move it to /usr/local/bin
   # You might be asked for your password for sudo permissions
   ```

3. **Initialize:**
   ```bash
   forklift init
   ```

## Usage

### Initialize configuration
```bash
forklift init
```
This will ask for your Google Sheet URL and the path to your credentials.json file.

### Set merge branch for current repository
```bash
forklift set merge-branch <branch-name>
```

### Get merge branch for current repository
```bash
forklift get merge-branch
```

## Setup Requirements

1. **Google Sheet**: Create a Google Sheet with columns: `Repository`, `Merge Branch`, `Updated At`.
2. **Service Account**:
   - Create a Google Cloud service account with Sheets API access.
   - Download the `credentials.json` file.
   - Share your Google Sheet with the service account email.

## Configuration

Forklift stores its configuration in `~/.config/forklift/config.json`.
