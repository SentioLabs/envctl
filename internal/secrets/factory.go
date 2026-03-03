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
	Env     *config.Environment
	Cache   *cache.Manager
	NoCache bool
	Refresh bool
}

// NewClient creates a secrets client based on the resolved environment's backend.
// Uses config.ResolveBackend to determine which backend to use, with precedence:
// environment block > global block > default (aws).
func NewClient(ctx context.Context, opts Options) (Client, error) {
	backend := config.BackendAWS
	if opts.Config != nil {
		backend = opts.Config.ResolveBackend(opts.Env)
	}

	switch backend {
	case config.Backend1Pass:
		opCfg := config.OnePassConfig{}
		if opts.Config != nil {
			opCfg = opts.Config.ResolveOnePassConfig(opts.Env)
		}
		return newOnePasswordClient(opCfg, opts)
	default:
		awsCfg := config.AWSConfig{}
		if opts.Config != nil {
			awsCfg = opts.Config.ResolveAWSConfig(opts.Env)
		}
		return newAWSClient(ctx, awsCfg, opts)
	}
}

// newAWSClient creates an AWS Secrets Manager client using resolved AWS config.
func newAWSClient(ctx context.Context, awsCfg config.AWSConfig, opts Options) (Client, error) {
	return aws.NewSecretsClientWithOptions(ctx, aws.ClientOptions{
		Region:  awsCfg.Region,
		Profile: awsCfg.Profile,
		Cache:   opts.Cache,
		NoCache: opts.NoCache,
		Refresh: opts.Refresh,
	})
}

// newOnePasswordClient creates a 1Password client using resolved OnePass config.
func newOnePasswordClient(opCfg config.OnePassConfig, opts Options) (Client, error) {
	return onepassword.NewClient(onepassword.ClientOptions{
		DefaultVault: opCfg.Vault,
		Account:      opCfg.Account,
	})
}
