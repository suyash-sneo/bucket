package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	defaultTheme = "auto"
)

const defaultConfigYAML = `database_path: "~/.config/bucket/bucket.db"
theme: "auto" # auto|dark|light
editor: ""    # if empty, use $EDITOR then vim
log_max_mb: 10
`

type Config struct {
	DatabasePath string `yaml:"database_path"`
	Theme        string `yaml:"theme"`
	Editor       string `yaml:"editor"`
	LogMaxMB     int    `yaml:"log_max_mb"`
}

func Default() Config {
	return Config{
		DatabasePath: "~/.config/bucket/bucket.db",
		Theme:        defaultTheme,
		Editor:       "",
		LogMaxMB:     10,
	}
}

func ResolveConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".config", "bucket"), nil
}

func EnsureConfigDir() (string, error) {
	dir, err := ResolveConfigDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create config dir %s: %w", dir, err)
	}
	return dir, nil
}

func ConfigPath() (string, error) {
	dir, err := ResolveConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yml"), nil
}

func LogPath() (string, error) {
	dir, err := ResolveConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "log.txt"), nil
}

func DraftsDir() (string, error) {
	dir, err := ResolveConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "drafts"), nil
}

func BackupsDir() (string, error) {
	dir, err := ResolveConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "backups"), nil
}

func LoadOrCreate(path string) (Config, error) {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(path, []byte(defaultConfigYAML), 0o600); err != nil {
			return Config{}, fmt.Errorf("write default config %s: %w", path, err)
		}
	}

	payload, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}
	cfg := Default()
	if err := yaml.Unmarshal(payload, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	expanded, err := ExpandHome(cfg.DatabasePath)
	if err != nil {
		return Config{}, fmt.Errorf("expand database_path: %w", err)
	}
	cfg.DatabasePath = expanded
	return cfg, nil
}

func (cfg Config) Validate() error {
	switch cfg.Theme {
	case "auto", "dark", "light":
	default:
		return fmt.Errorf("invalid theme %q", cfg.Theme)
	}
	if cfg.LogMaxMB < 1 {
		return fmt.Errorf("log_max_mb must be >= 1")
	}
	if strings.TrimSpace(cfg.DatabasePath) == "" {
		return fmt.Errorf("database_path is required")
	}
	return nil
}

func ExpandHome(input string) (string, error) {
	if !strings.HasPrefix(input, "~") {
		return input, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if input == "~" {
		return home, nil
	}
	trimmed := strings.TrimPrefix(input, "~/")
	if trimmed == input {
		trimmed = strings.TrimPrefix(input, "~\\")
	}
	return filepath.Join(home, trimmed), nil
}
