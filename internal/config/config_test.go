package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.RefreshRate != 2 {
		t.Errorf("expected refresh rate 2, got %d", cfg.RefreshRate)
	}
	if cfg.CpuThreshold != 80.0 {
		t.Errorf("expected cpu threshold 80.0, got %f", cfg.CpuThreshold)
	}
	if cfg.LogPath != DefaultNginxLogPath {
		t.Errorf("expected default LogPath %s, got %s", DefaultNginxLogPath, cfg.LogPath)
	}
}

func TestFindNginxLogPath(t *testing.T) {
	path := FindNginxLogPath()
	if path == "" {
		t.Errorf("expected non-empty nginx log path")
	}
}

func TestFindApacheLogPath(t *testing.T) {
	path := FindApacheLogPath()
	if path == "" {
		t.Errorf("expected non-empty apache log path")
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	tempDir := t.TempDir()
	cfgPath := filepath.Join(tempDir, "test_config.yaml")

	originalCfg := Config{
		LogPath:         "/var/log/custom.log",
		RefreshRate:     5,
		CpuThreshold:    75.0,
		MemoryThreshold: 80.0,
		DiskThreshold:   85.0,
	}

	err := SaveConfig(originalCfg, cfgPath)
	if err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	loadedCfg, pathUsed, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if pathUsed != cfgPath {
		t.Errorf("expected path %s, got %s", cfgPath, pathUsed)
	}

	if loadedCfg.LogPath != originalCfg.LogPath {
		t.Errorf("expected LogPath %s, got %s", originalCfg.LogPath, loadedCfg.LogPath)
	}
	if loadedCfg.RefreshRate != originalCfg.RefreshRate {
		t.Errorf("expected RefreshRate %d, got %d", originalCfg.RefreshRate, loadedCfg.RefreshRate)
	}
}

func TestLoadConfigFallback(t *testing.T) {
	_, _, err := LoadConfig(filepath.Join(os.TempDir(), "non_existent_config_12345.yaml"))
	if err == nil {
		t.Errorf("expected error loading non-existent config file")
	}
}
