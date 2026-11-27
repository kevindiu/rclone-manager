package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/kevindiu/rclone-manager/internal/auth"
	"github.com/kevindiu/rclone-manager/internal/db"
	"github.com/kevindiu/rclone-manager/internal/domain"
)

type mockAuthStore struct {
	users    map[string]domain.User
	sessions map[string]domain.Session
}

func newMockAuthStore() *mockAuthStore {
	return &mockAuthStore{
		users:    make(map[string]domain.User),
		sessions: make(map[string]domain.Session),
	}
}

func (m *mockAuthStore) CountUsers(_ context.Context) (int, error) {
	return len(m.users), nil
}

func (m *mockAuthStore) CreateInitialUser(_ context.Context, username, passwordHash string) (*domain.User, error) {
	if len(m.users) > 0 {
		return nil, db.ErrAlreadyInitialized
	}
	now := time.Now().UTC()
	u := domain.User{
		ID:           "user-1",
		Username:     username,
		PasswordHash: passwordHash,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	m.users[u.ID] = u
	copyUser := u
	return &copyUser, nil
}

func (m *mockAuthStore) GetUserByUsername(_ context.Context, username string) (*domain.User, error) {
	for _, u := range m.users {
		if strings.EqualFold(u.Username, username) {
			copyUser := u
			return &copyUser, nil
		}
	}
	return nil, db.ErrNotFound
}

func (m *mockAuthStore) GetUserByID(_ context.Context, id string) (*domain.User, error) {
	u, ok := m.users[id]
	if !ok {
		return nil, db.ErrNotFound
	}
	copyUser := u
	return &copyUser, nil
}

func (m *mockAuthStore) UpdateUserPassword(_ context.Context, userID, passwordHash string) error {
	u, ok := m.users[userID]
	if !ok {
		return db.ErrNotFound
	}
	u.PasswordHash = passwordHash
	u.UpdatedAt = time.Now().UTC()
	m.users[userID] = u
	return nil
}

func (m *mockAuthStore) CreateSession(_ context.Context, session domain.Session) error {
	m.sessions[session.TokenHash] = session
	return nil
}

func (m *mockAuthStore) GetSessionWithUser(_ context.Context, tokenHash string) (*domain.Session, *domain.User, error) {
	sess, ok := m.sessions[tokenHash]
	if !ok {
		return nil, nil, db.ErrNotFound
	}
	u, ok := m.users[sess.UserID]
	if !ok {
		return nil, nil, db.ErrNotFound
	}
	sCopy := sess
	uCopy := u
	return &sCopy, &uCopy, nil
}

func (m *mockAuthStore) DeleteSession(_ context.Context, tokenHash string) error {
	delete(m.sessions, tokenHash)
	return nil
}

func (m *mockAuthStore) DeleteUserSessionsExcept(_ context.Context, userID, keepTokenHash string) error {
	for k, s := range m.sessions {
		if s.UserID == userID && k != keepTokenHash {
			delete(m.sessions, k)
		}
	}
	return nil
}

func (m *mockAuthStore) PruneExpiredSessions(_ context.Context) (int64, error) {
	now := time.Now().UTC()
	var pruned int64
	for k, s := range m.sessions {
		if !s.ExpiresAt.After(now) {
			delete(m.sessions, k)
			pruned++
		}
	}
	return pruned, nil
}

func newTestAuthService() (*AuthService, *mockAuthStore) {
	st := newMockAuthStore()
	svc := NewAuthService(st, 24*time.Hour, slog.New(slog.NewTextHandler(io.Discard, nil)))
	// Use minimum allowed PBKDF2 iterations in unit tests so -race stays fast.
	svc.pbkdf2Iterations = 10000
	return svc, st
}

func TestAuthServiceSetupStatusAndSeed(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestAuthService()

	status, err := svc.Status(ctx, "")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !status.SetupRequired || status.Authenticated {
		t.Fatalf("expected setup_required before any user exists: %+v", status)
	}

	// Short password rejected with ErrValidation.
	if _, err := svc.SetupInitialAdmin(ctx, "admin", "short"); !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation for short password, got %v", err)
	}
	// Invalid username rejected.
	if _, err := svc.SetupInitialAdmin(ctx, "bad user!", "StrongPass123!"); !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation for invalid username, got %v", err)
	}

	// Setup succeeds once.
	res, err := svc.SetupInitialAdmin(ctx, "admin", "StrongPass123!")
	if err != nil {
		t.Fatalf("SetupInitialAdmin: %v", err)
	}
	if res.RawToken == "" || res.User.Username != "admin" {
		t.Fatalf("unexpected setup result: %+v", res)
	}

	// Second setup rejected with ErrAlreadyInitialized.
	if _, err := svc.SetupInitialAdmin(ctx, "admin2", "StrongPass123!"); !errors.Is(err, ErrAlreadyInitialized) {
		t.Fatalf("expected ErrAlreadyInitialized on second setup, got %v", err)
	}

	// Status unauthenticated vs authenticated.
	unauth, err := svc.Status(ctx, "")
	if err != nil || unauth.SetupRequired || unauth.Authenticated {
		t.Fatalf("unexpected unauthenticated status: %+v err=%v", unauth, err)
	}
	authed, err := svc.Status(ctx, res.RawToken)
	if err != nil || !authed.Authenticated || authed.User == nil || authed.User.Username != "admin" {
		t.Fatalf("unexpected authenticated status: %+v err=%v", authed, err)
	}

	// Env seed is skipped if users already exist.
	if err := svc.SeedAdminFromEnv(ctx, "other", "AnotherStrong1!"); err != nil {
		t.Fatalf("expected SeedAdminFromEnv skip to succeed: %v", err)
	}

	// Fresh store seeds from env and validates invalid password.
	svc2, _ := newTestAuthService()
	if err := svc2.SeedAdminFromEnv(ctx, "admin", "123"); !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation on weak env seed password, got %v", err)
	}
	if err := svc2.SeedAdminFromEnv(ctx, "admin", "SeededStrong123!"); err != nil {
		t.Fatalf("SeedAdminFromEnv valid: %v", err)
	}
	st2, err := svc2.Status(ctx, "")
	if err != nil || st2.SetupRequired {
		t.Fatalf("expected SetupRequired=false after env seed, got %+v err=%v", st2, err)
	}
}

