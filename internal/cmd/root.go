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
	rootCmd.PersistentFlags().StringVarP(&envName, "env", "e", "", "environment name (default: from config)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVar(&noCache, "no-cache", false, "bypass secret cache")
	rootCmd.PersistentFlags().BoolVar(&refresh, "refresh", false, "force refresh secrets and update cache")

	// Register custom completion for --env flag
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
func createSecretsClient(ctx context.Context, cfg *config.Config, region string) (*aws.SecretsClient, error) {
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

	return aws.NewSecretsClientWithOptions(ctx, aws.ClientOptions{
		Region:  region,
		Cache:   cacheManager,
		NoCache: noCache,
		Refresh: refresh,
	})
}

// completeEnvironmentNames provides completion for environment names from config.
func completeEnvironmentNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// Try to find and load config
	configPath := configFile
	if configPath == "" {
		var err error
		configPath, err = config.FindConfig()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Collect environment names
	envs := make([]string, 0, len(cfg.Environments))
	for name := range cfg.Environments {
		envs = append(envs, name)
	}
	sort.Strings(envs)

	return envs, cobra.ShellCompDirectiveNoFileComp
}
