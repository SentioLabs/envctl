//nolint:testpackage // Testing internal functions requires same package
package config

import (
	"os"
	"path/filepath"
	"testing"
)

// loadFixture reads a YAML fixture file from testdata/ and writes it to a temp
// directory as .envctl.yaml, returning the path to the temp config file.
func loadFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ConfigFileName)
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}
	return configPath
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid simple config",
			fixture: "valid_simple.yaml",
			wantErr: false,
		},
		{
			name:    "valid complex config",
			fixture: "valid_complex.yaml",
			wantErr: false,
		},
		{
			name:    "invalid version",
			fixture: "invalid_version.yaml",
			wantErr: true,
			errMsg:  "unsupported config version",
		},
		{
			name:    "missing environments",
			fixture: "missing_environments.yaml",
			wantErr: true,
			errMsg:  "no applications or environments defined",
		},
		{
			name:    "missing secret in environment",
			fixture: "missing_secret.yaml",
			wantErr: true,
			errMsg:  "missing required 'secret' field",
		},
		{
			name:    "invalid default_environment",
			fixture: "invalid_default_env.yaml",
			wantErr: true,
			errMsg:  "references undefined environment",
		},
		{
			name:    "unknown field",
			fixture: "unknown_field.yaml",
			wantErr: true,
		},
		{
			name:    "valid config with 1pass backend",
			fixture: "valid_1pass.yaml",
			wantErr: false,
		},
		{
			name:    "valid config with per-env backends",
			fixture: "valid_per_env_backends.yaml",
			wantErr: false,
		},
		{
			name:    "valid config with both global backends",
			fixture: "valid_both_backends_global.yaml",
			wantErr: false,
		},
		{
			name:    "invalid both backends on environment",
			fixture: "invalid_both_backends_env.yaml",
			wantErr: true,
			errMsg:  "cannot specify both 'aws' and '1pass'",
		},
		{
			name:    "invalid both global backends without default_backend",
			fixture: "invalid_both_backends_no_default.yaml",
			wantErr: true,
			errMsg:  "default_backend is required",
		},
		{
			name:    "invalid default_backend value",
			fixture: "invalid_default_backend_value.yaml",
			wantErr: true,
			errMsg:  "invalid default_backend value",
		},
		{
			name:    "invalid default_backend without both backends",
			fixture: "invalid_default_backend_without_both.yaml",
			wantErr: true,
			errMsg:  "default_backend is only valid when both",
		},
		{
			name:    "valid list format",
			fixture: "env_list_format.yaml",
			wantErr: false,
		},
		{
			name:    "valid mixed format",
			fixture: "env_mixed_format.yaml",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := loadFixture(t, tt.fixture)

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

	// Copy fixture to the root of temp dir
	data, err := os.ReadFile(filepath.Join("testdata", "find_config.yaml"))
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}
	configPath := filepath.Join(tmpDir, ConfigFileName)
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
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
			"dev":     NewEnvironment(IncludeEntry{Secret: "myapp/dev"}),
			"staging": NewEnvironment(IncludeEntry{Secret: "myapp/staging"}),
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

			if env.Secret() != tt.wantSecret {
				t.Errorf("GetEnvironment() secret = %q, want %q", env.Secret(), tt.wantSecret)
			}
		})
	}
}

