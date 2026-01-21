package identity

import "strings"

// PlaceholderNames are values that indicate missing/test data
var PlaceholderNames = map[string]bool{
	"":          true,
	"unknown":   true,
	"no name":   true,
	"noname":    true,
	"anonymous": true,
	"test":      true,
	"patient":   true,
}

// PlaceholderDOBs are values that indicate missing/test DOB data
var PlaceholderDOBs = map[string]bool{
	"":         true,
	"00000000": true,
	"11111111": true,
	"19000101": true,
	"99999999": true,
}

// IsValidIdentity checks if name and DOB are real values, not placeholders.
func IsValidIdentity(name, dob string) bool {
	nameNormalized := strings.ToLower(NormalizeName(name))
	dobStr := strings.TrimSpace(dob)

	// Check if name is placeholder or too short
	if PlaceholderNames[nameNormalized] || len(nameNormalized) < 3 {
		return false
	}

	// Check if DOB is placeholder or wrong length
	if PlaceholderDOBs[dobStr] || len(dobStr) != 8 {
		return false
	}

	return true
}
