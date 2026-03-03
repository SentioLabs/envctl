// Package config handles parsing and validation of .envctl.yaml files.
package config

import (
	"bytes"
	"os"
	"path/filepath"
	"time"

	"github.com/sentiolabs/envctl/internal/errors"
	"gopkg.in/yaml.v3"
)

const (
	// ConfigFileName is the name of the configuration file.
	ConfigFileName = ".envctl.yaml"
	// CurrentVersion is the current config file version.
	CurrentVersion = 1
)

// Backend constants for secret providers.
const (
	BackendAWS   = "aws"
	Backend1Pass = "1pass"
)

// Config represents the root configuration structure.
// YAML tags use snake_case for consistency with standard YAML conventions.
//
//nolint:tagliatelle // Using snake_case for YAML field names is intentional
type Config struct {
	Version            int                     `yaml:"version"`
	DefaultApplication string                  `yaml:"default_application,omitempty"`
	DefaultEnvironment string                  `yaml:"default_environment,omitempty"`
	IncludeAll         *bool                   `yaml:"include_all,omitempty"`
	AWS                *AWSConfig              `yaml:"aws,omitempty"`
	OnePass            *OnePassConfig           `yaml:"1pass,omitempty"`
	Applications       map[string]*Application `yaml:"applications,omitempty"`
	Environments       map[string]Environment  `yaml:"environments,omitempty"`
	Include            []IncludeEntry          `yaml:"include,omitempty"`
	Mapping            map[string]string       `yaml:"mapping,omitempty"`
	Cache              *CacheConfig            `yaml:"cache,omitempty"`
}

// Application represents an application with its environment configurations.
//
//nolint:tagliatelle // Using snake_case for YAML field names is intentional
type Application struct {
	Environments map[string]Environment `yaml:",inline"`
	Include      []IncludeEntry         `yaml:"include,omitempty"`
	Mapping      map[string]string      `yaml:"mapping,omitempty"`
	IncludeAll   *bool                  `yaml:"include_all,omitempty"`
}

// CacheConfig represents cache configuration.
type CacheConfig struct {
	Enabled *bool  `yaml:"enabled,omitempty"` // Pointer to distinguish unset from false
	TTL     string `yaml:"ttl,omitempty"`     // Duration string like "15m", "1h"
	Backend string `yaml:"backend,omitempty"` // "auto", "keyring", "file", "none"
}

// AWSConfig holds AWS Secrets Manager-specific settings.
type AWSConfig struct {
	Region  string `yaml:"region,omitempty"`
	Profile string `yaml:"profile,omitempty"`
}

// OnePassConfig holds 1Password-specific settings.
type OnePassConfig struct {
	Vault   string `yaml:"vault,omitempty"`
	Account string `yaml:"account,omitempty"`
}

// Environment represents a single environment configuration.
//
//nolint:tagliatelle // Using snake_case for YAML field names is intentional
type Environment struct {
	Secret     string         `yaml:"secret"` //nolint:gosec // G117: not credentials
	IncludeAll *bool          `yaml:"include_all,omitempty"`
	AWS        *AWSConfig     `yaml:"aws,omitempty"`
	OnePass    *OnePassConfig `yaml:"1pass,omitempty"`
}

