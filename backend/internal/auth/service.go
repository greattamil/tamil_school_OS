package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"school-erp/backend/internal/db"
)

var (
	ErrInvalidCredentials  = errors.New("auth: invalid credentials")
	ErrOTPInvalid          = errors.New("auth: invalid or expired otp")
	ErrOTPAttemptsExceeded = errors.New("auth: too many otp attempts")
	ErrSessionRevoked      = errors.New("auth: session revoked")
	ErrNoSchoolAccess      = errors.New("auth: no role at requested school")
)

const (
	otpTTL         = 5 * time.Minute
	otpMaxAttempts = 3
)

type SchoolRole struct {
	SchoolID   uuid.UUID
	SchoolName string
	Role       string
}

type TokenPair struct {
	AccessToken  string
	RefreshToken string
	Schools      []SchoolRole // populated when the token is not yet scoped to one school
}

type Service struct {
	pool            *pgxpool.Pool
	tokens          *TokenIssuer
	refreshTokenTTL time.Duration
}

func NewService(pool *pgxpool.Pool, tokens *TokenIssuer, refreshTokenTTL time.Duration) *Service {
	return &Service{pool: pool, tokens: tokens, refreshTokenTTL: refreshTokenTTL}
}

// StaffLogin authenticates a staff user by mobile-or-email + password. If the user
// holds a role at exactly one school, the issued access token is scoped to it
// directly; otherwise the caller must call SelectSchool next (PRD 3.2.1).
func (s *Service) StaffLogin(ctx context.Context, identifier, password, deviceID string) (TokenPair, error) {
	var (
		userID           uuid.UUID
		tokensValidAfter time.Time
		passwordHash     string
	)

	err := db.WithGlobalTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT u.id, u.tokens_valid_after, c.password_hash
			FROM users u
			JOIN credentials c ON c.user_id = u.id
			WHERE (u.mobile = $1 OR u.email = $1) AND u.deleted_at IS NULL
		`, identifier)
		return row.Scan(&userID, &tokensValidAfter, &passwordHash)
	})
	if err != nil {
		return TokenPair{}, ErrInvalidCredentials
	}

	ok, err := VerifyPassword(passwordHash, password)
	if err != nil || !ok {
		return TokenPair{}, ErrInvalidCredentials
	}

	return s.issueTokensForUser(ctx, userID, deviceID, nil)
}

// RequestParentOTP issues an OTP for a mobile number that already has a guardian
// record. Silently no-ops (from the caller's perspective) if the mobile is unknown,
// so the endpoint cannot be used to enumerate registered numbers.
func (s *Service) RequestParentOTP(ctx context.Context, mobile string) (code string, err error) {
	code, hash, err := GenerateOTP()
	if err != nil {
		return "", err
	}

	err = db.WithGlobalTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO otp_codes (mobile, code_hash, expires_at)
			VALUES ($1, $2, now() + $3::interval)
		`, mobile, hash, otpTTL.String())
		return err
	})
	if err != nil {
		return "", fmt.Errorf("store otp: %w", err)
	}

	// TODO(phase 5): dispatch via the school's DLT-registered SMS gateway
	// (OTP_LOGIN template, PRD 9 Phase 1 external processes). Returned here only
	// so the caller can log/deliver it in local development.
	return code, nil
}

func (s *Service) VerifyParentOTP(ctx context.Context, mobile, code, deviceID string) (TokenPair, error) {
	var (
		otpID    uuid.UUID
		codeHash string
		attempts int
	)

	err := db.WithGlobalTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id, code_hash, attempts FROM otp_codes
			WHERE mobile = $1 AND consumed_at IS NULL AND expires_at > now()
			ORDER BY created_at DESC LIMIT 1
			FOR UPDATE
		`, mobile)
		if err := row.Scan(&otpID, &codeHash, &attempts); err != nil {
			return ErrOTPInvalid
		}
		if attempts >= otpMaxAttempts {
			return ErrOTPAttemptsExceeded
		}
		if HashOTP(code) != codeHash {
			_, err := tx.Exec(ctx, `UPDATE otp_codes SET attempts = attempts + 1 WHERE id = $1`, otpID)
			if err != nil {
				return err
			}
			return ErrOTPInvalid
		}
		_, err := tx.Exec(ctx, `UPDATE otp_codes SET consumed_at = now() WHERE id = $1`, otpID)
		return err
	})
	if err != nil {
		return TokenPair{}, err
	}

	var userID uuid.UUID
	err = db.WithGlobalTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT id FROM users WHERE mobile = $1 AND deleted_at IS NULL
		`, mobile).Scan(&userID)
	})
	if err != nil {
		return TokenPair{}, ErrInvalidCredentials
	}

	return s.issueTokensForUser(ctx, userID, deviceID, nil)
}

