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
			name:  caseItemOnly,
			input: testItem,
			want: &Reference{
				Item: testItem,
			},
		},
		{
			name:  caseVaultItem,
			input: testRefVaultItem,
			want: &Reference{
				Vault: testVault,
				Item:  testItem,
			},
		},
		{
			name:  "vault/item/field",
			input: "MyVault/MyItem/password",
			want: &Reference{
				Vault: testVault,
				Item:  testItem,
				Field: testFieldPassword,
			},
		},
		{
			name:  "vault/item/section/field",
			input: "MyVault/MyItem/login/password",
			want: &Reference{
				Vault:   testVault,
				Item:    testItem,
				Section: testSectionLogin,
				Field:   testFieldPassword,
			},
		},
		{
			name:  "op:// prefix - item only",
			input: "op://MyItem",
			want: &Reference{
				Item: testItem,
			},
		},
		{
			name:  "op:// prefix - vault/item",
			input: "op://MyVault/MyItem",
			want: &Reference{
				Vault: testVault,
				Item:  testItem,
			},
		},
		{
			name:  "op:// prefix - vault/item/field",
			input: testRefFull,
			want: &Reference{
				Vault: testVault,
				Item:  testItem,
				Field: testFieldPassword,
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
			errSubstr: msgInvalidRefFormat,
		},
		{
			name:      "empty item",
			input:     "MyVault/",
			wantErr:   true,
			errSubstr: msgInvalidRefFormat,
		},
		{
			name:      "too many parts",
			input:     "a/b/c/d/e",
			wantErr:   true,
			errSubstr: msgInvalidRefFormat,
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
			name: caseItemOnly,
			ref:  Reference{Item: testItem},
			want: "op://MyItem",
		},
		{
			name: caseVaultItem,
			ref:  Reference{Vault: testVault, Item: testItem},
			want: "op://MyVault/MyItem",
		},
		{
			name: "vault/item/field",
			ref:  Reference{Vault: testVault, Item: testItem, Field: testFieldPassword},
			want: testRefFull,
		},
		{
			name: "vault/item/section/field",
			ref:  Reference{Vault: testVault, Item: testItem, Section: testSectionLogin, Field: testFieldPassword},
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
			name: caseItemOnly,
			ref:  Reference{Item: testItem},
			want: testItem,
		},
		{
			name: caseVaultItem,
			ref:  Reference{Vault: testVault, Item: testItem},
			want: testRefVaultItem,
		},
		{
			name: "vault/item/field - returns without field",
			ref:  Reference{Vault: testVault, Item: testItem, Field: testFieldPassword},
			want: testRefVaultItem,
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
			ref:  Reference{Item: testItem},
			want: []string{cmdItem, "get", testItem, flagFormat, formatJSON},
		},
		{
			name: "vault/item - uses item get with vault",
			ref:  Reference{Vault: testVault, Item: testItem},
			want: []string{cmdItem, "get", testItem, flagFormat, formatJSON, flagVault, testVault},
		},
		{
			name: "with field - uses read",
			ref:  Reference{Vault: testVault, Item: testItem, Field: testFieldPassword},
			want: []string{"read", testRefFull},
		},
		{
			name: "with section and field - uses read",
			ref:  Reference{Vault: testVault, Item: testItem, Section: testSectionLogin, Field: testFieldPassword},
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
