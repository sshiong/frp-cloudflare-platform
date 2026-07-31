// Package devices 提供设备管理 HTTP 处理器。
// 包括设备注册、信息获取、Token 轮换和解绑操作。
package devices

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/frp-panel/server-panel/internal/audit"
	"github.com/frp-panel/server-panel/internal/auth"
	"github.com/frp-panel/server-panel/internal/crypto"
	"github.com/google/uuid"
)

// Handler 设备管理处理器。
type Handler struct {
	db     *sql.DB
	logger *slog.Logger
	audit  *audit.Logger
}

// NewHandler 创建设备管理处理器。
func NewHandler(db *sql.DB, logger *slog.Logger, auditLog *audit.Logger) *Handler {
	return &Handler{db: db, logger: logger, audit: auditLog}
}

// Device 设备实体。
type Device struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	Name      string `json:"name"`
	Platform  string `json:"platform"`
	PublicKey string `json:"public_key"`
	Enabled   bool   `json:"enabled"`
	BoundAt   string `json:"bound_at"`
	LastSeen  string `json:"last_seen,omitempty"`
}

// RegisterRequest 设备注册请求。
type RegisterRequest struct {
	Name      string `json:"name"`
	Platform  string `json:"platform"`
	PublicKey string `json:"public_key"` // Ed25519 公钥（hex 编码）
}

// Register 设备首次注册（绑定到当前用户）。
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.PublicKey == "" {
		writeError(w, http.StatusBadRequest, "public_key is required")
		return
	}

	userID := auth.GetUserIDFromContext(r.Context())

	// 生成设备 token 和 ID
	deviceID := uuid.New().String()
	deviceToken := crypto.RandomToken(32)
	tokenHash := crypto.HMACSHA256Hex([]byte(deviceToken), []byte(deviceID))

	_, err := h.db.ExecContext(r.Context(), `
		INSERT INTO devices (id, user_id, name, platform, public_key, token_hash)
		VALUES (?, ?, ?, ?, ?, ?)
	`, deviceID, userID, req.Name, req.Platform, req.PublicKey, tokenHash)
	if err != nil {
		h.logger.Error("failed to register device", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to register device")
		return
	}

	h.audit.Log(r.Context(), audit.Entry{
		RequestID:  auth.GetRequestIDFromContext(r.Context()),
		UserID:     userID,
		Action:     "device.register",
		TargetType: "device",
		TargetID:   deviceID,
		Detail:     map[string]interface{}{"name": req.Name, "platform": req.Platform},
		IP:         r.RemoteAddr,
	})

	// 返回设备 token（仅此一次，后续不可获取）
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":           deviceID,
		"device_token": deviceToken,
	})
}

// GetCurrent 获取当前设备信息（设备 API）。
func (h *Handler) GetCurrent(w http.ResponseWriter, r *http.Request) {
	deviceID := auth.GetDeviceIDFromContext(r.Context())
	if deviceID == "" {
		writeError(w, http.StatusUnauthorized, "device authentication required")
		return
	}

	device, err := h.getByID(r.Context(), deviceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get device")
		return
	}
	if device == nil {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}

	writeJSON(w, http.StatusOK, device)
}

// RotateToken 轮换设备 Token。
func (h *Handler) RotateToken(w http.ResponseWriter, r *http.Request) {
	deviceID := r.URL.Query().Get("id")
	if deviceID == "" {
		deviceID = auth.GetDeviceIDFromContext(r.Context())
	}
	if deviceID == "" {
		writeError(w, http.StatusBadRequest, "device id is required")
		return
	}

	userID := auth.GetUserIDFromContext(r.Context())

	// 验证设备属于当前用户
	var ownerID string
	if err := h.db.QueryRowContext(r.Context(), "SELECT user_id FROM devices WHERE id = ?", deviceID).Scan(&ownerID); err != nil {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}
	if ownerID != userID {
		role := auth.GetUserRoleFromContext(r.Context())
		if role != "admin" {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
	}

	// 生成新 token
	newToken := crypto.RandomToken(32)
	newHash := crypto.HMACSHA256Hex([]byte(newToken), []byte(deviceID))

	_, err := h.db.ExecContext(r.Context(), `
		UPDATE devices SET token_hash = ? WHERE id = ?
	`, newHash, deviceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to rotate token")
		return
	}

	h.audit.LogSimple(r.Context(), auth.GetRequestIDFromContext(r.Context()),
		userID, "device.rotate_token", "device", deviceID, r.RemoteAddr)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":           deviceID,
		"device_token": newToken,
	})
}