func TestAuthServiceLoginRateLimitExpirationAndChangePassword(t *testing.T) {
	ctx := context.Background()
	svc, st := newTestAuthService()

	res1, err := svc.SetupInitialAdmin(ctx, "admin", "InitialPass123!")
	if err != nil {
		t.Fatalf("SetupInitialAdmin: %v", err)
	}
	tok1Hash := auth.HashSessionToken(res1.RawToken)

	// Wrong password & unknown user fail with ErrInvalidCredentials.
	if _, err := svc.Login(ctx, "10.0.0.1", "nobody", "InitialPass123!"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials for unknown user, got %v", err)
	}
	if _, err := svc.Login(ctx, "10.0.0.1", "admin", "wrong"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials for wrong pass, got %v", err)
	}

	// 8 more failures from same IP trigger rate limit on 11th attempt.
	for i := 0; i < 8; i++ {
		_, _ = svc.Login(ctx, "10.0.0.1", "admin", "wrong")
	}
	if _, err := svc.Login(ctx, "10.0.0.1", "admin", "InitialPass123!"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited after 10 failures, got %v", err)
	}

	// Separate IP can still log in.
	res2, err := svc.Login(ctx, "10.0.0.2", "admin", "InitialPass123!")
	if err != nil {
		t.Fatalf("Login on clean IP: %v", err)
	}

	// ChangePassword rejects wrong current password.
	if err := svc.ChangePassword(ctx, res1.User.ID, tok1Hash, "wrong-current", "NewPassword456!"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials on wrong current password, got %v", err)
	}

	// ChangePassword succeeds, keeps tok1 valid, revokes res2.
	if err := svc.ChangePassword(ctx, res1.User.ID, tok1Hash, "InitialPass123!", "NewPassword456!"); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	if _, _, err := svc.Authenticate(ctx, res1.RawToken); err != nil {
		t.Fatalf("expected current session to remain valid, got %v", err)
	}
	if _, _, err := svc.Authenticate(ctx, res2.RawToken); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected other session revoked, got %v", err)
	}

	// New login with new password works.
	if _, err := svc.Login(ctx, "10.0.0.2", "admin", "NewPassword456!"); err != nil {
		t.Fatalf("login with new password failed: %v", err)
	}

	// Expired session fails Authenticate and gets removed.
	expiredRaw, expiredHash, _ := auth.GenerateSessionToken()
	_ = st.CreateSession(ctx, domain.Session{
		TokenHash: expiredHash,
		UserID:    res1.User.ID,
		ExpiresAt: time.Now().UTC().Add(-time.Minute),
		CreatedAt: time.Now().UTC().Add(-2 * time.Hour),
	})
	if _, _, err := svc.Authenticate(ctx, expiredRaw); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized for expired session, got %v", err)
	}
	if _, ok := st.sessions[expiredHash]; ok {
		t.Fatal("expected expired session to be deleted on Authenticate")
	}
}
