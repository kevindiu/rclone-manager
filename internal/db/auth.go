package db

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/kevindiu/rclone-manager/internal/domain"
)

// ErrAlreadyInitialized is returned when trying to create an initial admin user
// after an account already exists.
var ErrAlreadyInitialized = errors.New("already initialized")

// sqliteTimeFormat stores fixed-width UTC timestamps so SQLite TEXT comparisons
// (<= / >) match chronological ordering down to the nanosecond.
const sqliteTimeFormat = "2006-01-02T15:04:05.000000000Z"

func formatSQLiteTime(t time.Time) string {
	return t.UTC().Format(sqliteTimeFormat)
}

func parseSQLiteTime(raw string) (time.Time, error) {
	if t, err := time.Parse(sqliteTimeFormat, raw); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339Nano, raw)
}

// CountUsers returns the total number of persisted user accounts.
func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// CreateInitialUser atomically creates the first admin user only if the users table is empty.
// It returns ErrAlreadyInitialized if any user already exists.
func (s *Store) CreateInitialUser(ctx context.Context, username, passwordHash string) (*domain.User, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, ErrAlreadyInitialized
	}

	now := time.Now().UTC()
	nowStr := formatSQLiteTime(now)
	user := domain.User{
		ID:           domain.NewID(),
		Username:     username,
		PasswordHash: passwordHash,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO users (id, username, password_hash, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		user.ID,
		user.Username,
		user.PasswordHash,
		nowStr,
		nowStr,
	); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByUsername fetches a user case-insensitively by username.
func (s *Store) GetUserByUsername(ctx context.Context, username string) (*domain.User, error) {
	var user domain.User
	var createdStr, updatedStr string
	err := s.db.QueryRowContext(
		ctx,
		`SELECT id, username, password_hash, created_at, updated_at FROM users WHERE username = ?`,
		username,
	).Scan(&user.ID, &user.Username, &user.PasswordHash, &createdStr, &updatedStr)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	user.CreatedAt, _ = parseSQLiteTime(createdStr)
	user.UpdatedAt, _ = parseSQLiteTime(updatedStr)
	return &user, nil
}

// GetUserByID fetches a user by primary ID.
func (s *Store) GetUserByID(ctx context.Context, id string) (*domain.User, error) {
	var user domain.User
	var createdStr, updatedStr string
	err := s.db.QueryRowContext(
		ctx,
		`SELECT id, username, password_hash, created_at, updated_at FROM users WHERE id = ?`,
		id,
	).Scan(&user.ID, &user.Username, &user.PasswordHash, &createdStr, &updatedStr)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	user.CreatedAt, _ = parseSQLiteTime(createdStr)
	user.UpdatedAt, _ = parseSQLiteTime(updatedStr)
	return &user, nil
}

// GetFirstUser returns the earliest created user account.
func (s *Store) GetFirstUser(ctx context.Context) (*domain.User, error) {
	var user domain.User
	var createdStr, updatedStr string
	err := s.db.QueryRowContext(
		ctx,
		`SELECT id, username, password_hash, created_at, updated_at FROM users ORDER BY created_at ASC LIMIT 1`,
	).Scan(&user.ID, &user.Username, &user.PasswordHash, &createdStr, &updatedStr)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	user.CreatedAt, _ = parseSQLiteTime(createdStr)
	user.UpdatedAt, _ = parseSQLiteTime(updatedStr)
	return &user, nil
}

// UpdateUserPassword replaces a user's stored password hash and updates updated_at.
func (s *Store) UpdateUserPassword(ctx context.Context, userID, passwordHash string) error {
	res, err := s.db.ExecContext(
		ctx,
		`UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?`,
		passwordHash,
		formatSQLiteTime(time.Now().UTC()),
		userID,
	)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// CreateSession inserts a newly minted server-side session record.
func (s *Store) CreateSession(ctx context.Context, session domain.Session) error {
	createdAt := session.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO sessions (token_hash, user_id, expires_at, created_at) VALUES (?, ?, ?, ?)`,
		session.TokenHash,
		session.UserID,
		formatSQLiteTime(session.ExpiresAt),
		formatSQLiteTime(createdAt),
	)
	return err
}

// GetSessionWithUser retrieves the session and its associated user for a hashed token.
func (s *Store) GetSessionWithUser(ctx context.Context, tokenHash string) (*domain.Session, *domain.User, error) {
	var sess domain.Session
	var user domain.User
	var expiresStr, sessCreatedStr, userCreatedStr, userUpdatedStr string

	err := s.db.QueryRowContext(
		ctx,
		`SELECT s.token_hash, s.user_id, s.expires_at, s.created_at,
		        u.id, u.username, u.password_hash, u.created_at, u.updated_at
		 FROM sessions s
		 JOIN users u ON u.id = s.user_id
		 WHERE s.token_hash = ?`,
		tokenHash,
	).Scan(
		&sess.TokenHash,
		&sess.UserID,
		&expiresStr,
		&sessCreatedStr,
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&userCreatedStr,
		&userUpdatedStr,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, err
	}

	var parseErr error
	sess.ExpiresAt, parseErr = parseSQLiteTime(expiresStr)
	if parseErr != nil {
		return nil, nil, parseErr
	}
	sess.CreatedAt, _ = parseSQLiteTime(sessCreatedStr)
	user.CreatedAt, _ = parseSQLiteTime(userCreatedStr)
	user.UpdatedAt, _ = parseSQLiteTime(userUpdatedStr)
	return &sess, &user, nil
}

// DeleteSession deletes a single session by token hash.
func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	return err
}

// ListUserSessions returns all non-expired sessions for userID, ordered newest first.
func (s *Store) ListUserSessions(ctx context.Context, userID string) ([]domain.Session, error) {
	nowStr := formatSQLiteTime(time.Now().UTC())
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT token_hash, user_id, expires_at, created_at
		 FROM sessions
		 WHERE user_id = ? AND expires_at > ?
		 ORDER BY created_at DESC`,
		userID,
		nowStr,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]domain.Session, 0)
	for rows.Next() {
		var sess domain.Session
		var expiresStr, createdStr string
		if err := rows.Scan(&sess.TokenHash, &sess.UserID, &expiresStr, &createdStr); err != nil {
			return nil, err
		}
		sess.ExpiresAt, _ = parseSQLiteTime(expiresStr)
		sess.CreatedAt, _ = parseSQLiteTime(createdStr)
		out = append(out, sess)
	}
	return out, rows.Err()
}

// DeleteUserSessionsExcept revokes all sessions belonging to userID except keepTokenHash.
// If keepTokenHash is empty, all sessions for userID are deleted.
func (s *Store) DeleteUserSessionsExcept(ctx context.Context, userID, keepTokenHash string) error {
	if keepTokenHash == "" {
		_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, userID)
		return err
	}
	_, err := s.db.ExecContext(
		ctx,
		`DELETE FROM sessions WHERE user_id = ? AND token_hash != ?`,
		userID,
		keepTokenHash,
	)
	return err
}

// PruneExpiredSessions removes all sessions whose expiration timestamp has passed.
func (s *Store) PruneExpiredSessions(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(
		ctx,
		`DELETE FROM sessions WHERE expires_at <= ?`,
		formatSQLiteTime(time.Now().UTC()),
	)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
