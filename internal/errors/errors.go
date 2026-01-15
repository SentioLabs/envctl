// Package errors provides custom error types for envctl.
package errors

import (
	"fmt"
	"strings"
)

// ConfigError represents an error in the configuration file.
type ConfigError struct {
	Path    string
	Line    int
	Message string
}

func (e *ConfigError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("%s:%d: %s", e.Path, e.Line, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Path, e.Message)
}

// ConfigNotFoundError is returned when no config file is found.
type ConfigNotFoundError struct {
	SearchPath string
}

func (e *ConfigNotFoundError) Error() string {
	return fmt.Sprintf("config file not found (searched from %s) - run 'envctl init' to create one", e.SearchPath)
}

// AWSError represents an AWS-related error with helpful context.
type AWSError struct {
	SecretName string
	Operation  string
	Message    string
	Hint       string
	Underlying error
}

func (e *AWSError) Error() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("AWS %s failed for %q: %s", e.Operation, e.SecretName, e.Message))
	if e.Hint != "" {
		sb.WriteString("\n  Hint: ")
		sb.WriteString(e.Hint)
	}
	return sb.String()
}

func (e *AWSError) Unwrap() error {
	return e.Underlying
}

// SecretNotFoundError is returned when a secret doesn't exist.
type SecretNotFoundError struct {
	SecretName string
}

func (e *SecretNotFoundError) Error() string {
	return fmt.Sprintf("secret %q not found - check the secret name in AWS console", e.SecretName)
}

// AccessDeniedError is returned when access to a secret is denied.
type AccessDeniedError struct {
	SecretName string
}

func (e *AccessDeniedError) Error() string {
	return fmt.Sprintf("access denied to %q - check your IAM permissions", e.SecretName)
}

// InvalidSecretFormatError is returned when a secret has an unsupported format.
type InvalidSecretFormatError struct {
	SecretName string
}

func (e *InvalidSecretFormatError) Error() string {
	return fmt.Sprintf("secret %q has no string value (binary secrets are not supported)", e.SecretName)
}

// KeyNotFoundError is returned when a key doesn't exist in a secret.
type KeyNotFoundError struct {
	SecretName    string
	Key           string
	AvailableKeys []string
}

func (e *KeyNotFoundError) Error() string {
	available := strings.Join(e.AvailableKeys, ", ")
	if available == "" {
		available = "(none)"
	}
	return fmt.Sprintf("key %q not found in secret %q - available keys: %s", e.Key, e.SecretName, available)
}

// CredentialsError is returned when AWS credentials are missing or invalid.
type CredentialsError struct {
	Message string
}

func (e *CredentialsError) Error() string {
	return fmt.Sprintf("AWS credentials not found: %s\n  Run 'aws configure' or 'aws sso login' to set up credentials", e.Message)
}

// SecretRefError is returned when a secret reference is invalid.
type SecretRefError struct {
	Ref     string
	Message string
}

func (e *SecretRefError) Error() string {
	return fmt.Sprintf("invalid secret reference %q: %s", e.Ref, e.Message)
}