func boolPtr(v bool) *bool { return &v }

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
			env:    envPtr(NewEnvironment(IncludeEntry{Secret: "test", OnePass: &OnePassConfig{Vault: "Dev"}})),
			want:   Backend1Pass,
		},
		{
			name:   "env aws explicit",
			config: &Config{Version: 1},
			env:    envPtr(NewEnvironment(IncludeEntry{Secret: "test", AWS: &AWSConfig{Region: "eu-west-1"}})),
			want:   BackendAWS,
		},
		{
			name:   "inherit global 1pass",
			config: &Config{Version: 1, OnePass: &OnePassConfig{Vault: "Dev"}},
			env:    envPtr(NewEnvironment(IncludeEntry{Secret: "test"})),
			want:   Backend1Pass,
		},
		{
			name:   "inherit global aws",
			config: &Config{Version: 1, AWS: &AWSConfig{Region: "us-east-1"}},
			env:    envPtr(NewEnvironment(IncludeEntry{Secret: "test"})),
			want:   BackendAWS,
		},
		{
			name:   "default to aws when nothing set",
			config: &Config{Version: 1},
			env:    envPtr(NewEnvironment(IncludeEntry{Secret: "test"})),
			want:   BackendAWS,
		},
		{
			name:   "nil env falls back to global",
			config: &Config{Version: 1, OnePass: &OnePassConfig{Vault: "Dev"}},
			env:    nil,
			want:   Backend1Pass,
		},
		{
			name: "both global backends default_backend 1pass resolves to 1pass",
			config: &Config{
				Version: 1, AWS: &AWSConfig{Region: "us-east-1"},
				OnePass: &OnePassConfig{Vault: "Dev"}, DefaultBackend: Backend1Pass,
			},
			env:  envPtr(NewEnvironment(IncludeEntry{Secret: "test"})),
			want: Backend1Pass,
		},
		{
			name: "both global backends default_backend aws resolves to aws",
			config: &Config{
				Version: 1, AWS: &AWSConfig{Region: "us-east-1"},
				OnePass: &OnePassConfig{Vault: "Dev"}, DefaultBackend: BackendAWS,
			},
			env:  envPtr(NewEnvironment(IncludeEntry{Secret: "test"})),
			want: BackendAWS,
		},
		{
			name: "both global backends env backend aws overrides default_backend 1pass",
			config: &Config{
				Version: 1, AWS: &AWSConfig{Region: "us-east-1"},
				OnePass: &OnePassConfig{Vault: "Dev"}, DefaultBackend: Backend1Pass,
			},
			env:  envPtr(NewEnvironment(IncludeEntry{Secret: "test", Backend: BackendAWS})),
			want: BackendAWS,
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

// envPtr returns a pointer to an Environment.
func envPtr(e Environment) *Environment { return &e }

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
			env:         envPtr(NewEnvironment(IncludeEntry{Secret: "test"})),
			wantRegion:  "us-east-1",
			wantProfile: "default",
		},
		{
			name:        "env only",
			config:      &Config{Version: 1},
			env:         envPtr(NewEnvironment(IncludeEntry{Secret: "test", AWS: &AWSConfig{Region: "eu-west-1"}})),
			wantRegion:  "eu-west-1",
			wantProfile: "",
		},
		{
			name:        "env overrides global region",
			config:      &Config{Version: 1, AWS: &AWSConfig{Region: "us-east-1", Profile: "default"}},
			env:         envPtr(NewEnvironment(IncludeEntry{Secret: "test", AWS: &AWSConfig{Region: "eu-west-1"}})),
			wantRegion:  "eu-west-1",
			wantProfile: "default",
		},
		{
			name:        "neither set",
			config:      &Config{Version: 1},
			env:         envPtr(NewEnvironment(IncludeEntry{Secret: "test"})),
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
			env:         envPtr(NewEnvironment(IncludeEntry{Secret: "test"})),
			wantVault:   "Dev",
			wantAccount: "my-account",
		},
		{
			name:   "env only",
			config: &Config{Version: 1},
			env: envPtr(NewEnvironment(IncludeEntry{
				Secret: "test", OnePass: &OnePassConfig{Vault: "Staging"},
			})),
			wantVault:   "Staging",
			wantAccount: "",
		},
		{
			name:   "env overrides global vault",
			config: &Config{Version: 1, OnePass: &OnePassConfig{Vault: "Dev", Account: "my-account"}},
			env: envPtr(NewEnvironment(IncludeEntry{
				Secret: "test", OnePass: &OnePassConfig{Vault: "Staging"},
			})),
			wantVault:   "Staging",
			wantAccount: "my-account",
		},
		{
			name:        "neither set",
			config:      &Config{Version: 1},
			env:         envPtr(NewEnvironment(IncludeEntry{Secret: "test"})),
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

//nolint:gocognit,revive // Comprehensive source list parsing tests
func TestEnvSourceList(t *testing.T) {
	t.Run("parses environment source list correctly", func(t *testing.T) {
		configPath := loadFixture(t, "env_keyed_includes.yaml")
		cfg, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load() unexpected error: %v", err)
		}

		devEnv := cfg.Environments["dev"]
		if len(devEnv.Sources) != 3 {
			t.Fatalf("expected 3 dev sources, got %d", len(devEnv.Sources))
		}
		if devEnv.Sources[0].Secret != "myapp/dev" {
			t.Errorf("expected first dev source secret = %q, got %q", "myapp/dev", devEnv.Sources[0].Secret)
		}
		if devEnv.Sources[1].Secret != "shared/datadog" {
			t.Errorf("expected second dev source secret = %q, got %q", "shared/datadog", devEnv.Sources[1].Secret)
		}
		if devEnv.Sources[2].Secret != "shared/stripe" {
			t.Errorf("expected third dev source secret = %q, got %q", "shared/stripe", devEnv.Sources[2].Secret)
		}
		if devEnv.Sources[2].Key != "api_key" {
			t.Errorf("expected third dev source key = %q, got %q", "api_key", devEnv.Sources[2].Key)
		}
		if devEnv.Sources[2].As != "STRIPE_KEY" {
			t.Errorf("expected third dev source as = %q, got %q", "STRIPE_KEY", devEnv.Sources[2].As)
		}

		stagingEnv := cfg.Environments["staging"]
		if len(stagingEnv.Sources) != 2 {
			t.Fatalf("expected 2 staging sources, got %d", len(stagingEnv.Sources))
		}
		if stagingEnv.Sources[0].Secret != "myapp/staging" {
			t.Errorf("expected staging source[0] secret = %q, got %q", "myapp/staging", stagingEnv.Sources[0].Secret)
		}
		if stagingEnv.Sources[1].Secret != "shared/monitoring" {
			t.Errorf("expected staging source[1] secret = %q, got %q", "shared/monitoring", stagingEnv.Sources[1].Secret)
		}
	})

	t.Run("source entry with aws config parses correctly", func(t *testing.T) {
		configPath := loadFixture(t, "include_with_aws.yaml")
		cfg, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load() unexpected error: %v", err)
		}

		devEnv := cfg.Environments["dev"]
		if len(devEnv.Sources) != 2 {
			t.Fatalf("expected 2 dev sources, got %d", len(devEnv.Sources))
		}
		if devEnv.Sources[1].AWS == nil {
			t.Fatal("expected AWS config on source entry, got nil")
		}
		if devEnv.Sources[1].AWS.Region != "eu-west-1" {
			t.Errorf("expected AWS region = %q, got %q", "eu-west-1", devEnv.Sources[1].AWS.Region)
		}
		if devEnv.Sources[1].AWS.Profile != "datadog-profile" {
			t.Errorf("expected AWS profile = %q, got %q", "datadog-profile", devEnv.Sources[1].AWS.Profile)
		}
	})

	t.Run("source entry with 1pass config parses correctly", func(t *testing.T) {
		configPath := loadFixture(t, "include_with_1pass.yaml")
		cfg, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load() unexpected error: %v", err)
		}

		devEnv := cfg.Environments["dev"]
		if len(devEnv.Sources) != 2 {
			t.Fatalf("expected 2 dev sources, got %d", len(devEnv.Sources))
		}
		if devEnv.Sources[1].OnePass == nil {
			t.Fatal("expected OnePass config on source entry, got nil")
		}
		if devEnv.Sources[1].OnePass.Vault != "shared-vault" {
			t.Errorf("expected OnePass vault = %q, got %q", "shared-vault", devEnv.Sources[1].OnePass.Vault)
		}
		if devEnv.Sources[1].OnePass.Account != "team-account" {
			t.Errorf("expected OnePass account = %q, got %q", "team-account", devEnv.Sources[1].OnePass.Account)
		}
	})

	t.Run("source entry with both aws and 1pass fails validation", func(t *testing.T) {
		configPath := loadFixture(t, "include_both_backends.yaml")
		_, err := Load(configPath)
		if err == nil {
			t.Fatal("Load() expected error for source with both aws and 1pass, got nil")
		}
		if !containsSubstring(err.Error(), "cannot specify both 'aws' and '1pass'") {
			t.Errorf("Load() error = %q, want containing %q",
				err.Error(), "cannot specify both 'aws' and '1pass'")
		}
	})
}

