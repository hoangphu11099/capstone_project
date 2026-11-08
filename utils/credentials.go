package utils

import (
	"crypto/sha256"
	"encoding/hex"
)

// Binding a session to the stored password hash invalidates it on password changes.
func CredentialFingerprint(passwordHash string) string {
	sum := sha256.Sum256([]byte(passwordHash))
	return hex.EncodeToString(sum[:])
}
