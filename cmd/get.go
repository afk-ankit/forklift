package cmd

import (
	"context"
	"fmt"
	"forklift/internal/config"
	"forklift/internal/git"
	"forklift/internal/sheets"

	"github.com/spf13/cobra"
)

var getCmd = &cobra.Command{
	Use:   "get [merge-branch]",
	Short: "Get the current merge branch for a repository",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if args[0] != "merge-branch" {
			fatalf("unknown command. did you mean 'get merge-branch'?")
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

		repoName, err := git.DetectRepoName()
		if err != nil {
			fatalf("failed to detect repo name: %v", err)
		}

		info, err := service.GetRepoInfo(ctx, cfg.SheetID, cfg.SheetName, repoName)
		if err != nil {
			fatalf("failed to read repo info: %v", err)
		}

		if info == nil || info.MergeBranch == "" {
			fmt.Printf("Merge branch not set for %s\n", repoName)
		} else {
			fmt.Printf("Merge branch for %s: %s\n", repoName, info.MergeBranch)
			if info.LastUser != "" {
				fmt.Printf("Last updated by: %s\n", info.LastUser)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(getCmd)
}