//nolint:revive // Comprehensive source keys parsing tests
func TestSourceWithKeys(t *testing.T) {
	t.Run("parses source with keys list", func(t *testing.T) {
		configPath := loadFixture(t, "include_with_keys.yaml")
		cfg, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load() unexpected error: %v", err)
		}

		devEnv := cfg.Environments["dev"]
		if len(devEnv.Sources) != 2 {
			t.Fatalf("expected 2 dev sources, got %d", len(devEnv.Sources))
		}

		src := devEnv.Sources[1]
		if src.Secret != "dev/app/secrets" {
			t.Errorf("expected secret = %q, got %q", "dev/app/secrets", src.Secret)
		}
		if src.AWS == nil || src.AWS.Region != "us-east-1" {
			t.Error("expected AWS config with region us-east-1")
		}
		if len(src.Keys) != 2 {
			t.Fatalf("expected 2 keys, got %d", len(src.Keys))
		}
		if src.Keys[0].Key != "database_host" || src.Keys[0].As != "DATABASE_HOST" {
			t.Errorf("keys[0] = %+v, want key=database_host as=DATABASE_HOST", src.Keys[0])
		}
		if src.Keys[1].Key != "database_password" || src.Keys[1].As != "DATABASE_PASSWORD" {
			t.Errorf("keys[1] = %+v, want key=database_password as=DATABASE_PASSWORD", src.Keys[1])
		}
	})

	t.Run("key and keys conflict fails validation", func(t *testing.T) {
		configPath := loadFixture(t, "include_keys_conflict.yaml")
		_, err := Load(configPath)
		if err == nil {
			t.Fatal("Load() expected error, got nil")
		}
		if !containsSubstring(err.Error(), "cannot specify both 'key' and 'keys'") {
			t.Errorf("Load() error = %q, want containing %q", err.Error(), "cannot specify both 'key' and 'keys'")
		}
	})

	t.Run("empty key in keys list fails validation", func(t *testing.T) {
		configPath := loadFixture(t, "include_keys_empty_key.yaml")
		_, err := Load(configPath)
		if err == nil {
			t.Fatal("Load() expected error, got nil")
		}
		if !containsSubstring(err.Error(), "missing required 'key' field") {
			t.Errorf("Load() error = %q, want containing %q", err.Error(), "missing required 'key' field")
		}
	})
}

