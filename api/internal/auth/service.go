package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/models"
	"github.com/lute/api/internal/db/repos"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrInvalidToken       = errors.New("invalid refresh token")
	ErrTokenReuse         = errors.New("refresh token reuse detected")
)

type Service struct {
	users   *repos.UserRepository
	refresh *repos.RefreshTokenRepository
	tokens  *TokenService
}

func NewService(users *repos.UserRepository, refresh *repos.RefreshTokenRepository, tokens *TokenService) *Service {
	return &Service{users: users, refresh: refresh, tokens: tokens}
}

// IssuedTokens carries the refresh token in plaintext for the cookie; only its hash is stored.
type IssuedTokens struct {
	Access           string
	AccessExpiresAt  time.Time
	RefreshPlaintext string
	RefreshExpiresAt time.Time
	User             *models.User
}

type SessionMeta struct {
	UserAgent string
	IP        string
}

// Login starts a new session family, so several devices can be signed in at once.
func (s *Service) Login(ctx context.Context, email, password string, meta SessionMeta) (*IssuedTokens, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || password == "" {
		return nil, ErrInvalidCredentials
	}
	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repos.ErrNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if user.PasswordHash == "" || !CheckPassword(user.PasswordHash, password) {
		return nil, ErrInvalidCredentials
	}
	return s.issue(ctx, user, id.New(), meta)
}

// Refresh rotates the token within its family; replaying a used token revokes the family.
func (s *Service) Refresh(ctx context.Context, refreshPlaintext string, meta SessionMeta) (*IssuedTokens, error) {
	if refreshPlaintext == "" {
		return nil, ErrInvalidToken
	}
	hash := HashRefresh(refreshPlaintext)
	row, err := s.refresh.GetByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, repos.ErrNotFound) {
			return nil, ErrInvalidToken
		}
		return nil, err
	}
	nowMs := time.Now().UTC().UnixMilli()

	if row.RevokedAt != nil {
		return nil, ErrInvalidToken
	}
	if row.UsedAt != nil {
		_ = s.refresh.RevokeFamily(ctx, row.FamilyID)
		return nil, ErrTokenReuse
	}
	if row.ExpiresAt <= nowMs {
		return nil, ErrInvalidToken
	}

	// Losing a race for the row counts as reuse.
	if err := s.refresh.MarkUsed(ctx, row.ID); err != nil {
		if errors.Is(err, repos.ErrNotFound) {
			_ = s.refresh.RevokeFamily(ctx, row.FamilyID)
			return nil, ErrTokenReuse
		}
		return nil, err
	}

	user, err := s.users.GetByID(ctx, row.UserID)
	if err != nil {
		return nil, err
	}
	return s.issue(ctx, user, row.FamilyID, meta)
}

// Logout revokes this session's family only; the user's other sessions stay.
func (s *Service) Logout(ctx context.Context, refreshPlaintext string) error {
	if refreshPlaintext == "" {
		return nil
	}
	row, err := s.refresh.GetByHash(ctx, HashRefresh(refreshPlaintext))
	if err != nil {
		if errors.Is(err, repos.ErrNotFound) {
			return nil
		}
		return err
	}
	return s.refresh.RevokeFamily(ctx, row.FamilyID)
}

func (s *Service) issue(ctx context.Context, user *models.User, familyID id.ID, meta SessionMeta) (*IssuedTokens, error) {
	access, accessExp, err := s.tokens.SignAccess(user.ID, user.Email)
	if err != nil {
		return nil, err
	}
	plaintext, hash, err := NewRefreshToken()
	if err != nil {
		return nil, err
	}
	refreshExp := time.Now().UTC().Add(s.tokens.RefreshTTL())
	row := &models.RefreshToken{
		UserID:    user.ID,
		FamilyID:  familyID,
		TokenHash: hash,
		ExpiresAt: refreshExp.UnixMilli(),
		UserAgent: truncate(meta.UserAgent, 255),
		IP:        truncate(meta.IP, 64),
	}
	if err := s.refresh.Create(ctx, row); err != nil {
		return nil, err
	}
	return &IssuedTokens{
		Access:           access,
		AccessExpiresAt:  accessExp,
		RefreshPlaintext: plaintext,
		RefreshExpiresAt: refreshExp,
		User:             user,
	}, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
