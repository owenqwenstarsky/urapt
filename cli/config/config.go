// Package config manages the urapt CLI's local configuration: the server URL,
// username, and API token stored in ~/.config/urapt/config.toml (perm 0600).
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// File is the on-disk CLI config shape.
type File struct {
	Default Profile `toml:"default"`
}

// Profile is the active connection profile.
type Profile struct {
	Server string `toml:"server"`
	User   string `toml:"user"`
	Token  string `toml:"token"`
}

// configPath returns the path to the CLI config file.
func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.Getenv("HOME")
		dir = filepath.Join(dir, ".config")
	}
	return filepath.Join(dir, "urapt", "config.toml"), nil
}

// Load reads the CLI config, returning an empty config if none exists.
func Load() (*File, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &File{}, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}
	var f File
	if err := toml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	f.Default.Server = strings.TrimRight(f.Default.Server, "/")
	return &f, nil
}

// Save writes the CLI config with 0600 permissions.
func Save(f *File) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	var buf strings.Builder
	enc := toml.NewEncoder(&buf)
	if err := enc.Encode(f); err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	return os.WriteFile(path, []byte(buf.String()), 0o600)
}

// Path returns the config file path (for display).
func Path() (string, error) { return configPath() }