// Unbind 解绑设备。
func (h *Handler) Unbind(w http.ResponseWriter, r *http.Request) {
	deviceID := r.URL.Query().Get("id")
	if deviceID == "" {
		writeError(w, http.StatusBadRequest, "device id is required")
		return
	}

	userID := auth.GetUserIDFromContext(r.Context())
	role := auth.GetUserRoleFromContext(r.Context())

	// 验证权限
	var ownerID string
	if err := h.db.QueryRowContext(r.Context(), "SELECT user_id FROM devices WHERE id = ?", deviceID).Scan(&ownerID); err != nil {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}
	if ownerID != userID && role != "admin" {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	// 清除关联的映射
	_, _ = h.db.ExecContext(r.Context(), "UPDATE mappings SET device_id = NULL WHERE device_id = ?", deviceID)

	// 删除设备
	_, err := h.db.ExecContext(r.Context(), "DELETE FROM devices WHERE id = ?", deviceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to unbind device")
		return
	}

	h.audit.LogSimple(r.Context(), auth.GetRequestIDFromContext(r.Context()),
		userID, "device.unbind", "device", deviceID, r.RemoteAddr)

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// List 列出设备（管理员可看全部，普通用户看自己的）。
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r.Context())
	role := auth.GetUserRoleFromContext(r.Context())

	query := "SELECT id, user_id, name, platform, public_key, enabled, bound_at, COALESCE(last_seen, '') FROM devices"
	var args []interface{}

	if role != "admin" {
		query += " WHERE user_id = ?"
		args = append(args, userID)
	}
	query += " ORDER BY bound_at DESC"

	rows, err := h.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list devices")
		return
	}
	defer rows.Close()

	var devices []Device
	for rows.Next() {
		var d Device
		var enabled int
		if err := rows.Scan(&d.ID, &d.UserID, &d.Name, &d.Platform, &d.PublicKey, &enabled, &d.BoundAt, &d.LastSeen); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to scan device")
			return
		}
		d.Enabled = enabled == 1
		devices = append(devices, d)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"devices": devices})
}

// Delete 删除设备（管理员）。
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	deviceID := r.URL.Query().Get("id")
	if deviceID == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	_, _ = h.db.ExecContext(r.Context(), "UPDATE mappings SET device_id = NULL WHERE device_id = ?", deviceID)
	result, err := h.db.ExecContext(r.Context(), "DELETE FROM devices WHERE id = ?", deviceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete device")
		return
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}

	h.audit.LogSimple(r.Context(), auth.GetRequestIDFromContext(r.Context()),
		auth.GetUserIDFromContext(r.Context()), "device.delete", "device", deviceID, r.RemoteAddr)

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// getByID 通过 ID 获取设备。
func (h *Handler) getByID(ctx sqlQueryContext, id string) (*Device, error) {
	var d Device
	var enabled int
	err := ctx.QueryRow(`
		SELECT id, user_id, name, platform, public_key, enabled, bound_at, COALESCE(last_seen, '')
		FROM devices WHERE id = ?
	`, id).Scan(&d.ID, &d.UserID, &d.Name, &d.Platform, &d.PublicKey, &enabled, &d.BoundAt, &d.LastSeen)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	d.Enabled = enabled == 1
	return &d, nil
}

type sqlQueryContext interface {
	QueryRow(query string, args ...interface{}) *sql.Row
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
