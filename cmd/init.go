package cmd

import (
	"bufio"
	"fmt"
	"forklift/internal/config"
	"forklift/internal/sheets"
	"forklift/internal/structures"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize forklift configuration",
	Long:  `Set up the Google Sheet ID, credentials path, optional GitHub polling, and optional Slack notifications for forklift.`,
	Run: func(cmd *cobra.Command, args []string) {
		reader := bufio.NewReader(os.Stdin)

		// Load existing config
		currentCfg, _ := config.Load()

		// Helper to prompt with default
		prompt := func(label, currentVal string, required bool) string {
			if currentVal != "" {
				fmt.Printf("%s [%s]: ", label, currentVal)
			} else {
				fmt.Printf("%s: ", label)
			}
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)
			if input == "" {
				return currentVal
			}
			return input
		}

		// 1. Google Sheet URL/ID
		sheetInput := prompt("Enter Google Sheet URL or ID", currentCfg.SheetID, true)
		if sheetInput == "" && currentCfg.SheetID == "" {
			fatalf("Sheet ID is required")
		}
		sheetID, err := sheets.ExtractSheetID(sheetInput)
		if err != nil {
			// If extraction fails, assume it's already an ID if no change
			if sheetInput == currentCfg.SheetID {
				sheetID = sheetInput
			} else {
				fatalf("Invalid Sheet URL/ID: %v", err)
			}
		}

		// 2. Credentials Path
		credPath := prompt("Enter path to credentials.json", currentCfg.CredentialsPath, true)
		if credPath == "" {
			fatalf("Credentials path is required")
		}
		absPath, err := filepath.Abs(credPath)
		if err != nil {
			fatalf("Invalid path: %v", err)
		}

		// 3. Sheet Name
		defaultSheetName := currentCfg.SheetName
		if defaultSheetName == "" {
			defaultSheetName = config.DefaultSheetName
		}
		sheetName := prompt("Enter Sheet Name", defaultSheetName, false)
		if sheetName == "" {
			sheetName = defaultSheetName
		}

		// GitHub token
		fmt.Print("\n--- GitHub Actions Polling (Optional) ---\n")
		githubToken := prompt("Enter GitHub Token (press Enter to skip/keep)", currentCfg.GitHubToken, false)

		// Polling interval
		defaultInterval := currentCfg.PollInterval
		if defaultInterval == 0 {
			defaultInterval = 30
		}
		pollInterval := defaultInterval

		if githubToken != "" {
			intervalInput := prompt("Enter polling interval in seconds", fmt.Sprintf("%d", defaultInterval), false)
			if intervalInput != "" {
				fmt.Sscanf(intervalInput, "%d", &pollInterval)
			}
		}

		// Polling timeout
		defaultTimeout := currentCfg.PollTimeout
		if defaultTimeout == 0 {
			defaultTimeout = 30
		}
		pollTimeout := defaultTimeout

		if githubToken != "" {
			timeoutInput := prompt("Enter polling timeout in minutes", fmt.Sprintf("%d", defaultTimeout), false)
			if timeoutInput != "" {
				fmt.Sscanf(timeoutInput, "%d", &pollTimeout)
			}
		}

		fmt.Print("\n--- Slack Notifications (Optional) ---\n")
		slackToken := prompt("Enter Slack Bot Token (press Enter to skip/keep)", currentCfg.SlackBotToken, false)
		slackChannel := prompt("Enter Slack Channel (ID or #name)", currentCfg.SlackChannel, false)
		slackMemberID := prompt("Enter Slack Member ID (optional, press Enter to use email lookup)", currentCfg.SlackMemberID, false)

		if err := config.ValidateSlack(slackToken, slackChannel); err != nil {
			fatalf("%v", err)
		}

		fmt.Print("\n--- Tagging ---\n")
		tagDefault := "Y"
		if currentCfg.CreateTag != nil && !*currentCfg.CreateTag {
			tagDefault = "n"
		}
		tagAnswer := prompt("Create and push a tag during build merge? (Y/n)", tagDefault, false)
		createTag := parseYes(tagAnswer)

		cfg := structures.Config{
			SheetID:         sheetID,
			SheetName:       sheetName,
			CredentialsPath: absPath,
			GitHubToken:     githubToken,
			PollInterval:    pollInterval,
			PollTimeout:     pollTimeout,
			SlackBotToken:   slackToken,
			SlackChannel:    slackChannel,
			SlackMemberID:   slackMemberID,
			CreateTag:       &createTag,
		}

		if err := config.Save(cfg); err != nil {
			fatalf("Failed to save config: %v", err)
		}

		fmt.Println("🏗️  Configuration saved successfully! You're ready to roll.")
	},
}

func parseYes(input string) bool {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "n", "no", "false", "0":
		return false
	default:
		return true
	}
}

func init() {
	rootCmd.AddCommand(initCmd)
}
