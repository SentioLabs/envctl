//nolint:testpackage // Exercises envctl's command wiring with a real native installer.
package cmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/sentiolabs/selfupdate-go"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const archiveTestTag = "v9.0.0"

// testArchive builds the same single-binary tarball shape as GoReleaser.
func testArchive(t *testing.T, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: binaryName, Mode: 0o755, Size: int64(len(content))}))
	_, err := tw.Write(content)
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

// archiveFixture serves GitHub metadata and assets entirely over localhost.
type archiveFixture struct {
	archive       []byte
	badChecksum   bool
	missingAsset  bool
	blockDownload bool
	downloaded    chan struct{}
}

func (f *archiveFixture) server(t *testing.T) *httptest.Server {
	t.Helper()
	asset := fmt.Sprintf("envctl_9.0.0_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	var baseURL string
	handler := func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/sentiolabs/envctl/releases/latest":
			assets := []map[string]any{
				{"name": asset, "size": len(f.archive), "browser_download_url": baseURL + "/archive"},
				{"name": "checksums.txt", "browser_download_url": baseURL + "/checksums"},
			}
			if f.missingAsset {
				assets = assets[1:]
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"tag_name": archiveTestTag, "assets": assets})
		case "/archive":
			if f.blockDownload {
				w.WriteHeader(http.StatusOK)
				w.(http.Flusher).Flush()
				close(f.downloaded)
				<-r.Context().Done()
				return
			}
			_, _ = w.Write(f.archive)
		case "/checksums":
			sum := sha256.Sum256(f.archive)
			if f.badChecksum {
				sum[0] ^= 0xff
			}
			_, _ = fmt.Fprintf(w, "%x  %s\n", sum, asset)
		default:
			http.NotFound(w, r)
		}
	}
	srv := httptest.NewServer(http.HandlerFunc(handler))
	baseURL = srv.URL
	t.Cleanup(srv.Close)
	return srv
}

func archiveCommand(t *testing.T, target string, fixture *archiveFixture) (*cobra.Command, *bytes.Buffer) {
	t.Helper()
	srv := fixture.server(t)
	updater := newSelfUpdater()
	updater.Version = "v1.0.0"
	updater.Store = &selfupdate.MemStore{}
	updater.Source = &selfupdate.GitHubSource{Owner: repoOwner, Repo: binaryName, BaseURL: srv.URL}
	updater.Installer = &selfupdate.ArchiveInstaller{Name: binaryName, TargetPath: target}
	command := newSelfCommand(updater)
	output := &bytes.Buffer{}
	command.SetOut(output)
	command.SetErr(output)
	return command, output
}

func TestNativeSelfUpdate(t *testing.T) {
	for _, tc := range []struct {
		name                                      string
		badChecksum, missingAsset, check, decline bool
	}{
		{name: "success"},
		{name: "checksum", badChecksum: true},
		{name: "missing", missingAsset: true},
		{name: "check", check: true},
		{name: "decline", decline: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), binaryName)
			original := []byte("original executable")
			replacement := []byte("replacement executable")
			writeTestExecutable(t, target, original)
			fixture := &archiveFixture{
				archive: testArchive(t, replacement), badChecksum: tc.badChecksum, missingAsset: tc.missingAsset,
			}
			command, output := archiveCommand(t, target, fixture)
			args := []string{updateCmdName, "-y"}
			if tc.check {
				args = []string{updateCmdName, "--check"}
			}
			if tc.decline {
				args = []string{updateCmdName}
				command.SetIn(bytes.NewBufferString("n\n"))
			}
			command.SetArgs(args)
			err := command.ExecuteContext(t.Context())
			if tc.badChecksum || tc.missingAsset {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			got, err := os.ReadFile(target)
			require.NoError(t, err)
			if tc.name == "success" {
				assert.Equal(t, replacement, got)
				assert.Contains(t, output.String(), "Downloading")
				assert.Contains(t, output.String(), "Verified")
				assert.Contains(t, output.String(), "Installed")
			} else {
				assert.Equal(t, original, got)
			}
		})
	}
}

func writeTestExecutable(t *testing.T, target string, content []byte) {
	t.Helper()
	require.NoError(t, os.WriteFile(target, content, 0o600)) //nolint:gosec // target is inside t.TempDir()
	require.NoError(t, os.Chmod(target, 0o755))
}

func TestNativeUpdateThroughSymlink(t *testing.T) {
	target := filepath.Join(t.TempDir(), binaryName)
	writeTestExecutable(t, target, []byte("original"))
	link := filepath.Join(t.TempDir(), binaryName)
	require.NoError(t, os.Symlink(target, link))
	command, _ := archiveCommand(t, link, &archiveFixture{archive: testArchive(t, []byte("replacement"))})
	command.SetArgs([]string{updateCmdName, "-y"})
	require.NoError(t, command.ExecuteContext(t.Context()))
	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "replacement", string(got))
	info, err := os.Stat(target)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o755), info.Mode().Perm())
	info, err = os.Lstat(link)
	require.NoError(t, err)
	assert.NotZero(t, info.Mode()&os.ModeSymlink)
}

