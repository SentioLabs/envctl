//nolint:testpackage // Testing internal functions requires same package
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid simple config",
			content: `version: 1
default_environment: dev
environments:
  dev:
    secret: myapp/dev
`,
			wantErr: false,
		},
		{
			name: "valid complex config",
			content: `version: 1
default_environment: dev
environments:
  dev:
    secret: myapp/dev
    aws:
      region: us-west-2
  staging:
    secret: myapp/staging
include:
  dev:
    - secret: shared/datadog
    - secret: shared/stripe
      key: api_key
      as: STRIPE_KEY
mapping:
  DB_URL: myapp/dev#database_url
`,
			wantErr: false,
		},
		{
			name: "invalid version",
			content: `version: 2
environments:
  dev:
    secret: myapp/dev
`,
			wantErr: true,
			errMsg:  "unsupported config version",
		},
		{
			name: "missing environments",
			content: `version: 1
`,
			wantErr: true,
			errMsg:  "no applications or environments defined",
		},
		{
			name: "missing secret in environment",
			content: `version: 1
environments:
  dev:
    aws:
      region: us-west-2
`,
			wantErr: true,
			errMsg:  "missing required 'secret' field",
		},
		{
			name: "invalid default_environment",
			content: `version: 1
default_environment: prod
environments:
  dev:
    secret: myapp/dev
`,
			wantErr: true,
			errMsg:  "references undefined environment",
		},
		{
			name: "unknown field",
			content: `version: 1
environments:
  dev:
    secret: myapp/dev
unknownField: true
`,
			wantErr: true,
		},
		{
			name: "valid config with 1pass backend",
			content: `version: 1
1pass:
  vault: dev-vault
  account: my-account
environments:
  dev:
    secret: myapp/dev
    1pass:
      vault: env-vault
`,
			wantErr: false,
		},
		{
			name: "valid config with per-env backends",
			content: `version: 1
environments:
  dev:
    secret: myapp/dev
    aws:
      region: us-west-2
  staging:
    secret: myapp/staging
    1pass:
      vault: staging-vault
`,
			wantErr: false,
		},
		{
			name: "invalid both backends at global level",
			content: `version: 1
aws:
  region: us-east-1
1pass:
  vault: my-vault
environments:
  dev:
    secret: myapp/dev
`,
			wantErr: true,
			errMsg:  "cannot specify both 'aws' and '1pass' at the global level",
		},
		{
			name: "invalid both backends on environment",
			content: `version: 1
environments:
  dev:
    secret: myapp/dev
    aws:
      region: us-west-2
    1pass:
      vault: my-vault
`,
			wantErr: true,
			errMsg:  "cannot specify both 'aws' and '1pass'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, ConfigFileName)
			if err := os.WriteFile(configPath, []byte(tt.content), 0o600); err != nil {
				t.Fatalf("failed to write test config: %v", err)
			}

			cfg, err := Load(configPath)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Load() expected error, got nil")
				} else if tt.errMsg != "" && !containsSubstring(err.Error(), tt.errMsg) {
					t.Errorf("Load() error = %q, want error containing %q", err.Error(), tt.errMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("Load() unexpected error: %v", err)
				return
			}

			if cfg == nil {
				t.Error("Load() returned nil config without error")
			}
		})
	}
}

func TestFindConfigFrom(t *testing.T) {
	// Create a directory structure
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "a", "b", "c")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("failed to create dirs: %v", err)
	}

	// Create config in the root
	configPath := filepath.Join(tmpDir, ConfigFileName)
	content := `version: 1
environments:
  dev:
    secret: test/dev
`
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Find from subdirectory
	found, err := FindConfigFrom(subDir)
	if err != nil {
		t.Errorf("FindConfigFrom() error = %v", err)
		return
	}

	if found != configPath {
		t.Errorf("FindConfigFrom() = %q, want %q", found, configPath)
	}
}

