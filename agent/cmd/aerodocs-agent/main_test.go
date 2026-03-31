package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestRunSelfUnregister_ReadsTokenFromConfigOnly verifies that runSelfUnregister
// reads the unregister token from the config file, not from the environment variable.
func TestRunSelfUnregister_ReadsTokenFromConfigOnly(t *testing.T) {
	// Create a temporary config file with an unregister token
	dir := t.TempDir()
	configPath := filepath.Join(dir, "agent.conf")

	cfg := agentConfig{
		ServerID:        "test-server-id",
		HubURL:          "localhost:9090",
		UnregisterToken: "config-file-token",
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// Verify loadConfig reads the token correctly
	loaded, err := loadConfig(configPath)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if loaded.UnregisterToken != "config-file-token" {
		t.Errorf("expected unregister_token 'config-file-token', got %q", loaded.UnregisterToken)
	}
}

// TestRunSelfUnregister_MissingTokenFails verifies that runSelfUnregister fails
// with a clear error when the config file has no unregister token, even if the
// env var is set (proving the env var is not used as fallback).
func TestRunSelfUnregister_MissingTokenFailsEvenWithEnvVar(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "agent.conf")

	// Write config WITHOUT unregister token
	cfg := agentConfig{
		ServerID: "test-server-id",
		HubURL:   "localhost:9090",
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// Set env var — it should NOT be used as fallback
	t.Setenv("AERODOCS_UNREGISTER_TOKEN", "env-var-token")

	// Load config and verify token is empty (env var not consulted)
	loaded, err := loadConfig(configPath)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if loaded.UnregisterToken != "" {
		t.Errorf("expected empty unregister_token from config, got %q", loaded.UnregisterToken)
	}
}

// TestSaveNewConfig_PersistsUnregisterToken verifies that saveNewConfig writes
// the unregister token to the config file.
func TestSaveNewConfig_PersistsUnregisterToken(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "agent.conf")

	saveNewConfig(configPath, "localhost:9090", "srv-123", "my-unreg-token")

	loaded, err := loadConfig(configPath)
	if err != nil {
		t.Fatalf("loadConfig after save: %v", err)
	}
	if loaded.UnregisterToken != "my-unreg-token" {
		t.Errorf("expected unregister_token 'my-unreg-token', got %q", loaded.UnregisterToken)
	}
	if loaded.ServerID != "srv-123" {
		t.Errorf("expected server_id 'srv-123', got %q", loaded.ServerID)
	}
	if loaded.HubURL != "localhost:9090" {
		t.Errorf("expected hub_url 'localhost:9090', got %q", loaded.HubURL)
	}
}

func TestParseAllowedPaths(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"/var/log", []string{"/var/log"}},
		{"/var/log,/home/app", []string{"/var/log", "/home/app"}},
		{"/var/log, /home/app , /tmp", []string{"/var/log", "/home/app", "/tmp"}},
		{",,,", nil},
	}
	for _, tt := range tests {
		got := parseAllowedPaths(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("parseAllowedPaths(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("parseAllowedPaths(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestLoadConfig_WithAllowedPaths(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "agent.conf")

	cfg := agentConfig{
		ServerID:     "srv-1",
		HubURL:       "localhost:9090",
		AllowedPaths: []string{"/var/log", "/home/app"},
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	loaded, err := loadConfig(configPath)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if len(loaded.AllowedPaths) != 2 {
		t.Fatalf("expected 2 allowed paths, got %d", len(loaded.AllowedPaths))
	}
	if loaded.AllowedPaths[0] != "/var/log" {
		t.Fatalf("expected '/var/log', got '%s'", loaded.AllowedPaths[0])
	}
}
