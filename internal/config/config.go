// Package config handles parsing and validation of .envctl.yaml files.
package config

import (
	"bytes"
	"fmt"
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
	DefaultBackend     string                  `yaml:"default_backend,omitempty"`
	IncludeAll         *bool                   `yaml:"include_all,omitempty"`
	AWS                *AWSConfig              `yaml:"aws,omitempty"`
	OnePass            *OnePassConfig          `yaml:"1pass,omitempty"`
	Applications       map[string]*Application `yaml:"applications,omitempty"`
	Environments       map[string]Environment  `yaml:"environments,omitempty"`
	Mapping            map[string]string       `yaml:"mapping,omitempty"`
	Cache              *CacheConfig            `yaml:"cache,omitempty"`
}

// Application represents an application with its environment configurations.
//
//nolint:tagliatelle // Using snake_case for YAML field names is intentional
type Application struct {
	Environments map[string]Environment `yaml:",inline"`
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
// It supports two YAML formats:
//   - Mapping (legacy): {secret: "...", aws: {...}} → single-entry Sources
//   - Sequence (new): [{secret: "..."}, {secret: "...", aws: {...}}] → multi-entry Sources
type Environment struct {
	Sources    []IncludeEntry // Populated by UnmarshalYAML
	IncludeAll *bool          // Populated by UnmarshalYAML (legacy format only)
	AWS        *AWSConfig     // First source's backend (for primary client creation)
	OnePass    *OnePassConfig // First source's backend (for primary client creation)
}

// NewEnvironment creates an Environment from a list of sources.
// Backend config (AWS/OnePass) is set from the first source.
// If the first source has a backend field but no explicit config block,
// an empty config struct is promoted to enable backend resolution.
func NewEnvironment(sources ...IncludeEntry) Environment {
	env := Environment{Sources: sources}
	if len(sources) > 0 {
		PromoteBackend(&env.Sources[0])
		env.AWS = env.Sources[0].AWS
		env.OnePass = env.Sources[0].OnePass
	}
	return env
}

// Secret returns the primary secret reference (from the first source).
func (e *Environment) Secret() string {
	if len(e.Sources) > 0 {
		return e.Sources[0].Secret
	}
	return ""
}

// PromoteBackend converts a backend field value into an empty config struct
// when no explicit aws:/1pass: block is present. This enables the existing
// ResolveBackend logic to detect the correct backend.
func PromoteBackend(src *IncludeEntry) {
	if src.Backend == "" || src.AWS != nil || src.OnePass != nil {
		return
	}
	switch src.Backend {
	case BackendAWS:
		src.AWS = &AWSConfig{}
	case Backend1Pass:
		src.OnePass = &OnePassConfig{}
	}
}

// UnmarshalYAML implements custom YAML unmarshaling to support both
// legacy mapping format and new sequence format.
func (e *Environment) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.MappingNode:
		// Legacy single-secret format: {secret: "...", aws: {...}, include_all: true}
		//nolint:tagliatelle // Using snake_case for YAML field names is intentional
		var legacy struct {
			Secret     string         `yaml:"secret"`            //nolint:gosec // G117: refers to secret ref, not creds
			Backend    string         `yaml:"backend,omitempty"` // routing hint
			IncludeAll *bool          `yaml:"include_all,omitempty"`
			AWS        *AWSConfig     `yaml:"aws,omitempty"`
			OnePass    *OnePassConfig `yaml:"1pass,omitempty"`
		}
		if err := value.Decode(&legacy); err != nil {
			return err
		}
		e.Sources = []IncludeEntry{{
			Secret:  legacy.Secret,
			Backend: legacy.Backend,
			AWS:     legacy.AWS,
			OnePass: legacy.OnePass,
		}}
		PromoteBackend(&e.Sources[0])
		e.IncludeAll = legacy.IncludeAll
		e.AWS = e.Sources[0].AWS
		e.OnePass = e.Sources[0].OnePass
		return nil
	case yaml.SequenceNode:
		// New list format: [{secret: "..."}, {secret: "...", aws: {...}}]
		var sources []IncludeEntry
		if err := value.Decode(&sources); err != nil {
			return err
		}
		e.Sources = sources
		if len(sources) > 0 {
			PromoteBackend(&e.Sources[0])
			e.AWS = e.Sources[0].AWS
			e.OnePass = e.Sources[0].OnePass
		}
		return nil
	default:
		return fmt.Errorf("environment must be a mapping or sequence, got %v", value.Kind)
	}
}

// KeyMapping represents a single key extraction from a secret with optional renaming.
type KeyMapping struct {
	Key string `yaml:"key"`
	As  string `yaml:"as,omitempty"`
}

