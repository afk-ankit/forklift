package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"forklift/internal/build"
	"forklift/internal/structures"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset forklift local state",
}

var resetStateCmd = &cobra.Command{
	Use:   "state",
	Short: "Remove build state for this repo/worktree",
	Run: func(cmd *cobra.Command, args []string) {
		path, err := build.GetStatePath()
		if err != nil {
			fatalf("failed to resolve build state path: %v", err)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Println("No build state found.")
				return
			}
			fatalf("failed to read build state: %v", err)
		}

		var state structures.BuildState
		_ = json.Unmarshal(data, &state)

		fmt.Printf("Build state found at:\n  %s\n", path)
		if state.OriginalBranch != "" || state.MergeBranch != "" {
			fmt.Printf("  original_branch: %s\n  merge_branch: %s\n", state.OriginalBranch, state.MergeBranch)
		}
		fmt.Print("Delete this build state? (y/n): ")

		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))
		if input != "y" && input != "yes" {
			fmt.Println("Aborted.")
			return
		}

		if err := os.Remove(path); err != nil {
			fatalf("failed to remove build state: %v", err)
		}
		fmt.Println("Build state cleared.")
	},
}

func init() {
	resetCmd.AddCommand(resetStateCmd)
	rootCmd.AddCommand(resetCmd)
}
