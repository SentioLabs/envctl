//nolint:testpackage // Testing internal functions requires same package
package onepassword

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseReference(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      *Reference
		wantErr   bool
		errSubstr string
	}{
		{
			name:  "item only",
			input: "MyItem",
			want: &Reference{
				Item: "MyItem",
			},
		},
		{
			name:  "vault/item",
			input: "MyVault/MyItem",
			want: &Reference{
				Vault: "MyVault",
				Item:  "MyItem",
			},
		},
		{
			name:  "vault/item/field",
			input: "MyVault/MyItem/password",
			want: &Reference{
				Vault: "MyVault",
				Item:  "MyItem",
				Field: "password",
			},
		},
		{
			name:  "vault/item/section/field",
			input: "MyVault/MyItem/login/password",
			want: &Reference{
				Vault:   "MyVault",
				Item:    "MyItem",
				Section: "login",
				Field:   "password",
			},
		},
		{
			name:  "op:// prefix - item only",
			input: "op://MyItem",
			want: &Reference{
				Item: "MyItem",
			},
		},
		{
			name:  "op:// prefix - vault/item",
			input: "op://MyVault/MyItem",
			want: &Reference{
				Vault: "MyVault",
				Item:  "MyItem",
			},
		},
		{
			name:  "op:// prefix - vault/item/field",
			input: "op://MyVault/MyItem/password",
			want: &Reference{
				Vault: "MyVault",
				Item:  "MyItem",
				Field: "password",
			},
		},
		{
			name:  "op:// prefix - vault/item/section/field",
			input: "op://Development/API Keys/stripe/secret_key",
			want: &Reference{
				Vault:   "Development",
				Item:    "API Keys",
				Section: "stripe",
				Field:   "secret_key",
			},
		},
		{
			name:      "empty string",
			input:     "",
			wantErr:   true,
			errSubstr: "empty reference",
		},
		{
			name:      "op:// only",
			input:     "op://",
			wantErr:   true,
			errSubstr: "empty reference",
		},
		{
			name:      "empty vault",
			input:     "/MyItem",
			wantErr:   true,
			errSubstr: "invalid reference format",
		},
		{
			name:      "empty item",
			input:     "MyVault/",
			wantErr:   true,
			errSubstr: "invalid reference format",
		},
		{
			name:      "too many parts",
			input:     "a/b/c/d/e",
			wantErr:   true,
			errSubstr: "invalid reference format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseReference(tt.input)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errSubstr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want.Vault, got.Vault, "Vault mismatch")
			assert.Equal(t, tt.want.Item, got.Item, "Item mismatch")
			assert.Equal(t, tt.want.Section, got.Section, "Section mismatch")
			assert.Equal(t, tt.want.Field, got.Field, "Field mismatch")
		})
	}
}

func TestReference_String(t *testing.T) {
	tests := []struct {
		name string
		ref  Reference
		want string
	}{
		{
			name: "item only",
			ref:  Reference{Item: "MyItem"},
			want: "op://MyItem",
		},
		{
			name: "vault/item",
			ref:  Reference{Vault: "MyVault", Item: "MyItem"},
			want: "op://MyVault/MyItem",
		},
		{
			name: "vault/item/field",
			ref:  Reference{Vault: "MyVault", Item: "MyItem", Field: "password"},
			want: "op://MyVault/MyItem/password",
		},
		{
			name: "vault/item/section/field",
			ref:  Reference{Vault: "MyVault", Item: "MyItem", Section: "login", Field: "password"},
			want: "op://MyVault/MyItem/login/password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ref.String()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestReference_HasField(t *testing.T) {
	tests := []struct {
		name string
		ref  Reference
		want bool
	}{
		{
			name: "no field",
			ref:  Reference{Vault: "v", Item: "i"},
			want: false,
		},
		{
			name: "has field",
			ref:  Reference{Vault: "v", Item: "i", Field: "f"},
			want: true,
		},
		{
			name: "empty field",
			ref:  Reference{Vault: "v", Item: "i", Field: ""},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.ref.HasField())
		})
	}
}

func TestReference_ItemRef(t *testing.T) {
	tests := []struct {
		name string
		ref  Reference
		want string
	}{
		{
			name: "item only",
			ref:  Reference{Item: "MyItem"},
			want: "MyItem",
		},
		{
			name: "vault/item",
			ref:  Reference{Vault: "MyVault", Item: "MyItem"},
			want: "MyVault/MyItem",
		},
		{
			name: "vault/item/field - returns without field",
			ref:  Reference{Vault: "MyVault", Item: "MyItem", Field: "password"},
			want: "MyVault/MyItem",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.ref.ItemRef())
		})
	}
}

func TestReference_CLIArgs(t *testing.T) {
	tests := []struct {
		name string
		ref  Reference
		want []string
	}{
		{
			name: "item only - uses item get",
			ref:  Reference{Item: "MyItem"},
			want: []string{"item", "get", "MyItem", "--format", "json"},
		},
		{
			name: "vault/item - uses item get with vault",
			ref:  Reference{Vault: "MyVault", Item: "MyItem"},
			want: []string{"item", "get", "MyItem", "--format", "json", "--vault", "MyVault"},
		},
		{
			name: "with field - uses read",
			ref:  Reference{Vault: "MyVault", Item: "MyItem", Field: "password"},
			want: []string{"read", "op://MyVault/MyItem/password"},
		},
		{
			name: "with section and field - uses read",
			ref:  Reference{Vault: "MyVault", Item: "MyItem", Section: "login", Field: "password"},
			want: []string{"read", "op://MyVault/MyItem/login/password"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ref.CLIArgs()
			assert.Equal(t, tt.want, got)
		})
	}
}
