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
	"github.com/google/uuid"
)

var ErrInvalidToken = errors.New("auth: invalid or expired token")

// AccessClaims is deliberately minimal: identity plus the tenant/role the token was
// scoped to at issuance. Selecting a school issues a new token; there is no
// parameter that changes tenant on an existing one (PRD 3.2.1).
type AccessClaims struct {
	UserID   uuid.UUID `json:"uid"`
	SchoolID *uuid.UUID `json:"sid,omitempty"`
	Role     string    `json:"role,omitempty"`
	jwt.RegisteredClaims
}

type TokenIssuer struct {
	secret          []byte
	accessTokenTTL  time.Duration
}

func NewTokenIssuer(secret string, accessTokenTTL time.Duration) *TokenIssuer {
	return &TokenIssuer{secret: []byte(secret), accessTokenTTL: accessTokenTTL}
}

// IssueAccessToken mints a short-lived (default 15 min, PRD 6.2) signed access
// token. schoolID is nil for a pre-school-selection token (used only to call
// select-school) and set once a tenant has been chosen.
func (i *TokenIssuer) IssueAccessToken(userID uuid.UUID, schoolID *uuid.UUID, role string) (string, error) {
	now := time.Now()
	claims := AccessClaims{
		UserID:   userID,
		SchoolID: schoolID,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(i.accessTokenTTL)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(i.secret)
}

func (i *TokenIssuer) ParseAccessToken(raw string) (*AccessClaims, error) {
	claims := &AccessClaims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return i.secret, nil
	})
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// GenerateRefreshToken returns a random opaque token plus its SHA-256 hash for
// storage. Only the hash is persisted (PRD 6.2); the raw value is returned to the
// client once and never stored.
func GenerateRefreshToken() (raw string, hash string, err error) {
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}
	raw = base64.RawURLEncoding.EncodeToString(buf)
	hash = HashRefreshToken(raw)
	return raw, hash, nil
}

func HashRefreshToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
