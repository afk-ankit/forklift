package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const DefaultSheetName = "merge_branches"

type Config struct {
	SheetID         string `json:"sheet_id"`
	SheetName       string `json:"sheet_name"`
	CredentialsPath string `json:"credentials_path"`
}

func Load() (Config, error) {
	cfgPath, err := Path()
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, nil
		}
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func Save(cfg Config) error {
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
