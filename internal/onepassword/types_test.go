//nolint:testpackage // Testing internal functions requires same package
package onepassword

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func getJSONTag(t *testing.T, typ reflect.Type, fieldName string) string {
	t.Helper()
	field, ok := typ.FieldByName(fieldName)
	require.True(t, ok, "field %s not found on %s", fieldName, typ.Name())
	return field.Tag.Get("json")
}

func TestOmitzeroTags_SliceAndStructPointerFields(t *testing.T) {
	t.Run("Item.Sections slice uses omitzero", func(t *testing.T) {
		tag := getJSONTag(t, reflect.TypeOf(Item{}), "Sections")
		assert.Equal(t, "sections,omitzero", tag,
			"[]Section field should use omitzero, not omitempty")
	})

	t.Run("Field.Section struct pointer uses omitzero", func(t *testing.T) {
		tag := getJSONTag(t, reflect.TypeOf(Field{}), "Section")
		assert.Equal(t, "section,omitzero", tag,
			"*struct field should use omitzero, not omitempty")
	})
}

func TestOmitemptyTags_StringFieldsUnchanged(t *testing.T) {
	t.Run("VaultRef.Name string keeps omitempty", func(t *testing.T) {
		tag := getJSONTag(t, reflect.TypeOf(VaultRef{}), "Name")
		assert.Equal(t, "name,omitempty", tag,
			"string field should keep omitempty")
	})

	t.Run("Section.Label string keeps omitempty", func(t *testing.T) {
		tag := getJSONTag(t, reflect.TypeOf(Section{}), "Label")
		assert.Equal(t, "label,omitempty", tag,
			"string field should keep omitempty")
	})

	t.Run("Field inner Section.Label string keeps omitempty", func(t *testing.T) {
		// The Section field on Field is an anonymous struct pointer.
		// We need to check the Label field inside it.
		sectionField, ok := reflect.TypeOf(Field{}).FieldByName("Section")
		require.True(t, ok, "Section field not found on Field")
		innerType := sectionField.Type.Elem() // dereference the pointer
		labelField, ok := innerType.FieldByName("Label")
		require.True(t, ok, "Label field not found on Field.Section inner struct")
		assert.Equal(t, "label,omitempty", labelField.Tag.Get("json"),
			"string field inside Section struct pointer should keep omitempty")
	})
}
