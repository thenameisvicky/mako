package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	CurrentProfile string `json:"CurrentProfile"`
	DefaultAgent   string `json:"DefaultAgent"`
	DefaultProxy   string `json:"DefaultProxy"`
	Version        string `json:"Version"`
}

func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(home, ".config", "tokenly", "config.json"), nil
}

func Exists() bool {
	path, err := Path()
	if err != nil {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return !os.IsNotExist(err)
	}
	return !info.IsDir()
}

func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return nil, fmt.Errorf("failed to determine config path: %w", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to decode config JSON: %w", err)
	}

	return &cfg, nil
}

func Save(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("cannot save a nil config")
	}

	path, err := Path()
	if err != nil {
		return fmt.Errorf("failed to determine config path: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory %q: %w", dir, err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode config JSON: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file to %q: %w", path, err)
	}

	return nil
}
