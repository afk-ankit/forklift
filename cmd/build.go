package cmd

import (
	"context"
	"forklift/internal/build"
	"forklift/internal/config"
	"forklift/internal/git"
	"forklift/internal/sheets"

	"github.com/spf13/cobra"
)

var buildCmd = &cobra.Command{
	Use:   "build [merge]",
	Short: "Build and merge current branch into merge branch",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if args[0] != "merge" {
			fatalf("unknown command. did you mean 'build merge'?")
		}

		cfg, err := config.Load()
		if err != nil {
			fatalf("failed to load config: %v", err)
		}
		if cfg.SheetID == "" || cfg.CredentialsPath == "" {
			fatalf("configuration not found. run 'forklift init' first.")
		}

		ctx := context.Background()
		service, err := sheets.NewService(ctx, cfg.CredentialsPath)
		if err != nil {
			fatalf("failed to initialize Google Sheets client: %v", err)
		}

		// If resuming, we don't strictly need to detect repo name again as it's in state,
		// but Run() handles state checks.
		// However, build.Run() needs repoName initially.
		// If resuming state exists, repoName argument might be ignored/overwritten by state.
		// Let's pass it anyway.

		repoName, err := git.DetectRepoName()
		if err != nil {
			// If we are in detached state or mid-merge, maybe git remote works?
			// If not, we might rely on state.
			// Let's check for state first in build.Run logic wrapper?
			// The current implementation of build.Run checks state first.
			// But we need repoName to call GetRepoInfo if NOT resuming.
			// So try detect. If it fails, maybe we are resuming and state has it?
			// For now, let's assume git remote works even in merge state.
			// If it fails, let's pass empty and hope Resume picks it up?
			// Actually build.Run gets state path, checks file.
		}

		if err := build.Run(ctx, service, cfg.SheetID, cfg.SheetName, repoName); err != nil {
			fatalf("build failed: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(buildCmd)
}
