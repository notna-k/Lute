package id

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
)

// ID is a 24-character hex document identifier (compatible with former Mongo ObjectID hex).
type ID string

var ErrInvalidID = errors.New("invalid id")

// New returns a random 12-byte ID encoded as 24 hex characters.
func New() ID {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("id.New: crypto/rand: " + err.Error())
	}
	return ID(hex.EncodeToString(b[:]))
}

// FromHex parses a 24-hex-character string.
func FromHex(s string) (ID, error) {
	if len(s) != 24 {
		return "", fmt.Errorf("%w: wrong length", ErrInvalidID)
	}
	if _, err := hex.DecodeString(s); err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidID, err)
	}
	return ID(s), nil
}

func (i ID) Hex() string { return string(i) }

func (i ID) IsZero() bool { return i == "" }
