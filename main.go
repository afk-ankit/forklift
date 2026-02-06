package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

const (
	defaultSheetName = "merge_branches"
)

type config struct {
	SheetID         string `json:"sheet_id"`
	SheetName       string `json:"sheet_name"`
	CredentialsPath string `json:"credentials_path"`
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}

	command := os.Args[1]
	if command == "init" {
		if err := runInit(); err != nil {
			fatalf("init failed: %v", err)
		}
		return
	}

	if len(os.Args) < 3 {
		printUsage()
		os.Exit(2)
	}

	subject := os.Args[2]

	if command != "set" && command != "get" {
		printUsage()
		os.Exit(2)
	}

	if subject != "merge-branch" {
		printUsage()
		os.Exit(2)
	}

	repoName, err := detectRepoName()
	if err != nil {
		fatalf("failed to detect repo name: %v", err)
	}

	cfg, err := loadConfig()
	if err != nil {
		fatalf("failed to load config: %v", err)
	}

	sheetID := os.Getenv("FORKLIFT_SHEET_ID")
	if sheetID == "" {
		sheetID = cfg.SheetID
	}
	if sheetID == "" {
		fatalf("FORKLIFT_SHEET_ID is not set")
	}

	sheetName := os.Getenv("FORKLIFT_SHEET_NAME")
	if sheetName == "" {
		sheetName = cfg.SheetName
	}
	if sheetName == "" {
		sheetName = defaultSheetName
	}

	credentialsPath := os.Getenv("FORKLIFT_GOOGLE_CREDENTIALS")
	if credentialsPath == "" {
		credentialsPath = cfg.CredentialsPath
	}
	if credentialsPath == "" {
		fatalf("FORKLIFT_GOOGLE_CREDENTIALS is not set")
	}

	ctx := context.Background()
	service, err := newSheetsService(ctx, credentialsPath)
	if err != nil {
		fatalf("failed to initialize Google Sheets client: %v", err)
	}

	switch command {
	case "set":
		if len(os.Args) < 4 {
			fatalf("missing merge-branch name")
		}
		branch := os.Args[3]
		if strings.TrimSpace(branch) == "" {
			fatalf("merge-branch name cannot be empty")
		}

		existing, err := getMergeBranch(ctx, service, sheetID, sheetName, repoName)
		if err != nil {
			fatalf("failed to read merge-branch: %v", err)
		}
		if existing != "" {
			fatalf("merge-branch already set for %s: %s", repoName, existing)
		}

		if err := setMergeBranch(ctx, service, sheetID, sheetName, repoName, branch); err != nil {
			fatalf("failed to set merge-branch: %v", err)
		}
		fmt.Printf("merge-branch set for %s: %s\n", repoName, branch)
	case "get":
		existing, err := getMergeBranch(ctx, service, sheetID, sheetName, repoName)
		if err != nil {
			fatalf("failed to read merge-branch: %v", err)
		}
		if existing == "" {
			fmt.Println("no merge branches set yet. to set run: forklift set merge-branch <name>")
			return
		}
		fmt.Println(existing)
	}
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  forklift set merge-branch <name>")
	fmt.Println("  forklift get merge-branch")
	fmt.Println("  forklift init")
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func newSheetsService(ctx context.Context, credentialsPath string) (*sheets.Service, error) {
	if !filepath.IsAbs(credentialsPath) {
		return nil, fmt.Errorf("FORKLIFT_GOOGLE_CREDENTIALS must be an absolute path: %s", credentialsPath)
	}

	return sheets.NewService(ctx, option.WithCredentialsFile(credentialsPath), option.WithScopes(sheets.SpreadsheetsScope))
}

func detectRepoName() (string, error) {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	remote := strings.TrimSpace(string(output))
	if remote == "" {
		return "", errors.New("origin remote is empty")
	}

	repo, err := parseRepoName(remote)
	if err != nil {
		return "", err
	}
	return repo, nil
}

