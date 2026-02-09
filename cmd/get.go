package cmd

import (
	"context"
	"fmt"
	"forklift/internal/clipboard"
	"forklift/internal/config"
	"forklift/internal/git"
	"forklift/internal/sheets"
	"forklift/internal/structures"

	"github.com/spf13/cobra"
)

var copyTag bool

var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Get repository information (branch or tag)",
}

var getBranchCmd = &cobra.Command{
	Use:   "branch",
	Short: "Get the current merge branch for the repository",
	Run: func(cmd *cobra.Command, args []string) {
		info := fetchRepoInfo()
		if info == nil || info.MergeBranch == "" {
			fmt.Println("Merge branch not set.")
		} else {
			fmt.Printf("Merge branch: %s\n", info.MergeBranch)
			if info.LastUser != "" {
				fmt.Printf("Last updated by: %s\n", info.LastUser)
			}
		}
	},
}

var getTagCmd = &cobra.Command{
	Use:   "tag",
	Short: "Get the latest tag for the merge branch",
	Run: func(cmd *cobra.Command, args []string) {
		info := fetchRepoInfo()
		if info == nil || info.LatestTag == "" {
			fmt.Println("No tag found.")
			return
		}

		fmt.Printf("Latest tag: %s\n", info.LatestTag)

		if copyTag {
			if err := clipboard.Copy(info.LatestTag); err != nil {
				fmt.Printf("Failed to copy to clipboard: %v\n", err)
			} else {
				fmt.Println("📋 Tag copied to clipboard!")
			}
		}
	},
}

func fetchRepoInfo() *structures.RepoInfo {
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
	return info
}

func init() {
	getTagCmd.Flags().BoolVarP(&copyTag, "copy", "c", false, "Copy the tag to the clipboard")
	getCmd.AddCommand(getBranchCmd)
	getCmd.AddCommand(getTagCmd)
	rootCmd.AddCommand(getCmd)
}