func TestGetEnvironment(t *testing.T) {
	cfg := &Config{
		Version:            1,
		DefaultEnvironment: "dev",
		Environments: map[string]Environment{
			"dev":     {Secret: "myapp/dev"},
			"staging": {Secret: "myapp/staging"},
		},
	}

	tests := []struct {
		name       string
		envName    string
		wantSecret string
		wantErr    bool
	}{
		{
			name:       "explicit environment",
			envName:    "staging",
			wantSecret: "myapp/staging",
			wantErr:    false,
		},
		{
			name:       "default environment",
			envName:    "",
			wantSecret: "myapp/dev",
			wantErr:    false,
		},
		{
			name:    "unknown environment",
			envName: "prod",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, err := cfg.GetEnvironment(tt.envName)
			if tt.wantErr {
				if err == nil {
					t.Error("GetEnvironment() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("GetEnvironment() unexpected error: %v", err)
				return
			}

			if env.Secret != tt.wantSecret {
				t.Errorf("GetEnvironment() secret = %q, want %q", env.Secret, tt.wantSecret)
			}
		})
	}
}

func containsSubstring(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	if s == substr {
		return true
	}
	return s[:len(substr)] == substr ||
		s[len(s)-len(substr):] == substr ||
		findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestResolveBackend(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
		env    *Environment
		want   string
	}{
		{
			name:   "env 1pass overrides global aws",
			config: &Config{Version: 1, AWS: &AWSConfig{Region: "us-east-1"}},
			env:    &Environment{Secret: "test", OnePass: &OnePassConfig{Vault: "Dev"}},
			want:   Backend1Pass,
		},
		{
			name:   "env aws explicit",
			config: &Config{Version: 1},
			env:    &Environment{Secret: "test", AWS: &AWSConfig{Region: "eu-west-1"}},
			want:   BackendAWS,
		},
		{
			name:   "inherit global 1pass",
			config: &Config{Version: 1, OnePass: &OnePassConfig{Vault: "Dev"}},
			env:    &Environment{Secret: "test"},
			want:   Backend1Pass,
		},
		{
			name:   "inherit global aws",
			config: &Config{Version: 1, AWS: &AWSConfig{Region: "us-east-1"}},
			env:    &Environment{Secret: "test"},
			want:   BackendAWS,
		},
		{
			name:   "default to aws when nothing set",
			config: &Config{Version: 1},
			env:    &Environment{Secret: "test"},
			want:   BackendAWS,
		},
		{
			name:   "nil env falls back to global",
			config: &Config{Version: 1, OnePass: &OnePassConfig{Vault: "Dev"}},
			env:    nil,
			want:   Backend1Pass,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.ResolveBackend(tt.env)
			if got != tt.want {
				t.Errorf("ResolveBackend() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveAWSConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		env         *Environment
		wantRegion  string
		wantProfile string
	}{
		{
			name:        "global only",
			config:      &Config{Version: 1, AWS: &AWSConfig{Region: "us-east-1", Profile: "default"}},
			env:         &Environment{Secret: "test"},
			wantRegion:  "us-east-1",
			wantProfile: "default",
		},
		{
			name:        "env only",
			config:      &Config{Version: 1},
			env:         &Environment{Secret: "test", AWS: &AWSConfig{Region: "eu-west-1"}},
			wantRegion:  "eu-west-1",
			wantProfile: "",
		},
		{
			name:        "env overrides global region",
			config:      &Config{Version: 1, AWS: &AWSConfig{Region: "us-east-1", Profile: "default"}},
			env:         &Environment{Secret: "test", AWS: &AWSConfig{Region: "eu-west-1"}},
			wantRegion:  "eu-west-1",
			wantProfile: "default",
		},
		{
			name:        "neither set",
			config:      &Config{Version: 1},
			env:         &Environment{Secret: "test"},
			wantRegion:  "",
			wantProfile: "",
		},
		{
			name:        "nil env",
			config:      &Config{Version: 1, AWS: &AWSConfig{Region: "us-east-1"}},
			env:         nil,
			wantRegion:  "us-east-1",
			wantProfile: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.ResolveAWSConfig(tt.env)
			if got.Region != tt.wantRegion {
				t.Errorf("ResolveAWSConfig().Region = %q, want %q", got.Region, tt.wantRegion)
			}
			if got.Profile != tt.wantProfile {
				t.Errorf("ResolveAWSConfig().Profile = %q, want %q", got.Profile, tt.wantProfile)
			}
		})
	}
}

func TestResolveOnePassConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		env         *Environment
		wantVault   string
		wantAccount string
	}{
		{
			name:        "global only",
			config:      &Config{Version: 1, OnePass: &OnePassConfig{Vault: "Dev", Account: "my-account"}},
			env:         &Environment{Secret: "test"},
			wantVault:   "Dev",
			wantAccount: "my-account",
		},
		{
			name:        "env only",
			config:      &Config{Version: 1},
			env:         &Environment{Secret: "test", OnePass: &OnePassConfig{Vault: "Staging"}},
			wantVault:   "Staging",
			wantAccount: "",
		},
		{
			name:        "env overrides global vault",
			config:      &Config{Version: 1, OnePass: &OnePassConfig{Vault: "Dev", Account: "my-account"}},
			env:         &Environment{Secret: "test", OnePass: &OnePassConfig{Vault: "Staging"}},
			wantVault:   "Staging",
			wantAccount: "my-account",
		},
		{
			name:        "neither set",
			config:      &Config{Version: 1},
			env:         &Environment{Secret: "test"},
			wantVault:   "",
			wantAccount: "",
		},
		{
			name:        "nil env",
			config:      &Config{Version: 1, OnePass: &OnePassConfig{Vault: "Dev", Account: "my-account"}},
			env:         nil,
			wantVault:   "Dev",
			wantAccount: "my-account",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.ResolveOnePassConfig(tt.env)
			if got.Vault != tt.wantVault {
				t.Errorf("ResolveOnePassConfig().Vault = %q, want %q", got.Vault, tt.wantVault)
			}
			if got.Account != tt.wantAccount {
				t.Errorf("ResolveOnePassConfig().Account = %q, want %q", got.Account, tt.wantAccount)
			}
		})
	}
}

