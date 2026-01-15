// Package cmd implements the CLI commands for envctl.
package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/sentiolabs/envctl/internal/aws"
	"github.com/sentiolabs/envctl/internal/cache"
	"github.com/sentiolabs/envctl/internal/config"
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

	rootCmd = &cobra.Command{
		Use:   "envctl",
		Short: "AWS Secrets Manager CLI for local development",
		Long: `envctl enables developers to use AWS Secrets Manager as the single source
of truth for application secrets during local development.

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

	// Register custom completions
	rootCmd.RegisterFlagCompletionFunc("app", completeApplicationNames)
	rootCmd.RegisterFlagCompletionFunc("env", completeEnvironmentNames)
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

// verboseLog prints a message if verbose mode is enabled.
func verboseLog(format string, args ...any) {
	if verbose {
		fmt.Fprintf(os.Stderr, "[envctl] "+format+"\n", args...)
	}
}

// createSecretsClient creates an AWS Secrets Manager client with caching support.
func createSecretsClient(ctx context.Context, cfg *config.Config, region, profile string) (*aws.SecretsClient, error) {
	var cacheManager *cache.Manager

	// Set up cache if enabled (and not bypassed)
	if cfg.CacheEnabled() && !noCache {
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

	if profile != "" {
		verboseLog("Using AWS profile: %s", profile)
	}

	return aws.NewSecretsClientWithOptions(ctx, aws.ClientOptions{
		Region:  region,
		Profile: profile,
		Cache:   cacheManager,
		NoCache: noCache,
		Refresh: refresh,
	})
}

// completeApplicationNames provides completion for application names from config.
func completeApplicationNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
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
func completeEnvironmentNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	cfg := loadConfigForCompletion()
	if cfg == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	envs := make([]string, 0)

	// If using applications and --app is set, get envs from that app
	if cfg.HasApplications() && appName != "" {
		if app, ok := cfg.Applications[appName]; ok {
			for name := range app.Environments {
				envs = append(envs, name)
			}
		}
	} else if cfg.HasApplications() {
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
	} else {
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
