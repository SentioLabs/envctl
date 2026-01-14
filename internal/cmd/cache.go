package cmd

import (
	"fmt"
	"os"

	"github.com/sentiolabs/envctl/internal/cache"
	"github.com/spf13/cobra"
)

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage the secret cache",
	Long: `Manage the local secret cache.

envctl caches secrets locally to improve performance and reduce AWS API calls.
Use these commands to inspect and manage the cache.`,
}

var cacheClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear all cached secrets",
	Long: `Clear all locally cached secrets.

This removes all cached secret data from the local cache backend
(keyring or encrypted files).`,
	RunE: runCacheClear,
}

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

func init() {
	cacheCmd.AddCommand(cacheClearCmd)
	cacheCmd.AddCommand(cacheStatusCmd)
	rootCmd.AddCommand(cacheCmd)
}

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

func runCacheStatus(cmd *cobra.Command, args []string) error {
	// Try to create a cache manager
	manager, err := cache.NewManager(cache.DefaultOptions())
	if err != nil {
		fmt.Fprintln(os.Stdout, "Cache: disabled (initialization failed)")
		return nil
	}

	if !manager.IsEnabled() {
		fmt.Fprintln(os.Stdout, "Cache: disabled")
		// Check why
		if !cache.ShouldCache() {
			if cache.IsCI() {
				fmt.Fprintln(os.Stdout, "  Reason: CI environment detected")
			} else if os.Geteuid() == 0 {
				fmt.Fprintln(os.Stdout, "  Reason: Running as root")
			}
		}
		return nil
	}

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

// formatSize formats a byte size into a human-readable string.
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
