//nolint:testpackage // Testing internal functions requires same package
package cmd

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/sentiolabs/envctl/internal/mocks"
	"github.com/sentiolabs/envctl/internal/output"
	"github.com/sentiolabs/envctl/internal/secrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Two applications that both include "shared", the way bacstack's api and
// web items both pull a shared 1Password item. include_all lets each source
// contribute every key.
const appsConfig = `version: 1
include_all: true
default_application: api
default_environment: dev
applications:
  api:
    dev:
      - secret: api/dev
      - secret: shared
      - secret: core
        keys:
          - key: TENANT
  web:
    dev:
      - secret: web/dev
      - secret: shared
      - secret: core
        keys:
          - key: TENANT
`

// Applications and keys the --apps tests share.
const (
	appAPI    = "api"
	appWeb    = "web"
	keyRole   = "ROLE"
	keyShared = "SHARED"
)

// setupAppsExport points the CLI at appsConfig and installs client as every
// backend, restoring the package globals on cleanup.
func setupAppsExport(t *testing.T, client secrets.Client) {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".envctl.yaml")
	require.NoError(t, os.WriteFile(path, []byte(appsConfig), 0o600))

	prevConfig, prevEnv, prevApp := configFile, envName, appName
	prevFormat, prevApps, prevFactory := exportFormat, exportApps, newSecretsClient
	t.Cleanup(func() {
		configFile, envName, appName = prevConfig, prevEnv, prevApp
		exportFormat, exportApps, newSecretsClient = prevFormat, prevApps, prevFactory
	})
	configFile, envName, appName = path, testDevEnv, ""
	exportFormat = string(output.FormatJSON)
	newSecretsClient = func(context.Context, secrets.Options) (secrets.Client, error) {
		return client, nil
	}
}

func TestExportApps_ReadsSharedSecretsOnce(t *testing.T) {
	client := mocks.NewMockClient(t)
	client.On("Name").Return("mock").Maybe()
	// .Once() fails the test if a second application reaches the backend
	// again for a secret the first one already read.
	client.On("GetSecret", mock.Anything, "api/dev").Return(map[string]string{keyRole: appAPI}, nil).Once()
	client.On("GetSecret", mock.Anything, "web/dev").Return(map[string]string{keyRole: appWeb}, nil).Once()
	client.On("GetSecret", mock.Anything, "shared").Return(map[string]string{keyShared: "s"}, nil).Once()
	client.On("GetSecretKey", mock.Anything, "core", "TENANT").Return("t1", nil).Once()
	setupAppsExport(t, client)
	exportApps = []string{appWeb, appAPI}

	stdout, err := captureStdout(t, func() error { return runExport(exportCmd, nil) })
	require.NoError(t, err)

	var got map[string]map[string]string
	require.NoError(t, json.Unmarshal([]byte(stdout), &got), stdout)
	assert.Equal(t, map[string]map[string]string{
		appAPI: {keyRole: appAPI, keyShared: "s", "TENANT": "t1"},
		appWeb: {keyRole: appWeb, keyShared: "s", "TENANT": "t1"},
	}, got)
}

func TestExportApps_NeedsJSON(t *testing.T) {
	setupAppsExport(t, mocks.NewMockClient(t))
	exportApps = []string{appAPI}
	exportFormat = string(output.FormatShell)

	err := runExport(exportCmd, nil)
	require.ErrorContains(t, err, "--apps needs --format json")
}

func TestExportApps_RejectsApp(t *testing.T) {
	setupAppsExport(t, mocks.NewMockClient(t))
	exportApps = []string{appAPI}
	appName = appWeb

	err := runExport(exportCmd, nil)
	require.ErrorContains(t, err, "--apps and --app can't be combined")
}

func TestExportApps_RejectsEmptyName(t *testing.T) {
	setupAppsExport(t, mocks.NewMockClient(t))
	exportApps = []string{appAPI, "", appWeb}

	err := runExport(exportCmd, nil)
	require.ErrorContains(t, err, "--apps has an empty application name")
}

func TestExportApps_UnknownAppFailsWholeExport(t *testing.T) {
	client := mocks.NewMockClient(t)
	client.On("Name").Return("mock").Maybe()
	client.On("GetSecret", mock.Anything, mock.Anything).Return(map[string]string{"K": "v"}, nil).Maybe()
	client.On("GetSecretKey", mock.Anything, mock.Anything, mock.Anything).Return("v", nil).Maybe()
	setupAppsExport(t, client)
	exportApps = []string{appAPI, "missing"}

	stdout, err := captureStdout(t, func() error { return runExport(exportCmd, nil) })
	require.ErrorContains(t, err, "application missing")
	assert.Empty(t, stdout, "a failed export must not print a partial object")
}
