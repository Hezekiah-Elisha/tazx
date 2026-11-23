package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	LogPath         string  `yaml:"log_path" json:"log_path"`
	RefreshRate     int     `yaml:"refresh_rate" json:"refresh_rate"`
	CpuThreshold    float64 `yaml:"cpu_threshold" json:"cpu_threshold"`
	MemoryThreshold float64 `yaml:"memory_threshold" json:"memory_threshold"`
	DiskThreshold   float64 `yaml:"disk_threshold" json:"disk_threshold"`
}

func DefaultConfig() Config {
	return Config{
		LogPath:         findDefaultLogPath(),
		RefreshRate:     2,
		CpuThreshold:    80.0,
		MemoryThreshold: 85.0,
		DiskThreshold:   85.0,
	}
}

const (
	DefaultNginxLogPath  = "/var/log/nginx/access.log"
	DefaultApacheLogPath = "/var/log/apache2/access.log"
)

var NginxCandidatePaths = []string{
	"/var/log/nginx/access.log",
	"/usr/local/var/log/nginx/access.log",    // macOS Homebrew Intel
	"/opt/homebrew/var/log/nginx/access.log", // macOS Homebrew Apple Silicon
	"/var/log/nginx-access.log",
	"/var/log/nginx/access_log",
}

var ApacheCandidatePaths = []string{
	"/var/log/apache2/access.log",
	"/var/log/httpd/access_log",
	"/var/log/httpd/access.log",
	"/var/log/apache2/access_log",
}

// Validate checks that configuration bounds and paths are safe and sensible.
func (c Config) Validate() error {
	if strings.TrimSpace(c.LogPath) == "" {
		return errors.New("log_path must not be empty")
	}
	if strings.ContainsRune(c.LogPath, '\x00') {
		return errors.New("log_path contains invalid null byte")
	}
	if c.RefreshRate < 1 || c.RefreshRate > 3600 {
		return fmt.Errorf("refresh_rate must be between 1 and 3600 seconds (got %d)", c.RefreshRate)
	}
	if c.CpuThreshold <= 0 || c.CpuThreshold > 100 {
		return fmt.Errorf("cpu_threshold must be > 0 and <= 100 (got %.1f)", c.CpuThreshold)
	}
	if c.MemoryThreshold <= 0 || c.MemoryThreshold > 100 {
		return fmt.Errorf("memory_threshold must be > 0 and <= 100 (got %.1f)", c.MemoryThreshold)
	}
	if c.DiskThreshold <= 0 || c.DiskThreshold > 100 {
		return fmt.Errorf("disk_threshold must be > 0 and <= 100 (got %.1f)", c.DiskThreshold)
	}
	return nil
}

// ValidateRegularFile verifies that path points to a readable regular file (not a directory,
// device node, FIFO, or socket) and resolves symlinks safely.
func ValidateRegularFile(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("file path must not be empty")
	}
	if strings.ContainsRune(path, '\x00') {
		return "", errors.New("file path contains invalid null byte")
	}
	cleaned := filepath.Clean(path)
	linfo, err := os.Lstat(cleaned)
	if err != nil {
		return "", err
	}
	target := cleaned
	if linfo.Mode()&os.ModeSymlink != 0 {
		resolved, err := filepath.EvalSymlinks(cleaned)
		if err != nil {
			return "", fmt.Errorf("unable to resolve symlink %s: %w", cleaned, err)
		}
		target = resolved
	}
	info, err := os.Stat(target)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s is a directory, expected a regular file", path)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s is not a regular file", path)
	}
	return target, nil
}

func FindNginxLogPath() string {
	for _, p := range NginxCandidatePaths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return DefaultNginxLogPath
}

func FindApacheLogPath() string {
	for _, p := range ApacheCandidatePaths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return DefaultApacheLogPath
}

func findDefaultLogPath() string {
	return FindNginxLogPath()
}

func GetConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".tazx.yaml"
	}
	return filepath.Join(home, ".tazx.yaml")
}

// ConfigExists reports whether a file or symlink already exists at path.
func ConfigExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Lstat(path)
	return err == nil
}

func LoadConfig(customPath string) (Config, string, error) {
	cfgPath := customPath
	if cfgPath == "" {
		cfgPath = GetConfigPath()
		if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
			if _, errLocal := os.Stat(".tazx.yaml"); errLocal == nil {
				cfgPath = ".tazx.yaml"
			}
		}
	}

	resolvedPath, err := ValidateRegularFile(cfgPath)
	if err != nil {
		return DefaultConfig(), cfgPath, err
	}

	cfg := DefaultConfig()
	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return cfg, cfgPath, err
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return DefaultConfig(), cfgPath, fmt.Errorf("failed to parse YAML config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return DefaultConfig(), cfgPath, fmt.Errorf("invalid configuration: %w", err)
	}
	return cfg, cfgPath, nil
}

func SaveConfig(cfg Config, path string) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if path == "" {
		path = GetConfigPath()
	}
	if strings.ContainsRune(path, '\x00') {
		return errors.New("config path contains invalid null byte")
	}
	cleaned := filepath.Clean(path)
	if info, err := os.Lstat(cleaned); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to write config through symlink: %s", cleaned)
		}
		if info.IsDir() {
			return fmt.Errorf("config path is a directory: %s", cleaned)
		}
	}

	dir := filepath.Dir(cleaned)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0750); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(cleaned, data, 0600)
}