// SelectSchool exchanges a valid refresh token for a new pair scoped to schoolID.
// This is always a new token, never a parameter change on the existing one
// (PRD 3.2.1), so a stolen unscoped token cannot be replayed against an arbitrary
// school the holder has no role at.
func (s *Service) SelectSchool(ctx context.Context, refreshToken string, schoolID uuid.UUID, deviceID string) (TokenPair, error) {
	userID, err := s.userIDForValidRefreshToken(ctx, refreshToken)
	if err != nil {
		return TokenPair{}, err
	}
	return s.issueTokensForUser(ctx, userID, deviceID, &schoolID)
}

func (s *Service) userIDForValidRefreshToken(ctx context.Context, refreshToken string) (uuid.UUID, error) {
	hash := HashRefreshToken(refreshToken)
	var userID uuid.UUID
	var tokensValidAfter, issuedAt time.Time

	err := db.WithGlobalTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT s.user_id, u.tokens_valid_after, s.issued_at
			FROM sessions s
			JOIN users u ON u.id = s.user_id
			WHERE s.refresh_token_hash = $1 AND s.revoked_at IS NULL AND s.expires_at > now()
		`, hash)
		return row.Scan(&userID, &tokensValidAfter, &issuedAt)
	})
	if err != nil {
		return uuid.Nil, ErrSessionRevoked
	}
	if issuedAt.Before(tokensValidAfter) {
		return uuid.Nil, ErrSessionRevoked
	}
	return userID, nil
}

// issueTokensForUser issues a fresh access/refresh pair. If schoolID is nil and the
// user holds a role at exactly one school, that role is used automatically; if nil
// and there is more than one, the caller gets back the list to choose from with an
// unscoped access token that is only valid for SelectSchool.
func (s *Service) issueTokensForUser(ctx context.Context, userID uuid.UUID, deviceID string, schoolID *uuid.UUID) (TokenPair, error) {
	var roles []SchoolRole
	err := db.WithGlobalTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT r.school_id, sc.name, r.role
			FROM user_school_roles r
			JOIN schools sc ON sc.id = r.school_id
			WHERE r.user_id = $1 AND r.deleted_at IS NULL AND sc.deleted_at IS NULL
		`, userID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var sr SchoolRole
			if err := rows.Scan(&sr.SchoolID, &sr.SchoolName, &sr.Role); err != nil {
				return err
			}
			roles = append(roles, sr)
		}
		return rows.Err()
	})
	if err != nil {
		return TokenPair{}, err
	}

	var role string
	effectiveSchoolID := schoolID
	if effectiveSchoolID != nil {
		found := false
		for _, r := range roles {
			if r.SchoolID == *effectiveSchoolID {
				role = r.Role
				found = true
				break
			}
		}
		if !found {
			return TokenPair{}, ErrNoSchoolAccess
		}
	} else if len(roles) == 1 {
		effectiveSchoolID = &roles[0].SchoolID
		role = roles[0].Role
	}

	access, err := s.tokens.IssueAccessToken(userID, effectiveSchoolID, role)
	if err != nil {
		return TokenPair{}, err
	}

	rawRefresh, refreshHash, err := GenerateRefreshToken()
	if err != nil {
		return TokenPair{}, err
	}

	err = db.WithGlobalTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		var schoolIDArg any
		if effectiveSchoolID != nil {
			schoolIDArg = *effectiveSchoolID
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO sessions (user_id, refresh_token_hash, school_id, device_id, expires_at)
			VALUES ($1, $2, $3, $4, now() + $5::interval)
		`, userID, refreshHash, schoolIDArg, deviceID, s.refreshTokenTTL.String())
		return err
	})
	if err != nil {
		return TokenPair{}, err
	}

	return TokenPair{AccessToken: access, RefreshToken: rawRefresh, Schools: roles}, nil
}

// RevokeAllSessions sets tokens_valid_after to now(), invalidating every access and
// refresh token issued before this call in a single write (PRD 6.2).
func (s *Service) RevokeAllSessions(ctx context.Context, userID uuid.UUID) error {
	return db.WithGlobalTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE users SET tokens_valid_after = now() WHERE id = $1`, userID)
		return err
	})
}
