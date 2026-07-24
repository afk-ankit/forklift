package cmd

import (
	"bufio"
	"fmt"
	"forklift/internal/git"
	"math/rand"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create forklift resources (e.g. merge branch)",
}

var adjectives = []string{
	"neon", "cosmic", "silent", "rapid", "ember", "lunar", "vivid", "frost",
	"golden", "wild", "brave", "swift", "amber", "crystal", "shadow", "bright",
}

var nouns = []string{
	"falcon", "comet", "river", "ember", "orchid", "phoenix", "canyon", "harbor",
	"spark", "mirror", "nova", "willow", "thunder", "glacier", "raven", "summit",
}

var createMergeBranchCmd = &cobra.Command{
	Use:   "merge-branch",
	Short: "Create a new merge branch, push it, and set it in forklift",
	Run: func(cmd *cobra.Command, args []string) {
		reader := bufio.NewReader(os.Stdin)

		name, ok := pickCoolName(reader)
		if !ok {
			fmt.Println("Aborted.")
			return
		}

		current, err := git.CurrentBranch()
		if err != nil {
			fatalf("failed to get current branch: %v", err)
		}
		mainBranch, err := git.DefaultMainBranch()
		if err != nil {
			fatalf("%v", err)
		}

		fmt.Printf("Create %s from:\n  1) current branch (%s)\n  2) main branch (%s)\nChoice [1/2]: ", name, current, mainBranch)
		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(choice)

		var startPoint string
		switch choice {
		case "1":
			startPoint = current
		case "2":
			startPoint = mainBranch
		default:
			fmt.Println("Aborted.")
			return
		}

		if git.BranchExistsLocal(name) {
			fatalf("branch %s already exists locally", name)
		}
		if git.BranchExistsRemote("origin", name) {
			fatalf("branch %s already exists on origin", name)
		}

		fmt.Printf("Creating branch %s from %s...\n", name, startPoint)
		if err := git.CreateBranch(name, startPoint); err != nil {
			fatalf("failed to create branch: %v", err)
		}

		fmt.Printf("Pushing %s and setting upstream...\n", name)
		if err := git.PushSetUpstream("origin", name); err != nil {
			fatalf("failed to push branch (local branch left in place): %v", err)
		}

		setMergeBranchInteractive(name)
		fmt.Printf("Left checkout on %s.\n", current)
	},
}

func pickCoolName(reader *bufio.Reader) (string, bool) {
	for {
		name := coolName()
		fmt.Printf("Suggested name: %s\n  [y] use  [n] regenerate  [a] abort: ", name)
		input, _ := reader.ReadString('\n')
		switch strings.TrimSpace(strings.ToLower(input)) {
		case "y", "yes":
			return name, true
		case "n", "no":
			continue
		default:
			return "", false
		}
	}
}

func coolName() string {
	return adjectives[rand.Intn(len(adjectives))] + "-" + nouns[rand.Intn(len(nouns))]
}

func init() {
	createCmd.AddCommand(createMergeBranchCmd)
	rootCmd.AddCommand(createCmd)
}
