package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// GenerateOTP returns a 6-digit code and its SHA-256 hash for storage (PRD 6.2:
// stored hashed, 5-minute expiry, max 3 verification attempts -- enforced by the
// caller against otp_codes.attempts).
func GenerateOTP() (code string, hash string, err error) {
	max := 1000000
	buf := make([]byte, 4)
	if _, err = rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate otp: %w", err)
	}
	n := (int(buf[0])<<24 | int(buf[1])<<16 | int(buf[2])<<8 | int(buf[3])) % max
	if n < 0 {
		n = -n
	}
	code = fmt.Sprintf("%06d", n)
	hash = HashOTP(code)
	return code, hash, nil
}

func HashOTP(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}
