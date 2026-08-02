package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	LogPath         string  `yaml:"log_path"`
	RefreshRate     int     `yaml:"refresh_rate"`
	CpuThreshold    float64 `yaml:"cpu_threshold"`
	MemoryThreshold float64 `yaml:"memory_threshold"`
	DiskThreshold   float64 `yaml:"disk_threshold"`
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

func findDefaultLogPath() string {
	commonPaths := []string{
		"/var/log/nginx/access.log",
		"/var/log/apache2/access.log",
		"/var/log/httpd/access_log",
		"./access.log",
		"./server.log",
	}
	for _, p := range commonPaths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "./access.log"
}

func GetConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".tazx.yaml"
	}
	return filepath.Join(home, ".tazx.yaml")
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

	cfg := DefaultConfig()
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return cfg, cfgPath, err
	}

	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return DefaultConfig(), cfgPath, err
	}
	return cfg, cfgPath, nil
}

func SaveConfig(cfg Config, path string) error {
	if path == "" {
		path = GetConfigPath()
	}
	dir := filepath.Dir(path)
	if dir != "" {
		_ = os.MkdirAll(dir, 0755)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
