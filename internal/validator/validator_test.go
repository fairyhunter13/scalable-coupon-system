package validator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNew verifies that New() returns a properly configured validator
func TestNew(t *testing.T) {
	v := New()
	require.NotNil(t, v, "New() should return a non-nil validator")
}

// TestNotblankValidator tests the custom notblank validation
func TestNotblankValidator(t *testing.T) {
	v := New()

	type TestStruct struct {
		Name string `validate:"notblank"`
	}

	testCases := []struct {
		name        string
		input       string
		expectError bool
		description string
	}{
		{
			name:        "valid_string",
			input:       "valid",
			expectError: false,
			description: "Normal string should pass",
		},
		{
			name:        "valid_with_spaces",
			input:       "  valid  ",
			expectError: false,
			description: "String with leading/trailing spaces should pass (has content)",
		},
		{
			name:        "whitespace_only_spaces",
			input:       "   ",
			expectError: true,
			description: "Whitespace-only (spaces) should fail",
		},
		{
			name:        "whitespace_only_tabs",
			input:       "\t\t",
			expectError: true,
			description: "Whitespace-only (tabs) should fail",
		},
		{
			name:        "whitespace_only_newlines",
			input:       "\n\n",
			expectError: true,
			description: "Whitespace-only (newlines) should fail",
		},
		{
			name:        "whitespace_mixed",
			input:       " \t\n ",
			expectError: true,
			description: "Mixed whitespace-only should fail",
		},
		{
			name:        "empty_string",
			input:       "",
			expectError: true,
			description: "Empty string should fail (TrimSpace returns empty)",
		},
		{
			name:        "single_char",
			input:       "a",
			expectError: false,
			description: "Single character should pass",
		},
		{
			name:        "unicode_content",
			input:       "日本語",
			expectError: false,
			description: "Unicode content should pass",
		},
		{
			name:        "unicode_with_whitespace",
			input:       "  日本語  ",
			expectError: false,
			description: "Unicode with whitespace padding should pass",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ts := TestStruct{Name: tc.input}
			err := v.Struct(ts)

			if tc.expectError {
				assert.Error(t, err, tc.description)
			} else {
				assert.NoError(t, err, tc.description)
			}
		})
	}
}

// TestNotblankCombinedWithRequired tests notblank combined with required tag
func TestNotblankCombinedWithRequired(t *testing.T) {
	v := New()

	type TestStruct struct {
		Name string `validate:"required,notblank"`
	}

	testCases := []struct {
		name        string
		input       string
		expectError bool
	}{
		{"valid", "valid", false},
		{"whitespace_only", "   ", true},
		{"empty", "", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ts := TestStruct{Name: tc.input}
			err := v.Struct(ts)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestNotblankWithMaxLength tests notblank combined with max length tag
func TestNotblankWithMaxLength(t *testing.T) {
	v := New()

	type TestStruct struct {
		Name string `validate:"required,notblank,max=10"`
	}

	testCases := []struct {
		name        string
		input       string
		expectError bool
	}{
		{"valid_short", "valid", false},
		{"valid_max_length", "1234567890", false},
		{"exceeds_max", "12345678901", true},
		{"whitespace_only", "   ", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ts := TestStruct{Name: tc.input}
			err := v.Struct(ts)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestNotblankOnNonStringField tests that notblank handles non-string fields gracefully
func TestNotblankOnNonStringField(t *testing.T) {
	v := New()

	// notblank on int should pass (returns true for non-string types)
	type TestStructInt struct {
		Value int `validate:"notblank"`
	}

	ts := TestStructInt{Value: 0}
	err := v.Struct(ts)
	assert.NoError(t, err, "notblank should pass for non-string types")
}