func TestEnvKeyedIncludes(t *testing.T) {
	t.Run("parses env-keyed includes correctly", func(t *testing.T) {
		content := `version: 1
environments:
  dev:
    secret: myapp/dev
include:
  dev:
    - secret: shared/datadog
    - secret: shared/stripe
      key: api_key
      as: STRIPE_KEY
  staging:
    - secret: shared/monitoring
`
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ConfigFileName)
		if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
			t.Fatalf("failed to write test config: %v", err)
		}

		cfg, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load() unexpected error: %v", err)
		}

		if len(cfg.Include) != 2 {
			t.Fatalf("expected 2 environment keys in include, got %d", len(cfg.Include))
		}

		devIncludes := cfg.Include["dev"]
		if len(devIncludes) != 2 {
			t.Fatalf("expected 2 dev includes, got %d", len(devIncludes))
		}
		if devIncludes[0].Secret != "shared/datadog" {
			t.Errorf("expected first dev include secret = %q, got %q", "shared/datadog", devIncludes[0].Secret)
		}
		if devIncludes[1].Secret != "shared/stripe" {
			t.Errorf("expected second dev include secret = %q, got %q", "shared/stripe", devIncludes[1].Secret)
		}
		if devIncludes[1].Key != "api_key" {
			t.Errorf("expected second dev include key = %q, got %q", "api_key", devIncludes[1].Key)
		}
		if devIncludes[1].As != "STRIPE_KEY" {
			t.Errorf("expected second dev include as = %q, got %q", "STRIPE_KEY", devIncludes[1].As)
		}

		stagingIncludes := cfg.Include["staging"]
		if len(stagingIncludes) != 1 {
			t.Fatalf("expected 1 staging include, got %d", len(stagingIncludes))
		}
		if stagingIncludes[0].Secret != "shared/monitoring" {
			t.Errorf("expected staging include secret = %q, got %q", "shared/monitoring", stagingIncludes[0].Secret)
		}
	})

	t.Run("include entry with aws config parses correctly", func(t *testing.T) {
		content := `version: 1
environments:
  dev:
    secret: myapp/dev
include:
  dev:
    - secret: shared/datadog
      aws:
        region: eu-west-1
        profile: datadog-profile
`
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ConfigFileName)
		if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
			t.Fatalf("failed to write test config: %v", err)
		}

		cfg, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load() unexpected error: %v", err)
		}

		devIncludes := cfg.Include["dev"]
		if len(devIncludes) != 1 {
			t.Fatalf("expected 1 dev include, got %d", len(devIncludes))
		}
		if devIncludes[0].AWS == nil {
			t.Fatal("expected AWS config on include entry, got nil")
		}
		if devIncludes[0].AWS.Region != "eu-west-1" {
			t.Errorf("expected AWS region = %q, got %q", "eu-west-1", devIncludes[0].AWS.Region)
		}
		if devIncludes[0].AWS.Profile != "datadog-profile" {
			t.Errorf("expected AWS profile = %q, got %q", "datadog-profile", devIncludes[0].AWS.Profile)
		}
	})

	t.Run("include entry with 1pass config parses correctly", func(t *testing.T) {
		content := `version: 1
environments:
  dev:
    secret: myapp/dev
include:
  dev:
    - secret: shared/creds
      1pass:
        vault: shared-vault
        account: team-account
`
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ConfigFileName)
		if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
			t.Fatalf("failed to write test config: %v", err)
		}

		cfg, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load() unexpected error: %v", err)
		}

		devIncludes := cfg.Include["dev"]
		if len(devIncludes) != 1 {
			t.Fatalf("expected 1 dev include, got %d", len(devIncludes))
		}
		if devIncludes[0].OnePass == nil {
			t.Fatal("expected OnePass config on include entry, got nil")
		}
		if devIncludes[0].OnePass.Vault != "shared-vault" {
			t.Errorf("expected OnePass vault = %q, got %q", "shared-vault", devIncludes[0].OnePass.Vault)
		}
		if devIncludes[0].OnePass.Account != "team-account" {
			t.Errorf("expected OnePass account = %q, got %q", "team-account", devIncludes[0].OnePass.Account)
		}
	})

	t.Run("include entry with both aws and 1pass fails validation", func(t *testing.T) {
		content := `version: 1
environments:
  dev:
    secret: myapp/dev
include:
  dev:
    - secret: shared/creds
      aws:
        region: us-east-1
      1pass:
        vault: shared-vault
`
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ConfigFileName)
		if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
			t.Fatalf("failed to write test config: %v", err)
		}

		_, err := Load(configPath)
		if err == nil {
			t.Fatal("Load() expected error for include with both aws and 1pass, got nil")
		}
		if !containsSubstring(err.Error(), "cannot specify both 'aws' and '1pass'") {
			t.Errorf("Load() error = %q, want error containing %q", err.Error(), "cannot specify both 'aws' and '1pass'")
		}
	})
}

