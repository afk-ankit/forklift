package cmd

import (
	"context"
	"fmt"
	"forklift/internal/config"
	"forklift/internal/git"
	"forklift/internal/github"
	"forklift/internal/notification"
	"forklift/internal/sheets"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	pollInterval int
	pollTimeout  int
	noNotify     bool
	pollLatest   bool
)

var pollCmd = &cobra.Command{
	Use:   "poll",
	Short: "Poll GitHub Actions workflow status",
}

var pollTagCmd = &cobra.Command{
	Use:   "tag [tag-name]",
	Short: "Poll GitHub Actions workflow status for a tag",
	Long:  `Monitor the GitHub Actions workflow run for a specific tag and get notified when it completes.`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load()
		if err != nil {
			fatalf("failed to load config: %v", err)
		}

		// Get tag to poll
		var tag string
		if len(args) == 1 {
			tag = args[0]
		}

		if pollLatest || tag == "" {
			// Poll the latest tag from the sheet
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

			if info == nil || info.LatestTag == "" {
				fatalf("no tag found in sheet for %s", repoName)
			}

			tag = info.LatestTag
			fmt.Printf("📋 Using latest tag from sheet: %s\n", tag)
		}

		// Parse org/repo from git remote
		repoName, err := git.DetectRepoName()
		if err != nil {
			fatalf("failed to detect repo name: %v", err)
		}

		parts := strings.Split(repoName, "/")
		if len(parts) != 2 {
			fatalf("invalid repo format: %s (expected org/repo)", repoName)
		}
		owner, repo := parts[0], parts[1]

		// Check if GitHub token is configured
		if cfg.GitHubToken == "" {
			fmt.Println("⚠️  No GitHub token configured. API rate limits will be very restrictive.")
			fmt.Println("   Run 'forklift init' to add your GitHub token.")
		}

		// Use config defaults or flags
		interval := pollInterval
		if interval == 0 {
			if cfg.PollInterval > 0 {
				interval = cfg.PollInterval
			} else {
				interval = 30 // default
			}
		}

		timeout := pollTimeout
		if timeout == 0 {
			if cfg.PollTimeout > 0 {
				timeout = cfg.PollTimeout
			} else {
				timeout = 30 // default (minutes)
			}
		}

		// Create GitHub client
		client := github.NewClient(cfg.GitHubToken, owner, repo)

		fmt.Printf("🏷️  Polling workflow for tag %s...\n", tag)
		fmt.Printf("⏱️  Interval: %ds | Timeout: %dm\n\n", interval, timeout)

		startTime := time.Now()
		ticker := time.NewTicker(time.Duration(interval) * time.Second)
		defer ticker.Stop()

		timeoutDuration := time.Duration(timeout) * time.Minute

		for {
			status, err := client.CheckWorkflowStatusForTag(tag)
			if err != nil {
				// Workflow might not have started yet
				elapsed := time.Since(startTime)
				fmt.Printf("⏳ Waiting for workflow to start... (%s elapsed)\n", formatDuration(elapsed))

				if elapsed > timeoutDuration {
					fmt.Println("\n⏰ Timeout reached. No workflow found.")
					return
				}

				<-ticker.C
				continue
			}

			elapsed := time.Since(status.CreatedAt)

			switch status.Status {
			case "queued":
				fmt.Printf("⏳ Status: queued (waiting to start)\n")
			case "in_progress":
				fmt.Printf("⏳ Status: in_progress (running for %s)\n", formatDuration(elapsed))
			case "completed":
				fmt.Printf("\n")
				switch status.Conclusion {
				case "success":
					fmt.Println("✅ Workflow completed successfully! 🎉")
					if !noNotify {
						notification.Send("Forklift Build Complete", fmt.Sprintf("Tag %s built successfully!", tag))
					}
				case "failure":
					fmt.Println("❌ Workflow failed.")
					if !noNotify {
						notification.Send("Forklift Build Failed", fmt.Sprintf("Tag %s build failed.", tag))
					}
				case "cancelled":
					fmt.Println("⚠️  Workflow was cancelled.")
					if !noNotify {
						notification.Send("Forklift Build Cancelled", fmt.Sprintf("Tag %s build was cancelled.", tag))
					}
				default:
					fmt.Printf("⚠️  Workflow completed with status: %s\n", status.Conclusion)
				}
				fmt.Printf("🔗 %s\n", status.HTMLURL)
				return
			}

			if time.Since(startTime) > timeoutDuration {
				fmt.Println("\n⏰ Timeout reached.")
				return
			}

			<-ticker.C
		}
	},
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%dh%dm%ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

func init() {
	pollTagCmd.Flags().IntVarP(&pollInterval, "interval", "i", 0, "Polling interval in seconds (default: 30)")
	pollTagCmd.Flags().IntVarP(&pollTimeout, "timeout", "t", 0, "Timeout in minutes (default: 30)")
	pollTagCmd.Flags().BoolVar(&noNotify, "no-notify", false, "Disable desktop notifications")
	pollTagCmd.Flags().BoolVarP(&pollLatest, "latest", "l", false, "Poll the latest tag from the sheet")

	pollCmd.AddCommand(pollTagCmd)
	rootCmd.AddCommand(pollCmd)
}
