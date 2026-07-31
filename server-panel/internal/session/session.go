// Package session 提供基于 Cookie 的 Web 会话管理。
// 每个 client_id 仅允许一个活跃会话（单点登录）。
package session

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/frp-panel/server-panel/internal/crypto"
)

const (
	// CookieName 会话 Cookie 名称。
	CookieName = "frp_session"
	// DefaultTTL 默认会话有效期。
	DefaultTTL = 24 * time.Hour
	// MaxTTL 最大会话有效期。
	MaxTTL = 7 * 24 * time.Hour
)

// Manager 会话管理器。
type Manager struct {
	db         *sql.DB
	logger     *slog.Logger
	cookieName string
	domain     string
	secure     bool
	ttl        time.Duration
}

// Config 会话管理器配置。
type Config struct {
	Domain     string        // Cookie 域名
	Secure     bool          // 是否仅 HTTPS
	TTL        time.Duration // 会话有效期
	CookieName string        // Cookie 名称（默认 frp_session）
}

// NewManager 创建会话管理器。
func NewManager(db *sql.DB, logger *slog.Logger, cfg Config) *Manager {
	cookieName := cfg.CookieName
	if cookieName == "" {
		cookieName = CookieName
	}
	ttl := cfg.TTL
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	return &Manager{
		db:         db,
		logger:     logger,
		cookieName: cookieName,
		domain:     cfg.Domain,
		secure:     cfg.Secure,
		ttl:        ttl,
	}
}

// Session 会话实体。
type Session struct {
	ID         string `json:"id"`
	UserID     string `json:"user_id"`
	ClientID   string `json:"client_id"`
	Generation int    `json:"generation"`
	IP         string `json:"ip"`
	UserAgent  string `json:"user_agent"`
	CreatedAt  string `json:"created_at"`
	ExpiresAt  string `json:"expires_at"`
}

// Create 创建新会话。
// 如果同一 client_id 已有活跃会话，会先注销旧会话（单点登录）。
func (m *Manager) Create(ctx context.Context, userID, clientID, ip, userAgent string) (*Session, error) {
	// 删除同一 client_id 的所有旧会话
	if _, err := m.db.ExecContext(ctx, `
		DELETE FROM sessions WHERE client_id = ?
	`, clientID); err != nil {
		return nil, fmt.Errorf("delete old sessions: %w", err)
	}

	// 获取当前代次
	var gen int
	if err := m.db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(generation), 0) FROM sessions WHERE client_id = ?
	`, clientID).Scan(&gen); err != nil {
		// 如果没有旧记录，代次从 1 开始
		gen = 0
	}
	gen++

	id := crypto.RandomToken(32)
	now := time.Now().UTC()
	expires := now.Add(m.ttl)

	if _, err := m.db.ExecContext(ctx, `
		INSERT INTO sessions (id, user_id, client_id, generation, ip, user_agent, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, id, userID, clientID, gen, ip, userAgent, now.Format(time.RFC3339), expires.Format(time.RFC3339)); err != nil {
		return nil, fmt.Errorf("insert session: %w", err)
	}

	return &Session{
		ID:         id,
		UserID:     userID,
		ClientID:   clientID,
		Generation: gen,
		IP:         ip,
		UserAgent:  userAgent,
		CreatedAt:  now.Format(time.RFC3339),
		ExpiresAt:  expires.Format(time.RFC3339),
	}, nil
}

// Validate 验证会话 ID 是否有效。
func (m *Manager) Validate(ctx context.Context, sessionID string) (*Session, error) {
	var s Session
	err := m.db.QueryRowContext(ctx, `
		SELECT id, user_id, client_id, generation, ip, user_agent, created_at, expires_at
		FROM sessions WHERE id = ?
	`, sessionID).Scan(&s.ID, &s.UserID, &s.ClientID, &s.Generation, &s.IP, &s.UserAgent, &s.CreatedAt, &s.ExpiresAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query session: %w", err)
	}

	// 检查过期
	expires, err := time.Parse(time.RFC3339, s.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("parse expiry: %w", err)
	}
	if time.Now().UTC().After(expires) {
		// 清理过期会话
		_, _ = m.db.ExecContext(ctx, "DELETE FROM sessions WHERE id = ?", sessionID)
		return nil, nil
	}

	return &s, nil
}

// Revoke 注销会话。
func (m *Manager) Revoke(ctx context.Context, sessionID string) error {
	_, err := m.db.ExecContext(ctx, "DELETE FROM sessions WHERE id = ?", sessionID)
	return err
}

// RevokeAllForUser 注销用户的所有会话。
func (m *Manager) RevokeAllForUser(ctx context.Context, userID string) error {
	_, err := m.db.ExecContext(ctx, "DELETE FROM sessions WHERE user_id = ?", userID)
	return err
}

// RevokeAllForClient 注销指定 client_id 的所有会话。
func (m *Manager) RevokeAllForClient(ctx context.Context, clientID string) error {
	_, err := m.db.ExecContext(ctx, "DELETE FROM sessions WHERE client_id = ?", clientID)
	return err
}

// GetGeneration 获取 client_id 的当前代次。
func (m *Manager) GetGeneration(ctx context.Context, clientID string) (int, error) {
	var gen int
	err := m.db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(generation), 0) FROM sessions WHERE client_id = ?
	`, clientID).Scan(&gen)
	return gen, err
}

// CleanupExpired 清理所有过期会话。
func (m *Manager) CleanupExpired(ctx context.Context) (int64, error) {
	result, err := m.db.ExecContext(ctx, `
		DELETE FROM sessions WHERE expires_at < ?
	`, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// SetSessionCookie 在 HTTP 响应中设置会话 Cookie。
func (m *Manager) SetSessionCookie(w http.ResponseWriter, sessionID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     m.cookieName,
		Value:    sessionID,
		Path:     "/",
		Domain:   m.domain,
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(m.ttl.Seconds()),
	})
}

// ClearSessionCookie 清除会话 Cookie。
func (m *Manager) ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     m.cookieName,
		Value:    "",
		Path:     "/",
		Domain:   m.domain,
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}

// GetSessionIDFromRequest 从请求中提取会话 ID。
func (m *Manager) GetSessionIDFromRequest(r *http.Request) string {
	cookie, err := r.Cookie(m.cookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}
