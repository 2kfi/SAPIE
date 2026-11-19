// Package auth handles password hashes, session tokens and login rate limits.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonMemory      = 64 * 1024 // KiB
	argonIterations  = 3
	argonParallelism = 4
	keyLength        = 32
	tokenBytes       = 32
	params           = "m=65536,t=3,p=4"
)

// HashPassword returns a self-describing argon2id hash.
func HashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, argonIterations, argonMemory, argonParallelism, keyLength)
	enc := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=19$%s$%s$%s", params, enc.EncodeToString(salt), enc.EncodeToString(key)), nil
}

func VerifyPassword(hash, password string) bool {
	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" || parts[3] != params {
		return false
	}
	enc := base64.RawStdEncoding
	salt, err := enc.DecodeString(parts[4])
	if err != nil {
		return false
	}
	want, err := enc.DecodeString(parts[5])
	if err != nil || len(want) != keyLength {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, argonIterations, argonMemory, argonParallelism, keyLength)
	return subtle.ConstantTimeCompare(got, want) == 1
}

// NewToken returns a URL-safe 32-byte random session token.
func NewToken() (string, error) {
	buf := make([]byte, tokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// HashToken derives the value stored in the sessions table; the raw token is never stored.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func Equal(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