func TestValidateSourceEntries(t *testing.T) {
	t.Run("global source entry with empty secret", func(t *testing.T) {
		configPath := loadFixture(t, "include_empty_secret.yaml")
		_, err := Load(configPath)
		if err == nil {
			t.Fatal("Load() expected error, got nil")
		}
		if !containsSubstring(err.Error(), "missing required 'secret' field") {
			t.Errorf("Load() error = %q, want error containing %q", err.Error(), "missing required 'secret' field")
		}
	})

	t.Run("application source entry with empty secret", func(t *testing.T) {
		configPath := loadFixture(t, "app_include_empty_secret.yaml")
		_, err := Load(configPath)
		if err == nil {
			t.Fatal("Load() expected error, got nil")
		}
		if !containsSubstring(err.Error(), "missing required 'secret' field") {
			t.Errorf("Load() error = %q, want error containing %q", err.Error(), "missing required 'secret' field")
		}
	})
}

func TestMixedFormat(t *testing.T) {
	t.Run("some envs use legacy mapping, some use list", func(t *testing.T) {
		configPath := loadFixture(t, "env_mixed_format.yaml")
		cfg, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load() unexpected error: %v", err)
		}

		// local uses legacy mapping format
		localEnv := cfg.Environments["local"]
		if len(localEnv.Sources) != 1 {
			t.Fatalf("expected 1 local source, got %d", len(localEnv.Sources))
		}
		if localEnv.Secret() != "myapp/local" {
			t.Errorf("local secret = %q, want %q", localEnv.Secret(), "myapp/local")
		}

		// dev uses list format
		devEnv := cfg.Environments["dev"]
		if len(devEnv.Sources) != 2 {
			t.Fatalf("expected 2 dev sources, got %d", len(devEnv.Sources))
		}
		if devEnv.Secret() != "myapp/dev" {
			t.Errorf("dev secret = %q, want %q", devEnv.Secret(), "myapp/dev")
		}
		if devEnv.Sources[1].Key != "api_key" {
			t.Errorf("dev source[1] key = %q, want %q", devEnv.Sources[1].Key, "api_key")
		}
	})
}

