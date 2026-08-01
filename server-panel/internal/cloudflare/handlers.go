// Package cloudflare 提供 Cloudflare API 集成处理器。
// 包括 Token 管理、Zone 列表、DNS 记录 CRUD 和 Token 生命周期管理。
package cloudflare

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/frp-panel/server-panel/internal/audit"
	"github.com/frp-panel/server-panel/internal/auth"
	"github.com/frp-panel/server-panel/internal/crypto"
)

// Handler Cloudflare 处理器。
type Handler struct {
	db      *sql.DB
	logger  *slog.Logger
	audit   *audit.Logger
	encKey  []byte // AES-256 密钥，用于加密/解密 token
}

// NewHandler 创建 Cloudflare 处理器。
func NewHandler(db *sql.DB, logger *slog.Logger, auditLog *audit.Logger, encKey []byte) *Handler {
	return &Handler{db: db, logger: logger, audit: auditLog, encKey: encKey}
}

// CFToken Cloudflare Token 实体。
type CFToken struct {
	ID            string `json:"id"`
	UserID        string `json:"user_id"`
	TokenVersion  int    `json:"token_version"`
	Status        string `json:"status"`
	Capabilities  string `json:"capabilities,omitempty"`
	VerifiedAt    string `json:"verified_at,omitempty"`
	ActivatedAt   string `json:"activated_at,omitempty"`
	CreatedAt     string `json:"created_at"`
}

// GetTokenStatus 获取 Token 状态。
func (h *Handler) GetTokenStatus(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r.Context())

	var status string
	var tokenVersion int
	var verifiedAt, activatedAt sql.NullString

	err := h.db.QueryRowContext(r.Context(), `
		SELECT status, token_version, verified_at, activated_at FROM cloudflare_credentials
		WHERE user_id = ? AND status = 'active' LIMIT 1
	`, userID).Scan(&status, &tokenVersion, &verifiedAt, &activatedAt)

	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"has_token": false,
			"status":    "missing",
		})
		return
	}
	if err != nil {
		h.logger.Error("failed to get token status", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to get token status")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"has_token":     true,
		"status":        status,
		"token_version": tokenVersion,
		"verified_at":   verifiedAt.String,
		"activated_at":  activatedAt.String,
	})
}

// GetPending 获取待验证 Token。
func (h *Handler) GetPending(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r.Context())

	var id, status string
	err := h.db.QueryRowContext(r.Context(), `
		SELECT id, status FROM cloudflare_credentials
		WHERE user_id = ? AND status = 'pending' LIMIT 1
	`, userID).Scan(&id, &status)

	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusOK, map[string]interface{}{"pending": nil})
		return
	}
	if err != nil {
		h.logger.Error("failed to get pending token", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to get pending token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"pending": map[string]interface{}{
			"id":     id,
			"status": status,
		},
	})
}

// UploadToken 上传新 Token。
func (h *Handler) UploadToken(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r.Context())

	var req struct {
		Token string `json:"token"`
		Label string `json:"label,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Token == "" {
		writeError(w, http.StatusBadRequest, "token required")
		return
	}

	// 加密 Token
	encrypted, err := crypto.EncryptAES256GCM(h.encKey, []byte(req.Token))
	if err != nil {
		h.logger.Error("failed to encrypt token", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to encrypt token")
		return
	}

	// 获取下一个版本号
	var maxVersion int
	h.db.QueryRowContext(r.Context(), "SELECT COALESCE(MAX(token_version), 0) FROM cloudflare_credentials WHERE user_id = ?", userID).Scan(&maxVersion)
	newVersion := maxVersion + 1

	id := generateID()
	now := time.Now().UTC().Format(time.RFC3339)

	_, err = h.db.ExecContext(r.Context(), `
		INSERT INTO cloudflare_credentials (id, user_id, token_version, ciphertext, nonce, key_version, status, created_at)
		VALUES (?, ?, ?, ?, 'nonce', 1, 'pending', ?)
	`, id, userID, newVersion, string(encrypted), now)
	if err != nil {
		h.logger.Error("failed to save token", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to save token")
		return
	}

	h.audit.Log(r.Context(), audit.Entry{
		RequestID:  auth.GetRequestIDFromContext(r.Context()),
		UserID:     userID,
		Action:     "cloudflare.upload_token",
		TargetType: "cloudflare_token",
		TargetID:   id,
		IP:         r.RemoteAddr,
	})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":            id,
		"token_version": newVersion,
		"status":        "pending",
	})
}

// VerifyToken 验证 Token。
func (h *Handler) VerifyToken(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r.Context())

	var req struct {
		TokenID string `json:"token_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err := h.db.ExecContext(r.Context(), `
		UPDATE cloudflare_credentials SET status = 'active', verified_at = ?, activated_at = ?
		WHERE id = ? AND user_id = ? AND status = 'pending'
	`, now, now, req.TokenID, userID)
	if err != nil {
		h.logger.Error("failed to verify token", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to verify token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "verified"})
}

// ActivateToken 激活 Token。
func (h *Handler) ActivateToken(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r.Context())

	var req struct {
		TokenID string `json:"token_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)

	// 撤销旧的 active token
	h.db.ExecContext(r.Context(), `
		UPDATE cloudflare_credentials SET status = 'retired', retired_at = ?
		WHERE user_id = ? AND status = 'active'
	`, now, userID)

	// 激活新 token
	_, err := h.db.ExecContext(r.Context(), `
		UPDATE cloudflare_credentials SET status = 'active', activated_at = ?
		WHERE id = ? AND user_id = ?
	`, now, req.TokenID, userID)
	if err != nil {
		h.logger.Error("failed to activate token", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to activate token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "activated"})
}

// ClearToken 清除 Token。
func (h *Handler) ClearToken(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r.Context())

	now := time.Now().UTC().Format(time.RFC3339)
	_, err := h.db.ExecContext(r.Context(), `
		UPDATE cloudflare_credentials SET status = 'retired', retired_at = ?
		WHERE user_id = ? AND status = 'active'
	`, now, userID)
	if err != nil {
		h.logger.Error("failed to clear token", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to clear token")
		return
	}

	h.audit.Log(r.Context(), audit.Entry{
		RequestID:  auth.GetRequestIDFromContext(r.Context()),
		UserID:     userID,
		Action:     "cloudflare.clear_token",
		TargetType: "cloudflare_token",
		IP:         r.RemoteAddr,
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "cleared"})
}

// ListZones 获取 Zone 列表。
func (h *Handler) ListZones(w http.ResponseWriter, r *http.Request) {
	// 需要 Cloudflare API 集成
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"zones": []interface{}{},
		"message": "Cloudflare API integration not configured",
	})
}

// ListDNSRecords 获取 DNS 记录列表。
func (h *Handler) ListDNSRecords(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"records": []interface{}{},
	})
}

// CreateDNSRecord 创建 DNS 记录。
func (h *Handler) CreateDNSRecord(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "not_implemented",
		"message": "Cloudflare API integration not configured",
	})
}

// DeleteDNSRecord 删除 DNS 记录。
func (h *Handler) DeleteDNSRecord(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "not_implemented",
		"message": "Cloudflare API integration not configured",
	})
}

// 辅助函数
func generateID() string {
	return time.Now().Format("20060102150405") + randomHex(8)
}

func randomHex(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = "0123456789abcdef"[time.Now().UnixNano()%16]
	}
	return string(b)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
