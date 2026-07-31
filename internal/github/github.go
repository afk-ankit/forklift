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
	token   string
	owner   string
	repo    string
	baseURL string // default https://api.github.com; overridable in tests
	http    *http.Client
}

// NewClient creates a new GitHub API client
func NewClient(token, owner, repo string) *Client {
	return &Client{
		token:   token,
		owner:   owner,
		repo:    repo,
		baseURL: "https://api.github.com",
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

// CheckWorkflowStatusForTag checks the status of workflow runs triggered by a specific tag
func (c *Client) CheckWorkflowStatusForTag(tag string) (*WorkflowStatus, error) {
	u := fmt.Sprintf("%s/repos/%s/%s/actions/runs?branch=%s&per_page=5",
		c.baseURL, c.owner, c.repo, url.QueryEscape(tag))

	var result struct {
		WorkflowRuns []workflowRun `json:"workflow_runs"`
	}
	if err := c.getJSON(u, &result); err != nil {
		return nil, err
	}
	if len(result.WorkflowRuns) > 0 {
		return result.WorkflowRuns[0].toStatus(), nil
	}

	u = fmt.Sprintf("%s/repos/%s/%s/actions/runs?event=push&per_page=30",
		c.baseURL, c.owner, c.repo)
	if err := c.getJSON(u, &result); err != nil {
		return nil, err
	}
	for _, run := range result.WorkflowRuns {
		if run.HeadBranch == tag {
			return run.toStatus(), nil
		}
	}
	return nil, ErrNotFound
}

// GetWorkflowRun fetches a single workflow run by ID
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
