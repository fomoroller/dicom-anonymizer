package identity

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var nonAlphaRegex = regexp.MustCompile(`[^A-Z\s]`)

// NormalizeName normalizes a patient name for consistent matching.
// Handles: "SMITH^JOHN", "John Smith", "smith, john", etc.
func NormalizeName(name string) string {
	if name == "" {
		return ""
	}

	// Convert to uppercase
	name = strings.ToUpper(name)

	// Replace DICOM separators with spaces
	name = strings.ReplaceAll(name, "^", " ")
	name = strings.ReplaceAll(name, ",", " ")

	// Remove non-alphanumeric characters (keep spaces)
	name = nonAlphaRegex.ReplaceAllString(name, "")

	// Split into parts, sort alphabetically, rejoin
	parts := strings.Fields(name)
	sort.Strings(parts)

	return strings.Join(parts, "")
}

// CreateIdentityHash creates a consistent hash from patient name, DOB, and optional salt.
// Returns uppercase 12-character hex string.
func CreateIdentityHash(name, dob, salt string) string {
	nameNormalized := NormalizeName(name)
	dobStr := strings.TrimSpace(dob)

	identityString := fmt.Sprintf("%s|%s|%s", nameNormalized, dobStr, salt)
	hash := sha256.Sum256([]byte(identityString))
	return strings.ToUpper(hex.EncodeToString(hash[:])[:12])
}