func TestShouldIncludeAll(t *testing.T) {
	tests := []struct {
		name       string
		config     *Config
		app        *Application
		env        *Environment
		wantResult bool
	}{
		{
			name:       "default_is_false",
			config:     &Config{Version: 1},
			wantResult: false,
		},
		{
			name:       "global_true",
			config:     &Config{Version: 1, IncludeAll: new(true)},
			wantResult: true,
		},
		{
			name:       "global_false_explicit",
			config:     &Config{Version: 1, IncludeAll: new(false)},
			wantResult: false,
		},
		{
			name:       "app_overrides_global_true",
			config:     &Config{Version: 1, IncludeAll: new(false)},
			app:        &Application{IncludeAll: new(true)},
			wantResult: true,
		},
		{
			name:       "app_overrides_global_false",
			config:     &Config{Version: 1, IncludeAll: new(true)},
			app:        &Application{IncludeAll: new(false)},
			wantResult: false,
		},
		{
			name:       "env_overrides_app_true",
			config:     &Config{Version: 1, IncludeAll: new(false)},
			app:        &Application{IncludeAll: new(false)},
			env:        &Environment{Secret: "test", IncludeAll: new(true)},
			wantResult: true,
		},
		{
			name:       "env_overrides_app_false",
			config:     &Config{Version: 1, IncludeAll: new(true)},
			app:        &Application{IncludeAll: new(true)},
			env:        &Environment{Secret: "test", IncludeAll: new(false)},
			wantResult: false,
		},
		{
			name:       "env_overrides_global_no_app",
			config:     &Config{Version: 1, IncludeAll: new(false)},
			env:        &Environment{Secret: "test", IncludeAll: new(true)},
			wantResult: true,
		},
		{
			name:       "app_nil_inherits_global",
			config:     &Config{Version: 1, IncludeAll: new(true)},
			app:        nil,
			env:        &Environment{Secret: "test"},
			wantResult: true,
		},
		{
			name:       "env_nil_inherits_app",
			config:     &Config{Version: 1, IncludeAll: new(false)},
			app:        &Application{IncludeAll: new(true)},
			env:        nil,
			wantResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.ShouldIncludeAll(tt.app, tt.env)
			if got != tt.wantResult {
				t.Errorf("ShouldIncludeAll() = %v, want %v", got, tt.wantResult)
			}
		})
	}
}
