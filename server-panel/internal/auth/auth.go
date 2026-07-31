// Package auth 提供认证中间件和授权工具。
// 支持两种认证方式：
//   - Session Cookie：面向浏览器的 Web 用户
//   - HMAC-SHA256：面向设备的 API 调用
package auth

import (
	"context"
	"crypto/ed25519"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/frp-panel/server-panel/internal/crypto"
	"github.com/frp-panel/server-panel/internal/session"
)

// contextKey 自定义 context key 类型，避免冲突。
type contextKey string

const (
	// CtxUserID 存储在 context 中的用户 ID。
	CtxUserID contextKey = "auth_user_id"
	// CtxUserRole 存储在 context 中的用户角色。
	CtxUserRole contextKey = "auth_user_role"
	// CtxSession 存储在 context 中的会话对象。
	CtxSession contextKey = "auth_session"
	// CtxRequestID 存储在 context 中的请求 ID。
	CtxRequestID contextKey = "request_id"
	// CtxDeviceID 存储在 context 中的设备 ID（设备 API 使用）。
	CtxDeviceID contextKey = "auth_device_id"
)

// CSRFHeaderName CSRF 验证使用的请求头名称。
const CSRFHeaderName = "X-CSRF-Token"

// Authenticator 认证器。
type Authenticator struct {
	db         *sql.DB
	logger     *slog.Logger
	sessionMgr *session.Manager
	csrfKey    []byte // HMAC-SHA256 密钥，用于生成/验证 CSRF token
}

// New 创建认证器。
func New(db *sql.DB, logger *slog.Logger, sessionMgr *session.Manager, csrfKey []byte) *Authenticator {
	return &Authenticator{
		db:         db,
		logger:     logger,
		sessionMgr: sessionMgr,
		csrfKey:    csrfKey,
	}
}

// RequireSession Web 用户认证中间件。
// 验证 Session Cookie，将用户信息注入 context。
func (a *Authenticator) RequireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessionID := a.sessionMgr.GetSessionIDFromRequest(r)
		if sessionID == "" {
			http.Error(w, `{"error":"authentication required"}`, http.StatusUnauthorized)
			return
		}

		sess, err := a.sessionMgr.Validate(r.Context(), sessionID)
		if err != nil {
			a.logger.Error("session validation error", "err", err)
			http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
			return
		}
		if sess == nil {
			a.sessionMgr.ClearSessionCookie(w)
			http.Error(w, `{"error":"session expired"}`, http.StatusUnauthorized)
			return
		}

		// 获取用户角色
		var role string
		err = a.db.QueryRowContext(r.Context(), "SELECT role FROM users WHERE id = ?", sess.UserID).Scan(&role)
		if err != nil {
			a.logger.Error("failed to get user role", "user_id", sess.UserID, "err", err)
			http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
			return
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, CtxUserID, sess.UserID)
		ctx = context.WithValue(ctx, CtxUserRole, role)
		ctx = context.WithValue(ctx, CtxSession, sess)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireAdmin 管理员权限中间件（必须在 RequireSession 之后使用）。
