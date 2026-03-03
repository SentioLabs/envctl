//nolint:testpackage // Testing internal functions requires same package
package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sentiolabs/envctl/internal/version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnsureVPrefix(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string becomes dev version",
			input:    "",
			expected: "v0.0.0-dev",
		},
		{
			name:     "dev becomes dev version",
			input:    "dev",
			expected: "v0.0.0-dev",
		},
		{
			name:     "version without v prefix gets v added",
			input:    "0.1.0",
			expected: "v0.1.0",
		},
		{
			name:     "version with v prefix unchanged",
			input:    "v0.1.0",
			expected: "v0.1.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ensureVPrefix(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetLatestVersion(t *testing.T) {
	t.Run("success returns tag_name", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/latest", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			err := json.NewEncoder(w).Encode(githubRelease{TagName: "v1.2.3"})
			assert.NoError(t, err)
		}))
		defer server.Close()

		original := githubReleasesURL
		githubReleasesURL = server.URL
		defer func() { githubReleasesURL = original }()

		ver, err := getLatestVersion()
		require.NoError(t, err)
		assert.Equal(t, "v1.2.3", ver)
	})

	t.Run("non-200 status returns error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		original := githubReleasesURL
		githubReleasesURL = server.URL
		defer func() { githubReleasesURL = original }()

		_, err := getLatestVersion()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected status")
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, err := w.Write([]byte("not json"))
			assert.NoError(t, err)
		}))
		defer server.Close()

		original := githubReleasesURL
		githubReleasesURL = server.URL
		defer func() { githubReleasesURL = original }()

		_, err := getLatestVersion()
		require.Error(t, err)
	})
}

// setupCheckModeTest creates a mock GitHub server and configures the test environment
// for a self-update check mode test case. It returns the output buffer.
func setupCheckModeTest(t *testing.T, latestTag, currentVersion string) *bytes.Buffer {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(githubRelease{TagName: latestTag})
		assert.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	original := githubReleasesURL
	githubReleasesURL = server.URL
	t.Cleanup(func() { githubReleasesURL = original })

	origVersion := version.Version
	version.Version = currentVersion
	t.Cleanup(func() { version.Version = origVersion })

	buf := new(bytes.Buffer)
	selfUpdateCmd.SetOut(buf)
	selfUpdateCmd.SetErr(buf)

	err := selfUpdateCmd.Flags().Set("check", "true")
	require.NoError(t, err)
	t.Cleanup(func() {
		err := selfUpdateCmd.Flags().Set("check", "false")
		if err != nil {
			t.Logf("failed to reset flag: %v", err)
		}
	})

	return buf
}

func TestSelfUpdateCheckMode_UpToDate(t *testing.T) {
	buf := setupCheckModeTest(t, "v1.0.0", "v1.0.0")

	err := selfUpdateCmd.RunE(selfUpdateCmd, []string{})
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "up to date")
}

func TestSelfUpdateCheckMode_UpdateAvailable(t *testing.T) {
	buf := setupCheckModeTest(t, "v2.0.0", "v1.0.0")

	err := selfUpdateCmd.RunE(selfUpdateCmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "v2.0.0")
	assert.Contains(t, output, "available")
}

func TestSelfUpdateCheckMode_CurrentIsNewer(t *testing.T) {
	buf := setupCheckModeTest(t, "v1.0.0", "v2.0.0")

	err := selfUpdateCmd.RunE(selfUpdateCmd, []string{})
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "newer than latest")
}
