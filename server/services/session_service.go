package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	sessionCookieName = "testcenter_session"
	sessionTTL        = 30 * 24 * time.Hour
)

func ensureUserSessionsTable() error {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return err
	}

	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS user_sessions (
			id VARCHAR(64) PRIMARY KEY,
			user_id VARCHAR(128) NOT NULL,
			token_hash CHAR(64) NOT NULL UNIQUE,
			created_at DATETIME NOT NULL,
			expires_at DATETIME NOT NULL,
			revoked_at DATETIME NULL,
			last_seen_at DATETIME NOT NULL,
			INDEX idx_user_sessions_user_id (user_id),
			INDEX idx_user_sessions_expires_at (expires_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
	`)
	return err
}

func createUserSession(c *gin.Context, userID string) error {
	if err := ensureUserSessionsTable(); err != nil {
		return err
	}

	token, tokenHash, err := newSessionToken()
	if err != nil {
		return err
	}

	now := time.Now()
	expiresAt := now.Add(sessionTTL)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return err
	}

	_, err = db.ExecContext(ctx, `
		INSERT INTO user_sessions (id, user_id, token_hash, created_at, expires_at, last_seen_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, uuid.New().String(), userID, tokenHash, now, expiresAt, now)
	if err != nil {
		return err
	}

	setSessionCookie(c, token, int(sessionTTL.Seconds()))
	return nil
}

func CurrentUserFromRequest(c *gin.Context) (*User, error) {
	return currentUserFromRequest(c)
}

func currentUserFromRequest(c *gin.Context) (*User, error) {
	if user, err := currentUserFromSessionCookie(c); err == nil {
		return user, nil
	}

	token := strings.TrimSpace(c.GetHeader("Authorization"))
	if token == "" {
		return nil, fmt.Errorf("missing session")
	}
	return GetUserByID(token)
}

func currentUserFromSessionCookie(c *gin.Context) (*User, error) {
	token, err := c.Cookie(sessionCookieName)
	if err != nil || strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("missing session cookie")
	}
	if err := ensureUserSessionsTable(); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return nil, err
	}

	tokenHash := hashSessionToken(token)
	var userID string
	err = db.QueryRowContext(ctx, `
		SELECT user_id
		FROM user_sessions
		WHERE token_hash = ?
			AND revoked_at IS NULL
			AND expires_at > ?
		LIMIT 1
	`, tokenHash, time.Now()).Scan(&userID)
	if err != nil {
		return nil, err
	}

	_, _ = db.ExecContext(ctx, "UPDATE user_sessions SET last_seen_at = ? WHERE token_hash = ?", time.Now(), tokenHash)
	return GetUserByID(userID)
}

func revokeCurrentSession(c *gin.Context) {
	token, err := c.Cookie(sessionCookieName)
	if err == nil && strings.TrimSpace(token) != "" {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
		defer cancel()
		if db, _, dbErr := DatabaseManager.DB(ctx); dbErr == nil {
			_, _ = db.ExecContext(ctx, "UPDATE user_sessions SET revoked_at = ? WHERE token_hash = ?", time.Now(), hashSessionToken(token))
		}
	}
	clearSessionCookie(c)
}

func setSessionCookie(c *gin.Context, token string, maxAge int) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(sessionCookieName, token, maxAge, "/", "", c.Request.TLS != nil, true)
}

func clearSessionCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(sessionCookieName, "", -1, "/", "", c.Request.TLS != nil, true)
}

func newSessionToken() (string, string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", err
	}
	token := hex.EncodeToString(bytes)
	return token, hashSessionToken(token), nil
}

func hashSessionToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
