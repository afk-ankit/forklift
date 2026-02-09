package build

import (
	"context"
	"encoding/json"
	"fmt"
	"forklift/internal/git"
	"forklift/internal/sheets"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type State struct {
	OriginalBranch string `json:"original_branch"`
	MergeBranch    string `json:"merge_branch"`
	Stashed        bool   `json:"stashed"`
	RepoName       string `json:"repo_name"`
	RowIdx         int    `json:"row_idx"`
}

func Run(ctx context.Context, s *sheets.Service, sheetID, sheetName, repoName string) error {
	statePath, err := GetStatePath()
	if err == nil {
		if _, err := os.Stat(statePath); err == nil {
			return Resume(ctx, s, sheetID, sheetName, statePath)
		}
	}

	// 1. Get Repo Info
	rowIdx, mergeBranch, lastTag, err := s.GetRepoInfo(ctx, sheetID, sheetName, repoName)
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
	stashed, err := git.Stash()
	if err != nil {
		return fmt.Errorf("git stash failed: %w", err)
	}

	originalBranch, err := git.CurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	// Save state before switching branches
	state := State{
		OriginalBranch: originalBranch,
		MergeBranch:    mergeBranch,
		Stashed:        stashed,
		RepoName:       repoName,
		RowIdx:         rowIdx,
	}
	if err := SaveState(state); err != nil {
		fmt.Printf("Warning: failed to save build state: %v\n", err)
	}

	// Define cleanup defer (only used if we finish successfully or fail early)
	skipCleanup := false
	defer func() {
		if !skipCleanup {
			Cleanup(state)
		}
	}()

	// 3. Checkout Merge Branch and Pull
	fmt.Printf("Switching to merge branch: %s...\n", mergeBranch)
	if err := git.Checkout(mergeBranch); err != nil {
		return fmt.Errorf("failed to checkout %s: %w", mergeBranch, err)
	}

	fmt.Printf("Pulling latest for %s...\n", mergeBranch)
	if err := git.Pull("origin", mergeBranch); err != nil {
		return fmt.Errorf("failed to pull %s: %w", mergeBranch, err)
	}

	// 4. Merge Original Branch
	fmt.Printf("Merging %s into %s...\n", originalBranch, mergeBranch)
	if err := git.Merge(originalBranch); err != nil {
		if git.IsMergeInProgress() {
			skipCleanup = true
			fmt.Println("\n⚠️  MERGE CONFLICTS DETECTED!")
			fmt.Println("Please resolve the conflicts manually, commit the changes, and then run 'forklift build merge' again to finish.")
			fmt.Println("Note: You are currently on the " + mergeBranch + " branch.")
			return nil
		}
		return fmt.Errorf("merge failed: %w", err)
	}

	return Finish(ctx, s, sheetID, sheetName, state, lastTag)
}

func Resume(ctx context.Context, s *sheets.Service, sheetID, sheetName, statePath string) error {
	data, err := os.ReadFile(statePath)
	if err != nil {
		return err
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}

	fmt.Println("Detected previous build in progress. Resuming...")

	if git.IsMergeInProgress() {
		return fmt.Errorf("merge is still in progress. Please resolve conflicts and commit first.")
	}

	// Check if we are on the right branch
	current, _ := git.CurrentBranch()
	if current != state.MergeBranch {
		return fmt.Errorf("you are on branch %s, but build state says were merging into %s. Please switch and resolve conflicts.", current, state.MergeBranch)
	}

	// Get latest tag again to be sure
	_, _, lastTag, err := s.GetRepoInfo(ctx, sheetID, sheetName, state.RepoName)
	if err != nil {
		return err
	}

	err = Finish(ctx, s, sheetID, sheetName, state, lastTag)
	if err == nil {
		Cleanup(state)
	}
	return err
}

func Finish(ctx context.Context, s *sheets.Service, sheetID, sheetName string, state State, lastTag string) error {
	// 5. Determine New Tag (and handle existing tags)
	newTag, err := IncrementTag(lastTag, state.MergeBranch)
	if err != nil {
		return err
	}

	for git.TagExists(newTag) {
		fmt.Printf("Tag %s already exists, incrementing further...\n", newTag)
		newTag, err = IncrementTag(newTag, state.MergeBranch)
		if err != nil {
			return err
		}
	}
	fmt.Printf("New tag: %s\n", newTag)

	// 6. Push Merge Branch (Commit)
	fmt.Println("Pushing merge commit...")
	if err := git.PushBranch("origin", state.MergeBranch); err != nil {
		return fmt.Errorf("failed to push branch %s: %w", state.MergeBranch, err)
	}

	// 7. Create and Push Tag
	fmt.Println("Creating tag...")
	if err := git.Tag(newTag); err != nil {
		return fmt.Errorf("failed to create tag %s: %w", newTag, err)
	}

	fmt.Println("Pushing tag...")
	if err := git.PushTag("origin", newTag); err != nil {
		return fmt.Errorf("failed to push tag %s: %w", newTag, err)
	}

	// 8. Update Sheet
	fmt.Println("Updating sheet...")
	if err := s.UpdateRepoTag(ctx, sheetID, sheetName, state.RowIdx, newTag); err != nil {
		return fmt.Errorf("failed to update sheet: %w", err)
	}

	fmt.Println("Build merge completed successfully!")
	return nil
}

func Cleanup(state State) {
	if current, _ := git.CurrentBranch(); current != state.OriginalBranch {
		fmt.Printf("Switching back to %s...\n", state.OriginalBranch)
		git.Checkout(state.OriginalBranch)
	}
	if state.Stashed {
		fmt.Println("Popping stash...")
		git.StashPop()
	}
	path, _ := GetStatePath()
	if path != "" {
		os.Remove(path)
	}
}

func GetStatePath() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--git-dir").Output()
	if err != nil {
		return "", err
	}
	return filepath.Join(strings.TrimSpace(string(out)), "forklift_build_state.json"), nil
}

func SaveState(state State) error {
	path, err := GetStatePath()
	if err != nil {
		return err
	}
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func IncrementTag(lastTag, branchName string) (string, error) {
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
