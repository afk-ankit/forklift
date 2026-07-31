package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"forklift/internal/structures"
	"os"
	"path/filepath"
	"strings"
)

const DefaultSheetName = "merge_branches"

func Load() (structures.Config, error) {
	cfgPath, err := Path()
	if err != nil {
		return structures.Config{}, err
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return structures.Config{}, nil
		}
		return structures.Config{}, err
	}
	var cfg structures.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return structures.Config{}, err
	}
	return cfg, nil
}

func Save(cfg structures.Config) error {
	cfgPath, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cfgPath, data, 0600)
}

func Path() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "forklift", "config.json"), nil
}

func ValidateSlack(token, channel string) error {
	token = strings.TrimSpace(token)
	channel = strings.TrimSpace(channel)
	if (token == "") == (channel == "") {
		return nil
	}
	return fmt.Errorf("slack bot token and channel must both be set, or both left empty")
}

func SlackConfigured(cfg structures.Config) bool {
	return strings.TrimSpace(cfg.SlackBotToken) != "" && strings.TrimSpace(cfg.SlackChannel) != ""
}
