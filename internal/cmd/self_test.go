//nolint:testpackage // Testing internal functions requires same package
package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sentiolabs/go-selfupdate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// selfCmdName is the command group under test.
	selfCmdName = "self"
	// updateCmdName is the subcommand that installs a new release.
	updateCmdName = "update"
	// tagNameKey and prereleaseKey are the GitHub release JSON fields the
	// httptest server returns.
	tagNameKey    = "tag_name"
	prereleaseKey = "prerelease"
)

func TestSelfUpdaterWiring(t *testing.T) {
	u := newSelfUpdater()
	assert.Equal(t, binaryName, u.Name)
	src, ok := u.Source.(*selfupdate.GitHubSource)
	require.True(t, ok)
	assert.Equal(t, repoOwner, src.Owner)
	assert.Equal(t, binaryName, src.Repo)
	inst, ok := u.Installer.(*selfupdate.ScriptInstaller)
	require.True(t, ok)
	assert.Equal(t, installScriptURL, inst.ScriptURL)
	assert.NotNil(t, u.Store)
}

func TestSelfCommandTree(t *testing.T) {
	self, _, err := rootCmd.Find([]string{selfCmdName})
	require.NoError(t, err)
	update, _, err := rootCmd.Find([]string{selfCmdName, "update"})
	require.NoError(t, err)
	channel, _, err := rootCmd.Find([]string{selfCmdName, "channel"})
	require.NoError(t, err)
	assert.Equal(t, selfCmdName, self.Name())

	check := update.Flags().Lookup("check")
	require.NotNil(t, check)
	assert.Empty(t, check.Shorthand, "-c belongs to --config on the root command")
	assert.Equal(t, "f", update.Flags().Lookup("force").Shorthand)
	assert.Equal(t, "y", update.Flags().Lookup("yes").Shorthand)
	assert.Equal(t, "y", channel.Flags().Lookup("yes").Shorthand)
}

// withTestReleases points the shared updater at an httptest GitHub and an
// in-memory store for the duration of the test.
func withTestReleases(t *testing.T, channel selfupdate.Channel, latest string, list []map[string]any) *bytes.Buffer {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/sentiolabs/envctl/releases/latest":
			_ = json.NewEncoder(w).Encode(map[string]any{tagNameKey: latest})
		case "/repos/sentiolabs/envctl/releases":
			_ = json.NewEncoder(w).Encode(list)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	prevSource, prevStore := selfUpdater.Source, selfUpdater.Store
	prevOut, prevErr := selfUpdater.Out, selfUpdater.ErrOut
	t.Cleanup(func() {
		selfUpdater.Source, selfUpdater.Store = prevSource, prevStore
		selfUpdater.Out, selfUpdater.ErrOut = prevOut, prevErr
	})
	out := &bytes.Buffer{}
	selfUpdater.Source = &selfupdate.GitHubSource{Owner: repoOwner, Repo: binaryName, BaseURL: srv.URL}
	selfUpdater.Store = &selfupdate.MemStore{Current: channel}
	selfUpdater.Out = out
	selfUpdater.ErrOut = out
	return out
}

func execRoot(t *testing.T, args ...string) error {
	t.Helper()
	rootCmd.SetArgs(args)
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	return rootCmd.Execute()
}

func TestSelfUpdateCheck_StableChannel(t *testing.T) {
	out := withTestReleases(t, selfupdate.ChannelStable, "v9.0.0", nil)
	require.NoError(t, execRoot(t, selfCmdName, "update", "--check"))
	assert.Contains(t, out.String(), "Update available: v0.0.0-dev -> v9.0.0 (stable channel)")
}

func TestSelfUpdateCheck_RCChannelPrefersRC(t *testing.T) {
	list := []map[string]any{
		{tagNameKey: "v9.1.0-rc.1", prereleaseKey: true},
		{tagNameKey: "v9.0.0", prereleaseKey: false},
	}
	out := withTestReleases(t, selfupdate.ChannelRC, "v9.0.0", list)
	require.NoError(t, execRoot(t, selfCmdName, "update", "--check"))
	assert.Contains(t, out.String(), "v9.1.0-rc.1 (rc channel)")
}

func TestSelfUpdateCheck_RCChannelStableNewerWins(t *testing.T) {
	list := []map[string]any{
		{tagNameKey: "v9.2.0", prereleaseKey: false},
		{tagNameKey: "v9.1.0-rc.1", prereleaseKey: true},
	}
	out := withTestReleases(t, selfupdate.ChannelRC, "v9.2.0", list)
	require.NoError(t, execRoot(t, selfCmdName, "update", "--check"))
	assert.Contains(t, out.String(), "-> v9.2.0 (rc channel)")
}

func TestSelfChannelShowAndSwitch(t *testing.T) {
	out := withTestReleases(t, selfupdate.ChannelStable, "v1.0.0", nil)
	require.NoError(t, execRoot(t, selfCmdName, "channel"))
	assert.Contains(t, out.String(), "Current update channel: stable")

	out.Reset()
	require.NoError(t, execRoot(t, selfCmdName, "channel", "rc", "-y"))
	assert.Contains(t, out.String(), "Switched to rc channel")
	store := selfUpdater.Store.(*selfupdate.MemStore)
	assert.Equal(t, selfupdate.ChannelRC, store.Current)

	err := execRoot(t, selfCmdName, "channel", "bogus", "-y")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid channel")
}

// blockingInstaller stands in for ScriptInstaller. It reports that it
// started and then returns only once its context is done, so an installer
// handed a context with a nil Done channel would block forever.
type blockingInstaller struct{ started chan struct{} }

func (b *blockingInstaller) Install(ctx context.Context, _ string) error {
	close(b.started)
	<-ctx.Done()
	return ctx.Err()
}

// TestSelfUpdateCancelsWithRootContext checks that the command context
// reaches the installer. Cancelling it must end self update instead of
// leaving it waiting on the install.
func TestSelfUpdateCancelsWithRootContext(t *testing.T) {
	withTestReleases(t, selfupdate.ChannelStable, "v9.0.0", nil)

	inst := &blockingInstaller{started: make(chan struct{})}
	prevInstaller := selfUpdater.Installer
	t.Cleanup(func() { selfUpdater.Installer = prevInstaller })
	selfUpdater.Installer = inst

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go func() {
		select {
		case <-inst.started:
			cancel()
		case <-ctx.Done():
		}
	}()

	// The context goes on the update command itself: cobra copies the root
	// context down only while the subcommand's own context is nil, and the
	// --check tests above already stamped context.Background() onto this one.
	// --check=false is explicit for the same reason, because cobra keeps the
	// value an earlier Execute parsed into the flag variable.
	update, _, err := rootCmd.Find([]string{selfCmdName, updateCmdName})
	require.NoError(t, err)
	update.SetContext(ctx)
	rootCmd.SetArgs([]string{selfCmdName, updateCmdName, "-y", "--check=false"})
	t.Cleanup(func() {
		rootCmd.SetArgs(nil)
		update.SetContext(context.Background())
		rootCmd.SetContext(context.Background())
	})

	done := make(chan error, 1)
	go func() { done <- rootCmd.Execute() }()

	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("self update did not return after its context was cancelled")
	}
}
