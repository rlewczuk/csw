package shared

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// GenerateUUIDv7 generates a UUIDv7 string.
// UUIDv7 is sortable by time and uses 48-bit timestamp in milliseconds since Unix epoch.
// Format: xxxxxxxx-xxxx-7xxx-xxxx-xxxxxxxxxxxx where 7 indicates version 7.
func GenerateUUIDv7() string {
	var uuid [16]byte

	// Get current time in milliseconds since Unix epoch (48 bits)
	now := time.Now().UnixMilli()

	// Fill first 6 bytes with timestamp (48 bits)
	uuid[0] = byte(now >> 40)
	uuid[1] = byte(now >> 32)
	uuid[2] = byte(now >> 24)
	uuid[3] = byte(now >> 16)
	uuid[4] = byte(now >> 8)
	uuid[5] = byte(now)

	// Fill remaining 10 bytes with random data
	randBytes := make([]byte, 10)
	rand.Read(randBytes)
	copy(uuid[6:], randBytes)

	// Set version (4 bits) to 7 (0111)
	// This affects byte 6, bits 4-7
	uuid[6] = (uuid[6] & 0x0F) | 0x70

	// Set variant (2 bits) to 10
	// This affects byte 8, bits 6-7
	uuid[8] = (uuid[8] & 0x3F) | 0x80

	// Format as standard UUID string: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	return formatUUID(uuid)
}

// formatUUID formats a 16-byte array as a UUID string.
func formatUUID(uuid [16]byte) string {
	buf := make([]byte, 36)
	hex.Encode(buf[0:8], uuid[0:4])
	buf[8] = '-'
	hex.Encode(buf[9:13], uuid[4:6])
	buf[13] = '-'
	hex.Encode(buf[14:18], uuid[6:8])
	buf[18] = '-'
	hex.Encode(buf[19:23], uuid[8:10])
	buf[23] = '-'
	hex.Encode(buf[24:36], uuid[10:16])
	return string(buf)
}
