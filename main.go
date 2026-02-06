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
	"strconv"
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

	if command != "set" && command != "get" && command != "build" {
		printUsage()
		os.Exit(2)
	}

	if command != "build" && subject != "merge-branch" {
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

		// Check if exists
		rowIdx, currentBranch, _, err := getRepoInfo(ctx, service, sheetID, sheetName, repoName)
		if err != nil {
			fatalf("failed to read repo info: %v", err)
		}
		if currentBranch != "" {
			fatalf("merge-branch already set for %s: %s", repoName, currentBranch)
		}

		if err := setMergeBranch(ctx, service, sheetID, sheetName, repoName, branch, rowIdx); err != nil {
			fatalf("failed to set merge-branch: %v", err)
		}
		fmt.Printf("merge-branch set for %s: %s\n", repoName, branch)

	case "get":
		_, branch, tag, err := getRepoInfo(ctx, service, sheetID, sheetName, repoName)
		if err != nil {
			fatalf("failed to read repo info: %v", err)
		}
		if branch == "" {
			fmt.Println("no merge branches set yet. to set run: forklift set merge-branch <name>")
			return
		}
		fmt.Printf("Merge Branch: %s\n", branch)
		if tag != "" {
			fmt.Printf("Latest Tag: %s\n", tag)
		}

	case "build":
		if subject != "merge" {
			printUsage()
			os.Exit(2)
		}
		if err := runBuildMerge(ctx, service, sheetID, sheetName, repoName); err != nil {
			fatalf("build merge failed: %v", err)
		}
	}
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  forklift set merge-branch <name>")
	fmt.Println("  forklift get merge-branch")
	fmt.Println("  forklift build merge")
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

// getRepoInfo returns the row index, merge branch, and latest tag.
// Row index is 0-indexed relative to the sheet (e.g. Row 1 is index 0).
func getRepoInfo(ctx context.Context, service *sheets.Service, sheetID, sheetName, repo string) (int, string, string, error) {
	rangeName := fmt.Sprintf("%s!A:D", sheetName)
	resp, err := service.Spreadsheets.Values.Get(sheetID, rangeName).Context(ctx).Do()
	if err != nil {
		return -1, "", "", err
	}

	for i, row := range resp.Values {
		if len(row) < 1 {
			continue
		}
		repoName, ok := row[0].(string)
		if !ok {
			continue
		}
		if repoName == repo {
			branch := ""
			tag := ""
			if len(row) > 1 {
				branch, _ = row[1].(string)
			}
			// Column D is index 3 (Repo, Branch, Time, Tag) - Wait.
			// Current Schema: A=Repo, B=Branch, C=Time.
			// We are adding D=Tag.
			// Ideally we want to keep C=Time for backward compat?
			// The user said "store tag name in sheets too".
			// Let's assume schema: Repo (A), Branch (B), Time (C), Tag (D).
			if len(row) > 3 {
				tag, _ = row[3].(string)
			}
			return i, strings.TrimSpace(branch), strings.TrimSpace(tag), nil
		}
	}

	return -1, "", "", nil
}

func setMergeBranch(ctx context.Context, service *sheets.Service, sheetID, sheetName, repo, branch string, rowIdx int) error {
	timestamp := time.Now().UTC().Format(time.RFC3339)

	if rowIdx >= 0 {
		// Update existing row
		// We update Branch (Col B) and Time (Col C). We leave Tag (Col D) alone?
		// Or we might want to clear Tag if branch changes?
		// For now, let's just update Branch and Timestamp.
		values := []interface{}{branch, timestamp}
		vr := &sheets.ValueRange{Values: [][]interface{}{values}}
		rangeName := fmt.Sprintf("%s!B%d:C%d", sheetName, rowIdx+1, rowIdx+1) // Sheet is 1-indexed
		_, err := service.Spreadsheets.Values.Update(sheetID, rangeName, vr).
			ValueInputOption("RAW").
			Context(ctx).
			Do()
		return err
	}

	// Append new row
	values := []interface{}{repo, branch, timestamp, ""}
	vr := &sheets.ValueRange{Values: [][]interface{}{values}}
	_, err := service.Spreadsheets.Values.Append(sheetID, fmt.Sprintf("%s!A:D", sheetName), vr).
		ValueInputOption("RAW").
		InsertDataOption("INSERT_ROWS").
		Context(ctx).
		Do()
	return err
}

