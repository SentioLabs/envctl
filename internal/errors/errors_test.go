//nolint:testpackage // Testing internal error types requires same package
package errors

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  ConfigError
		want string
	}{
		{
			name: "with line number",
			err:  ConfigError{Path: "/path/to/.envctl.yaml", Line: 10, Message: "invalid syntax"},
			want: "/path/to/.envctl.yaml:10: invalid syntax",
		},
		{
			name: "without line number",
			err:  ConfigError{Path: "/path/to/.envctl.yaml", Line: 0, Message: "missing required field"},
			want: "/path/to/.envctl.yaml: missing required field",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.err.Error())
		})
	}
}

func TestConfigNotFoundError_Error(t *testing.T) {
	err := ConfigNotFoundError{SearchPath: "/home/user/project"}
	assert.Contains(t, err.Error(), "/home/user/project")
	assert.Contains(t, err.Error(), "config file not found")
	assert.Contains(t, err.Error(), "envctl init")
}

func TestAWSError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  AWSError
		want string
	}{
		{
			name: "without hint",
			err: AWSError{
				SecretName: "my-secret",
				Operation:  "GetSecretValue",
				Message:    "access denied",
			},
			want: `AWS GetSecretValue failed for "my-secret": access denied`,
		},
		{
			name: "with hint",
			err: AWSError{
				SecretName: "my-secret",
				Operation:  "GetSecretValue",
				Message:    "access denied",
				Hint:       "Check your IAM permissions",
			},
			want: "AWS GetSecretValue failed for \"my-secret\": access denied\n  Hint: Check your IAM permissions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.err.Error())
		})
	}
}

func TestAWSError_Unwrap(t *testing.T) {
	underlying := errors.New("underlying error")
	err := &AWSError{
		SecretName: "test",
		Operation:  "GetSecretValue",
		Message:    "failed",
		Underlying: underlying,
	}

	assert.Equal(t, underlying, err.Unwrap())
	assert.ErrorIs(t, err, underlying)
}

func TestSecretNotFoundError_Error(t *testing.T) {
	err := SecretNotFoundError{SecretName: "my-app/prod"}
	assert.Contains(t, err.Error(), "my-app/prod")
	assert.Contains(t, err.Error(), "not found")
}

func TestAccessDeniedError_Error(t *testing.T) {
	err := AccessDeniedError{SecretName: "my-app/prod"}
	assert.Contains(t, err.Error(), "my-app/prod")
	assert.Contains(t, err.Error(), "access denied")
	assert.Contains(t, err.Error(), "IAM permissions")
}

func TestInvalidSecretFormatError_Error(t *testing.T) {
	err := InvalidSecretFormatError{SecretName: "my-binary-secret"}
	assert.Contains(t, err.Error(), "my-binary-secret")
	assert.Contains(t, err.Error(), "binary")
	assert.Contains(t, err.Error(), "not supported")
}

func TestKeyNotFoundError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  KeyNotFoundError
		want []string // substrings to check
	}{
		{
			name: "with available keys",
			err: KeyNotFoundError{
				SecretName:    "my-secret",
				Key:           "DB_PASSWORD",
				AvailableKeys: []string{"DB_HOST", "DB_USER", "DB_NAME"},
			},
			want: []string{"DB_PASSWORD", "my-secret", "DB_HOST", "DB_USER", "DB_NAME"},
		},
		{
			name: "no available keys",
			err: KeyNotFoundError{
				SecretName:    "empty-secret",
				Key:           "ANY_KEY",
				AvailableKeys: nil,
			},
			want: []string{"ANY_KEY", "empty-secret", "(none)"},
		},
		{
			name: "empty available keys",
			err: KeyNotFoundError{
				SecretName:    "empty-secret",
				Key:           "ANY_KEY",
				AvailableKeys: []string{},
			},
			want: []string{"ANY_KEY", "empty-secret", "(none)"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errMsg := tt.err.Error()
			for _, substr := range tt.want {
				assert.Contains(t, errMsg, substr)
			}
		})
	}
}

func TestCredentialsError_Error(t *testing.T) {
	err := CredentialsError{Message: "no valid credential sources found"}
	assert.Contains(t, err.Error(), "no valid credential sources found")
	assert.Contains(t, err.Error(), "aws configure")
	assert.Contains(t, err.Error(), "aws sso login")
}

func TestSecretRefError_Error(t *testing.T) {
	err := SecretRefError{
		Ref:     "invalid#ref#format",
		Message: "too many # characters",
	}
	assert.Contains(t, err.Error(), "invalid#ref#format")
	assert.Contains(t, err.Error(), "too many # characters")
}

func TestIncludeAllRequiredError_Error(t *testing.T) {
	err := IncludeAllRequiredError{Secret: "shared/common"}
	assert.Contains(t, err.Error(), "shared/common")
	assert.Contains(t, err.Error(), "key")
	assert.Contains(t, err.Error(), "include_all")
}
