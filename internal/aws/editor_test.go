//nolint:testpackage // Testing internal functions requires same package
package aws

import (
	"context"
	"encoding/json"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// smOpts is a type alias for AWS SDK option functions.
type smOpts = func(*secretsmanager.Options)

// testSecretJSON is a common test fixture for secrets.
const testSecretJSON = `{"DB_HOST":"localhost","DB_PORT":"5432"}`

// mockSMClient implements smClient for testing.
type mockSMClient struct {
	listSecretsFunc func(
		ctx context.Context,
		params *secretsmanager.ListSecretsInput,
		optFns ...smOpts,
	) (*secretsmanager.ListSecretsOutput, error)

	getSecretValueFunc func(
		ctx context.Context,
		params *secretsmanager.GetSecretValueInput,
		optFns ...smOpts,
	) (*secretsmanager.GetSecretValueOutput, error)

	putSecretValueFunc func(
		ctx context.Context,
		params *secretsmanager.PutSecretValueInput,
		optFns ...smOpts,
	) (*secretsmanager.PutSecretValueOutput, error)

	createSecretFunc func(
		ctx context.Context,
		params *secretsmanager.CreateSecretInput,
		optFns ...smOpts,
	) (*secretsmanager.CreateSecretOutput, error)
}

func (m *mockSMClient) ListSecrets(
	ctx context.Context,
	params *secretsmanager.ListSecretsInput,
	optFns ...smOpts,
) (*secretsmanager.ListSecretsOutput, error) {
	return m.listSecretsFunc(ctx, params, optFns...)
}

func (m *mockSMClient) GetSecretValue(
	ctx context.Context,
	params *secretsmanager.GetSecretValueInput,
	optFns ...smOpts,
) (*secretsmanager.GetSecretValueOutput, error) {
	return m.getSecretValueFunc(ctx, params, optFns...)
}

func (m *mockSMClient) PutSecretValue(
	ctx context.Context,
	params *secretsmanager.PutSecretValueInput,
	optFns ...smOpts,
) (*secretsmanager.PutSecretValueOutput, error) {
	return m.putSecretValueFunc(ctx, params, optFns...)
}

func (m *mockSMClient) CreateSecret(
	ctx context.Context,
	params *secretsmanager.CreateSecretInput,
	optFns ...smOpts,
) (*secretsmanager.CreateSecretOutput, error) {
	return m.createSecretFunc(ctx, params, optFns...)
}

func newTestEditor(mock *mockSMClient) *AWSEditor {
	return &AWSEditor{client: mock, region: "us-east-1"}
}

// mockGetSecretValue returns a mock function that returns
// the given JSON string.
func mockGetSecretValue(
	jsonStr string,
) func(context.Context, *secretsmanager.GetSecretValueInput, ...smOpts) (*secretsmanager.GetSecretValueOutput, error) {
	return func(
		_ context.Context,
		_ *secretsmanager.GetSecretValueInput,
		_ ...smOpts,
	) (*secretsmanager.GetSecretValueOutput, error) {
		return &secretsmanager.GetSecretValueOutput{
			SecretString: awssdk.String(jsonStr),
		}, nil
	}
}

// capturePutSecretValue returns a mock function that captures
// the PutSecretValueInput for later assertions.
func capturePutSecretValue(
	captured **secretsmanager.PutSecretValueInput,
) func(context.Context, *secretsmanager.PutSecretValueInput, ...smOpts) (*secretsmanager.PutSecretValueOutput, error) {
	return func(
		_ context.Context,
		params *secretsmanager.PutSecretValueInput,
		_ ...smOpts,
	) (*secretsmanager.PutSecretValueOutput, error) {
		*captured = params
		return &secretsmanager.PutSecretValueOutput{}, nil
	}
}

func TestListVaults(t *testing.T) {
	mock := &mockSMClient{
		listSecretsFunc: func(
			_ context.Context,
			_ *secretsmanager.ListSecretsInput,
			_ ...smOpts,
		) (*secretsmanager.ListSecretsOutput, error) {
			return &secretsmanager.ListSecretsOutput{
				SecretList: []smtypes.SecretListEntry{
					{Name: awssdk.String("prod/app1/db")},
					{Name: awssdk.String("prod/app2/api")},
					{Name: awssdk.String("staging/app1/db")},
					{Name: awssdk.String("toplevel")},
				},
			}, nil
		},
	}

	editor := newTestEditor(mock)
	vaults, err := editor.ListEditorVaults(t.Context())
	require.NoError(t, err)
	require.Len(t, vaults, 3)

	// Should be sorted
	assert.Equal(t, "prod", vaults[0].Name)
	assert.Equal(t, "prod", vaults[0].ID)
	assert.Equal(t, "staging", vaults[1].Name)
	assert.Equal(t, "staging", vaults[1].ID)
	assert.Equal(t, "toplevel", vaults[2].Name)
	assert.Equal(t, "toplevel", vaults[2].ID)
}

func TestListVaultsPaginated(t *testing.T) {
	callCount := 0
	nextToken := "page2"
	mock := &mockSMClient{
		listSecretsFunc: func(
			_ context.Context,
			params *secretsmanager.ListSecretsInput,
			_ ...smOpts,
		) (*secretsmanager.ListSecretsOutput, error) {
			callCount++
			if params.NextToken == nil {
				return &secretsmanager.ListSecretsOutput{
					SecretList: []smtypes.SecretListEntry{
						{Name: awssdk.String("prod/app1")},
					},
					NextToken: &nextToken,
				}, nil
			}
			return &secretsmanager.ListSecretsOutput{
				SecretList: []smtypes.SecretListEntry{
					{Name: awssdk.String("staging/app1")},
				},
			}, nil
		},
	}

	editor := newTestEditor(mock)
	vaults, err := editor.ListEditorVaults(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 2, callCount)
	require.Len(t, vaults, 2)
	assert.Equal(t, "prod", vaults[0].Name)
	assert.Equal(t, "staging", vaults[1].Name)
}

func TestListItems(t *testing.T) {
	mock := &mockSMClient{
		listSecretsFunc: func(
			_ context.Context,
			_ *secretsmanager.ListSecretsInput,
			_ ...smOpts,
		) (*secretsmanager.ListSecretsOutput, error) {
			return &secretsmanager.ListSecretsOutput{
				SecretList: []smtypes.SecretListEntry{
					{Name: awssdk.String("prod/app1/db")},
					{Name: awssdk.String("prod/app2/api")},
					{Name: awssdk.String("staging/other")},
				},
			}, nil
		},
	}

	editor := newTestEditor(mock)
	items, err := editor.ListEditorItems(t.Context(), "prod")
	require.NoError(t, err)
	require.Len(t, items, 2)

	assert.Equal(t, "prod/app1/db", items[0].ID)
	assert.Equal(t, "prod/app1/db", items[0].Name)
	assert.Equal(t, "prod", items[0].Vault)
	assert.Equal(t, "prod/app2/api", items[1].ID)
	assert.Equal(t, "prod/app2/api", items[1].Name)
	assert.Equal(t, "prod", items[1].Vault)
}

func TestGetFields(t *testing.T) {
	fieldsJSON := `{"DB_HOST":"localhost","DB_PORT":"5432","API_KEY":"secret123"}`
	mock := &mockSMClient{
		getSecretValueFunc: func(
			_ context.Context,
			params *secretsmanager.GetSecretValueInput,
			_ ...smOpts,
		) (*secretsmanager.GetSecretValueOutput, error) {
			assert.Equal(t, "prod/app1", *params.SecretId)
			return &secretsmanager.GetSecretValueOutput{
				SecretString: awssdk.String(fieldsJSON),
			}, nil
		},
	}

	editor := newTestEditor(mock)
	fields, err := editor.GetEditorFields(t.Context(), "prod/app1")
	require.NoError(t, err)
	require.Len(t, fields, 3)

	// Should be sorted by key
	assert.Equal(t, "API_KEY", fields[0].Key)
	assert.Equal(t, "secret123", fields[0].Value)
	assert.Equal(t, EditorFieldText, fields[0].Type)

	assert.Equal(t, "DB_HOST", fields[1].Key)
	assert.Equal(t, "localhost", fields[1].Value)

	assert.Equal(t, "DB_PORT", fields[2].Key)
	assert.Equal(t, "5432", fields[2].Value)
}

func TestUpdateField(t *testing.T) {
	var putInput *secretsmanager.PutSecretValueInput

	mock := &mockSMClient{
		getSecretValueFunc: mockGetSecretValue(testSecretJSON),
		putSecretValueFunc: capturePutSecretValue(&putInput),
	}

	editor := newTestEditor(mock)
	err := editor.UpdateEditorField(
		t.Context(), "prod/app1", "DB_HOST", "remotehost",
	)
	require.NoError(t, err)
	require.NotNil(t, putInput)
	assert.Equal(t, "prod/app1", *putInput.SecretId)

	var result map[string]string
	require.NoError(t, json.Unmarshal(
		[]byte(*putInput.SecretString), &result,
	))
	assert.Equal(t, "remotehost", result["DB_HOST"])
	assert.Equal(t, "5432", result["DB_PORT"])
}

func TestDeleteField(t *testing.T) {
	var putInput *secretsmanager.PutSecretValueInput

	mock := &mockSMClient{
		getSecretValueFunc: mockGetSecretValue(testSecretJSON),
		putSecretValueFunc: capturePutSecretValue(&putInput),
	}

	editor := newTestEditor(mock)
	err := editor.DeleteEditorField(t.Context(), "prod/app1", "DB_HOST")
	require.NoError(t, err)
	require.NotNil(t, putInput)

	var result map[string]string
	require.NoError(t, json.Unmarshal(
		[]byte(*putInput.SecretString), &result,
	))
	assert.Len(t, result, 1)
	assert.Equal(t, "5432", result["DB_PORT"])
	_, exists := result["DB_HOST"]
	assert.False(t, exists)
}

func TestRenameField(t *testing.T) {
	renameJSON := `{"OLD_KEY":"myvalue","OTHER":"keep"}`
	var putInput *secretsmanager.PutSecretValueInput

	mock := &mockSMClient{
		getSecretValueFunc: mockGetSecretValue(renameJSON),
		putSecretValueFunc: capturePutSecretValue(&putInput),
	}

	editor := newTestEditor(mock)
	err := editor.RenameEditorField(
		t.Context(), "prod/app1", "OLD_KEY", "NEW_KEY",
	)
	require.NoError(t, err)
	require.NotNil(t, putInput)

	var result map[string]string
	require.NoError(t, json.Unmarshal(
		[]byte(*putInput.SecretString), &result,
	))
	assert.Len(t, result, 2)
	assert.Equal(t, "myvalue", result["NEW_KEY"])
	assert.Equal(t, "keep", result["OTHER"])
	_, exists := result["OLD_KEY"]
	assert.False(t, exists)
}

func TestCreateItem(t *testing.T) {
	var createInput *secretsmanager.CreateSecretInput

	mock := &mockSMClient{
		createSecretFunc: func(
			_ context.Context,
			params *secretsmanager.CreateSecretInput,
			_ ...smOpts,
		) (*secretsmanager.CreateSecretOutput, error) {
			createInput = params
			return &secretsmanager.CreateSecretOutput{}, nil
		},
	}

	editor := newTestEditor(mock)

	t.Run("with vault prefix", func(t *testing.T) {
		err := editor.CreateEditorItem(
			t.Context(), "prod", "myapp", []EditorFieldPair{
				{Key: "DB_HOST", Value: "localhost"},
				{Key: "DB_PORT", Value: "5432"},
			},
		)
		require.NoError(t, err)
		require.NotNil(t, createInput)
		assert.Equal(t, "prod/myapp", *createInput.Name)

		var result map[string]string
		require.NoError(t, json.Unmarshal(
			[]byte(*createInput.SecretString), &result,
		))
		assert.Equal(t, "localhost", result["DB_HOST"])
		assert.Equal(t, "5432", result["DB_PORT"])
	})

	t.Run("with empty vault", func(t *testing.T) {
		err := editor.CreateEditorItem(
			t.Context(), "", "myapp", nil,
		)
		require.NoError(t, err)
		require.NotNil(t, createInput)
		assert.Equal(t, "myapp", *createInput.Name)

		var result map[string]string
		require.NoError(t, json.Unmarshal(
			[]byte(*createInput.SecretString), &result,
		))
		assert.Empty(t, result)
	})
}

func TestEditorGetSecret(t *testing.T) {
	mock := &mockSMClient{
		getSecretValueFunc: mockGetSecretValue(testSecretJSON),
	}

	editor := newTestEditor(mock)
	result, err := editor.GetSecret(t.Context(), "prod/app1")
	require.NoError(t, err)
	assert.Equal(t, "localhost", result["DB_HOST"])
	assert.Equal(t, "5432", result["DB_PORT"])
}

func TestEditorGetSecretKey(t *testing.T) {
	mock := &mockSMClient{
		getSecretValueFunc: mockGetSecretValue(testSecretJSON),
	}

	editor := newTestEditor(mock)

	t.Run("existing key", func(t *testing.T) {
		val, err := editor.GetSecretKey(
			t.Context(), "prod/app1", "DB_HOST",
		)
		require.NoError(t, err)
		assert.Equal(t, "localhost", val)
	})

	t.Run("missing key", func(t *testing.T) {
		_, err := editor.GetSecretKey(
			t.Context(), "prod/app1", "MISSING",
		)
		require.Error(t, err)
	})
}

func TestEditorName(t *testing.T) {
	editor := &AWSEditor{}
	assert.Equal(t, "aws", editor.Name())
}