//nolint:revive // Comprehensive backend field parsing tests
func TestBackendField(t *testing.T) {
	t.Run("backend aws parses correctly on non-first source", func(t *testing.T) {
		configPath := loadFixture(t, "backend_field_aws.yaml")
		cfg, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load() unexpected error: %v", err)
		}

		devEnv := cfg.Environments["dev"]
		if len(devEnv.Sources) != 2 {
			t.Fatalf("expected 2 sources, got %d", len(devEnv.Sources))
		}
		src := devEnv.Sources[1]
		if src.Backend != BackendAWS {
			t.Errorf("expected backend = %q, got %q", BackendAWS, src.Backend)
		}
		// Non-first sources are NOT promoted at parse time; promotion
		// happens at resolution time in clientForInclude/clientForValidate.
		if src.Key != "api_key" {
			t.Errorf("expected key = %q, got %q", "api_key", src.Key)
		}
	})

	t.Run("backend 1pass parses correctly on non-first source", func(t *testing.T) {
		configPath := loadFixture(t, "backend_field_1pass.yaml")
		cfg, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load() unexpected error: %v", err)
		}

		devEnv := cfg.Environments["dev"]
		src := devEnv.Sources[1]
		if src.Backend != Backend1Pass {
			t.Errorf("expected backend = %q, got %q", Backend1Pass, src.Backend)
		}
	})

	t.Run("backend field conflict with opposite block", func(t *testing.T) {
		configPath := loadFixture(t, "backend_field_conflict.yaml")
		_, err := Load(configPath)
		if err == nil {
			t.Fatal("Load() expected error, got nil")
		}
		if !containsSubstring(err.Error(), "has backend") {
			t.Errorf("Load() error = %q, want error about backend conflict", err.Error())
		}
	})

	t.Run("invalid backend value", func(t *testing.T) {
		configPath := loadFixture(t, "backend_field_invalid.yaml")
		_, err := Load(configPath)
		if err == nil {
			t.Fatal("Load() expected error, got nil")
		}
		if !containsSubstring(err.Error(), "invalid backend value") {
			t.Errorf("Load() error = %q, want error about invalid backend value", err.Error())
		}
	})

	t.Run("legacy mapping format with backend field", func(t *testing.T) {
		configPath := loadFixture(t, "backend_field_legacy.yaml")
		cfg, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load() unexpected error: %v", err)
		}

		devEnv := cfg.Environments["dev"]
		if devEnv.AWS == nil {
			t.Fatal("expected env.AWS to be promoted from backend field")
		}
		if devEnv.Sources[0].Backend != BackendAWS {
			t.Errorf("expected source backend = %q, got %q", BackendAWS, devEnv.Sources[0].Backend)
		}
	})
}

func TestResolveBackend_WithBackendField(t *testing.T) {
	t.Run("backend aws on first source resolves to AWS", func(t *testing.T) {
		cfg := &Config{Version: 1}
		env := envPtr(NewEnvironment(IncludeEntry{Secret: "test", Backend: BackendAWS}))
		got := cfg.ResolveBackend(env)
		if got != BackendAWS {
			t.Errorf("ResolveBackend() = %q, want %q", got, BackendAWS)
		}
	})

	t.Run("backend 1pass on first source resolves to 1Pass", func(t *testing.T) {
		cfg := &Config{Version: 1}
		env := envPtr(NewEnvironment(IncludeEntry{Secret: "test", Backend: Backend1Pass}))
		got := cfg.ResolveBackend(env)
		if got != Backend1Pass {
			t.Errorf("ResolveBackend() = %q, want %q", got, Backend1Pass)
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
			config:     &Config{Version: 1, IncludeAll: boolPtr(true)},
			wantResult: true,
		},
		{
			name:       "global_false_explicit",
			config:     &Config{Version: 1, IncludeAll: boolPtr(false)},
			wantResult: false,
		},
		{
			name:       "app_overrides_global_true",
			config:     &Config{Version: 1, IncludeAll: boolPtr(false)},
			app:        &Application{IncludeAll: boolPtr(true)},
			wantResult: true,
		},
		{
			name:       "app_overrides_global_false",
			config:     &Config{Version: 1, IncludeAll: boolPtr(true)},
			app:        &Application{IncludeAll: boolPtr(false)},
			wantResult: false,
		},
		{
			name:   "env_overrides_app_true",
			config: &Config{Version: 1, IncludeAll: boolPtr(false)},
			app:    &Application{IncludeAll: boolPtr(false)},
			env: &Environment{
				Sources:    []IncludeEntry{{Secret: "test"}},
				IncludeAll: boolPtr(true),
			},
			wantResult: true,
		},
		{
			name:   "env_overrides_app_false",
			config: &Config{Version: 1, IncludeAll: boolPtr(true)},
			app:    &Application{IncludeAll: boolPtr(true)},
			env: &Environment{
				Sources:    []IncludeEntry{{Secret: "test"}},
				IncludeAll: boolPtr(false),
			},
			wantResult: false,
		},
		{
			name:   "env_overrides_global_no_app",
			config: &Config{Version: 1, IncludeAll: boolPtr(false)},
			env: &Environment{
				Sources:    []IncludeEntry{{Secret: "test"}},
				IncludeAll: boolPtr(true),
			},
			wantResult: true,
		},
		{
			name:       "app_nil_inherits_global",
			config:     &Config{Version: 1, IncludeAll: boolPtr(true)},
			app:        nil,
			env:        envPtr(NewEnvironment(IncludeEntry{Secret: "test"})),
			wantResult: true,
		},
		{
			name:       "env_nil_inherits_app",
			config:     &Config{Version: 1, IncludeAll: boolPtr(false)},
			app:        &Application{IncludeAll: boolPtr(true)},
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
