package git

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// Helpers

func Stash() (bool, error) {
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

func StashPop() error {
	return exec.Command("git", "stash", "pop").Run()
}

func IsMergeInProgress() bool {
	// Check if .git/MERGE_HEAD exists
	cmd := exec.Command("git", "rev-parse", "-q", "--verify", "MERGE_HEAD")
	err := cmd.Run()
	return err == nil
}

func TagExists(tag string) bool {
	err := exec.Command("git", "rev-parse", tag).Run()
	return err == nil
}

func CurrentBranch() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func Checkout(branch string) error {
	return exec.Command("git", "checkout", branch).Run()
}

func Pull(remote, branch string) error {
	return exec.Command("git", "pull", remote, branch).Run()
}

func Merge(branch string) error {
	return exec.Command("git", "merge", branch, "--no-edit").Run()
}

func PushBranch(remote, branch string) error {
	return exec.Command("git", "push", remote, branch).Run()
}

func Tag(tag string) error {
	return exec.Command("git", "tag", tag).Run()
}

func PushTag(remote, tag string) error {
	return exec.Command("git", "push", remote, tag).Run()
}

func DetectRepoName() (string, error) {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	remote := strings.TrimSpace(string(output))
	if remote == "" {
		return "", fmt.Errorf("origin remote is empty")
	}

	repo, err := ParseRepoName(remote)
	if err != nil {
		return "", err
	}
	return repo, nil
}

func ParseRepoName(remote string) (string, error) {
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
			return fmt.Sprintf("%s/%s", matches[1], matches[2]), nil
		}
	}

	// Fallback for other structures, e.g. path/to/repo
	parts := strings.Split(remote, "/")
	if len(parts) >= 2 {
		return fmt.Sprintf("%s/%s", parts[len(parts)-2], parts[len(parts)-1]), nil
	}

	if len(parts) == 1 {
		return parts[0], nil
	}

	return "", fmt.Errorf("unable to parse repo from remote: %s", remote)
}

func UserIdentity() string {
	nameOut, _ := exec.Command("git", "config", "user.name").Output()
	emailOut, _ := exec.Command("git", "config", "user.email").Output()
	name := strings.TrimSpace(string(nameOut))
	email := strings.TrimSpace(string(emailOut))

	if name == "" && email == "" {
		host, _ := os.Hostname()
		user := os.Getenv("USER")
		return fmt.Sprintf("%s@%s", user, host)
	}

	if name != "" && email != "" {
		return fmt.Sprintf("%s <%s>", name, email)
	}
	if name != "" {
		return name
	}
	return email
}
