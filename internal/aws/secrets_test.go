//nolint:testpackage // Testing internal functions requires same package
package aws

import (
	"context"
	stderrors "errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/aws/smithy-go"
	"github.com/sentiolabs/envctl/internal/cache"
	"github.com/sentiolabs/envctl/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockAPIError implements smithy.APIError for testing.
type mockAPIError struct {
	code    string
	message string
}

func (e *mockAPIError) Error() string                 { return e.message }
func (e *mockAPIError) ErrorCode() string             { return e.code }
func (e *mockAPIError) ErrorMessage() string          { return e.message }
func (e *mockAPIError) ErrorFault() smithy.ErrorFault { return smithy.FaultClient }

func TestIsNonRetryableError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "ResourceNotFoundException is non-retryable",
			err:  &types.ResourceNotFoundException{Message: new("not found")},
			want: true,
		},
		{
			name: "wrapped ResourceNotFoundException is non-retryable",
			err:  fmt.Errorf("wrapped: %w", &types.ResourceNotFoundException{Message: new("not found")}),
			want: true,
		},
		{
			name: "InvalidParameterException is non-retryable",
			err:  &types.InvalidParameterException{Message: new("invalid param")},
			want: true,
		},
		{
			name: "InvalidRequestException is non-retryable",
			err:  &types.InvalidRequestException{Message: new("invalid request")},
			want: true,
		},
		{
			name: "AccessDeniedException is non-retryable",
			err:  &mockAPIError{code: "AccessDeniedException", message: "access denied"},
			want: true,
		},
		{
			name: "generic error is retryable",
			err:  stderrors.New("network timeout"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isNonRetryableError(tt.err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsAccessDenied(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "AccessDeniedException",
			err:  &mockAPIError{code: "AccessDeniedException", message: "access denied"},
			want: true,
		},
		{
			name: "UnauthorizedAccess",
			err:  &mockAPIError{code: "UnauthorizedAccess", message: "unauthorized"},
			want: true,
		},
		{
			name: "code containing AccessDenied",
			err:  &mockAPIError{code: "SomeAccessDeniedError", message: "denied"},
			want: true,
		},
		{
			name: "wrapped AccessDeniedException",
			err:  fmt.Errorf("wrapped: %w", &mockAPIError{code: "AccessDeniedException", message: "access denied"}),
			want: true,
		},
		{
			name: "non-access-denied API error",
			err:  &mockAPIError{code: "ThrottlingException", message: "throttled"},
			want: false,
		},
		{
			name: "non-API error",
			err:  stderrors.New("some other error"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isAccessDenied(tt.err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMapAWSError(t *testing.T) {
	t.Run("ResourceNotFoundException maps to SecretNotFoundError", func(t *testing.T) {
		err := &types.ResourceNotFoundException{Message: new("not found")}
		result := mapAWSError("my-secret", err)

		var notFound *errors.SecretNotFoundError
		require.ErrorAs(t, result, &notFound)
		assert.Equal(t, "my-secret", notFound.SecretName)
	})

	t.Run("wrapped ResourceNotFoundException maps to SecretNotFoundError", func(t *testing.T) {
		err := fmt.Errorf("wrapped: %w", &types.ResourceNotFoundException{Message: new("not found")})
		result := mapAWSError("my-secret", err)

		var notFound *errors.SecretNotFoundError
		require.ErrorAs(t, result, &notFound)
		assert.Equal(t, "my-secret", notFound.SecretName)
	})

	t.Run("AccessDeniedException maps to AccessDeniedError", func(t *testing.T) {
		err := &mockAPIError{code: "AccessDeniedException", message: "access denied"}
		result := mapAWSError("my-secret", err)

		var accessDenied *errors.AccessDeniedError
		require.ErrorAs(t, result, &accessDenied)
		assert.Equal(t, "my-secret", accessDenied.SecretName)
	})

	t.Run("generic error maps to AWSError", func(t *testing.T) {
		err := stderrors.New("network failure")
		result := mapAWSError("my-secret", err)

		var awsErr *errors.AWSError
		require.ErrorAs(t, result, &awsErr)
		assert.Equal(t, "my-secret", awsErr.SecretName)
		assert.Equal(t, "GetSecretValue", awsErr.Operation)
		assert.Contains(t, awsErr.Message, "network failure")
	})
}

// fakeSecretValueAPI records calls and returns a canned output.
type fakeSecretValueAPI struct {
	calls  int
	output *secretsmanager.GetSecretValueOutput
	err    error
}

func (f *fakeSecretValueAPI) GetSecretValue(
	_ context.Context,
	_ *secretsmanager.GetSecretValueInput,
	_ ...func(*secretsmanager.Options),
) (*secretsmanager.GetSecretValueOutput, error) {
	f.calls++
	return f.output, f.err
}

func newRawTestClient(fake *fakeSecretValueAPI, mgr *cache.Manager) *SecretsClient {
	return &SecretsClient{client: fake, region: "us-east-1", cache: mgr}
}

// newFileCache builds an encrypted file-backed cache rooted in a temp dir.
// cache.GetCacheDir() honors XDG_CACHE_HOME, which is how the location is redirected.
func newFileCache(t *testing.T) *cache.Manager {
	t.Helper()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	mgr, err := cache.NewManager(cache.Options{
		Enabled: true,
		TTL:     time.Minute,
		Backend: cache.BackendFile,
	})
	require.NoError(t, err)
	if !mgr.IsEnabled() {
		t.Skip("file cache unavailable (running as root?)")
	}
	return mgr
}

func TestGetSecretRaw_StringVerbatim(t *testing.T) {
	pem := "-----BEGIN PRIVATE KEY-----\nabc\n-----END PRIVATE KEY-----\n"
	fake := &fakeSecretValueAPI{output: &secretsmanager.GetSecretValueOutput{SecretString: aws.String(pem)}}
	c := newRawTestClient(fake, nil)

	got, err := c.GetSecretRaw(t.Context(), "app/sp_key")
	require.NoError(t, err)
	assert.Equal(t, []byte(pem), got, "trailing newline must survive")
}

func TestGetSecretRaw_JSONStaysJSON(t *testing.T) {
	raw := `{"a":"1","b":"2"}`
	fake := &fakeSecretValueAPI{output: &secretsmanager.GetSecretValueOutput{SecretString: aws.String(raw)}}
	c := newRawTestClient(fake, nil)

	got, err := c.GetSecretRaw(t.Context(), "app/json")
	require.NoError(t, err)
	assert.Equal(t, raw, string(got))
}

func TestGetSecretRaw_Binary(t *testing.T) {
	bin := []byte{0x00, 0xff, 0x10, 0x80}
	fake := &fakeSecretValueAPI{output: &secretsmanager.GetSecretValueOutput{SecretBinary: bin}}
	c := newRawTestClient(fake, nil)

	got, err := c.GetSecretRaw(t.Context(), "app/bin")
	require.NoError(t, err)
	assert.Equal(t, bin, got)
}

func TestGetSecretRaw_Empty(t *testing.T) {
	fake := &fakeSecretValueAPI{output: &secretsmanager.GetSecretValueOutput{}}
	c := newRawTestClient(fake, nil)

	_, err := c.GetSecretRaw(t.Context(), "app/empty")
	require.Error(t, err)
	var formatErr *errors.InvalidSecretFormatError
	require.ErrorAs(t, err, &formatErr)
}

func TestGetSecretRaw_CachesUnderOwnKey(t *testing.T) {
	mgr := newFileCache(t)
	fake := &fakeSecretValueAPI{output: &secretsmanager.GetSecretValueOutput{SecretString: aws.String("plain\n")}}
	c := newRawTestClient(fake, mgr)

	first, err := c.GetSecretRaw(t.Context(), "app/plain")
	require.NoError(t, err)
	second, err := c.GetSecretRaw(t.Context(), "app/plain")
	require.NoError(t, err)
	assert.Equal(t, first, second)
	assert.Equal(t, 1, fake.calls, "second read must come from cache")

	parsed, err := mgr.Get("us-east-1", "app/plain")
	require.NoError(t, err)
	assert.Nil(t, parsed, "raw fetch must not populate the parsed cache entry")

	rawEntry, err := mgr.Get("us-east-1", rawCachePrefix+"app/plain")
	require.NoError(t, err)
	require.NotNil(t, rawEntry)
	assert.NotEqual(t, "plain\n", rawEntry[rawCacheKey], "cached raw value is base64, not plaintext")
}

func TestGetSecretRaw_NoCacheBypasses(t *testing.T) {
	mgr := newFileCache(t)
	fake := &fakeSecretValueAPI{output: &secretsmanager.GetSecretValueOutput{SecretString: aws.String("x")}}
	c := newRawTestClient(fake, mgr)
	c.noCache = true

	_, err := c.GetSecretRaw(t.Context(), "app/x")
	require.NoError(t, err)
	_, err = c.GetSecretRaw(t.Context(), "app/x")
	require.NoError(t, err)
	assert.Equal(t, 2, fake.calls)
}

func TestGetSecret_StillTrimsPlainText(t *testing.T) {
	fake := &fakeSecretValueAPI{output: &secretsmanager.GetSecretValueOutput{SecretString: aws.String("  v  \n")}}
	c := newRawTestClient(fake, nil)

	got, err := c.GetSecret(t.Context(), "app/plain")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"_value": "v"}, got)
}
