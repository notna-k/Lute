// Package apikey issues public-API tokens ("lute_sk_" + 24 base32 chars); only a prefix and a hash are stored.
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
	Prefix  = "lute_sk_"
	bodyLen = 24
	// publicPrefixLen chars are stored as a non-secret lookup key.
	publicPrefixLen = len(Prefix) + 8
)

var encoder = base32.StdEncoding.WithPadding(base32.NoPadding)

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

func Hash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// PrefixOf returns "" for a malformed token.
func PrefixOf(token string) string {
	if !strings.HasPrefix(token, Prefix) || len(token) < publicPrefixLen {
		return ""
	}
	return token[:publicPrefixLen]
}