func parseRepoName(remote string) (string, error) {
	remote = strings.TrimSuffix(remote, ".git")

	// Handles formats:
	// - git@github.com:org/repo
	// - https://github.com/org/repo
	// - ssh://git@github.com/org/repo
	patterns := []string{
		`^git@[^:]+:([^/]+)/([^/]+)$`,
		`^https?://[^/]+/([^/]+)/([^/]+)$`,
		`^ssh://git@[^/]+/([^/]+)/([^/]+)$`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(remote)
		if len(matches) == 3 {
			return matches[2], nil
		}
	}

	parts := strings.Split(remote, "/")
	if len(parts) == 0 {
		return "", fmt.Errorf("unable to parse repo from remote: %s", remote)
	}
	return parts[len(parts)-1], nil
}

func getMergeBranch(ctx context.Context, service *sheets.Service, sheetID, sheetName, repo string) (string, error) {
	rangeName := fmt.Sprintf("%s!A:B", sheetName)
	resp, err := service.Spreadsheets.Values.Get(sheetID, rangeName).Context(ctx).Do()
	if err != nil {
		return "", err
	}

	for _, row := range resp.Values {
		if len(row) < 2 {
			continue
		}
		repoName, ok := row[0].(string)
		if !ok {
			continue
		}
		if repoName == repo {
			branch, _ := row[1].(string)
			return strings.TrimSpace(branch), nil
		}
	}

	return "", nil
}

func setMergeBranch(ctx context.Context, service *sheets.Service, sheetID, sheetName, repo, branch string) error {
	values := []interface{}{repo, branch, time.Now().UTC().Format(time.RFC3339)}
	vr := &sheets.ValueRange{Values: [][]interface{}{values}}
	_, err := service.Spreadsheets.Values.Append(sheetID, fmt.Sprintf("%s!A:C", sheetName), vr).
		ValueInputOption("RAW").
		InsertDataOption("INSERT_ROWS").
		Context(ctx).
		Do()
	return err
}

func loadConfig() (config, error) {
	cfgPath, err := configPath()
	if err != nil {
		return config{}, err
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return config{}, nil
		}
		return config{}, err
	}
	var cfg config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return config{}, err
	}
	return cfg, nil
}

func saveConfig(cfg config) error {
	cfgPath, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cfgPath, data, 0600)
}

func configPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "forklift", "config.json"), nil
}

func runInit() error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Sheet URL: ")
	sheetURL, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	sheetURL = strings.TrimSpace(sheetURL)
	sheetID, err := extractSheetID(sheetURL)
	if err != nil {
		return err
	}

	fmt.Print("Service account JSON absolute path: ")
	credPath, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	credPath = strings.TrimSpace(credPath)
	if credPath == "" {
		return errors.New("credentials path cannot be empty")
	}
	if !filepath.IsAbs(credPath) {
		abs, err := filepath.Abs(credPath)
		if err != nil {
			return err
		}
		credPath = abs
	}

	fmt.Printf("Sheet tab name (default %s): ", defaultSheetName)
	sheetName, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	sheetName = strings.TrimSpace(sheetName)
	if sheetName == "" {
		sheetName = defaultSheetName
	}

	cfg := config{
		SheetID:         sheetID,
		SheetName:       sheetName,
		CredentialsPath: credPath,
	}
	if err := saveConfig(cfg); err != nil {
		return err
	}

	cfgPath, err := configPath()
	if err == nil {
		fmt.Printf("Saved config to %s\n", cfgPath)
	}
	fmt.Println("You can now run: forklift get merge-branch")
	return nil
}

func extractSheetID(sheetURL string) (string, error) {
	if sheetURL == "" {
		return "", errors.New("sheet URL cannot be empty")
	}
	// Supports full URLs like https://docs.google.com/spreadsheets/d/<id>/edit
	re := regexp.MustCompile(`^https?://docs\.google\.com/spreadsheets/d/([^/]+)/?`)
	matches := re.FindStringSubmatch(sheetURL)
	if len(matches) == 2 {
		return matches[1], nil
	}
	// Allow providing just the sheet ID.
	if !strings.Contains(sheetURL, "/") {
		return sheetURL, nil
	}
	return "", fmt.Errorf("unable to parse sheet id from URL: %s", sheetURL)
}
