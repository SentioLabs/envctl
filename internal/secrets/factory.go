package secrets

import (
	"context"

	"github.com/sentiolabs/envctl/internal/aws"
	"github.com/sentiolabs/envctl/internal/cache"
	"github.com/sentiolabs/envctl/internal/config"
	"github.com/sentiolabs/envctl/internal/onepassword"
)

// Options configures the secrets client factory.
type Options struct {
	Config  *config.Config
	Region  string // AWS region override
	Profile string // AWS profile override
	Cache   *cache.Manager
	NoCache bool
	Refresh bool
}

// NewClient creates a secrets client based on the configured backend.
// Returns an AWS client by default, or a 1Password client if configured.
func NewClient(ctx context.Context, opts Options) (Client, error) {
	backend := config.BackendAWS
	if opts.Config != nil {
		backend = opts.Config.GetBackend()
	}

	switch backend {
	case config.BackendOnePassword:
		return newOnePasswordClient(opts)
	default:
		return newAWSClient(ctx, opts)
	}
}

// newAWSClient creates an AWS Secrets Manager client.
func newAWSClient(ctx context.Context, opts Options) (Client, error) {
	return aws.NewSecretsClientWithOptions(ctx, aws.ClientOptions{
		Region:  opts.Region,
		Profile: opts.Profile,
		Cache:   opts.Cache,
		NoCache: opts.NoCache,
		Refresh: opts.Refresh,
	})
}

// newOnePasswordClient creates a 1Password client.
func newOnePasswordClient(opts Options) (Client, error) {
	var opOpts onepassword.ClientOptions

	if opts.Config != nil && opts.Config.OnePassword != nil {
		opOpts.DefaultVault = opts.Config.OnePassword.Vault
		opOpts.Account = opts.Config.OnePassword.Account
	}

	return onepassword.NewClient(opOpts)
}
