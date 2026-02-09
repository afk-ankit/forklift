package structures

// Config holds the application configuration
type Config struct {
	SheetID         string `json:"sheet_id"`
	SheetName       string `json:"sheet_name"`
	CredentialsPath string `json:"credentials_path"`
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