func updateRepoTag(ctx context.Context, service *sheets.Service, sheetID, sheetName string, rowIdx int, tag string) error {
	if rowIdx < 0 {
		return errors.New("cannot update tag for non-existent repo row")
	}
	// Update Tag (Col D) and Time (Col C)
	timestamp := time.Now().UTC().Format(time.RFC3339)
	values := []interface{}{timestamp, tag}
	vr := &sheets.ValueRange{Values: [][]interface{}{values}}
	rangeName := fmt.Sprintf("%s!C%d:D%d", sheetName, rowIdx+1, rowIdx+1)
	_, err := service.Spreadsheets.Values.Update(sheetID, rangeName, vr).
		ValueInputOption("RAW").
		Context(ctx).
		Do()
	return err
}

type buildState struct {
	OriginalBranch string `json:"original_branch"`
	MergeBranch    string `json:"merge_branch"`
	Stashed        bool   `json:"stashed"`
	RepoName       string `json:"repo_name"`
	RowIdx         int    `json:"row_idx"`
}

func runBuildMerge(ctx context.Context, service *sheets.Service, sheetID, sheetName, repoName string) error {
	statePath, err := getBuildStatePath()
	if err == nil {
		if _, err := os.Stat(statePath); err == nil {
			return resumeBuildMerge(ctx, service, sheetID, sheetName, statePath)
		}
	}

	// 1. Get Repo Info
	rowIdx, mergeBranch, lastTag, err := getRepoInfo(ctx, service, sheetID, sheetName, repoName)
	if err != nil {
		return fmt.Errorf("failed to get repo info: %w", err)
	}
	if rowIdx == -1 {
		return fmt.Errorf("repo %s not found in sheet", repoName)
	}
	if mergeBranch == "" {
		return fmt.Errorf("merge-branch not set for %s", repoName)
	}

	// 2. Git Stash
	fmt.Println("Stashing changes...")
	stashed, err := gitStash()
	if err != nil {
		return fmt.Errorf("git stash failed: %w", err)
	}

	originalBranch, err := gitCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	// Save state before switching branches
	state := buildState{
		OriginalBranch: originalBranch,
		MergeBranch:    mergeBranch,
		Stashed:        stashed,
		RepoName:       repoName,
		RowIdx:         rowIdx,
	}
	if err := saveBuildState(state); err != nil {
		fmt.Printf("Warning: failed to save build state: %v\n", err)
	}

	// Define cleanup defer (only used if we finish successfully or fail early)
	skipCleanup := false
	defer func() {
		if !skipCleanup {
			cleanupBuild(state)
		}
	}()

	// 3. Checkout Merge Branch and Pull
	fmt.Printf("Switching to merge branch: %s...\n", mergeBranch)
	if err := gitCheckout(mergeBranch); err != nil {
		return fmt.Errorf("failed to checkout %s: %w", mergeBranch, err)
	}

	fmt.Printf("Pulling latest for %s...\n", mergeBranch)
	if err := gitPull("origin", mergeBranch); err != nil {
		return fmt.Errorf("failed to pull %s: %w", mergeBranch, err)
	}

	// 4. Merge Original Branch
	fmt.Printf("Merging %s into %s...\n", originalBranch, mergeBranch)
	if err := gitMerge(originalBranch); err != nil {
		if isMergeInProgress() {
			skipCleanup = true
			fmt.Println("\n⚠️  MERGE CONFLICTS DETECTED!")
			fmt.Println("Please resolve the conflicts manually, commit the changes, and then run 'forklift build merge' again to finish.")
			fmt.Println("Note: You are currently on the " + mergeBranch + " branch.")
			return nil
		}
		return fmt.Errorf("merge failed: %w", err)
	}

	return finishBuildMerge(ctx, service, sheetID, sheetName, state, lastTag)
}

func resumeBuildMerge(ctx context.Context, service *sheets.Service, sheetID, sheetName, statePath string) error {
	data, err := os.ReadFile(statePath)
	if err != nil {
		return err
	}
	var state buildState
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}

	fmt.Println("Detected previous build in progress. Resuming...")

	if isMergeInProgress() {
		return fmt.Errorf("merge is still in progress. Please resolve conflicts and commit first.")
	}

	// Check if we are on the right branch
	current, _ := gitCurrentBranch()
	if current != state.MergeBranch {
		return fmt.Errorf("you are on branch %s, but build state says were merging into %s. Please switch and resolve conflicts.", current, state.MergeBranch)
	}

	// Get latest tag again to be sure
	_, _, lastTag, err := getRepoInfo(ctx, service, sheetID, sheetName, state.RepoName)
	if err != nil {
		return err
	}

	err = finishBuildMerge(ctx, service, sheetID, sheetName, state, lastTag)
	if err == nil {
		cleanupBuild(state)
	}
	return err
}

