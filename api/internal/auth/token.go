package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/lute/api/internal/db/id"
)

// AccessClaims is what we sign into the access JWT.
type AccessClaims struct {
	UserID string `json:"uid"`
	Email  string `json:"eml,omitempty"`
	jwt.RegisteredClaims
}

// TokenService signs and verifies access JWTs and generates refresh-token strings.
type TokenService struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
	issuer     string
}

func NewTokenService(secret string, accessTTL, refreshTTL time.Duration, issuer string) (*TokenService, error) {
	if len(secret) < 32 {
		return nil, errors.New("JWT secret must be at least 32 bytes (set JWT_SECRET)")
	}
	if accessTTL <= 0 {
		accessTTL = 15 * time.Minute
	}
	if refreshTTL <= 0 {
		refreshTTL = 30 * 24 * time.Hour
	}
	if issuer == "" {
		issuer = "lute"
	}
	return &TokenService{
		secret:     []byte(secret),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		issuer:     issuer,
	}, nil
}

func (s *TokenService) AccessTTL() time.Duration  { return s.accessTTL }
func (s *TokenService) RefreshTTL() time.Duration { return s.refreshTTL }

// SignAccess issues a short-lived access JWT for the given user.
func (s *TokenService) SignAccess(userID id.ID, email string) (string, time.Time, error) {
	now := time.Now().UTC()
	exp := now.Add(s.accessTTL)
	claims := AccessClaims{
		UserID: userID.Hex(),
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   userID.Hex(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString(s.secret)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, exp, nil
}

// ParseAccess verifies a signed access JWT and returns its claims.
func (s *TokenService) ParseAccess(raw string) (*AccessClaims, error) {
	parsed, err := jwt.ParseWithClaims(raw, &AccessClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.secret, nil
	}, jwt.WithIssuer(s.issuer), jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return nil, err
	}
	claims, ok := parsed.Claims.(*AccessClaims)
	if !ok || !parsed.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

// NewRefreshToken returns a high-entropy opaque refresh token and its sha256 hex hash.
// The plaintext is returned to the client (cookie); only the hash is persisted.
func NewRefreshToken() (plaintext, hash string, err error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", "", err
	}
	plaintext = base64.RawURLEncoding.EncodeToString(b[:])
	hash = HashRefresh(plaintext)
	return plaintext, hash, nil
}

// HashRefresh returns the storage hash for a refresh token string.
func HashRefresh(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}
