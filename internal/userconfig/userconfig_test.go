package userconfig_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sentiolabs/envctl/internal/userconfig"
	"github.com/sentiolabs/go-selfupdate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPath_XDGOverride(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-test")
	p, err := userconfig.Path()
	require.NoError(t, err)
	assert.Equal(t, "/tmp/xdg-test/envctl/config.yaml", p)
}

func TestPath_DefaultUnderHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	p, err := userconfig.Path()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, ".config", "envctl", "config.yaml"), p)
}

func TestLoad_MissingFileIsEmptyConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg, err := userconfig.Load()
	require.NoError(t, err)
	assert.Empty(t, cfg.Updates.Channel)
}

func TestSaveLoadRoundTripAndModes(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	require.NoError(t, userconfig.Save(&userconfig.Config{Updates: userconfig.Updates{Channel: "rc"}}))

	p, _ := userconfig.Path()
	info, err := os.Stat(p)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	dir, err := os.Stat(filepath.Dir(p))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), dir.Mode().Perm())

	raw, err := os.ReadFile(p)
	require.NoError(t, err)
	assert.Contains(t, string(raw), "channel: rc")

	cfg, err := userconfig.Load()
	require.NoError(t, err)
	assert.Equal(t, "rc", cfg.Updates.Channel)

	entries, err := os.ReadDir(filepath.Dir(p))
	require.NoError(t, err)
	assert.Len(t, entries, 1, "no temp file may remain")
}

func TestLoad_InvalidYAMLIsError(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	dir := filepath.Join(xdg, "envctl")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("updates: [\n"), 0o600))
	_, err := userconfig.Load()
	require.Error(t, err)
}

func TestChannelStore(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	store := userconfig.ChannelStore()

	ch, err := store.Channel()
	require.NoError(t, err)
	assert.Equal(t, selfupdate.Channel(""), ch, "fresh machine has no stored channel")

	require.NoError(t, store.SetChannel(selfupdate.ChannelNightly))
	ch, err = store.Channel()
	require.NoError(t, err)
	assert.Equal(t, selfupdate.ChannelNightly, ch)

	cfg, err := userconfig.Load()
	require.NoError(t, err)
	assert.Equal(t, "nightly", cfg.Updates.Channel)
}
