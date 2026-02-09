package cmd

import (
	"bufio"
	"fmt"
	"forklift/internal/config"
	"forklift/internal/sheets"
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

		cfg := config.Config{
			SheetID:         sheetID,
			SheetName:       sheetName,
			CredentialsPath: absPath,
		}

		if err := config.Save(cfg); err != nil {
			fatalf("Failed to save config: %v", err)
		}

		fmt.Println("Configuration saved successfully!")
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
