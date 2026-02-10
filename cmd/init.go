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
	Long:  `Set up the Google Sheet ID and credentials path for forklift.`,
	Run: func(cmd *cobra.Command, args []string) {
		reader := bufio.NewReader(os.Stdin)

		fmt.Print("Enter Google Sheet URL or ID: ")
		sheetURL, _ := reader.ReadString('\n')
		sheetURL = strings.TrimSpace(sheetURL)

		sheetID, err := sheets.ExtractSheetID(sheetURL)
		if err != nil {
			fatalf("Invalid Sheet URL/ID: %v", err)
		}

		fmt.Print("Enter path to credentials.json: ")
		credPath, _ := reader.ReadString('\n')
		credPath = strings.TrimSpace(credPath)

		absPath, err := filepath.Abs(credPath)
		if err != nil {
			fatalf("Invalid path: %v", err)
		}

		fmt.Printf("Enter Sheet Name (default: %s): ", config.DefaultSheetName)
		sheetName, _ := reader.ReadString('\n')
		sheetName = strings.TrimSpace(sheetName)
		if sheetName == "" {
			sheetName = config.DefaultSheetName
		}

		// GitHub token (optional)
		fmt.Print("\n--- Optional: GitHub Actions Polling ---\n")
		fmt.Print("Enter GitHub Token (press Enter to skip): ")
		githubToken, _ := reader.ReadString('\n')
		githubToken = strings.TrimSpace(githubToken)

		// Polling interval
		pollInterval := 30
		if githubToken != "" {
			fmt.Print("Enter polling interval in seconds (default: 30): ")
			intervalStr, _ := reader.ReadString('\n')
			intervalStr = strings.TrimSpace(intervalStr)
			if intervalStr != "" {
				fmt.Sscanf(intervalStr, "%d", &pollInterval)
			}
		}

		// Polling timeout
		pollTimeout := 30
		if githubToken != "" {
			fmt.Print("Enter polling timeout in minutes (default: 30): ")
			timeoutStr, _ := reader.ReadString('\n')
			timeoutStr = strings.TrimSpace(timeoutStr)
			if timeoutStr != "" {
				fmt.Sscanf(timeoutStr, "%d", &pollTimeout)
			}
		}

		cfg := structures.Config{
			SheetID:         sheetID,
			SheetName:       sheetName,
			CredentialsPath: absPath,
			GitHubToken:     githubToken,
			PollInterval:    pollInterval,
			PollTimeout:     pollTimeout,
		}

		if err := config.Save(cfg); err != nil {
			fatalf("Failed to save config: %v", err)
		}

		fmt.Println("🏗️  Configuration saved successfully! You're ready to roll.")
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
