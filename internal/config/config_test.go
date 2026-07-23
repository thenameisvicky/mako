package config

import (
	"path/filepath"
	"testing"
)

func TestConfig(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	path, err := Path()
	if err != nil {
		t.Fatalf("Path() unexpected error: %v", err)
	}
	expectedPath := filepath.Join(tempHome, ".config", "tokenly", "config.json")
	if path != expectedPath {
		t.Errorf("Path() = %q; want %q", path, expectedPath)
	}

	if Exists() {
		t.Error("Exists() = true; want false before config is saved")
	}

	_, err = Load()
	if err == nil {
		t.Error("Load() succeeded; want error when file does not exist")
	}

	if err := Save(nil); err == nil {
		t.Error("Save(nil) succeeded; want error")
	}

	cfg := &Config{
		CurrentProfile: "dev-profile",
		DefaultAgent:   "gemini",
		DefaultProxy:   "socks5://127.0.0.1:1080",
		Version:        "0.1.0",
	}

	if err := Save(cfg); err != nil {
		t.Fatalf("Save() unexpected error: %v", err)
	}

	if !Exists() {
		t.Error("Exists() = false; want true after config is saved")
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	if loaded.CurrentProfile != cfg.CurrentProfile {
		t.Errorf("loaded.CurrentProfile = %q; want %q", loaded.CurrentProfile, cfg.CurrentProfile)
	}
	if loaded.DefaultAgent != cfg.DefaultAgent {
		t.Errorf("loaded.DefaultAgent = %q; want %q", loaded.DefaultAgent, cfg.DefaultAgent)
	}
	if loaded.DefaultProxy != cfg.DefaultProxy {
		t.Errorf("loaded.DefaultProxy = %q; want %q", loaded.DefaultProxy, cfg.DefaultProxy)
	}
	if loaded.Version != cfg.Version {
		t.Errorf("loaded.Version = %q; want %q", loaded.Version, cfg.Version)
	}
}
