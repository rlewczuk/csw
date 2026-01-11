package shared

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateUUIDv7(t *testing.T) {
	tests := []struct {
		name string
	}{
		{"Generate single UUID"},
		{"Generate another UUID"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uuid := GenerateUUIDv7()

			// Check length (format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx = 36 chars)
			assert.Len(t, uuid, 36, "UUID should be 36 characters long")

			// Check format with dashes at correct positions
			assert.Equal(t, "-", string(uuid[8]), "Dash should be at position 8")
			assert.Equal(t, "-", string(uuid[13]), "Dash should be at position 13")
			assert.Equal(t, "-", string(uuid[18]), "Dash should be at position 18")
			assert.Equal(t, "-", string(uuid[23]), "Dash should be at position 23")

			// Check version (should be 7)
			// Version is in the 15th character (0-indexed at 14), high nibble should be 7
			assert.Equal(t, "7", string(uuid[14]), "Version field should be 7")

			// Check variant (should be 8, 9, a, or b for RFC 4122 variant)
			variantChar := uuid[19]
			assert.Contains(t, "89ab", string(variantChar), "Variant field should be 8, 9, a, or b")
		})
	}
}

func TestGenerateUUIDv7_Uniqueness(t *testing.T) {
	// Generate multiple UUIDs and check they are unique
	uuids := make(map[string]bool)
	count := 1000

	for i := 0; i < count; i++ {
		uuid := GenerateUUIDv7()
		require.False(t, uuids[uuid], "UUID should be unique, got duplicate: %s", uuid)
		uuids[uuid] = true
	}

	assert.Len(t, uuids, count, "Should have generated %d unique UUIDs", count)
}

func TestGenerateUUIDv7_Sortability(t *testing.T) {
	// Generate UUIDs with a small delay and verify they are sortable by time
	uuid1 := GenerateUUIDv7()
	time.Sleep(2 * time.Millisecond)
	uuid2 := GenerateUUIDv7()
	time.Sleep(2 * time.Millisecond)
	uuid3 := GenerateUUIDv7()

	// UUIDs should be lexicographically sortable by timestamp
	assert.True(t, uuid1 < uuid2, "UUID1 should be less than UUID2")
	assert.True(t, uuid2 < uuid3, "UUID2 should be less than UUID3")
	assert.True(t, uuid1 < uuid3, "UUID1 should be less than UUID3")
}

func TestFormatUUID(t *testing.T) {
	tests := []struct {
		name     string
		input    [16]byte
		expected string
	}{
		{
			name:     "All zeros",
			input:    [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			expected: "00000000-0000-0000-0000-000000000000",
		},
		{
			name:     "All ones",
			input:    [16]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
			expected: "ffffffff-ffff-ffff-ffff-ffffffffffff",
		},
		{
			name:     "Mixed values",
			input:    [16]byte{0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef, 0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef},
			expected: "01234567-89ab-cdef-0123-456789abcdef",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatUUID(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
