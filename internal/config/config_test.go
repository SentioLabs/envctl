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
    region: us-west-2
  staging:
    secret: myapp/staging
include:
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, ConfigFileName)
			if err := os.WriteFile(configPath, []byte(tt.content), 0644); err != nil {
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
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create dirs: %v", err)
	}

	// Create config in the root
	configPath := filepath.Join(tmpDir, ConfigFileName)
	content := `version: 1
environments:
  dev:
    secret: test/dev
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
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
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
