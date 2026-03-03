package aws

import (
	stderrors "errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/aws/smithy-go"
	"github.com/sentiolabs/envctl/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockAPIError implements smithy.APIError for testing.
type mockAPIError struct {
	code    string
	message string
}

func (e *mockAPIError) Error() string            { return e.message }
func (e *mockAPIError) ErrorCode() string         { return e.code }
func (e *mockAPIError) ErrorMessage() string      { return e.message }
func (e *mockAPIError) ErrorFault() smithy.ErrorFault { return smithy.FaultClient }

func TestIsNonRetryableError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "ResourceNotFoundException is non-retryable",
			err:  &types.ResourceNotFoundException{Message: ptr("not found")},
			want: true,
		},
		{
			name: "wrapped ResourceNotFoundException is non-retryable",
			err:  fmt.Errorf("wrapped: %w", &types.ResourceNotFoundException{Message: ptr("not found")}),
			want: true,
		},
		{
			name: "InvalidParameterException is non-retryable",
			err:  &types.InvalidParameterException{Message: ptr("invalid param")},
			want: true,
		},
		{
			name: "InvalidRequestException is non-retryable",
			err:  &types.InvalidRequestException{Message: ptr("invalid request")},
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
		err := &types.ResourceNotFoundException{Message: ptr("not found")}
		result := mapAWSError("my-secret", err)

		var notFound *errors.SecretNotFoundError
		require.ErrorAs(t, result, &notFound)
		assert.Equal(t, "my-secret", notFound.SecretName)
	})

	t.Run("wrapped ResourceNotFoundException maps to SecretNotFoundError", func(t *testing.T) {
		err := fmt.Errorf("wrapped: %w", &types.ResourceNotFoundException{Message: ptr("not found")})
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

func ptr(s string) *string {
	return &s
}
