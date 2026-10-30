package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DefaultProfile is used when no profile is selected.
const DefaultProfile = "default"

// Config is the CLI's configuration file (agdir-email/config.json in the OS config dir).
type Config struct {
	CurrentProfile string              `json:"current_profile,omitempty"`
	Profiles       map[string]*Profile `json:"profiles"`
}

// Profile is a stored API key and, optionally, a base URL.
type Profile struct {
	APIKey  string `json:"api_key"`
	BaseURL string `json:"base_url,omitempty"`
}

// configPath returns $AGDIR_EMAIL_CONFIG or <user config dir>/agdir-email/config.json.
func (a *App) configPath() (string, error) {
	if p := a.getenv(EnvConfig); p != "" {
		return p, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locating the config directory: %w (set %s to a file path)", err, EnvConfig)
	}
	return filepath.Join(dir, "agdir-email", "config.json"), nil
}

func (a *App) loadConfig() (*Config, string, error) {
	path, err := a.configPath()
	if err != nil {
		return nil, "", err
	}
	cfg := &Config{Profiles: map[string]*Profile{}}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return cfg, path, nil
	}
	if err != nil {
		return nil, path, fmt.Errorf("reading %s: %w", path, err)
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, path, fmt.Errorf("reading %s: %w", path, err)
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]*Profile{}
	}
	for name, p := range cfg.Profiles {
		if p == nil {
			delete(cfg.Profiles, name)
		}
	}
	return cfg, path, nil
}

// saveConfig writes the config atomically, readable only by the current user (0600).
func saveConfig(path string, cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".config-*.json")
	if err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	defer os.Remove(tmp.Name())
	// CreateTemp already uses 0600; on Windows the file is private to the user profile.
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return os.Chmod(path, 0o600)
}

// profileName is the selected profile: --profile, $AGDIR_EMAIL_PROFILE, the config's current
// profile, else "default".
func (a *App) profileName(cfg *Config) string {
	if a.opts.profile != "" {
		return a.opts.profile
	}
	if p := a.getenv(EnvProfile); p != "" {
		return p
	}
	if cfg != nil && cfg.CurrentProfile != "" {
		return cfg.CurrentProfile
	}
	return DefaultProfile
}

func (c *Config) names() []string {
	names := make([]string, 0, len(c.Profiles))
	for n := range c.Profiles {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func validProfileName(name string) error {
	if name == "" || strings.ContainsAny(name, " \t\r\n/\\") {
		return usageErrorf("invalid profile name %q", name)
	}
	return nil
}
