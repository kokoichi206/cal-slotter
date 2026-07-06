package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config is the local runtime configuration for calendar access and members.
type Config struct {
	Timezone    string   `json:"timezone"`
	CalendarID  string   `json:"calendar_id"`
	Members     []string `json:"members"`
	Credentials string   `json:"credentials"`
	Token       string   `json:"token"`
}

// DefaultConfigPath returns the default JSON config path.
func DefaultConfigPath() (string, error) {
	dir, err := defaultConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// DefaultCredentialsPath returns the default OAuth client credentials path.
func DefaultCredentialsPath() (string, error) {
	dir, err := defaultConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "credentials.json"), nil
}

// DefaultTokenPath returns the default OAuth token cache path.
func DefaultTokenPath() (string, error) {
	dir, err := defaultConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "token.json"), nil
}

func defaultConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "cal-slotter"), nil
}

// Load reads a JSON config file.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	return cfg, nil
}

// WithDefaults fills optional config fields with local defaults.
func (c Config) WithDefaults() (Config, error) {
	out := c
	if out.Timezone == "" {
		out.Timezone = "Asia/Tokyo"
	}
	if out.CalendarID == "" {
		out.CalendarID = "primary"
	}
	if out.Credentials == "" {
		path, err := DefaultCredentialsPath()
		if err != nil {
			return Config{}, err
		}
		out.Credentials = path
	}
	if out.Token == "" {
		path, err := DefaultTokenPath()
		if err != nil {
			return Config{}, err
		}
		out.Token = path
	}
	return out, nil
}
