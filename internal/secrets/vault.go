package secrets

// Vault represents a secret vault (e.g., a 1Password vault).
type Vault struct {
	// ID is the unique identifier for the vault.
	ID string
	// Name is the human-readable name for the vault.
	Name string
}
