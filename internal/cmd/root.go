// Package cmd implements the CLI commands for envctl.
package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/sentiolabs/envctl/internal/cache"
	"github.com/sentiolabs/envctl/internal/config"
	"github.com/sentiolabs/envctl/internal/env"
	"github.com/sentiolabs/envctl/internal/secrets"
	"github.com/spf13/cobra"
)

var (
	// Persistent flags
	configFile string
	appName    string
	envName    string
	verbose    bool
	noCache    bool
	refresh    bool
	includeAll bool // Include all keys from primary secret (override config)

	rootCmd = &cobra.Command{
		Use:   "envctl",
		Short: "Secrets CLI for local development",
		Long: `envctl enables developers to use secrets backends (AWS Secrets Manager,
1Password) as the single source of truth for application secrets during
local development.

It provides a simple way to inject secrets into process environments or
generate .env files for Docker Compose workflows.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
)

func init() {
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "config file path (default: .envctl.yaml)")
	rootCmd.PersistentFlags().StringVarP(&appName, "app", "a", "", "application name (default: from config)")
	rootCmd.PersistentFlags().StringVarP(&envName, "env", "e", "", "environment name (default: from config)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVar(&noCache, "no-cache", false, "bypass secret cache")
	rootCmd.PersistentFlags().BoolVar(&refresh, "refresh", false, "force refresh secrets and update cache")
	rootCmd.PersistentFlags().BoolVar(
		&includeAll, "include-all", false, "include all keys from primary secret (override config)",
	)

	// Register custom completions
	_ = rootCmd.RegisterFlagCompletionFunc("app", completeApplicationNames)
	_ = rootCmd.RegisterFlagCompletionFunc("env", completeEnvironmentNames)
}

// Execute runs the root command.
//
//nolint:revive // CLI output to stderr always succeeds
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

// verboseLog prints a message if verbose mode is enabled.
//
//nolint:revive // CLI output to stderr always succeeds
func verboseLog(format string, args ...any) {
	if verbose {
		fmt.Fprintf(os.Stderr, "[envctl] "+format+"\n", args...)
	}
}

// createSecretsClient creates a secrets client based on the configured backend.
func createSecretsClient(
	ctx context.Context, cfg *config.Config, envConfig *config.Environment,
) (secrets.Client, error) {
	var cacheManager *cache.Manager

	backend := cfg.ResolveBackend(envConfig)

	// Set up cache if enabled (and not bypassed)
	// Note: cache only applies to AWS backend currently
	if cfg.CacheEnabled() && !noCache && backend != config.Backend1Pass {
		cacheOpts := cache.Options{
			Enabled: true,
			TTL:     cfg.CacheTTL(),
			Backend: cache.BackendType(cfg.CacheBackend()),
		}
		// Use defaults if not set
		if cacheOpts.TTL == 0 {
			cacheOpts.TTL = cache.DefaultTTL
		}
		if cacheOpts.Backend == "" {
			cacheOpts.Backend = cache.BackendAuto
		}

		var err error
		cacheManager, err = cache.NewManager(cacheOpts)
		if err != nil {
			// Cache initialization failed, continue without cache
			verboseLog("Cache initialization failed: %v", err)
		} else if cacheManager.IsEnabled() {
			verboseLog("Using cache backend: %s", cacheManager.BackendName())
		}
	}

	verboseLog("Using secrets backend: %s", backend)

	if backend == config.BackendAWS {
		awsCfg := cfg.ResolveAWSConfig(envConfig)
		if awsCfg.Profile != "" {
			verboseLog("Using AWS profile: %s", awsCfg.Profile)
		}
	}

	return secrets.NewClient(ctx, secrets.Options{
		Config:  cfg,
		Env:     envConfig,
		Cache:   cacheManager,
		NoCache: noCache,
		Refresh: refresh,
	})
}

// completeApplicationNames provides completion for application names from config.
func completeApplicationNames(
	cmd *cobra.Command,
	args []string,
	toComplete string,
) ([]string, cobra.ShellCompDirective) {
	cfg := loadConfigForCompletion()
	if cfg == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	apps := make([]string, 0, len(cfg.Applications))
	for name := range cfg.Applications {
		apps = append(apps, name)
	}
	sort.Strings(apps)

	return apps, cobra.ShellCompDirectiveNoFileComp
}

// completeEnvironmentNames provides completion for environment names from config.
func completeEnvironmentNames(
	cmd *cobra.Command,
	args []string,
	toComplete string,
) ([]string, cobra.ShellCompDirective) {
	cfg := loadConfigForCompletion()
	if cfg == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	envs := make([]string, 0)

	// Get environment names based on configuration mode
	switch {
	case cfg.HasApplications() && appName != "":
		// App specified - get envs from that app
		if app, ok := cfg.Applications[appName]; ok {
			for name := range app.Environments {
				envs = append(envs, name)
			}
		}
	case cfg.HasApplications():
		// Collect all unique env names from all applications
		envSet := make(map[string]struct{})
		for _, app := range cfg.Applications {
			for name := range app.Environments {
				envSet[name] = struct{}{}
			}
		}
		for name := range envSet {
			envs = append(envs, name)
		}
	default:
		// Legacy mode - use global environments
		for name := range cfg.Environments {
			envs = append(envs, name)
		}
	}

	sort.Strings(envs)
	return envs, cobra.ShellCompDirectiveNoFileComp
}

// loadConfigForCompletion loads the config file for shell completion.
func loadConfigForCompletion() *config.Config {
	configPath := configFile
	if configPath == "" {
		var err error
		configPath, err = config.FindConfig()
		if err != nil {
			return nil
		}
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return nil
	}
	return cfg
}

// resolveEnvironmentConfig resolves the environment config based on mode.
// Returns the environment config and, if in application mode, the application config.
func resolveEnvironmentConfig(cfg *config.Config) (*config.Environment, *config.Application, error) {
	if cfg.HasApplications() {
		env, app, err := cfg.GetApplicationEnvironment(appName, envName)
		if err != nil {
			return nil, nil, err
		}
		return env, app, nil
	}
	// Legacy mode
	env, err := cfg.GetEnvironment(envName)
	if err != nil {
		return nil, nil, err
	}
	return env, nil, nil
}

// getIncludeAllOverride returns a pointer to the includeAll flag if it was explicitly set,
// or nil if the user didn't specify the flag (to use config default).
func getIncludeAllOverride(cmd *cobra.Command) *bool {
	if cmd.Flags().Changed("include-all") {
		return &includeAll
	}
	return nil
}

// loadAndBuild loads config, creates a secrets client, and builds environment entries.
// This consolidates the common config→client→builder pattern used by env, run, export, and get commands.
func loadAndBuild(
	ctx context.Context,
	cmd *cobra.Command,
	overrides map[string]string,
) ([]env.Entry, *config.Config, error) {
	configPath := configFile
	if configPath == "" {
		var err error
		configPath, err = config.FindConfig()
		if err != nil {
			return nil, nil, err
		}
	}
	verboseLog("Using config: %s", configPath)

	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, nil, err
	}

	envConfig, _, err := resolveEnvironmentConfig(cfg)
	if err != nil {
		return nil, nil, err
	}

	selectedEnv := envName
	if selectedEnv == "" {
		selectedEnv = cfg.DefaultEnvironment
	}
	verboseLog("Using environment: %s (secret: %s)", selectedEnv, envConfig.Secret())

	client, err := createSecretsClient(ctx, cfg, envConfig)
	if err != nil {
		return nil, nil, err
	}

	builder := env.NewBuilder(client, cfg, appName, envName).
		WithIncludeAll(getIncludeAllOverride(cmd)).
		WithOptions(env.BuilderOptions{NoCache: noCache, Refresh: refresh})

	entries, err := builder.Build(ctx, overrides)
	if err != nil {
		return nil, nil, err
	}
	verboseLog("Loaded %d environment variables", len(entries))

	return entries, cfg, nil
}
