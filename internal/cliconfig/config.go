// Package cliconfig loads and persists the CLI config file. Stored at
// ~/.config/pngr/config.yaml (XDG-compliant). Environment variables override
// every field so CI usage works without writing a config file.
package cliconfig

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	defaultAPIURL = "https://api.pngr.dev/v1"
	defaultOutput = "table"
)

type Config struct {
	APIURL      string `yaml:"api_url"`
	Token       string `yaml:"token,omitempty"`
	ActiveOrgID string `yaml:"active_org_id,omitempty"`
	Output      string `yaml:"output,omitempty"` // table | json | yaml
}

// Load reads ~/.config/pngr/config.yaml (or the file at $PNGR_CONFIG) and
// applies env-var overrides. Missing file is not an error — a zero-value
// Config with defaults is returned.
func Load() (*Config, error) {
	cfg := &Config{
		APIURL: defaultAPIURL,
		Output: defaultOutput,
	}

	path, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", path, err)
		}
	}

	// Env vars override file. PNGR_TOKEN is the canonical name (also what
	// the spec uses for CI / MCP integration).
	if v := os.Getenv("PNGR_TOKEN"); v != "" {
		cfg.Token = v
	}
	if v := os.Getenv("PNGR_API_URL"); v != "" {
		cfg.APIURL = v
	}
	if v := os.Getenv("PNGR_OUTPUT"); v != "" {
		cfg.Output = v
	}
	if v := os.Getenv("PNGR_ORG"); v != "" {
		cfg.ActiveOrgID = v
	}
	return cfg, nil
}

// Save writes the config to ~/.config/pngr/config.yaml, creating the directory
// if needed. File permissions are 0600 since it contains the API token.
func (c *Config) Save() error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

// Path returns the config file location, honoring $PNGR_CONFIG, $XDG_CONFIG_HOME,
// then $HOME/.config/pngr/config.yaml.
func Path() (string, error) {
	if v := os.Getenv("PNGR_CONFIG"); v != "" {
		return v, nil
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolving home dir: %w", err)
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "pngr", "config.yaml"), nil
}
