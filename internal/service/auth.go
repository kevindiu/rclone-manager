package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/kevindiu/rclone-manager/internal/auth"
	"github.com/kevindiu/rclone-manager/internal/db"
	"github.com/kevindiu/rclone-manager/internal/domain"
)

var (
	// ErrUnauthorized indicates missing, invalid, or expired authentication.
	ErrUnauthorized = errors.New("unauthorized")
	// ErrInvalidCredentials indicates an incorrect username or password.
	ErrInvalidCredentials = errors.New("invalid username or password")
	// ErrAlreadyInitialized indicates initial admin setup has already completed.
	ErrAlreadyInitialized = errors.New("admin account already configured")
	// ErrRateLimited indicates login attempts are temporarily throttled.
	ErrRateLimited = errors.New("too many failed login attempts")
)

var usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9._-]{3,64}$`)

const (
	minPasswordLen = 8
	maxPasswordLen = 128
)

// RateLimitError wraps ErrRateLimited with the suggested RetryAfter duration.
type RateLimitError struct {
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("too many failed login attempts, retry in %ds", max(1, int(e.RetryAfter.Seconds())))
}

func (e *RateLimitError) Unwrap() error {
	return ErrRateLimited
}

// AuthStore defines persistence operations required by AuthService.
type AuthStore interface {
	CountUsers(ctx context.Context) (int, error)
	CreateInitialUser(ctx context.Context, username, passwordHash string) (*domain.User, error)
	GetUserByUsername(ctx context.Context, username string) (*domain.User, error)
	GetUserByID(ctx context.Context, id string) (*domain.User, error)
	GetFirstUser(ctx context.Context) (*domain.User, error)
	UpdateUserPassword(ctx context.Context, userID, passwordHash string) error
	CreateSession(ctx context.Context, session domain.Session) error
	GetSessionWithUser(ctx context.Context, tokenHash string) (*domain.Session, *domain.User, error)
	DeleteSession(ctx context.Context, tokenHash string) error
	DeleteUserSessionsExcept(ctx context.Context, userID, keepTokenHash string) error
	PruneExpiredSessions(ctx context.Context) (int64, error)
}

// AuthStatus describes current session state and whether initial setup is needed.
type AuthStatus struct {
	Authenticated bool         `json:"authenticated"`
	SetupRequired bool         `json:"setup_required"`
	User          *domain.User `json:"user,omitempty"`
}

// AuthSessionResult contains the user and freshly issued session token.
type AuthSessionResult struct {
	User      domain.User
	RawToken  string
	ExpiresAt time.Time
}

// AuthService manages mandatory authentication, first-run bootstrap, and sessions.
type AuthService struct {
	store            AuthStore
	sessionTTL       time.Duration
	pbkdf2Iterations int
	limiter          *auth.LoginRateLimiter
	logger           *slog.Logger
}

// NewAuthService creates an AuthService with the supplied store and session TTL.
func NewAuthService(store AuthStore, sessionTTL time.Duration, logger *slog.Logger) *AuthService {
	if logger == nil {
		logger = slog.Default()
	}
	if sessionTTL <= 0 {
		sessionTTL = 168 * time.Hour
	}
	return &AuthService{
		store:            store,
		sessionTTL:       sessionTTL,
		pbkdf2Iterations: auth.DefaultPBKDF2Iterations,
		limiter:          auth.NewLoginRateLimiter(10, 5*time.Minute),
		logger:           logger,
	}
}

// SeedAdminFromEnv creates the initial admin account from environment variables
// only when the users table is empty.
func (a *AuthService) SeedAdminFromEnv(ctx context.Context, username, password string) error {
	username = strings.TrimSpace(username)
	if username == "" && password == "" {
		return nil
	}
	if username == "" || password == "" {
		a.logger.Warn("ignoring partial admin seed environment configuration: both RCLONE_MANAGER_ADMIN_USER and RCLONE_MANAGER_ADMIN_PASSWORD must be set")
		return nil
	}

	count, err := a.store.CountUsers(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		a.logger.Info("admin account already exists, ignoring env seed")
		return nil
	}

	cleanUser, err := validateUsername(username)
	if err != nil {
		return err
	}
	if err := validatePassword(password); err != nil {
		return err
	}

	hash, err := auth.HashPasswordWithIterations(password, a.pbkdf2Iterations)
	if err != nil {
		return err
	}
	if _, err := a.store.CreateInitialUser(ctx, cleanUser, hash); err != nil {
		if errors.Is(err, db.ErrAlreadyInitialized) {
			return nil
		}
		return err
	}
	a.logger.Info("seeded initial admin account from environment", "username", cleanUser)
	return nil
}

// Status inspects user initialization and validates the current session token if present.
func (a *AuthService) Status(ctx context.Context, rawToken string) (AuthStatus, error) {
	count, err := a.store.CountUsers(ctx)
	if err != nil {
		return AuthStatus{}, err
	}
	if count == 0 {
		return AuthStatus{SetupRequired: true, Authenticated: false}, nil
	}

	if rawToken != "" {
		user, _, err := a.Authenticate(ctx, rawToken)
		if err == nil && user != nil {
			return AuthStatus{
				Authenticated: true,
				SetupRequired: false,
				User:          user,
			}, nil
		}
	}
	return AuthStatus{Authenticated: false, SetupRequired: false}, nil
}

// SetupInitialAdmin creates the first administrator account and opens a session.
func (a *AuthService) SetupInitialAdmin(ctx context.Context, username, password string) (*AuthSessionResult, error) {
	cleanUser, err := validateUsername(username)
	if err != nil {
		return nil, err
	}
	if err := validatePassword(password); err != nil {
		return nil, err
	}

	hash, err := auth.HashPasswordWithIterations(password, a.pbkdf2Iterations)
	if err != nil {
		return nil, err
	}

	user, err := a.store.CreateInitialUser(ctx, cleanUser, hash)
	if err != nil {
		if errors.Is(err, db.ErrAlreadyInitialized) {
			return nil, ErrAlreadyInitialized
		}
		return nil, err
	}

	res, err := a.createSessionForUser(ctx, *user)
	if err != nil {
		return nil, err
	}
	a.logger.Info("initial admin account created", "username", user.Username)
	return res, nil
}

// Login verifies credentials with rate limiting and constant-time comparison, returning a new session.
func (a *AuthService) Login(ctx context.Context, clientIP, username, password string) (*AuthSessionResult, error) {
	if allowed, retryAfter := a.limiter.Allow(clientIP); !allowed {
		return nil, &RateLimitError{RetryAfter: retryAfter}
	}

	cleanUser := strings.TrimSpace(username)
	if cleanUser == "" || password == "" {
		a.limiter.RecordFailure(clientIP)
		return nil, ErrInvalidCredentials
	}

	user, err := a.store.GetUserByUsername(ctx, cleanUser)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			auth.DummyVerify(password)
			a.limiter.RecordFailure(clientIP)
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if !auth.VerifyPassword(password, user.PasswordHash) {
		a.limiter.RecordFailure(clientIP)
		return nil, ErrInvalidCredentials
	}

	a.limiter.Reset(clientIP)
	res, err := a.createSessionForUser(ctx, *user)
	if err != nil {
		return nil, err
	}
	a.logger.Info("user signed in", "username", user.Username)
	return res, nil
}

// Authenticate validates a raw session token and returns the associated user and token hash.
func (a *AuthService) Authenticate(ctx context.Context, rawToken string) (*domain.User, string, error) {
	if strings.TrimSpace(rawToken) == "" {
		return nil, "", ErrUnauthorized
	}

	tokenHash := auth.HashSessionToken(rawToken)
	sess, user, err := a.store.GetSessionWithUser(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return nil, "", ErrUnauthorized
		}
		return nil, "", err
	}

	if !sess.ExpiresAt.After(time.Now().UTC()) {
		if delErr := a.store.DeleteSession(ctx, tokenHash); delErr != nil {
			a.logger.Warn("failed to delete expired session", "error", delErr)
		}
		return nil, "", ErrUnauthorized
	}

	return user, tokenHash, nil
}

// Logout invalidates the session identified by rawToken.
func (a *AuthService) Logout(ctx context.Context, rawToken string) error {
	if strings.TrimSpace(rawToken) == "" {
		return nil
	}
	return a.store.DeleteSession(ctx, auth.HashSessionToken(rawToken))
}

// ChangePassword verifies the current password, updates the password hash, and
// revokes all other active sessions for the user.
func (a *AuthService) ChangePassword(ctx context.Context, userID, currentTokenHash, currentPassword, newPassword string) error {
	if err := validatePassword(newPassword); err != nil {
		return err
	}
	user, err := a.store.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return ErrUnauthorized
		}
		return err
	}
	if !auth.VerifyPassword(currentPassword, user.PasswordHash) {
		return ErrInvalidCredentials
	}

	newHash, err := auth.HashPasswordWithIterations(newPassword, a.pbkdf2Iterations)
	if err != nil {
		return err
	}
	if err := a.store.UpdateUserPassword(ctx, userID, newHash); err != nil {
		return err
	}
	if err := a.store.DeleteUserSessionsExcept(ctx, userID, currentTokenHash); err != nil {
		a.logger.Warn("failed to revoke other user sessions after password change", "user_id", userID, "error", err)
	}
	a.logger.Info("user password updated", "username", user.Username)
	return nil
}

// PruneExpiredSessions removes expired sessions from persistence.
func (a *AuthService) PruneExpiredSessions(ctx context.Context) (int64, error) {
	return a.store.PruneExpiredSessions(ctx)
}

// ResetPassword resets an administrator password (defaulting to the earliest user
// when username is empty) and revokes all active sessions for that user.
func (a *AuthService) ResetPassword(ctx context.Context, username, newPassword string) (*domain.User, error) {
	if err := validatePassword(newPassword); err != nil {
		return nil, err
	}

	var (
		user *domain.User
		err  error
	)
	cleanUser := strings.TrimSpace(username)
	if cleanUser != "" {
		user, err = a.store.GetUserByUsername(ctx, cleanUser)
	} else {
		user, err = a.store.GetFirstUser(ctx)
	}
	if err != nil {
		return nil, err
	}

	newHash, err := auth.HashPasswordWithIterations(newPassword, a.pbkdf2Iterations)
	if err != nil {
		return nil, err
	}
	if err := a.store.UpdateUserPassword(ctx, user.ID, newHash); err != nil {
		return nil, err
	}
	if err := a.store.DeleteUserSessionsExcept(ctx, user.ID, ""); err != nil {
		a.logger.Warn("failed to revoke user sessions after CLI password reset", "user_id", user.ID, "error", err)
	}
	user.PasswordHash = ""
	a.logger.Info("administrator password reset via CLI", "username", user.Username)
	return user, nil
}

func (a *AuthService) createSessionForUser(ctx context.Context, user domain.User) (*AuthSessionResult, error) {
	raw, hash, err := auth.GenerateSessionToken()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	expiresAt := now.Add(a.sessionTTL)
	if err := a.store.CreateSession(ctx, domain.Session{
		TokenHash: hash,
		UserID:    user.ID,
		ExpiresAt: expiresAt,
		CreatedAt: now,
	}); err != nil {
		return nil, err
	}
	user.PasswordHash = ""
	return &AuthSessionResult{
		User:      user,
		RawToken:  raw,
		ExpiresAt: expiresAt,
	}, nil
}

func validateUsername(raw string) (string, error) {
	clean := strings.TrimSpace(raw)
	if !usernameRegex.MatchString(clean) {
		return "", fmt.Errorf("%w: username must be 3-64 characters using letters, numbers, '.', '_' or '-'", ErrValidation)
	}
	return clean, nil
}

func validatePassword(password string) error {
	if len(password) < minPasswordLen || len(password) > maxPasswordLen {
		return fmt.Errorf("%w: password must be %d-%d characters", ErrValidation, minPasswordLen, maxPasswordLen)
	}
	return nil
}
