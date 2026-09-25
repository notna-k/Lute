package id

import (
	"crypto/rand"
	"database/sql/driver"
	"encoding/hex"
	"errors"
	"fmt"
)

// ID is 12 random bytes as 24 hex characters.
type ID string

var ErrInvalidID = errors.New("invalid id")

func New() ID {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("id.New: crypto/rand: " + err.Error())
	}
	return ID(hex.EncodeToString(b[:]))
}

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

func (i ID) Value() (driver.Value, error) {
	if i.IsZero() {
		return nil, nil
	}
	return string(i), nil
}

func (i *ID) Scan(value interface{}) error {
	if value == nil {
		*i = ""
		return nil
	}
	switch v := value.(type) {
	case string:
		*i = ID(v)
	case []byte:
		*i = ID(v)
	default:
		return fmt.Errorf("id.Scan unsupported type %T", value)
	}
	return nil
}
