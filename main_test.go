package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfiguration_ConfigFlagMissingFileReturnsError(t *testing.T) {
	_, err := loadConfiguration("/tmp/does-not-exist-koryx-serv.json")
	if err == nil {
		t.Fatalf("expected error for missing config file via -config")
	}
	if !strings.Contains(err.Error(), "config file not found") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestLoadConfiguration_UsesEnvConfigPath(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	cfg := DefaultConfig()
	cfg.Server.Port = 9090
	if err := SaveConfig(configPath, cfg); err != nil {
		t.Fatalf("failed to save test config: %v", err)
	}

	t.Setenv(configPathEnvVar, configPath)

	loaded, err := loadConfiguration("")
	if err != nil {
		t.Fatalf("expected config to load from env var, got error: %v", err)
	}
	if loaded.Server.Port != 9090 {
		t.Fatalf("expected port 9090 from env config, got %d", loaded.Server.Port)
	}
}

func TestLoadConfiguration_EnvConfigPathMissingFileReturnsError(t *testing.T) {
	t.Setenv(configPathEnvVar, "/tmp/does-not-exist-koryx-serv-env.json")

	_, err := loadConfiguration("")
	if err == nil {
		t.Fatalf("expected error for missing config file from %s", configPathEnvVar)
	}
	if !strings.Contains(err.Error(), "points to a missing file") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestLoadConfiguration_ConfigFlagOverridesEnvConfigPath(t *testing.T) {
	tmpDir := t.TempDir()
	flagConfigPath := filepath.Join(tmpDir, "config-flag.json")
	envConfigPath := filepath.Join(tmpDir, "config-env.json")

	flagConfig := DefaultConfig()
	flagConfig.Server.Port = 7777
	if err := SaveConfig(flagConfigPath, flagConfig); err != nil {
		t.Fatalf("failed to save flag config: %v", err)
	}

	envConfig := DefaultConfig()
	envConfig.Server.Port = 8888
	if err := SaveConfig(envConfigPath, envConfig); err != nil {
		t.Fatalf("failed to save env config: %v", err)
	}

	t.Setenv(configPathEnvVar, envConfigPath)

	loaded, err := loadConfiguration(flagConfigPath)
	if err != nil {
		t.Fatalf("expected config to load from -config flag, got error: %v", err)
	}
	if loaded.Server.Port != 7777 {
		t.Fatalf("expected port 7777 from -config flag, got %d", loaded.Server.Port)
	}
}

func TestLoadConfiguration_UsesDefaultsWhenNoConfigSourcesFound(t *testing.T) {
	t.Setenv(configPathEnvVar, "")

	// This test assumes no writable /app/config.json in test environment.
	if _, err := os.Stat(defaultContainerConfigPath); err == nil {
		t.Skipf("skipping because %s exists in this environment", defaultContainerConfigPath)
	}

	loaded, err := loadConfiguration("")
	if err != nil {
		t.Fatalf("expected defaults when no config sources are available, got error: %v", err)
	}
	if loaded.Server.Port != 8080 {
		t.Fatalf("expected default port 8080, got %d", loaded.Server.Port)
	}
}
