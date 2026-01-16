// Package aws provides AWS Secrets Manager integration.
package aws

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/aws/smithy-go"
	"github.com/sentiolabs/envctl/internal/cache"
	"github.com/sentiolabs/envctl/internal/errors"
)

const (
	// maxRetries is the maximum number of retry attempts.
	maxRetries = 3
	// baseBackoff is the base backoff duration for retries.
	baseBackoff = 100 * time.Millisecond
)

// SecretsClient provides access to AWS Secrets Manager.
type SecretsClient struct {
	client  *secretsmanager.Client
	region  string
	cache   *cache.Manager
	noCache bool
	refresh bool
}

// ClientOptions configures the secrets client.
type ClientOptions struct {
	Region  string
	Profile string
	Cache   *cache.Manager
	NoCache bool // Bypass cache for this request
	Refresh bool // Force refresh and update cache
}

// NewSecretsClient creates a new Secrets Manager client.
func NewSecretsClient(ctx context.Context, region string) (*SecretsClient, error) {
	return NewSecretsClientWithOptions(ctx, ClientOptions{Region: region})
}

// NewSecretsClientWithOptions creates a new Secrets Manager client with options.
func NewSecretsClientWithOptions(ctx context.Context, opts ClientOptions) (*SecretsClient, error) {
	var loadOpts []func(*config.LoadOptions) error

	if opts.Region != "" {
		loadOpts = append(loadOpts, config.WithRegion(opts.Region))
	}
	if opts.Profile != "" {
		loadOpts = append(loadOpts, config.WithSharedConfigProfile(opts.Profile))
	}

	cfg, err := config.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, &errors.CredentialsError{Message: err.Error()}
	}

	client := secretsmanager.NewFromConfig(cfg)

	return &SecretsClient{
		client:  client,
		region:  opts.Region,
		cache:   opts.Cache,
		noCache: opts.NoCache,
		refresh: opts.Refresh,
	}, nil
}

// GetSecret retrieves all key-value pairs from a secret.
func (c *SecretsClient) GetSecret(ctx context.Context, secretName string) (map[string]string, error) {
	// Check cache first (unless disabled or refresh requested)
	if c.cache != nil && !c.noCache && !c.refresh {
		if cached, err := c.cache.Get(c.region, secretName); err == nil && cached != nil {
			return cached, nil
		}
	}

	// Fetch from AWS
	secrets, err := c.fetchSecret(ctx, secretName)
	if err != nil {
		return nil, err
	}

	// Store in cache
	if c.cache != nil && !c.noCache {
		_ = c.cache.Set(c.region, secretName, secrets)
	}

	return secrets, nil
}

// fetchSecret retrieves a secret from AWS with retry logic.
func (c *SecretsClient) fetchSecret(ctx context.Context, secretName string) (map[string]string, error) {
	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretName),
	}

	var result *secretsmanager.GetSecretValueOutput
	var err error

	// Retry with exponential backoff
	for attempt := 0; attempt < maxRetries; attempt++ {
		result, err = c.client.GetSecretValue(ctx, input)
		if err == nil {
			break
		}

		// Don't retry on certain errors
		if isNonRetryableError(err) {
			break
		}

		// Wait before retrying
		if attempt < maxRetries-1 {
			backoff := baseBackoff * time.Duration(1<<uint(attempt))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}
	}

	if err != nil {
		return nil, mapAWSError(secretName, err)
	}

	if result.SecretString == nil {
		return nil, &errors.InvalidSecretFormatError{SecretName: secretName}
	}

	// Try JSON first
	var secrets map[string]string
	if err := json.Unmarshal([]byte(*result.SecretString), &secrets); err == nil {
		return secrets, nil
	}

	// Fall back to plain text - expose as "_value" key
	return map[string]string{"_value": strings.TrimSpace(*result.SecretString)}, nil
}

// GetSecretKey retrieves a specific key from a secret.
func (c *SecretsClient) GetSecretKey(ctx context.Context, secretName, key string) (string, error) {
	secrets, err := c.GetSecret(ctx, secretName)
	if err != nil {
		return "", err
	}

	value, ok := secrets[key]
	if !ok {
		// Collect available keys for error message
		keys := make([]string, 0, len(secrets))
		for k := range secrets {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		return "", &errors.KeyNotFoundError{
			SecretName:    secretName,
			Key:           key,
			AvailableKeys: keys,
		}
	}

	return value, nil
}

// isNonRetryableError checks if an error should not be retried.
func isNonRetryableError(err error) bool {
	var notFound *types.ResourceNotFoundException
	var invalid *types.InvalidParameterException
	var invalidReq *types.InvalidRequestException

	if stderrors.As(err, &notFound) ||
		stderrors.As(err, &invalid) ||
		stderrors.As(err, &invalidReq) {
		return true
	}

	// Check for access denied via smithy API error
	if isAccessDenied(err) {
		return true
	}

	return false
}

// isAccessDenied checks if an error is an access denied error.
func isAccessDenied(err error) bool {
	var apiErr smithy.APIError
	if stderrors.As(err, &apiErr) {
		code := apiErr.ErrorCode()
		return code == "AccessDeniedException" ||
			code == "UnauthorizedAccess" ||
			strings.Contains(code, "AccessDenied")
	}
	return false
}

// Name returns the backend name.
func (c *SecretsClient) Name() string {
	return "aws"
}

// mapAWSError converts AWS errors to user-friendly error types.
func mapAWSError(secretName string, err error) error {
	var notFound *types.ResourceNotFoundException
	if stderrors.As(err, &notFound) {
		return &errors.SecretNotFoundError{SecretName: secretName}
	}

	if isAccessDenied(err) {
		return &errors.AccessDeniedError{SecretName: secretName}
	}

	// Generic AWS error
	return &errors.AWSError{
		SecretName: secretName,
		Operation:  "GetSecretValue",
		Message:    err.Error(),
		Hint:       "Check your AWS credentials and network connectivity",
		Underlying: err,
	}
}
