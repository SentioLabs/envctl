// Package cmd implements the CLI commands for envctl.
// This file contains cache-related commands for managing locally cached secrets.
package cmd

import (
	"fmt"
	"os"

	"github.com/sentiolabs/envctl/internal/cache"
	"github.com/spf13/cobra"
)

// cacheCmd is the parent command for cache-related subcommands.
// It provides a namespace for cache management operations.
var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage the secret cache",
	Long: `Manage the local secret cache.

envctl caches secrets locally to improve performance and reduce AWS API calls.
Use these commands to inspect and manage the cache.`,
}

// cacheClearCmd removes all cached secrets from the local cache backend.
var cacheClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear all cached secrets",
	Long: `Clear all locally cached secrets.

This removes all cached secret data from the local cache backend
(keyring or encrypted files).`,
	RunE: runCacheClear,
}

// cacheStatusCmd displays information about the cache including backend type,
// number of entries, hit/miss statistics, and storage size.
var cacheStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show cache status and statistics",
	Long: `Display information about the cache including:
- Active backend (keyring or file)
- Number of cached entries
- Cache hit/miss statistics
- Storage size (for file backend)`,
	RunE: runCacheStatus,
}

// init registers cache commands with the root command.
func init() {
	cacheCmd.AddCommand(cacheClearCmd)
	cacheCmd.AddCommand(cacheStatusCmd)
	rootCmd.AddCommand(cacheCmd)
}

// runCacheClear handles the 'cache clear' command, removing all cached secrets.
//
//nolint:revive // CLI output to stdout always succeeds
func runCacheClear(cmd *cobra.Command, args []string) error {
	// Try to create a cache manager to clear
	manager, err := cache.NewManager(cache.DefaultOptions())
	if err != nil {
		return fmt.Errorf("failed to initialize cache: %w", err)
	}

	if !manager.IsEnabled() {
		fmt.Fprintln(os.Stdout, "Cache is disabled or not available")
		return nil
	}

	if err := manager.Clear(); err != nil {
		return fmt.Errorf("failed to clear cache: %w", err)
	}

	fmt.Fprintf(os.Stdout, "Cache cleared (%s backend)\n", manager.BackendName())
	return nil
}

// runCacheStatus handles the 'cache status' command, displaying cache information.
//
//nolint:revive // CLI output to stdout always succeeds
func runCacheStatus(cmd *cobra.Command, args []string) error {
	// Try to create a cache manager
	manager, err := cache.NewManager(cache.DefaultOptions())
	if err != nil {
		// Cache initialization failed, but this is not a fatal error for status command.
		// Just report that cache is disabled due to initialization failure.
		fmt.Fprintln(os.Stdout, "Cache: disabled (initialization failed)")
		return nil //nolint:nilerr // Intentionally returning nil - status command handles this gracefully
	}

	if !manager.IsEnabled() {
		fmt.Fprintln(os.Stdout, "Cache: disabled")
		// Check why cache is disabled and report reason
		if !cache.ShouldCache() {
			if os.Geteuid() == 0 {
				fmt.Fprintln(os.Stdout, "  Reason: Running as root")
			}
		}
		return nil
	}

	// Get and display cache statistics
	stats, err := manager.Stats()
	if err != nil {
		return fmt.Errorf("failed to get cache stats: %w", err)
	}

	fmt.Fprintf(os.Stdout, "Cache: enabled\n")
	fmt.Fprintf(os.Stdout, "  Backend: %s\n", stats.Backend)
	fmt.Fprintf(os.Stdout, "  Entries: %d\n", stats.EntryCount)
	fmt.Fprintf(os.Stdout, "  Hits:    %d\n", stats.HitCount)
	fmt.Fprintf(os.Stdout, "  Misses:  %d\n", stats.MissCount)
	if stats.Size >= 0 {
		fmt.Fprintf(os.Stdout, "  Size:    %s\n", formatSize(stats.Size))
	}
	fmt.Fprintf(os.Stdout, "  Dir:     %s\n", cache.GetCacheDir())

	return nil
}

// formatSize formats a byte size into a human-readable string with appropriate units.
// It uses binary units (1024 bytes = 1 KB) and formats with one decimal place.
func formatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}
