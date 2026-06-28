// Package crypto provides password hashing and API token generation utilities
// shared by the server and (for verification symmetry) tests.
package crypto

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// TokenPrefix is the textual prefix attached to all urapt API tokens so they
// are easy to identify and not confused with other secrets.
const TokenPrefix = "urapt_"

// HashPassword returns a bcrypt hash of the given plaintext password.
func HashPassword(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", fmt.Errorf("bcrypt: %w", err)
	}
	return string(h), nil
}

// VerifyPassword reports whether password matches the stored bcrypt hash.
func VerifyPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// GenerateToken creates a new random API token (TokenPrefix + base64url of 32
// random bytes) and returns it together with its SHA-256 hash (for storage)
// and an 8-char display prefix.
func GenerateToken() (token, hash, prefix string, err error) {
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", "", "", fmt.Errorf("rand: %w", err)
	}
	body := base64.RawURLEncoding.EncodeToString(raw)
	token = TokenPrefix + body
	hash = HashToken(token)
	prefix = token[:len(TokenPrefix)+8]
	return token, hash, prefix, nil
}

// HashToken returns the SHA-256 hex digest of a token, for storage/lookup.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", sum)
}
