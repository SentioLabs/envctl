package tui

import (
	"sort"

	"github.com/sentiolabs/envctl/internal/config"
)

// Source represents a secret source from the config with its backend info.
type Source struct {
	Name    string // secret reference (e.g., "BACstack Local - Core API")
	Backend string // "1pass" or "aws"
}

// ConfigContext holds config-derived data for the config-driven TUI flow.
type ConfigContext struct {
	Apps       []string            // application names (sorted)
	Envs       map[string][]string // app name -> env names (sorted)
	Sources    map[string][]Source // "app/env" key -> sources
	DefaultApp string
	DefaultEnv string
}

// NewConfigContext extracts a ConfigContext from a config.Config.
// Returns nil if the config is nil or has no applications/environments.
func NewConfigContext(cfg *config.Config) *ConfigContext {
	if cfg == nil {
		return nil
	}

	if len(cfg.Applications) == 0 && len(cfg.Environments) == 0 {
		return nil
	}

	ctx := &ConfigContext{
		Envs:       make(map[string][]string),
		Sources:    make(map[string][]Source),
		DefaultApp: cfg.DefaultApplication,
		DefaultEnv: cfg.DefaultEnvironment,
	}

	if cfg.HasApplications() {
		extractApplications(cfg, ctx)
	} else {
		extractLegacyEnvironments(cfg, ctx)
	}

	return ctx
}

// extractApplications populates the ConfigContext from application-mode config.
func extractApplications(cfg *config.Config, ctx *ConfigContext) {
	for appName := range cfg.Applications {
		ctx.Apps = append(ctx.Apps, appName)
	}
	sort.Strings(ctx.Apps)

	for _, appName := range ctx.Apps {
		app := cfg.Applications[appName]
		var envNames []string
		for envName := range app.Environments {
			envNames = append(envNames, envName)
		}
		sort.Strings(envNames)
		ctx.Envs[appName] = envNames

		for _, envName := range envNames {
			env := app.Environments[envName]
			key := appName + "/" + envName
			ctx.Sources[key] = resolveSources(cfg, &env)
		}
	}
}

// extractLegacyEnvironments populates the ConfigContext from legacy-mode config.
func extractLegacyEnvironments(cfg *config.Config, ctx *ConfigContext) {
	ctx.Apps = []string{""}

	var envNames []string
	for envName := range cfg.Environments {
		envNames = append(envNames, envName)
	}
	sort.Strings(envNames)
	ctx.Envs[""] = envNames

	for _, envName := range envNames {
		env := cfg.Environments[envName]
		key := "/" + envName
		ctx.Sources[key] = resolveSources(cfg, &env)
	}
}

// resolveSources converts an environment's sources into Source structs with resolved backends.
func resolveSources(cfg *config.Config, env *config.Environment) []Source {
	sources := make([]Source, 0, len(env.Sources))
	for _, entry := range env.Sources {
		backend := resolveSourceBackend(cfg, env, &entry)
		sources = append(sources, Source{
			Name:    entry.Secret,
			Backend: backend,
		})
	}
	return sources
}

// resolveSourceBackend determines the backend for a single source entry.
// Uses the entry's explicit backend if set, otherwise falls back to config-level resolution.
func resolveSourceBackend(cfg *config.Config, env *config.Environment, entry *config.IncludeEntry) string {
	if entry.Backend != "" {
		return entry.Backend
	}
	if entry.AWS != nil {
		return config.BackendAWS
	}
	if entry.OnePass != nil {
		return config.Backend1Pass
	}
	return cfg.ResolveBackend(env)
}
