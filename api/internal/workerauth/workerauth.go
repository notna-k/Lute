// Package workerauth issues registration tokens and worker secrets, and checks the bearer
// credential an agent sends on Connect. Both are 32 random bytes, so a SHA-256 hash is
// enough to store them; the prefixes make a leaked one easy to grep for.
package workerauth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/hex"
	"errors"
	"strings"
)

const (
	TokenPrefix  = "lute_rt_"
	SecretPrefix = "lute_ws_"
	// DisplayLen chars of a token are shown in the panel so an operator can tell tokens apart.
	DisplayLen = len(TokenPrefix) + 6
)

var encoder = base32.StdEncoding.WithPadding(base32.NoPadding)

func NewToken() (string, error) { return random(TokenPrefix) }

func NewSecret() (string, error) { return random(SecretPrefix) }

func random(prefix string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return prefix + strings.ToLower(encoder.EncodeToString(raw)), nil
}

func Hash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// Display is the non-secret start of a token.
func Display(token string) string {
	if len(token) <= DisplayLen {
		return token
	}
	return token[:DisplayLen]
}

// Matches compares a presented secret with a stored hash in constant time.
func Matches(secret, hash string) bool {
	return hash != "" && subtle.ConstantTimeCompare([]byte(Hash(secret)), []byte(hash)) == 1
}

var ErrMalformed = errors.New(`expected "authorization: Bearer <worker_id>.<secret>"`)

// Bearer is what an agent sends on Connect.
func Bearer(workerID, secret string) string {
	return "Bearer " + workerID + "." + secret
}

// ParseBearer splits an authorization header value into worker id and secret.
func ParseBearer(header string) (workerID, secret string, err error) {
	rest, ok := strings.CutPrefix(header, "Bearer ")
	if !ok {
		return "", "", ErrMalformed
	}
	workerID, secret, ok = strings.Cut(strings.TrimSpace(rest), ".")
	if !ok || workerID == "" || secret == "" {
		return "", "", ErrMalformed
	}
	return workerID, secret, nil
}