// IncludeEntry represents an additional secret to include.
type IncludeEntry struct {
	Secret string `yaml:"secret"` //nolint:gosec // G117: field name refers to a secret reference, not credentials
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
//
//nolint:revive // Config validation requires checking multiple conditions in sequence
func (c *Config) Validate(path string) error {
	if c.Version != CurrentVersion {
		return &errors.ConfigError{
			Path:    path,
			Message: "unsupported config version (expected version: 1)",
		}
	}

	// Validate global backend config: cannot have both aws and 1pass
	if c.AWS != nil && c.OnePass != nil {
		return &errors.ConfigError{
			Path:    path,
			Message: "cannot specify both 'aws' and '1pass' at the global level",
		}
	}

	// Must have either applications or environments (or both for migration)
	if len(c.Applications) == 0 && len(c.Environments) == 0 {
		return &errors.ConfigError{
			Path:    path,
			Message: "no applications or environments defined",
		}
	}

	// Validate applications
	for appName, app := range c.Applications {
		if len(app.Environments) == 0 {
			return &errors.ConfigError{
				Path:    path,
				Message: "application " + appName + " has no environments defined",
			}
		}
		for envName, env := range app.Environments {
			if env.Secret == "" {
				msg := "application " + appName + " environment " + envName +
					" is missing required 'secret' field"
				return &errors.ConfigError{
					Path:    path,
					Message: msg,
				}
			}
		}
	}

	// Validate default_application reference
	if c.DefaultApplication != "" {
		if _, ok := c.Applications[c.DefaultApplication]; !ok {
			return &errors.ConfigError{
				Path:    path,
				Message: "default_application references undefined application: " + c.DefaultApplication,
			}
		}
	}

	// Validate legacy environments (for backwards compatibility)
	if c.DefaultEnvironment != "" && len(c.Environments) > 0 {
		if _, ok := c.Environments[c.DefaultEnvironment]; !ok {
			return &errors.ConfigError{
				Path:    path,
				Message: "default_environment references undefined environment: " + c.DefaultEnvironment,
			}
		}
	}

	// Validate per-environment: cannot have both aws and 1pass
	for envName, env := range c.Environments {
		if env.Secret == "" {
			return &errors.ConfigError{
				Path:    path,
				Message: "environment " + envName + " is missing required 'secret' field",
			}
		}
		if env.AWS != nil && env.OnePass != nil {
			return &errors.ConfigError{
				Path:    path,
				Message: "environment " + envName + " cannot specify both 'aws' and '1pass'",
			}
		}
	}

	// Same check for application environments
	for appName, app := range c.Applications {
		for envName, env := range app.Environments {
			if env.AWS != nil && env.OnePass != nil {
				return &errors.ConfigError{
					Path:    path,
					Message: "application " + appName + " environment " + envName + " cannot specify both 'aws' and '1pass'",
				}
			}
		}
	}

	return nil
}

// HasApplications returns true if the config uses application-based structure.
func (c *Config) HasApplications() bool {
	return len(c.Applications) > 0
}

// GetEnvironment returns the environment config for the given name.
// If name is empty, returns the default environment.
// This is for legacy (non-application) mode.
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

// GetApplication returns the application config for the given name.
// If name is empty, returns the default application.
func (c *Config) GetApplication(name string) (*Application, error) {
	if name == "" {
		name = c.DefaultApplication
	}
	if name == "" {
		// No name provided and no default set - use first application
		for _, app := range c.Applications {
			return app, nil
		}
	}

	app, ok := c.Applications[name]
	if !ok {
		return nil, &errors.ConfigError{
			Message: "application not found: " + name,
		}
	}
	return app, nil
}

// GetApplicationEnvironment returns the environment config for an application.
// If appName is empty, uses default_application.
// If envName is empty, uses default_environment.
func (c *Config) GetApplicationEnvironment(appName, envName string) (*Environment, *Application, error) {
	app, err := c.GetApplication(appName)
	if err != nil {
		return nil, nil, err
	}

	if envName == "" {
		envName = c.DefaultEnvironment
	}
	if envName == "" {
		// No name provided and no default set - use first environment
		for _, env := range app.Environments {
			return &env, app, nil
		}
	}

	env, ok := app.Environments[envName]
	if !ok {
		return nil, nil, &errors.ConfigError{
			Message: "environment " + envName + " not found in application " + appName,
		}
	}
	return &env, app, nil
}

// CacheEnabled returns whether caching is enabled in config.
// Returns true if not explicitly disabled.
func (c *Config) CacheEnabled() bool {
	if c.Cache == nil || c.Cache.Enabled == nil {
		return true // Enabled by default
	}
	return *c.Cache.Enabled
}

// CacheTTL returns the configured cache TTL.
// Returns 0 if not set or invalid (caller should use default).
func (c *Config) CacheTTL() time.Duration {
	if c.Cache == nil || c.Cache.TTL == "" {
		return 0
	}
	d, err := time.ParseDuration(c.Cache.TTL)
	if err != nil {
		return 0
	}
	return d
}

// CacheBackend returns the configured cache backend.
// Returns empty string if not set (caller should use default).
func (c *Config) CacheBackend() string {
	if c.Cache == nil {
		return ""
	}
	return c.Cache.Backend
}

// ResolveBackend determines the backend for a given environment.
// Precedence: environment block > global block > default (aws).
func (c *Config) ResolveBackend(env *Environment) string {
	if env != nil {
		if env.OnePass != nil {
			return Backend1Pass
		}
		if env.AWS != nil {
			return BackendAWS
		}
	}
	if c.OnePass != nil {
		return Backend1Pass
	}
	if c.AWS != nil {
		return BackendAWS
	}
	return BackendAWS
}

// ResolveAWSConfig merges global and environment-level AWS config.
func (c *Config) ResolveAWSConfig(env *Environment) AWSConfig {
	result := AWSConfig{}
	if c.AWS != nil {
		result = *c.AWS
	}
	if env != nil && env.AWS != nil {
		if env.AWS.Region != "" {
			result.Region = env.AWS.Region
		}
		if env.AWS.Profile != "" {
			result.Profile = env.AWS.Profile
		}
	}
	return result
}

// ResolveOnePassConfig merges global and environment-level 1Pass config.
func (c *Config) ResolveOnePassConfig(env *Environment) OnePassConfig {
	result := OnePassConfig{}
	if c.OnePass != nil {
		result = *c.OnePass
	}
	if env != nil && env.OnePass != nil {
		if env.OnePass.Vault != "" {
			result.Vault = env.OnePass.Vault
		}
		if env.OnePass.Account != "" {
			result.Account = env.OnePass.Account
		}
	}
	return result
}

// ShouldIncludeAll resolves include_all setting with precedence: env > app > global.
// Returns false by default (mappings-only mode).
func (c *Config) ShouldIncludeAll(app *Application, env *Environment) bool {
	// Environment-level has highest precedence
	if env != nil && env.IncludeAll != nil {
		return *env.IncludeAll
	}
	// Application-level next
	if app != nil && app.IncludeAll != nil {
		return *app.IncludeAll
	}
	// Global config
	if c.IncludeAll != nil {
		return *c.IncludeAll
	}
	// Default: mappings-only mode
	return false
}
