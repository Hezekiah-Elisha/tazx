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
	if err := cfg.Validate(); err != nil {
		t.Errorf("default config should validate cleanly, got %v", err)
	}
}

func TestFindLogPaths(t *testing.T) {
	if FindNginxLogPath() == "" {
		t.Error("expected non-empty nginx log path")
	}
	if FindApacheLogPath() == "" {
		t.Error("expected non-empty apache log path")
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr bool
	}{
		{"valid", func(c *Config) {}, false},
		{"zero_refresh", func(c *Config) { c.RefreshRate = 0 }, true},
		{"excessive_refresh", func(c *Config) { c.RefreshRate = 5000 }, true},
		{"cpu_over_100", func(c *Config) { c.CpuThreshold = 101 }, true},
		{"mem_zero", func(c *Config) { c.MemoryThreshold = 0 }, true},
		{"disk_negative", func(c *Config) { c.DiskThreshold = -5 }, true},
		{"empty_log_path", func(c *Config) { c.LogPath = "   " }, true},
		{"null_byte_path", func(c *Config) { c.LogPath = "/var/log/a\x00b" }, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tc.mutate(&cfg)
			err := cfg.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() err = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}

func TestSaveAndLoadConfigPermissions(t *testing.T) {
	tempDir := t.TempDir()
	cfgPath := filepath.Join(tempDir, "test_config.yaml")

	originalCfg := Config{
		LogPath:         "/var/log/custom.log",
		RefreshRate:     5,
		CpuThreshold:    75.0,
		MemoryThreshold: 80.0,
		DiskThreshold:   85.0,
	}

	if err := SaveConfig(originalCfg, cfgPath); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	info, err := os.Stat(cfgPath)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("expected config file mode 0600, got %04o", info.Mode().Perm())
	}

	loadedCfg, pathUsed, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	if pathUsed != cfgPath {
		t.Errorf("expected path %s, got %s", cfgPath, pathUsed)
	}
	if loadedCfg != originalCfg {
		t.Errorf("loaded config %+v != original %+v", loadedCfg, originalCfg)
	}
}

func TestSaveConfigRefusesSymlink(t *testing.T) {
	tempDir := t.TempDir()
	realFile := filepath.Join(tempDir, "target.yaml")
	if err := os.WriteFile(realFile, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(tempDir, "link.yaml")
	if err := os.Symlink(realFile, linkPath); err != nil {
		t.Skipf("symlinks unsupported in test env: %v", err)
	}

	if err := SaveConfig(DefaultConfig(), linkPath); err == nil {
		t.Fatal("expected SaveConfig to refuse overwriting a symlink")
	}
}

func TestLoadConfigRejectsInvalidYAMLAndInvalidValues(t *testing.T) {
	tempDir := t.TempDir()
	badPath := filepath.Join(tempDir, "bad.yaml")
	if err := os.WriteFile(badPath, []byte("refresh_rate: -10\nlog_path: /tmp/a.log\n"), 0600); err != nil {
		t.Fatal(err)
	}

	if _, _, err := LoadConfig(badPath); err == nil {
		t.Error("expected validation error for negative refresh_rate")
	}
}

func TestValidateRegularFileRejectsDir(t *testing.T) {
	tempDir := t.TempDir()
	if _, err := ValidateRegularFile(tempDir); err == nil {
		t.Error("expected directory to be rejected by ValidateRegularFile")
	}
	if _, err := ValidateRegularFile("bad\x00path"); err == nil {
		t.Error("expected null byte path to be rejected")
	}
}

func TestLoadConfigFallback(t *testing.T) {
	_, _, err := LoadConfig(filepath.Join(t.TempDir(), "non_existent_config_12345.yaml"))
	if err == nil {
		t.Errorf("expected error loading non-existent config file")
	}
}
