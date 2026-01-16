//nolint:testpackage // Testing internal functions requires same package
package config

import "testing"

func TestParseSecretRef(t *testing.T) {
	tests := []struct {
		name       string
		ref        string
		wantSecret string
		wantKey    string
		wantErr    bool
	}{
		{
			name:       "secret only",
			ref:        "myapp/dev",
			wantSecret: "myapp/dev",
			wantKey:    "",
			wantErr:    false,
		},
		{
			name:       "secret with key",
			ref:        "myapp/dev#DATABASE_URL",
			wantSecret: "myapp/dev",
			wantKey:    "DATABASE_URL",
			wantErr:    false,
		},
		{
			name:       "secret with key containing special chars",
			ref:        "shared/third-party#api_key_v2",
			wantSecret: "shared/third-party",
			wantKey:    "api_key_v2",
			wantErr:    false,
		},
		{
			name:    "empty reference",
			ref:     "",
			wantErr: true,
		},
		{
			name:    "empty secret name",
			ref:     "#key",
			wantErr: true,
		},
		{
			name:    "empty key name",
			ref:     "myapp/dev#",
			wantErr: true,
		},
		{
			name:       "with whitespace",
			ref:        " myapp/dev # DATABASE_URL ",
			wantSecret: "myapp/dev",
			wantKey:    "DATABASE_URL",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := ParseSecretRef(tt.ref)
			if tt.wantErr {
				if err == nil {
					t.Error("ParseSecretRef() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ParseSecretRef() unexpected error: %v", err)
				return
			}

			if ref.SecretName != tt.wantSecret {
				t.Errorf("ParseSecretRef() SecretName = %q, want %q", ref.SecretName, tt.wantSecret)
			}

			if ref.KeyName != tt.wantKey {
				t.Errorf("ParseSecretRef() KeyName = %q, want %q", ref.KeyName, tt.wantKey)
			}
		})
	}
}

func TestSecretRefString(t *testing.T) {
	tests := []struct {
		name       string
		secretName string
		keyName    string
		want       string
	}{
		{
			name:       "secret only",
			secretName: "myapp/dev",
			keyName:    "",
			want:       "myapp/dev",
		},
		{
			name:       "secret with key",
			secretName: "myapp/dev",
			keyName:    "DATABASE_URL",
			want:       "myapp/dev#DATABASE_URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref := &SecretRef{
				SecretName: tt.secretName,
				KeyName:    tt.keyName,
			}
			if got := ref.String(); got != tt.want {
				t.Errorf("SecretRef.String() = %q, want %q", got, tt.want)
			}
		})
	}
}
