package cmd

import (
	"bufio"
	"context"
	"fmt"
	"forklift/internal/config"
	"forklift/internal/git"
	"forklift/internal/sheets"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var setCmd = &cobra.Command{
	Use:   "set",
	Short: "Set repository configuration (e.g. branch)",
}

var setBranchCmd = &cobra.Command{
	Use:   "branch [branch-name]",
	Short: "Set the merge branch for the repository",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		branch := args[0]
		if strings.TrimSpace(branch) == "" {
			fatalf("branch name cannot be empty")
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

		// Check if exists
		info, err := service.GetRepoInfo(ctx, cfg.SheetID, cfg.SheetName, repoName)
		if err != nil {
			fatalf("failed to read repo info: %v", err)
		}
		rowIdx := -1
		if info != nil {
			rowIdx = info.RowIdx
			if info.MergeBranch != "" {
				fmt.Printf("Merge branch already set for %s: %s. Override and start new tag sequence? (y/n): ", repoName, info.MergeBranch)
				reader := bufio.NewReader(os.Stdin)
				input, _ := reader.ReadString('\n')
				input = strings.TrimSpace(strings.ToLower(input))
				if input != "y" && input != "yes" {
					fmt.Println("Aborted.")
					return
				}
			}
		}

		if err := service.SetMergeBranch(ctx, cfg.SheetID, cfg.SheetName, repoName, branch, rowIdx); err != nil {
			fatalf("failed to set merge-branch: %v", err)
		}
		fmt.Printf("Merge branch set for %s: %s\n", repoName, branch)
	},
}

func init() {
	setCmd.AddCommand(setBranchCmd)
	rootCmd.AddCommand(setCmd)
}
