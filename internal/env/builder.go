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
	envName string
}

// NewBuilder creates a new environment builder.
func NewBuilder(secrets *aws.SecretsClient, cfg *config.Config, envName string) *Builder {
	return &Builder{
		secrets: secrets,
		config:  cfg,
		envName: envName,
	}
}

// Build resolves all environment variables according to precedence rules.
// Order: primary secret -> include entries -> mapping -> overrides
func (b *Builder) Build(ctx context.Context, overrides map[string]string) ([]Entry, error) {
	entries := make(map[string]Entry)

	// Get the environment config
	env, err := b.config.GetEnvironment(b.envName)
	if err != nil {
		return nil, err
	}

	// 1. Load primary secret (all keys)
	secrets, err := b.secrets.GetSecret(ctx, env.Secret)
	if err != nil {
		return nil, err
	}
	for key, value := range secrets {
		entries[key] = Entry{
			Key:    key,
			Value:  value,
			Source: env.Secret,
		}
	}

	// 2. Process include entries (in order)
	for _, inc := range b.config.Include {
		if inc.Key != "" {
			// Extract specific key
			value, err := b.secrets.GetSecretKey(ctx, inc.Secret, inc.Key)
			if err != nil {
				return nil, err
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
				return nil, err
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

	// 3. Apply mapping entries
	for envVar, ref := range b.config.Mapping {
		secretRef, err := config.ParseSecretRef(ref)
		if err != nil {
			return nil, err
		}
		value, err := b.secrets.GetSecretKey(ctx, secretRef.SecretName, secretRef.KeyName)
		if err != nil {
			return nil, err
		}
		entries[envVar] = Entry{
			Key:    envVar,
			Value:  value,
			Source: "mapping",
		}
	}

	// 4. Apply overrides
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

// ToMap converts entries to a simple key-value map.
func ToMap(entries []Entry) map[string]string {
	result := make(map[string]string, len(entries))
	for _, e := range entries {
		result[e.Key] = e.Value
	}
	return result
}
