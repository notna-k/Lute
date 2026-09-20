// Package apikey provides generation, hashing, and lookup helpers for the
// public-API bearer tokens issued to users. Tokens have the form
// "lute_sk_<24-random-char-base32>"; only a public prefix and a SHA-256 hash
// of the full token are persisted.
package apikey

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	// Prefix identifies a Lute secret key and never changes for a given token.
	Prefix = "lute_sk_"
	// bodyLen is the number of base32 characters in the random body.
	bodyLen = 24
	// publicPrefixLen is how many chars (including Prefix) are persisted as a
	// non-secret lookup key. The remainder of the token is never stored.
	publicPrefixLen = len(Prefix) + 8
)

var encoder = base32.StdEncoding.WithPadding(base32.NoPadding)

// Generate returns a new token together with its public prefix and SHA-256 hash.
func Generate() (token, prefix, hash string, err error) {
	raw := make([]byte, 15)
	if _, err := rand.Read(raw); err != nil {
		return "", "", "", fmt.Errorf("apikey: read random: %w", err)
	}
	body := strings.ToLower(encoder.EncodeToString(raw))
	if len(body) > bodyLen {
		body = body[:bodyLen]
	}
	token = Prefix + body
	prefix = token[:publicPrefixLen]
	hash = Hash(token)
	return token, prefix, hash, nil
}

// Hash returns the hex-encoded SHA-256 hash of a token.
func Hash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// PrefixOf returns the public prefix for a token, or "" if the token is malformed.
func PrefixOf(token string) string {
	if !strings.HasPrefix(token, Prefix) || len(token) < publicPrefixLen {
		return ""
	}
	return token[:publicPrefixLen]
}
