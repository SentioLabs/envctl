// Package aws provides AWS Secrets Manager integration.
package aws

import (
	"context"
	"encoding/base64"
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

// rawCachePrefix namespaces raw cache entries so they never collide with parsed ones.
const rawCachePrefix = "raw:"

// rawCacheKey is the single key inside a raw cache entry's map.
const rawCacheKey = "_raw"

// secretValueAPI is the one Secrets Manager call this client needs.
type secretValueAPI interface {
	GetSecretValue(
		ctx context.Context,
		params *secretsmanager.GetSecretValueInput,
		optFns ...func(*secretsmanager.Options),
	) (*secretsmanager.GetSecretValueOutput, error)
}

// SecretsClient provides access to AWS Secrets Manager.
type SecretsClient struct {
	client  secretValueAPI
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

// fetchSecret retrieves a secret from AWS and parses it into key-value pairs.
func (c *SecretsClient) fetchSecret(ctx context.Context, secretName string) (map[string]string, error) {
	result, err := c.getSecretValue(ctx, secretName)
	if err != nil {
		return nil, err
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

// getSecretValue calls GetSecretValue with exponential backoff on retryable errors.
func (c *SecretsClient) getSecretValue(
	ctx context.Context, secretName string,
) (*secretsmanager.GetSecretValueOutput, error) {
	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretName),
	}

	var result *secretsmanager.GetSecretValueOutput
	var err error

	for attempt := 0; attempt < maxRetries; attempt++ {
		result, err = c.client.GetSecretValue(ctx, input)
		if err == nil {
			break
		}
		if isNonRetryableError(err) {
			break
		}
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
	return result, nil
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

// GetSecretRaw returns the secret's content verbatim. SecretString is returned
// byte-for-byte with no JSON parsing and no whitespace trimming. SecretBinary is
// returned when the secret holds binary data. Raw entries share the cache TTL
// and the noCache/refresh flags with parsed entries but live under their own key.
func (c *SecretsClient) GetSecretRaw(ctx context.Context, secretName string) ([]byte, error) {
	cacheName := rawCachePrefix + secretName

	if c.cache != nil && !c.noCache && !c.refresh {
		if data, ok := c.rawFromCache(cacheName); ok {
			return data, nil
		}
	}

	result, err := c.getSecretValue(ctx, secretName)
	if err != nil {
		return nil, err
	}

	var data []byte
	switch {
	case result.SecretString != nil:
		data = []byte(*result.SecretString)
	case len(result.SecretBinary) > 0:
		data = result.SecretBinary
	default:
		return nil, &errors.InvalidSecretFormatError{SecretName: secretName}
	}

	if c.cache != nil && !c.noCache {
		_ = c.cache.Set(c.region, cacheName, map[string]string{
			rawCacheKey: base64.StdEncoding.EncodeToString(data),
		})
	}

	return data, nil
}

// rawFromCache looks up a raw entry and decodes its base64 payload.
// It returns false on any cache miss or decode failure so the caller falls
// back to fetching from AWS.
func (c *SecretsClient) rawFromCache(cacheName string) ([]byte, bool) {
	cached, err := c.cache.Get(c.region, cacheName)
	if err != nil || cached == nil {
		return nil, false
	}
	enc, ok := cached[rawCacheKey]
	if !ok {
		return nil, false
	}
	data, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return nil, false
	}
	return data, true
}

// isNonRetryableError checks if an error should not be retried.
func isNonRetryableError(err error) bool {
	if _, ok := stderrors.AsType[*types.ResourceNotFoundException](err); ok {
		return true
	}
	if _, ok := stderrors.AsType[*types.InvalidParameterException](err); ok {
		return true
	}
	if _, ok := stderrors.AsType[*types.InvalidRequestException](err); ok {
		return true
	}

	if isAccessDenied(err) {
		return true
	}

	return false
}

// isAccessDenied checks if an error is an access denied error.
func isAccessDenied(err error) bool {
	if apiErr, ok := stderrors.AsType[smithy.APIError](err); ok {
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
	if _, ok := stderrors.AsType[*types.ResourceNotFoundException](err); ok {
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
