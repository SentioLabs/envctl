package config

import (
	"strings"

	"github.com/sentiolabs/envctl/internal/errors"
)

// SecretRef represents a parsed secret reference.
type SecretRef struct {
	SecretName string
	KeyName    string
}

// ParseSecretRef parses a secret reference in the format "secret_name#key_name".
// The key_name part is optional.
func ParseSecretRef(ref string) (*SecretRef, error) {
	if ref == "" {
		return nil, &errors.SecretRefError{
			Ref:     ref,
			Message: "empty reference",
		}
	}

	parts := strings.SplitN(ref, "#", 2)

	secretName := strings.TrimSpace(parts[0])
	if secretName == "" {
		return nil, &errors.SecretRefError{
			Ref:     ref,
			Message: "secret name is empty",
		}
	}

	result := &SecretRef{
		SecretName: secretName,
	}

	if len(parts) == 2 {
		keyName := strings.TrimSpace(parts[1])
		if keyName == "" {
			return nil, &errors.SecretRefError{
				Ref:     ref,
				Message: "key name is empty after '#'",
			}
		}
		result.KeyName = keyName
	}

	return result, nil
}

// String returns the string representation of the secret reference.
func (r *SecretRef) String() string {
	if r.KeyName == "" {
		return r.SecretName
	}
	return r.SecretName + "#" + r.KeyName
}
