//nolint:testpackage // Testing internal functions requires same package
package onepassword

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestItem_ToMap_ExcludePurposes(t *testing.T) {
	item := &Item{
		Fields: []Field{
			{Label: "username", Value: "admin", Type: "STRING", Purpose: "USERNAME"},
			{Label: "password", Value: "secret123", Type: "CONCEALED", Purpose: "PASSWORD"},
			{Label: "notes", Value: "some notes", Type: "STRING", Purpose: "NOTES"},
			{Label: "api_key", Value: "key-abc", Type: "CONCEALED", Purpose: ""},
		},
	}

	filter := FieldFilter{
		IncludeConcealed: true,
		IncludeStrings:   true,
		IncludeOther:     true,
		ExcludePurposes:  []string{"NOTES", "USERNAME"},
	}

	result := item.ToMap(filter)

	assert.Equal(t, "secret123", result["password"], "PASSWORD purpose should not be excluded")
	assert.Equal(t, "key-abc", result["api_key"], "empty purpose should not be excluded")
	assert.NotContains(t, result, "notes", "NOTES purpose should be excluded")
	assert.NotContains(t, result, "username", "USERNAME purpose should be excluded")
}

func TestItem_ToMap_DefaultFilter(t *testing.T) {
	item := &Item{
		Fields: []Field{
			{Label: "password", Value: "secret123", Type: "CONCEALED", Purpose: "PASSWORD"},
			{Label: "notes", Value: "some notes", Type: "STRING", Purpose: "NOTES"},
			{Label: "api_key", Value: "key-abc", Type: "STRING", Purpose: ""},
		},
	}

	filter := DefaultFieldFilter()
	result := item.ToMap(filter)

	assert.Equal(t, "secret123", result["password"])
	assert.Equal(t, "key-abc", result["api_key"])
	assert.NotContains(t, result, "notes", "NOTES should be excluded by default filter")
}

func TestItem_ToMap_EmptyExcludePurposes(t *testing.T) {
	item := &Item{
		Fields: []Field{
			{Label: "notes", Value: "some notes", Type: "STRING", Purpose: "NOTES"},
		},
	}

	filter := FieldFilter{
		IncludeConcealed: true,
		IncludeStrings:   true,
		IncludeOther:     true,
		ExcludePurposes:  []string{},
	}

	result := item.ToMap(filter)
	assert.Equal(t, "some notes", result["notes"], "nothing should be excluded with empty ExcludePurposes")
}
