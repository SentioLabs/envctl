// Package config handles parsing and validation of .envctl.yaml files.
package config

import (
	"bytes"
	"os"
	"path/filepath"

	"github.com/sentiolabs/envctl/internal/errors"
	"gopkg.in/yaml.v3"
)

const (
	// ConfigFileName is the name of the configuration file.
	ConfigFileName = ".envctl.yaml"
	// CurrentVersion is the current config file version.
	CurrentVersion = 1
)

// Config represents the root configuration structure.
type Config struct {
	Version            int                    `yaml:"version"`
	DefaultEnvironment string                 `yaml:"default_environment"`
	Environments       map[string]Environment `yaml:"environments"`
	Include            []IncludeEntry         `yaml:"include,omitempty"`
	Mapping            map[string]string      `yaml:"mapping,omitempty"`
}

// Environment represents a single environment configuration.
type Environment struct {
	Secret string `yaml:"secret"`
	Region string `yaml:"region,omitempty"`
}

// IncludeEntry represents an additional secret to include.
type IncludeEntry struct {
	Secret string `yaml:"secret"`
	Key    string `yaml:"key,omitempty"`
	As     string `yaml:"as,omitempty"`
}

// Load reads and parses a config file from the given path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &errors.ConfigNotFoundError{SearchPath: path}
		}
		return nil, &errors.ConfigError{Path: path, Message: err.Error()}
	}

	var config Config
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true) // Error on unknown fields

	if err := decoder.Decode(&config); err != nil {
		return nil, &errors.ConfigError{
			Path:    path,
			Message: err.Error(),
		}
	}

	if err := config.Validate(path); err != nil {
		return nil, err
	}

	return &config, nil
}

// FindConfig walks up from the current directory to find .envctl.yaml.
func FindConfig() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return FindConfigFrom(cwd)
}

// FindConfigFrom walks up from the given directory to find .envctl.yaml.
func FindConfigFrom(startPath string) (string, error) {
	dir, err := filepath.Abs(startPath)
	if err != nil {
		return "", err
	}

	for {
		configPath := filepath.Join(dir, ConfigFileName)
		if _, err := os.Stat(configPath); err == nil {
			return configPath, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root
			return "", &errors.ConfigNotFoundError{SearchPath: startPath}
		}
		dir = parent
	}
}

// Validate checks the config for required fields and valid values.
func (c *Config) Validate(path string) error {
	if c.Version != CurrentVersion {
		return &errors.ConfigError{
			Path:    path,
			Message: "unsupported config version (expected version: 1)",
		}
	}

	if len(c.Environments) == 0 {
		return &errors.ConfigError{
			Path:    path,
			Message: "no environments defined",
		}
	}

	if c.DefaultEnvironment != "" {
		if _, ok := c.Environments[c.DefaultEnvironment]; !ok {
			return &errors.ConfigError{
				Path:    path,
				Message: "default_environment references undefined environment: " + c.DefaultEnvironment,
			}
		}
	}

	for name, env := range c.Environments {
		if env.Secret == "" {
			return &errors.ConfigError{
				Path:    path,
				Message: "environment " + name + " is missing required 'secret' field",
			}
		}
	}

	return nil
}

// GetEnvironment returns the environment config for the given name.
// If name is empty, returns the default environment.
func (c *Config) GetEnvironment(name string) (*Environment, error) {
	if name == "" {
		name = c.DefaultEnvironment
	}
	if name == "" {
		// No name provided and no default set - use first environment
		for _, env := range c.Environments {
			return &env, nil
		}
	}

	env, ok := c.Environments[name]
	if !ok {
		return nil, &errors.ConfigError{
			Message: "environment not found: " + name,
		}
	}
	return &env, nil
}
