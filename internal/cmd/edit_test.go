//nolint:testpackage // Testing internal functions requires same package
package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEditCmdExists(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "edit" {
			found = true
			break
		}
	}
	assert.True(t, found, "edit command should be registered on rootCmd")
}

func TestEditCmdVaultFlag(t *testing.T) {
	flag := editCmd.Flags().Lookup("vault")
	require.NotNil(t, flag, "--vault flag should be registered")
	assert.Equal(t, "string", flag.Value.Type())
	assert.Equal(t, "", flag.DefValue)
}

func TestEditCmdItemFlag(t *testing.T) {
	flag := editCmd.Flags().Lookup("item")
	require.NotNil(t, flag, "--item flag should be registered")
	assert.Equal(t, "string", flag.Value.Type())
	assert.Equal(t, "", flag.DefValue)
}

func TestEditCmdBrowseFlag(t *testing.T) {
	flag := editCmd.Flags().Lookup("browse")
	require.NotNil(t, flag, "--browse flag should be registered")
	assert.Equal(t, "bool", flag.Value.Type())
	assert.Equal(t, "false", flag.DefValue)
}

func TestEditCmdItemWithoutVaultReturnsError(t *testing.T) {
	// Reset flags after test
	origVault := editVault
	origItem := editItem
	t.Cleanup(func() {
		editVault = origVault
		editItem = origItem
	})

	editVault = ""
	editItem = "some-item"

	err := runEdit(editCmd, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--item requires --vault")
}

func TestEditCmdHasAllExpectedFlags(t *testing.T) {
	expectedFlags := []string{"vault", "item", "browse"}
	for _, name := range expectedFlags {
		flag := editCmd.Flags().Lookup(name)
		assert.NotNil(t, flag, "flag --%s should be registered on edit command", name)
	}
}
