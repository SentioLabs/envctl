package secrets_test

import (
	"testing"

	"github.com/sentiolabs/envctl/internal/mocks"
	"github.com/sentiolabs/envctl/internal/secrets"
)

// --- Contract assertions ---
// These verify the file-sink design contracts. Do NOT modify
// without updating the approved plan.

var (
	_ secrets.RawReader = (*mocks.MockRawReader)(nil)
	_ secrets.Client    = (*mocks.MockRawClient)(nil)
	_ secrets.RawReader = (*mocks.MockRawClient)(nil)
)

func TestRawReaderContract(t *testing.T) {
	var c secrets.Client = mocks.NewMockRawClient(t)
	if _, ok := c.(secrets.RawReader); !ok {
		t.Fatal("MockRawClient must satisfy RawReader through a Client value")
	}
	var plain secrets.Client = mocks.NewMockClient(t)
	if _, ok := plain.(secrets.RawReader); ok {
		t.Fatal("MockClient must not satisfy RawReader")
	}
}