func TestNativeDownloadCancellation(t *testing.T) {
	target := filepath.Join(t.TempDir(), binaryName)
	writeTestExecutable(t, target, []byte("original"))
	fixture := &archiveFixture{
		archive: testArchive(t, []byte("replacement")), blockDownload: true, downloaded: make(chan struct{}),
	}
	command, _ := archiveCommand(t, target, fixture)
	command.SetArgs([]string{updateCmdName, "-y"})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go func() {
		select {
		case <-fixture.downloaded:
			cancel()
		case <-ctx.Done():
		}
	}()
	require.ErrorIs(t, command.ExecuteContext(ctx), context.Canceled)
	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "original", string(got))
	_, err = os.Stat(target + ".new")
	assert.True(t, os.IsNotExist(err))
}

// lifecycleInstaller records the integration contract of the new staged API.
type lifecycleInstaller struct {
	calls     []string
	commitErr error
}

func (i *lifecycleInstaller) Prepare(_ context.Context, _ selfupdate.Release) (selfupdate.Staged, error) {
	i.calls = append(i.calls, "prepare")
	return i, nil
}

func (i *lifecycleInstaller) Commit(_ context.Context) error {
	i.calls = append(i.calls, "commit")
	return i.commitErr
}

func (i *lifecycleInstaller) Close() error {
	i.calls = append(i.calls, "close")
	return nil
}

func TestSelfUpdateClosesStagedInstaller(t *testing.T) {
	for _, commitErr := range []error{nil, context.Canceled} {
		t.Run(fmt.Sprint(commitErr), func(t *testing.T) {
			fixture := &archiveFixture{}
			srv := fixture.server(t)
			u := newSelfUpdater()
			u.Source = &selfupdate.GitHubSource{Owner: repoOwner, Repo: binaryName, BaseURL: srv.URL}
			u.Store = &selfupdate.MemStore{}
			installer := &lifecycleInstaller{commitErr: commitErr}
			u.Installer = installer
			cmd := newSelfCommand(u)
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetArgs([]string{updateCmdName, "-y"})
			err := cmd.ExecuteContext(t.Context())
			require.ErrorIs(t, err, commitErr)
			assert.Equal(t, []string{"prepare", "commit", "close"}, installer.calls)
		})
	}
}

const nativeUpdateChildEnv = "ENVCTL_NATIVE_UPDATE_CHILD"

// TestNativeUpdateChild runs in a disposable copy of this test executable.
func TestNativeUpdateChild(t *testing.T) {
	if os.Getenv(nativeUpdateChildEnv) == "" {
		return
	}
	u := newSelfUpdater()
	u.Source = &selfupdate.GitHubSource{
		Owner: repoOwner, Repo: binaryName, BaseURL: os.Getenv(nativeUpdateChildEnv),
	}
	u.Store = &selfupdate.MemStore{}
	cmd := newSelfCommand(u)
	cmd.SetArgs([]string{updateCmdName, "-y"})
	require.NoError(t, cmd.ExecuteContext(t.Context()))
}

func TestNativeUpdateRunningExecutable(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a replacement envctl binary")
	}
	dir := t.TempDir()
	replacement := filepath.Join(dir, "new-envctl")
	build := exec.CommandContext(t.Context(), "go", "build", "-o", replacement, "-ldflags",
		"-X github.com/sentiolabs/envctl/internal/version.Version="+archiveTestTag, "../../cmd/envctl")
	output, err := build.CombinedOutput()
	require.NoError(t, err, "%s", output)
	payload, err := os.ReadFile(replacement)
	require.NoError(t, err)
	fixture := &archiveFixture{archive: testArchive(t, payload)}
	srv := fixture.server(t)
	executable, err := os.Executable()
	require.NoError(t, err)
	old, err := os.ReadFile(executable)
	require.NoError(t, err)
	target := filepath.Join(dir, binaryName)
	writeTestExecutable(t, target, old)
	child := exec.CommandContext(t.Context(), target, "-test.run=^TestNativeUpdateChild$")
	child.Env = append(os.Environ(), nativeUpdateChildEnv+"="+srv.URL)
	output, err = child.CombinedOutput()
	require.NoError(t, err, "%s", output)
	// The same path now runs the released CLI, rather than the test harness.
	updated := exec.CommandContext(t.Context(), target, "version")
	output, err = updated.CombinedOutput()
	require.NoError(t, err, "%s", output)
	assert.Contains(t, string(output), archiveTestTag)
}
