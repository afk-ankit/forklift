package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// WorkflowStatus represents the status of a GitHub Actions workflow run
type WorkflowStatus struct {
	Status     string // queued, in_progress, completed
	Conclusion string // success, failure, cancelled, skipped, null if not completed
	HTMLURL    string
	RunID      int64
	CreatedAt  time.Time
}

// Client is a GitHub API client for checking workflow status
type Client struct {
	token string
	owner string
	repo  string
}

// NewClient creates a new GitHub API client
func NewClient(token, owner, repo string) *Client {
	return &Client{
		token: token,
		owner: owner,
		repo:  repo,
	}
}

// CheckWorkflowStatusForTag checks the status of workflow runs triggered by a specific tag
func (c *Client) CheckWorkflowStatusForTag(tag string) (*WorkflowStatus, error) {
	// GitHub API endpoint for workflow runs
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/actions/runs?event=push&per_page=10", c.owner, c.repo)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication header
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch workflow runs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		WorkflowRuns []struct {
			ID         int64     `json:"id"`
			Status     string    `json:"status"`
			Conclusion *string   `json:"conclusion"`
			HTMLURL    string    `json:"html_url"`
			HeadBranch string    `json:"head_branch"`
			CreatedAt  time.Time `json:"created_at"`
		} `json:"workflow_runs"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Find the most recent workflow run for this tag
	for _, run := range result.WorkflowRuns {
		if run.HeadBranch == tag {
			conclusion := ""
			if run.Conclusion != nil {
				conclusion = *run.Conclusion
			}
			return &WorkflowStatus{
				Status:     run.Status,
				Conclusion: conclusion,
				HTMLURL:    run.HTMLURL,
				RunID:      run.ID,
				CreatedAt:  run.CreatedAt,
			}, nil
		}
	}

	return nil, fmt.Errorf("no workflow runs found for tag %s", tag)
}