func finishBuildMerge(ctx context.Context, service *sheets.Service, sheetID, sheetName string, state buildState, lastTag string) error {
	// 5. Determine New Tag (and handle existing tags)
	newTag, err := incrementTag(lastTag, state.MergeBranch)
	if err != nil {
		return err
	}

	for gitTagExists(newTag) {
		fmt.Printf("Tag %s already exists, incrementing further...\n", newTag)
		newTag, err = incrementTag(newTag, state.MergeBranch)
		if err != nil {
			return err
		}
	}
	fmt.Printf("New tag: %s\n", newTag)

	// 6. Push Merge Branch (Commit)
	fmt.Println("Pushing merge commit...")
	if err := gitPushBranch("origin", state.MergeBranch); err != nil {
		return fmt.Errorf("failed to push branch %s: %w", state.MergeBranch, err)
	}

	// 7. Create and Push Tag
	fmt.Println("Creating tag...")
	if err := gitTag(newTag); err != nil {
		return fmt.Errorf("failed to create tag %s: %w", newTag, err)
	}

	fmt.Println("Pushing tag...")
	if err := gitPushTag("origin", newTag); err != nil {
		return fmt.Errorf("failed to push tag %s: %w", newTag, err)
	}

	// 8. Update Sheet
	fmt.Println("Updating sheet...")
	if err := updateRepoTag(ctx, service, sheetID, sheetName, state.RowIdx, newTag); err != nil {
		return fmt.Errorf("failed to update sheet: %w", err)
	}

	fmt.Println("Build merge completed successfully!")
	return nil
}

func cleanupBuild(state buildState) {
	if current, _ := gitCurrentBranch(); current != state.OriginalBranch {
		fmt.Printf("Switching back to %s...\n", state.OriginalBranch)
		gitCheckout(state.OriginalBranch)
	}
	if state.Stashed {
		fmt.Println("Popping stash...")
		gitStashPop()
	}
	path, _ := getBuildStatePath()
	if path != "" {
		os.Remove(path)
	}
}

func getBuildStatePath() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--git-dir").Output()
	if err != nil {
		return "", err
	}
	return filepath.Join(strings.TrimSpace(string(out)), "forklift_build_state.json"), nil
}

func saveBuildState(state buildState) error {
	path, err := getBuildStatePath()
	if err != nil {
		return err
	}
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// Git Helpers

func gitStash() (bool, error) {
	// Use a message so we can identify our stash if needed
	cmd := exec.Command("git", "stash", "push", "-m", "forklift-auto-stash")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, err
	}
	output := string(out)
	if strings.Contains(output, "No local changes to save") {
		return false, nil
	}
	return true, nil
}

func gitStashPop() error {
	return exec.Command("git", "stash", "pop").Run()
}

func isMergeInProgress() bool {
	// Check if .git/MERGE_HEAD exists
	cmd := exec.Command("git", "rev-parse", "-q", "--verify", "MERGE_HEAD")
	err := cmd.Run()
	return err == nil
}

func gitTagExists(tag string) bool {
	err := exec.Command("git", "rev-parse", tag).Run()
	return err == nil
}

func gitCurrentBranch() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func gitCheckout(branch string) error {
	return exec.Command("git", "checkout", branch).Run()
}

func gitPull(remote, branch string) error {
	return exec.Command("git", "pull", remote, branch).Run()
}

func gitMerge(branch string) error {
	return exec.Command("git", "merge", branch, "--no-edit").Run()
}

func gitPushBranch(remote, branch string) error {
	return exec.Command("git", "push", remote, branch).Run()
}

func gitTag(tag string) error {
	return exec.Command("git", "tag", tag).Run()
}

func gitPushTag(remote, tag string) error {
	return exec.Command("git", "push", remote, tag).Run()
}

func incrementTag(lastTag, branchName string) (string, error) {
	if lastTag == "" {
		return fmt.Sprintf("v-%s-0.0.1", branchName), nil
	}

	// Try finding semantic version pattern with dots at the end (e.g. .1)
	reSem := regexp.MustCompile(`^(.*?)(\d+)$`)
	matchesSem := reSem.FindStringSubmatch(lastTag)
	if len(matchesSem) == 3 {
		prefix := matchesSem[1]
		numStr := matchesSem[2]
		num, err := strconv.Atoi(numStr)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s%d", prefix, num+1), nil
	}

	return lastTag + ".1", nil
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
