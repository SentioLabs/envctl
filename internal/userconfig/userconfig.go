// Package userconfig reads and writes envctl's per-user settings file. Unlike
// .envctl.yaml, which describes a project and is checked in, this file holds
// preferences that belong to one machine, such as the update channel.
package userconfig

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sentiolabs/go-selfupdate"
	"gopkg.in/yaml.v3"
)

const (
	// appDir and fileName join to form the path under XDG_CONFIG_HOME.
	appDir   = "envctl"
	fileName = "config.yaml"
	// dirMode and fileMode keep the settings file readable only by its owner.
	dirMode  = 0o700
	fileMode = 0o600
)

// Config is the per-user settings file.
type Config struct {
	Updates Updates `yaml:"updates,omitempty"`
}

// Updates holds settings for envctl self update.
type Updates struct {
	// Channel is the update channel: stable, rc, or nightly.
	Channel string `yaml:"channel,omitempty"`
}

// Path returns $XDG_CONFIG_HOME/envctl/config.yaml, or ~/.config/envctl/config.yaml.
func Path() (string, error) {
	if base := os.Getenv("XDG_CONFIG_HOME"); base != "" {
		return filepath.Join(base, appDir, fileName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".config", appDir, fileName), nil
}

// Load reads the file. A missing file is an empty Config, not an error.
func Load() (*Config, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", p, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", p, err)
	}
	return &cfg, nil
}

// Save writes cfg atomically with mode 0600 inside a 0700 directory.
func Save(cfg *Config) error {
	p, err := Path()
	if err != nil {
		return err
	}
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	tmp, err := os.CreateTemp(dir, "."+fileName+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("write %s: %w", p, err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close %s: %w", p, err)
	}
	if err := os.Chmod(tmpName, fileMode); err != nil {
		cleanup()
		return fmt.Errorf("chmod %s: %w", p, err)
	}
	if err := os.Rename(tmpName, p); err != nil {
		cleanup()
		return fmt.Errorf("replace %s: %w", p, err)
	}
	return nil
}

// ChannelStore adapts the file to selfupdate.Store. An absent file reads as
// an empty channel, which the updater treats as stable.
func ChannelStore() selfupdate.Store {
	return selfupdate.FuncStore{
		Get: func() (selfupdate.Channel, error) {
			cfg, err := Load()
			if err != nil {
				return "", err
			}
			return selfupdate.Channel(cfg.Updates.Channel), nil
		},
		Set: func(c selfupdate.Channel) error {
			cfg, err := Load()
			if err != nil {
				return err
			}
			cfg.Updates.Channel = string(c)
			return Save(cfg)
		},
	}
}
