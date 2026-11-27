package auth

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

const (
	// DefaultPBKDF2Iterations follows OWASP PBKDF2-HMAC-SHA256 recommendation.
	DefaultPBKDF2Iterations = 600000
	minPBKDF2Iterations     = 10000
	saltBytes               = 16
	keyBytes                = 32
	hashPrefix              = "pbkdf2_sha256"
	sessionTokenBytes       = 32
)

// dummyEncodedHash is used to equalize CPU time when a username does not exist.
const dummyEncodedHash = "pbkdf2_sha256$600000$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

// HashPassword hashes a plaintext password with PBKDF2-HMAC-SHA256 and a random salt.
func HashPassword(password string) (string, error) {
	return HashPasswordWithIterations(password, DefaultPBKDF2Iterations)
}

// HashPasswordWithIterations hashes a plaintext password using explicit PBKDF2 iterations.
func HashPasswordWithIterations(password string, iterations int) (string, error) {
	if iterations < minPBKDF2Iterations {
		return "", fmt.Errorf("iterations %d below minimum %d", iterations, minPBKDF2Iterations)
	}
	salt := make([]byte, saltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	dk, err := pbkdf2.Key(sha256.New, password, salt, iterations, keyBytes)
	if err != nil {
		return "", fmt.Errorf("derive password key: %w", err)
	}
	return fmt.Sprintf(
		"%s$%d$%s$%s",
		hashPrefix,
		iterations,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(dk),
	), nil
}

// VerifyPassword performs constant-time verification of password against an encoded PBKDF2 hash.
func VerifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != hashPrefix {
		return false
	}
	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations < minPBKDF2Iterations || iterations > 5000000 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil || len(salt) < 8 {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil || len(expected) == 0 {
		return false
	}
	dk, err := pbkdf2.Key(sha256.New, password, salt, iterations, len(expected))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(dk, expected) == 1
}

// DummyVerify consumes PBKDF2 CPU time comparable to VerifyPassword so unknown
// usernames do not short-circuit login latency.
func DummyVerify(password string) {
	_ = VerifyPassword(password, dummyEncodedHash)
}

// GenerateSessionToken creates a cryptographically random opaque session token
// along with its SHA-256 hex digest for storage in SQLite.
func GenerateSessionToken() (raw string, hash string, err error) {
	buf := make([]byte, sessionTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate session token: %w", err)
	}
	raw = base64.RawURLEncoding.EncodeToString(buf)
	return raw, HashSessionToken(raw), nil
}

// HashSessionToken returns the SHA-256 hex digest of an opaque session token.
func HashSessionToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
