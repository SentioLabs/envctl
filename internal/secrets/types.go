package secrets

// Item represents a secret item in a vault, independent of the backend.
type Item struct {
	// ID is the unique identifier for the item.
	ID string
	// Name is the display name of the item.
	Name string
}
