package cmd

import (
	"context"
	"fmt"
	"forklift/internal/clipboard"
	"forklift/internal/config"
	"forklift/internal/git"
	"forklift/internal/sheets"
	"forklift/internal/structures"
	"os"

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
			fmt.Printf("🌿 Merge branch: %s\n", info.MergeBranch)
			if info.LastUser != "" {
				fmt.Printf("👤 Last updated by: %s\n", info.LastUser)
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

		fmt.Printf("🏷️  Latest tag: %s\n", info.LatestTag)

		if copyTag {
			if err := clipboard.Copy(info.LatestTag); err != nil {
				fmt.Printf("Failed to copy to clipboard: %v\n", err)
			} else {
				fmt.Println("📋 Tag copied to clipboard!")
			}
		}
	},
}

var getMergedCmd = &cobra.Command{
	Use:   "merged",
	Short: "Check whether the current commits are on the merge branch",
	Run: func(cmd *cobra.Command, args []string) {
		info := fetchRepoInfo()
		if info == nil || info.MergeBranch == "" {
			fatalf("merge branch not set. run 'forklift set branch <name>' first.")
		}

		fmt.Printf("Fetching origin/%s...\n", info.MergeBranch)
		if err := git.Fetch("origin", info.MergeBranch); err != nil {
			fatalf("failed to fetch %s: %v", info.MergeBranch, err)
		}

		ref := "origin/" + info.MergeBranch
		merged, err := git.IsAncestor("HEAD", ref)
		if err != nil {
			fatalf("failed to compare with %s: %v", ref, err)
		}
		dirty, err := git.Dirty()
		if err != nil {
			fatalf("failed to read working tree: %v", err)
		}

		if merged && !dirty {
			fmt.Printf("Merged into %s.\n", info.MergeBranch)
			return
		}
		if !merged {
			fmt.Printf("Not merged into %s.\n", info.MergeBranch)
		}
		if dirty {
			fmt.Println("Working tree has uncommitted changes.")
		}
		os.Exit(1)
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
	getCmd.AddCommand(getMergedCmd)
	rootCmd.AddCommand(getCmd)
}
