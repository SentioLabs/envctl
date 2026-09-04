//nolint:testpackage // Testing internal functions requires same package
package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sentiolabs/envctl/internal/config"
	"github.com/sentiolabs/envctl/internal/env"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// keyFileVar is the path_as variable these tests attach to an ephemeral sink.
const keyFileVar = "KEY_FILE"

func TestMergeEntries_PathBeatsSecretButNotOverride(t *testing.T) {
	base := []env.Entry{
		{Key: "A", Value: "1", Source: "s"},
		{Key: keyFileVar, Value: "from-secret", Source: "s"},
		{Key: "DIR", Value: "custom", Source: overrideSource},
	}
	extra := []env.Entry{
		{Key: keyFileVar, Value: "/run/sp.key", Source: "file:x"},
		{Key: "DIR", Value: "/run", Source: "file"},
		{Key: "NEW", Value: "/run/new", Source: "file:y"},
	}
	got := env.ToMap(mergeEntries(base, extra))
	assert.Equal(t, "1", got["A"])
	assert.Equal(t, "/run/sp.key", got[keyFileVar])
	assert.Equal(t, "custom", got["DIR"], "--set override must win")
	assert.Equal(t, "/run/new", got["NEW"])
}

func TestMaterializeFiles_RunModeWritesEphemeralAndExportsDir(t *testing.T) {
	files := []env.ResolvedFile{
		{Sink: config.FileSink{Name: "sp.key", PathAs: keyFileVar}, Secret: "s/key", Content: []byte("k")},
		{
			Sink:    config.FileSink{Name: "sp.crt", Mode: "0644", PathAs: "CERT_FILE"},
			Secret:  "s/crt",
			Content: []byte("c"),
		},
	}
	m, err := materializeFiles(files, "FILES_DIR", t.TempDir(), sinkModeRun)
	require.NoError(t, err)
	t.Cleanup(m.Close)

	got := env.ToMap(m.entries)
	require.NotNil(t, m.dir)
	assert.Equal(t, filepath.Join(m.dir.Path(), "sp.key"), got[keyFileVar])
	assert.Equal(t, m.dir.Path(), got["FILES_DIR"])

	info, err := os.Stat(got[keyFileVar])
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	info, err = os.Stat(got["CERT_FILE"])
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o644), info.Mode().Perm())

	assert.Equal(t, []byte{0}, files[0].Content, "content buffer must be zeroed")

	m.Close()
	_, err = os.Stat(m.dir.Path())
	assert.True(t, os.IsNotExist(err))
}

func TestMaterializeFiles_PersistentOnlySkipsEphemeral(t *testing.T) {
	base := t.TempDir()
	files := []env.ResolvedFile{
		{Sink: config.FileSink{Name: "sp.key", PathAs: keyFileVar}, Secret: "s/key", Content: []byte("k")},
		{
			Sink:    config.FileSink{Path: filepath.Join(base, "st", "kube"), PathAs: "KUBECONFIG"},
			Secret:  "s/kube",
			Content: []byte("y"),
		},
	}
	m, err := materializeFiles(files, "FILES_DIR", base, sinkModePersistentOnly)
	require.NoError(t, err)
	t.Cleanup(m.Close)

	got := env.ToMap(m.entries)
	assert.Nil(t, m.dir, "no run directory in persistent-only mode")
	_, hasKey := got[keyFileVar]
	assert.False(t, hasKey, "ephemeral sink must be dropped")
	_, hasDir := got["FILES_DIR"]
	assert.False(t, hasDir, "files_dir_as only accompanies an ephemeral directory")
	assert.Equal(t, filepath.Join(base, "st", "kube"), got["KUBECONFIG"])
	_, err = os.Stat(got["KUBECONFIG"])
	require.NoError(t, err)
}

func TestMaterializeFiles_BadModeCleansUp(t *testing.T) {
	files := []env.ResolvedFile{
		{Sink: config.FileSink{Name: "a", PathAs: "A"}, Secret: "s/a", Content: []byte("a")},
		{Sink: config.FileSink{Name: "b", Mode: "zz", PathAs: "B"}, Secret: "s/b", Content: []byte("b")},
	}
	_, err := materializeFiles(files, "", t.TempDir(), sinkModeRun)
	require.Error(t, err)
	assert.Equal(t, []byte{0}, files[0].Content, "good file's buffer must be zeroed on a later failure")
	assert.Equal(t, []byte{0}, files[1].Content, "bad-mode file's buffer must be zeroed")
}