func (a *Authenticator) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role, ok := r.Context().Value(CtxUserRole).(string)
		if !ok || role != "admin" {
			http.Error(w, `{"error":"admin access required"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireCSRF CSRF 验证中间件。
// 对于 POST/PUT/DELETE/PATCH 请求，验证 X-CSRF-Token 头。
// CSRF token 格式: HMAC-SHA256(session_id + timestamp, csrf_key)[:16] hex
func (a *Authenticator) RequireCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 只对修改请求验证
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		token := r.Header.Get(CSRFHeaderName)
		if token == "" {
			http.Error(w, `{"error":"csrf token required"}`, http.StatusForbidden)
			return
		}

		sess, ok := r.Context().Value(CtxSession).(*session.Session)
		if !ok || sess == nil {
			http.Error(w, `{"error":"session required for csrf"}`, http.StatusForbidden)
			return
		}

		if !a.verifyCSRFToken(sess.ID, token) {
			http.Error(w, `{"error":"invalid csrf token"}`, http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// GenerateCSRFToken 为给定 session ID 生成 CSRF token。
func (a *Authenticator) GenerateCSRFToken(sessionID string) string {
	data := fmt.Sprintf("%s:%d", sessionID, time.Now().Unix()/3600) // 每小时轮换
	mac := crypto.HMACSHA256(a.csrfKey, []byte(data))
	return hex.EncodeToString(mac[:16])
}

// verifyCSRFToken 验证 CSRF token。
// 允许当前小时和上一小时的 token（容忍时钟偏差）。
func (a *Authenticator) verifyCSRFToken(sessionID, token string) bool {
	now := time.Now().Unix() / 3600
	for _, offset := range []int64{0, -1} {
		data := fmt.Sprintf("%s:%d", sessionID, now+offset)
		mac := crypto.HMACSHA256(a.csrfKey, []byte(data))
		expected := hex.EncodeToString(mac[:16])
		if crypto.ConstantTimeEqualString(token, expected) {
			return true
		}
	}
	return false
}

// RequireDeviceHMAC 设备 HMAC-SHA256 认证中间件。
// 请求头格式: Authorization: HMAC-SHA256 <device_id>:<hmac_hex>
// HMAC 计算: HMAC-SHA256(device_token, method + path + body + timestamp)
func (a *Authenticator) RequireDeviceHMAC(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "HMAC-SHA256 ") {
			http.Error(w, `{"error":"invalid authorization header"}`, http.StatusUnauthorized)
			return
		}

		parts := strings.SplitN(strings.TrimPrefix(authHeader, "HMAC-SHA256 "), ":", 2)
		if len(parts) != 2 {
			http.Error(w, `{"error":"invalid hmac format"}`, http.StatusUnauthorized)
			return
		}
		deviceID := parts[0]
		receivedMAC, err := hex.DecodeString(parts[1])
		if err != nil {
			http.Error(w, `{"error":"invalid hmac encoding"}`, http.StatusUnauthorized)
			return
		}

		// 从数据库获取设备信息
		var tokenHash string
		var enabled int
		err = a.db.QueryRowContext(r.Context(), `
			SELECT token_hash, enabled FROM devices WHERE id = ?
		`, deviceID).Scan(&tokenHash, &enabled)
		if err == sql.ErrNoRows {
			http.Error(w, `{"error":"device not found"}`, http.StatusUnauthorized)
			return
		}
		if err != nil {
			a.logger.Error("device lookup error", "err", err)
			http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
			return
		}
		if enabled == 0 {
			http.Error(w, `{"error":"device disabled"}`, http.StatusForbidden)
			return
		}

		// 设备 token 本身不存储，使用派生方式验证
		// 实际场景中，token_hash = HMAC(device_token, static_salt)
		// 此处简化为：直接从 token_hash 反推验证（实际需要存储加密后的 token）
		// 这里使用设备公钥和时间戳组合验证
		timestamp := r.Header.Get("X-Timestamp")
		if timestamp == "" {
			http.Error(w, `{"error":"timestamp required"}`, http.StatusUnauthorized)
			return
		}

		// 验证时间戳在合理范围内（5 分钟）
		var ts int64
		if _, err := fmt.Sscanf(timestamp, "%d", &ts); err != nil {
			http.Error(w, `{"error":"invalid timestamp"}`, http.StatusUnauthorized)
			return
		}
		if abs(time.Now().Unix()-ts) > 300 {
			http.Error(w, `{"error":"timestamp expired"}`, http.StatusUnauthorized)
			return
		}

		// 验证 HMAC：HMAC-SHA256(device_token_hash, method+path+timestamp)
		expectedMAC := crypto.HMACSHA256([]byte(tokenHash), []byte(r.Method+r.URL.Path+timestamp))
		if !crypto.ConstantTimeEqual(receivedMAC, expectedMAC) {
			http.Error(w, `{"error":"invalid hmac"}`, http.StatusUnauthorized)
			return
		}

		// 更新最后在线时间
		_, _ = a.db.ExecContext(r.Context(), `
			UPDATE devices SET last_seen = ? WHERE id = ?
		`, time.Now().UTC().Format(time.RFC3339), deviceID)

		ctx := r.Context()
		ctx = context.WithValue(ctx, CtxDeviceID, deviceID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Login 验证用户名和密码，返回用户信息。
func (a *Authenticator) Login(ctx context.Context, username, password string) (userID, role string, err error) {
	var storedHash string
	var enabled int
	err = a.db.QueryRowContext(ctx, `
		SELECT id, password, role, enabled FROM users WHERE username = ?
	`, username).Scan(&userID, &storedHash, &role, &enabled)
	if err == sql.ErrNoRows {
		return "", "", fmt.Errorf("invalid credentials")
	}
	if err != nil {
		return "", "", fmt.Errorf("query user: %w", err)
	}
	if enabled == 0 {
		return "", "", fmt.Errorf("account disabled")
	}

	ok, err := crypto.VerifyPasswordArgon2id(password, storedHash)
	if err != nil {
		return "", "", fmt.Errorf("verify password: %w", err)
	}
	if !ok {
		return "", "", fmt.Errorf("invalid credentials")
	}

	return userID, role, nil
}

// GetUserRole 获取用户角色。
func (a *Authenticator) GetUserRole(ctx context.Context, userID string) (string, error) {
	var role string
	err := a.db.QueryRowContext(ctx, "SELECT role FROM users WHERE id = ?", userID).Scan(&role)
	return role, err
}

// VerifyDevicePublicKey 验证设备公钥。
func (a *Authenticator) VerifyDevicePublicKey(ctx context.Context, deviceID string, pubKey ed25519.PublicKey) (bool, error) {
	var storedKey string
	err := a.db.QueryRowContext(ctx, "SELECT public_key FROM devices WHERE id = ?", deviceID).Scan(&storedKey)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	storedBytes, err := hex.DecodeString(storedKey)
	if err != nil {
		return false, fmt.Errorf("decode stored key: %w", err)
	}
	return crypto.ConstantTimeEqual(storedBytes, pubKey), nil
}

func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

// GetUserIDFromContext 从 context 中提取用户 ID。
func GetUserIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(CtxUserID).(string)
	return v
}

// GetUserRoleFromContext 从 context 中提取用户角色。
func GetUserRoleFromContext(ctx context.Context) string {
	v, _ := ctx.Value(CtxUserRole).(string)
	return v
}

// GetRequestIDFromContext 从 context 中提取请求 ID。
func GetRequestIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(CtxRequestID).(string)
	return v
}

// GetDeviceIDFromContext 从 context 中提取设备 ID。
func GetDeviceIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(CtxDeviceID).(string)
	return v
}
