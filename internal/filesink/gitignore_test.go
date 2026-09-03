package filesink_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/sentiolabs/envctl/internal/filesink"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// initRepo creates a git repo in a temp dir and returns its path.
func initRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	cmd := exec.Command("git", "-C", dir, "init", "-q")
	require.NoError(t, cmd.Run())
	return dir
}

func TestCheckPath_OutsideWorktree(t *testing.T) {
	dir := t.TempDir()
	assert.NoError(t, filesink.CheckPath(filepath.Join(dir, "f")))
}

func TestCheckPath_UnignoredInsideWorktree(t *testing.T) {
	repo := initRepo(t)
	err := filesink.CheckPath(filepath.Join(repo, "certs", "sp.key"))
	assert.ErrorIs(t, err, filesink.ErrNotIgnored)
}

func TestCheckPath_IgnoredInsideWorktree(t *testing.T) {
	repo := initRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(repo, ".gitignore"), []byte("certs/\n"), 0o600))
	assert.NoError(t, filesink.CheckPath(filepath.Join(repo, "certs", "sp.key")))
}

func TestCheckPath_TargetParentDoesNotExistYet(t *testing.T) {
	repo := initRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(repo, ".gitignore"), []byte("state/\n"), 0o600))
	// state/ and state/deep/ do not exist; CheckPath must still answer.
	assert.NoError(t, filesink.CheckPath(filepath.Join(repo, "state", "deep", "f")))
	assert.ErrorIs(t, filesink.CheckPath(filepath.Join(repo, "other", "deep", "f")), filesink.ErrNotIgnored)
}

func TestWritePersistent_RefusesUnignoredRepoPath(t *testing.T) {
	repo := initRepo(t)
	target := filepath.Join(repo, "certs", "sp.key")

	_, err := filesink.WritePersistent(target, repo, []byte("key"), 0o600)
	require.ErrorIs(t, err, filesink.ErrNotIgnored)
	_, statErr := os.Stat(target)
	assert.True(t, os.IsNotExist(statErr), "nothing may be written when refused")

	require.NoError(t, os.WriteFile(filepath.Join(repo, ".gitignore"), []byte("certs/\n"), 0o600))
	p, err := filesink.WritePersistent(target, repo, []byte("key"), 0o600)
	require.NoError(t, err)
	assert.Equal(t, target, p)
}
