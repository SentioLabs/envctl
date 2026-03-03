package secrets_test

import (
	"testing"

	"github.com/sentiolabs/envctl/internal/config"
	"github.com/sentiolabs/envctl/internal/secrets"
)

const (
	backendAWS       = "aws"
	backend1Password = "1password"
)

func TestOptionsStruct(t *testing.T) {
	t.Run("has Env field and no Region/Profile fields", func(t *testing.T) {
		env := &config.Environment{
			Secret: "test/secret",
			AWS: &config.AWSConfig{
				Region:  "us-east-1",
				Profile: "myprofile",
			},
		}

		opts := secrets.Options{
			Config:  &config.Config{Version: 1},
			Env:     env,
			NoCache: true,
			Refresh: false,
		}

		if opts.Env != env {
			t.Errorf("expected Env to be set, got %v", opts.Env)
		}
		if opts.Config == nil {
			t.Error("expected Config to be set")
		}
		if opts.NoCache != true {
			t.Error("expected NoCache to be true")
		}
	})
}

func TestNewClientDefaultsToAWS(t *testing.T) {
	t.Run("when config is nil", func(t *testing.T) {
		opts := secrets.Options{
			Config:  nil,
			Env:     nil,
			NoCache: true,
		}

		client, err := secrets.NewClient(t.Context(), opts)
		if err != nil {
			t.Fatalf("unexpected error creating default client: %v", err)
		}
		if client.Name() != backendAWS {
			t.Errorf("expected backend %q, got %q", backendAWS, client.Name())
		}
	})

	t.Run("when config has no backend specified", func(t *testing.T) {
		cfg := &config.Config{Version: 1}

		opts := secrets.Options{
			Config:  cfg,
			Env:     nil,
			NoCache: true,
		}

		client, err := secrets.NewClient(t.Context(), opts)
		if err != nil {
			t.Fatalf("unexpected error creating AWS client: %v", err)
		}
		if client.Name() != backendAWS {
			t.Errorf("expected backend %q, got %q", backendAWS, client.Name())
		}
	})

	t.Run("when env has aws config", func(t *testing.T) {
		env := &config.Environment{
			Secret: "test/secret",
			AWS:    &config.AWSConfig{Region: "eu-west-1"},
		}
		cfg := &config.Config{Version: 1}

		opts := secrets.Options{
			Config:  cfg,
			Env:     env,
			NoCache: true,
		}

		client, err := secrets.NewClient(t.Context(), opts)
		if err != nil {
			t.Fatalf("unexpected error creating AWS client: %v", err)
		}
		if client.Name() != backendAWS {
			t.Errorf("expected backend %q, got %q", backendAWS, client.Name())
		}
	})
}

func TestNewClientRoutesTo1Password(t *testing.T) {
	t.Run("when env has 1pass config", func(t *testing.T) {
		env := &config.Environment{
			Secret: "test/secret",
			OnePass: &config.OnePassConfig{
				Vault:   "TestVault",
				Account: "test.1password.com",
			},
		}
		cfg := &config.Config{Version: 1}

		opts := secrets.Options{
			Config:  cfg,
			Env:     env,
			NoCache: true,
		}

		client, err := secrets.NewClient(t.Context(), opts)
		if err != nil {
			t.Fatalf("unexpected error creating 1password client: %v", err)
		}
		if client.Name() != backend1Password {
			t.Errorf("expected backend %q, got %q", backend1Password, client.Name())
		}
	})

	t.Run("when global config has 1pass", func(t *testing.T) {
		cfg := &config.Config{
			Version: 1,
			OnePass: &config.OnePassConfig{Vault: "GlobalVault"},
		}

		opts := secrets.Options{
			Config:  cfg,
			Env:     nil,
			NoCache: true,
		}

		client, err := secrets.NewClient(t.Context(), opts)
		if err != nil {
			t.Fatalf("unexpected error creating 1password client: %v", err)
		}
		if client.Name() != backend1Password {
			t.Errorf("expected backend %q, got %q", backend1Password, client.Name())
		}
	})

	t.Run("env-level 1pass overrides global AWS", func(t *testing.T) {
		env := &config.Environment{
			Secret:  "test/secret",
			OnePass: &config.OnePassConfig{Vault: "EnvVault"},
		}
		cfg := &config.Config{
			Version: 1,
			AWS:     &config.AWSConfig{Region: "us-west-2"},
		}

		opts := secrets.Options{
			Config:  cfg,
			Env:     env,
			NoCache: true,
		}

		client, err := secrets.NewClient(t.Context(), opts)
		if err != nil {
			t.Fatalf("unexpected error creating 1password client: %v", err)
		}
		if client.Name() != backend1Password {
			t.Errorf("expected backend %q, got %q (env-level 1pass should override global AWS)",
				backend1Password, client.Name())
		}
	})
}