// IncludeEntry represents an additional secret to include.
type IncludeEntry struct {
	//nolint:gosec // G117: field name refers to a secret reference, not credentials
	Secret  string         `yaml:"secret"`
	Key     string         `yaml:"key,omitempty"`
	As      string         `yaml:"as,omitempty"`
	Keys    []KeyMapping   `yaml:"keys,omitempty"`
	Backend string         `yaml:"backend,omitempty"` // "aws" or "1pass": routing hint when no aws:/1pass: block
	AWS     *AWSConfig     `yaml:"aws,omitempty"`
	OnePass *OnePassConfig `yaml:"1pass,omitempty"`
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

	// Validate default_backend
	if c.DefaultBackend != "" {
		if c.DefaultBackend != BackendAWS && c.DefaultBackend != Backend1Pass {
			return &errors.ConfigError{
				Path: path,
				Message: fmt.Sprintf(
					"invalid default_backend value %q (must be %q or %q)",
					c.DefaultBackend, BackendAWS, Backend1Pass,
				),
			}
		}
		if c.AWS == nil || c.OnePass == nil {
			return &errors.ConfigError{
				Path:    path,
				Message: "default_backend is only valid when both 'aws' and '1pass' are configured",
			}
		}
	}
	if c.AWS != nil && c.OnePass != nil && c.DefaultBackend == "" {
		return &errors.ConfigError{
			Path:    path,
			Message: "default_backend is required when both 'aws' and '1pass' are configured",
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
			if err := validateEnvironment(env, "application "+appName+" environment "+envName, path); err != nil {
				return err
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

	// Validate per-environment sources
	for envName, env := range c.Environments {
		if err := validateEnvironment(env, "environment "+envName, path); err != nil {
			return err
		}
	}

	return nil
}

// validateEnvironment validates a single environment's sources.
func validateEnvironment(env Environment, location, path string) error {
	if len(env.Sources) == 0 || env.Sources[0].Secret == "" {
		return &errors.ConfigError{
			Path:    path,
			Message: location + " is missing required 'secret' field",
		}
	}
	for i, src := range env.Sources {
		if src.Secret == "" {
			return &errors.ConfigError{
				Path:    path,
				Message: fmt.Sprintf("%s source[%d] is missing required 'secret' field", location, i),
			}
		}
		if src.AWS != nil && src.OnePass != nil {
			return &errors.ConfigError{
				Path:    path,
				Message: fmt.Sprintf("%s source[%d] cannot specify both 'aws' and '1pass'", location, i),
			}
		}
		if err := validateBackendField(src, fmt.Sprintf("%s source[%d]", location, i), path); err != nil {
			return err
		}
		loc := fmt.Sprintf("%s source[%d]", location, i)
		if err := validateIncludeKeys(src, loc, path); err != nil {
			return err
		}
	}
	return nil
}

// validateBackendField checks that the backend field value is valid and doesn't conflict
// with explicit aws:/1pass: blocks.
func validateBackendField(src IncludeEntry, location, path string) error {
	if src.Backend == "" {
		return nil
	}
	if src.Backend != BackendAWS && src.Backend != Backend1Pass {
		return &errors.ConfigError{
			Path: path,
			Message: fmt.Sprintf(
				"%s has invalid backend value %q (must be %q or %q)",
				location, src.Backend, BackendAWS, Backend1Pass,
			),
		}
	}
	// Check for conflicts: backend field says one thing, explicit block says another.
	// Note: after PromoteBackend(), a non-conflicting backend will have set the matching
	// config struct. So we check the *original* Backend value against the *opposite* config.
	// Since PromoteBackend only sets the matching struct when the other is nil, a conflict
	// means the user explicitly set a block for the opposite backend.
	if src.Backend == BackendAWS && src.OnePass != nil {
		return &errors.ConfigError{
			Path:    path,
			Message: fmt.Sprintf("%s has backend %q but also specifies '1pass' block", location, src.Backend),
		}
	}
	if src.Backend == Backend1Pass && src.AWS != nil {
		return &errors.ConfigError{
			Path:    path,
			Message: fmt.Sprintf("%s has backend %q but also specifies 'aws' block", location, src.Backend),
		}
	}
	return nil
}

// validateIncludeKeys checks that key and keys are not both set, and that keys entries have non-empty key fields.
func validateIncludeKeys(inc IncludeEntry, location, path string) error {
	if inc.Key != "" && len(inc.Keys) > 0 {
		return &errors.ConfigError{
			Path:    path,
			Message: location + " cannot specify both 'key' and 'keys'",
		}
	}
	for j, km := range inc.Keys {
		if km.Key == "" {
			return &errors.ConfigError{
				Path:    path,
				Message: fmt.Sprintf("%s keys[%d] is missing required 'key' field", location, j),
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
// Precedence: environment block > default_backend > global block > default (aws).
func (c *Config) ResolveBackend(env *Environment) string {
	if env != nil {
		if env.OnePass != nil {
			return Backend1Pass
		}
		if env.AWS != nil {
			return BackendAWS
		}
	}
	if c.DefaultBackend != "" {
		return c.DefaultBackend
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
