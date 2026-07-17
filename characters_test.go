package main

import "testing"

func TestCanonicalCharacterType(t *testing.T) {
	tests := map[string]string{
		"Chud":      CharacterTypeGoon,
		"goon":      CharacterTypeGoon,
		"Harana":    CharacterTypeHarana,
		"barkada":   CharacterTypeBarkada,
		"Lockin":    CharacterTypeLockedIn,
		"lock_in":   CharacterTypeLockedIn,
		"Locked-In": CharacterTypeLockedIn,
		"locked in": CharacterTypeLockedIn,
	}

	for input, expected := range tests {
		t.Run(input, func(t *testing.T) {
			if actual := canonicalCharacterType(input); actual != expected {
				t.Fatalf("canonicalCharacterType(%q) = %q, want %q", input, actual, expected)
			}
		})
	}
}
