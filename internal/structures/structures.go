package structures

// Config holds the application configuration
type Config struct {
	SheetID         string `json:"sheet_id"`
	SheetName       string `json:"sheet_name"`
	CredentialsPath string `json:"credentials_path"`
	GitHubToken     string `json:"github_token,omitempty"`
	PollInterval    int    `json:"poll_interval,omitempty"` // seconds, default: 30
	PollTimeout     int    `json:"poll_timeout,omitempty"`  // minutes, default: 30
	SlackBotToken   string `json:"slack_bot_token,omitempty"`
	SlackChannel    string `json:"slack_channel,omitempty"`
	SlackMemberID   string `json:"slack_member_id,omitempty"`
	// CreateTag is nil when unset, which means tags are created (existing behavior).
	CreateTag *bool `json:"create_tag,omitempty"`
}

// RepoInfo represents the repository information stored in the Google Sheet
type RepoInfo struct {
	RowIdx      int
	MergeBranch string
	LatestTag   string
	LastUser    string
}

// BuildState represents the state of an ongoing build/merge process
type BuildState struct {
	OriginalBranch string `json:"original_branch"`
	MergeBranch    string `json:"merge_branch"`
	Stashed        bool   `json:"stashed"`
	RepoName       string `json:"repo_name"`
	RowIdx         int    `json:"row_idx"`
}
