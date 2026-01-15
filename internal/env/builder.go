// Package env handles environment variable resolution and merging.
package env

import (
	"context"

	"github.com/sentiolabs/envctl/internal/aws"
	"github.com/sentiolabs/envctl/internal/config"
)

// Entry represents a resolved environment variable with its source.
type Entry struct {
	Key    string
	Value  string
	Source string
}

// Builder builds environment variables from configuration.
type Builder struct {
	secrets *aws.SecretsClient
	config  *config.Config
	appName string
	envName string
	app     *config.Application // Resolved application (nil in legacy mode)
	env     *config.Environment // Resolved environment
}

// NewBuilder creates a new environment builder.
// appName can be empty for legacy (non-application) configs.
func NewBuilder(secrets *aws.SecretsClient, cfg *config.Config, appName, envName string) *Builder {
	return &Builder{
		secrets: secrets,
		config:  cfg,
		appName: appName,
		envName: envName,
	}
}

// Build resolves all environment variables according to precedence rules.
// Order: primary secret -> global includes -> app includes -> global mapping -> app mapping -> overrides
func (b *Builder) Build(ctx context.Context, overrides map[string]string) ([]Entry, error) {
	entries := make(map[string]Entry)

	// Resolve environment config based on mode
	if err := b.resolveConfig(); err != nil {
		return nil, err
	}

	// 1. Load primary secret (all keys)
	secrets, err := b.secrets.GetSecret(ctx, b.env.Secret)
	if err != nil {
		return nil, err
	}
	for key, value := range secrets {
		entries[key] = Entry{
			Key:    key,
			Value:  value,
			Source: b.env.Secret,
		}
	}

	// 2. Process global include entries (in order)
	if err := b.processIncludes(ctx, entries, b.config.Include); err != nil {
		return nil, err
	}

	// 3. Process app-level include entries (if in application mode)
	if b.app != nil && len(b.app.Include) > 0 {
		if err := b.processIncludes(ctx, entries, b.app.Include); err != nil {
			return nil, err
		}
	}

	// 4. Apply global mapping entries
	if err := b.processMapping(ctx, entries, b.config.Mapping); err != nil {
		return nil, err
	}

	// 5. Apply app-level mapping entries (if in application mode)
	if b.app != nil && len(b.app.Mapping) > 0 {
		if err := b.processMapping(ctx, entries, b.app.Mapping); err != nil {
			return nil, err
		}
	}

	// 6. Apply overrides
	for key, value := range overrides {
		entries[key] = Entry{
			Key:    key,
			Value:  value,
			Source: "override",
		}
	}

	// Convert map to slice
	result := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		result = append(result, entry)
	}

	return result, nil
}

// resolveConfig determines the environment config based on application mode.
func (b *Builder) resolveConfig() error {
	if b.config.HasApplications() {
		// Application mode
		env, app, err := b.config.GetApplicationEnvironment(b.appName, b.envName)
		if err != nil {
			return err
		}
		b.env = env
		b.app = app
	} else {
		// Legacy mode
		env, err := b.config.GetEnvironment(b.envName)
		if err != nil {
			return err
		}
		b.env = env
	}
	return nil
}

// processIncludes processes a list of include entries.
func (b *Builder) processIncludes(ctx context.Context, entries map[string]Entry, includes []config.IncludeEntry) error {
	for _, inc := range includes {
		if inc.Key != "" {
			// Extract specific key
			value, err := b.secrets.GetSecretKey(ctx, inc.Secret, inc.Key)
			if err != nil {
				return err
			}
			name := inc.Key
			if inc.As != "" {
				name = inc.As
			}
			entries[name] = Entry{
				Key:    name,
				Value:  value,
				Source: inc.Secret,
			}
		} else {
			// Include all keys from secret
			incSecrets, err := b.secrets.GetSecret(ctx, inc.Secret)
			if err != nil {
				return err
			}
			for key, value := range incSecrets {
				entries[key] = Entry{
					Key:    key,
					Value:  value,
					Source: inc.Secret,
				}
			}
		}
	}
	return nil
}

// processMapping processes mapping entries.
func (b *Builder) processMapping(ctx context.Context, entries map[string]Entry, mapping map[string]string) error {
	for envVar, ref := range mapping {
		secretRef, err := config.ParseSecretRef(ref)
		if err != nil {
			return err
		}
		value, err := b.secrets.GetSecretKey(ctx, secretRef.SecretName, secretRef.KeyName)
		if err != nil {
			return err
		}
		entries[envVar] = Entry{
			Key:    envVar,
			Value:  value,
			Source: "mapping",
		}
	}
	return nil
}

// ToMap converts entries to a simple key-value map.
func ToMap(entries []Entry) map[string]string {
	result := make(map[string]string, len(entries))
	for _, e := range entries {
		result[e.Key] = e.Value
	}
	return result
}
