package filesink_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sentiolabs/envctl/internal/filesink"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEphemeral_ModeAndLocation(t *testing.T) {
	d, err := filesink.NewEphemeral()
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })

	info, err := os.Stat(d.Path())
	require.NoError(t, err)
	assert.True(t, info.IsDir())
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())
	assert.True(t, strings.HasPrefix(d.Path(), os.TempDir()), "run dir must live under os.TempDir()")
}

func TestWriteEphemeral_ModeAndContent(t *testing.T) {
	d, err := filesink.NewEphemeral()
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })

	content := []byte("-----BEGIN PRIVATE KEY-----\nabc\n")
	p, err := d.WriteEphemeral("sp.key", content, 0o600)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(d.Path(), "sp.key"), p)

	got, err := os.ReadFile(p)
	require.NoError(t, err)
	assert.Equal(t, content, got)

	info, err := os.Stat(p)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	entries, err := os.ReadDir(d.Path())
	require.NoError(t, err)
	assert.Len(t, entries, 1, "no temp file may remain after a successful write")
}

func TestWriteEphemeral_CustomMode(t *testing.T) {
	d, err := filesink.NewEphemeral()
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })

	p, err := d.WriteEphemeral("sp.crt", []byte("cert"), 0o644)
	require.NoError(t, err)
	info, err := os.Stat(p)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o644), info.Mode().Perm())
}

func TestWriteEphemeral_ReplacesExisting(t *testing.T) {
	d, err := filesink.NewEphemeral()
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })

	_, err = d.WriteEphemeral("f", []byte("old"), 0o600)
	require.NoError(t, err)
	p, err := d.WriteEphemeral("f", []byte("new"), 0o600)
	require.NoError(t, err)
	got, err := os.ReadFile(p)
	require.NoError(t, err)
	assert.Equal(t, "new", string(got))
}

func TestWriteEphemeral_RejectsPathTraversal(t *testing.T) {
	d, err := filesink.NewEphemeral()
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })

	for _, bad := range []string{"", ".", "..", "a/b", `a\b`, "../escape"} {
		_, err := d.WriteEphemeral(bad, []byte("x"), 0o600)
		assert.Error(t, err, "name %q must be rejected", bad)
	}
}

func TestWriteEphemeral_SymlinkInDirIsNotFollowed(t *testing.T) {
	d, err := filesink.NewEphemeral()
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })

	outside := filepath.Join(t.TempDir(), "victim")
	require.NoError(t, os.WriteFile(outside, []byte("keep"), 0o600))
	require.NoError(t, os.Symlink(outside, filepath.Join(d.Path(), "link")))

	_, err = d.WriteEphemeral("link", []byte("overwritten"), 0o600)
	require.NoError(t, err, "rename replaces the symlink itself")

	victim, err := os.ReadFile(outside)
	require.NoError(t, err)
	assert.Equal(t, "keep", string(victim), "the symlink target must be untouched")

	info, err := os.Lstat(filepath.Join(d.Path(), "link"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0), info.Mode()&os.ModeSymlink, "link is now a regular file")
}

func TestClose_RemovesEverything(t *testing.T) {
	d, err := filesink.NewEphemeral()
	require.NoError(t, err)
	_, err = d.WriteEphemeral("a", []byte("a"), 0o600)
	require.NoError(t, err)
	_, err = d.WriteEphemeral("b", []byte("b"), 0o600)
	require.NoError(t, err)

	require.NoError(t, d.Close())
	_, err = os.Stat(d.Path())
	assert.True(t, os.IsNotExist(err))
	require.NoError(t, d.Close(), "second Close is a no-op")
}

func TestExpand(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	t.Setenv("ENVCTL_TEST_DIR", "/opt/state")
	cfgDir := t.TempDir()

	tests := map[string]string{
		"~/x/y":                filepath.Join(home, "x", "y"),
		"${ENVCTL_TEST_DIR}/k": "/opt/state/k",
		"$ENVCTL_TEST_DIR/k":   "/opt/state/k",
		"rel/file":             filepath.Join(cfgDir, "rel", "file"),
		"/abs/./file":          "/abs/file",
	}
	for in, want := range tests {
		got, err := filesink.Expand(in, cfgDir)
		require.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}
	_, err = filesink.Expand("", cfgDir)
	assert.Error(t, err)
}

func TestWritePersistent_CreatesParentAndWrites(t *testing.T) {
	base := t.TempDir() // outside any git worktree
	target := filepath.Join(base, "nested", "deeper", "kubeconfig")

	p, err := filesink.WritePersistent(target, base, []byte("cfg"), 0o600)
	require.NoError(t, err)
	assert.Equal(t, target, p)

	got, err := os.ReadFile(p)
	require.NoError(t, err)
	assert.Equal(t, "cfg", string(got))

	parent, err := os.Stat(filepath.Dir(target))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), parent.Mode().Perm())

	info, err := os.Stat(p)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestWritePersistent_RelativeResolvesAgainstConfigDir(t *testing.T) {
	cfgDir := t.TempDir()
	p, err := filesink.WritePersistent("state/f", cfgDir, []byte("x"), 0o600)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(cfgDir, "state", "f"), p)
}

func TestWriteEphemeral_FailedRenameLeavesNoTempFile(t *testing.T) {
	d, err := filesink.NewEphemeral()
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })

	target := filepath.Join(d.Path(), "target")
	require.NoError(t, os.Mkdir(target, 0o700))
	victim := filepath.Join(target, "victim")
	require.NoError(t, os.WriteFile(victim, []byte("keep"), 0o600))

	_, err = d.WriteEphemeral("target", []byte("x"), 0o600)
	require.Error(t, err, "rename over a non-empty directory must fail")

	entries, err := os.ReadDir(d.Path())
	require.NoError(t, err)
	require.Len(t, entries, 1, "only the pre-existing target directory may remain")
	assert.Equal(t, "target", entries[0].Name())
	for _, e := range entries {
		assert.False(t, strings.HasPrefix(e.Name(), ".target.tmp-"), "no leftover temp file")
	}

	got, err := os.ReadFile(victim)
	require.NoError(t, err)
	assert.Equal(t, "keep", string(got), "the pre-existing file must be untouched")
}

func TestWritePersistent_FailedRenameLeavesNoTempFile(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "target")
	require.NoError(t, os.Mkdir(target, 0o700))
	victim := filepath.Join(target, "victim")
	require.NoError(t, os.WriteFile(victim, []byte("keep"), 0o600))

	_, err := filesink.WritePersistent(target, base, []byte("x"), 0o600)
	require.Error(t, err, "rename over a non-empty directory must fail")

	entries, err := os.ReadDir(base)
	require.NoError(t, err)
	require.Len(t, entries, 1, "only the pre-existing target directory may remain")
	assert.Equal(t, "target", entries[0].Name())
	for _, e := range entries {
		assert.False(t, strings.HasPrefix(e.Name(), ".target.tmp-"), "no leftover temp file")
	}

	got, err := os.ReadFile(victim)
	require.NoError(t, err)
	assert.Equal(t, "keep", string(got), "the pre-existing file must be untouched")
}
