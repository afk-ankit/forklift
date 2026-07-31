package cmd

import (
	"context"
	"errors"
	"fmt"
	"forklift/internal/config"
	"forklift/internal/git"
	"forklift/internal/github"
	"forklift/internal/notification"
	"forklift/internal/sheets"
	"forklift/internal/slack"
	"forklift/internal/structures"
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

		startedNotified := false
		var runID int64

		for {
			var status *github.WorkflowStatus
			var err error
			if runID != 0 {
				status, err = client.GetWorkflowRun(runID)
			} else {
				status, err = client.CheckWorkflowStatusForTag(tag)
			}

			if err != nil {
				if errors.Is(err, github.ErrNotFound) {
					elapsed := time.Since(startTime)
					fmt.Printf("⏳ Waiting for workflow to start... (%s elapsed)\n", formatDuration(elapsed))
					if elapsed > timeoutDuration {
						fmt.Println("\n⏰ Timeout reached. No workflow found.")
						return
					}
					<-ticker.C
					continue
				}
				fatalf("failed to check workflow status: %v", err)
			}

			if runID == 0 {
				runID = status.RunID
			}

			elapsed := time.Since(status.CreatedAt)

			switch status.Status {
			case "queued":
				fmt.Printf("⏳ Status: queued (waiting to start)\n")
				if !startedNotified {
					notifyPoll(cfg, "started", tag, status.HTMLURL)
					startedNotified = true
				}
			case "in_progress":
				fmt.Printf("⏳ Status: in_progress (running for %s)\n", formatDuration(elapsed))
				if !startedNotified {
					notifyPoll(cfg, "started", tag, status.HTMLURL)
					startedNotified = true
				}
			case "completed":
				fmt.Printf("\n")
				switch status.Conclusion {
				case "success":
					fmt.Println("✅ Workflow completed successfully! 🎉")
					notifyPoll(cfg, "success", tag, status.HTMLURL)
				case "failure":
					fmt.Println("❌ Workflow failed.")
					notifyPoll(cfg, "failure", tag, status.HTMLURL)
				case "cancelled":
					fmt.Println("⚠️  Workflow was cancelled.")
					notifyPoll(cfg, "cancelled", tag, status.HTMLURL)
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

func notifyPoll(cfg structures.Config, event, tag, htmlURL string) {
	if noNotify {
		return
	}
	if config.SlackConfigured(cfg) {
		client := slack.NewClient(cfg.SlackBotToken, cfg.SlackChannel, cfg.SlackMemberID)
		mentioned, err := client.Notify(event, tag, htmlURL, git.UserEmail())
		if err != nil {
			fmt.Printf("⚠️  Slack notify failed: %v\n", err)
			return
		}
		if !mentioned {
			fmt.Println("⚠️  Could not resolve Slack user to tag; posted without a mention.")
		}
		return
	}
	title, message := desktopCopy(event, tag)
	_ = notification.Send(title, message)
}

func desktopCopy(event, tag string) (string, string) {
	switch event {
	case "started":
		return "Forklift Build Started", fmt.Sprintf("Tag %s workflow started.", tag)
	case "success":
		return "Forklift Build Complete", fmt.Sprintf("Tag %s built successfully!", tag)
	case "failure":
		return "Forklift Build Failed", fmt.Sprintf("Tag %s build failed.", tag)
	case "cancelled":
		return "Forklift Build Cancelled", fmt.Sprintf("Tag %s build was cancelled.", tag)
	default:
		return "Forklift", fmt.Sprintf("Tag %s: %s", tag, event)
	}
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
	pollTagCmd.Flags().BoolVar(&noNotify, "no-notify", false, "Disable all notifications (desktop and Slack)")
	pollTagCmd.Flags().BoolVarP(&pollLatest, "latest", "l", false, "Poll the latest tag from the sheet")

	pollCmd.AddCommand(pollTagCmd)
	rootCmd.AddCommand(pollCmd)
}
