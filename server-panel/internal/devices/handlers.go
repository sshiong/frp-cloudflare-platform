// Package devices 提供设备管理 HTTP 处理器。
// 包括设备注册、信息获取、Token 轮换和解绑操作。
package devices

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

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
	ID                string `json:"id"`
	UserID            string `json:"user_id"`
	Name              string `json:"name"`
	Status            string `json:"status"`
	ClientPanelVersion string `json:"client_panel_version,omitempty"`
	FrpcVersion       string `json:"frpc_version,omitempty"`
	LastSeenAt        string `json:"last_seen_at,omitempty"`
	RegisteredAt      string `json:"registered_at"`
}

// RegisterRequest 设备注册请求。
type RegisterRequest struct {
	Name                 string `json:"name"`
	InstallationInstanceID string `json:"installation_instance_id"`
	ClientPanelVersion   string `json:"client_panel_version"`
	FrpcVersion          string `json:"frpc_version"`
}

// Register 设备首次注册（绑定到当前用户）。
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		req.Name = "Unnamed Device"
	}
	if req.InstallationInstanceID == "" {
		req.InstallationInstanceID = uuid.New().String()
	}

	id := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := h.db.ExecContext(r.Context(), `
		INSERT INTO clients (id, server_instance_id, owner_user_id, installation_instance_id, name, status, client_panel_version, frpc_version, registered_at)
		VALUES (?, 'default', ?, ?, ?, 'bound', ?, ?, ?)
	`, id, userID, req.InstallationInstanceID, req.Name, req.ClientPanelVersion, req.FrpcVersion, now)
	if err != nil {
		h.logger.Error("failed to register device", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to register device")
		return
	}

	// 生成设备凭证
	deviceToken := crypto.RandomToken(32)
	frpDeviceToken := crypto.RandomToken(32)

	h.audit.Log(r.Context(), audit.Entry{
		RequestID: auth.GetRequestIDFromContext(r.Context()),
		UserID:    userID,
		Action:    "device.register",
		TargetType: "device",
		TargetID:   id,
		IP:        r.RemoteAddr,
	})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"device_id":        id,
		"device_token":     deviceToken,
		"frp_device_token": frpDeviceToken,
	})
}

// List 获取设备列表。
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r.Context())
	role := auth.GetUserRoleFromContext(r.Context())

	query := `SELECT id, owner_user_id, name, status, COALESCE(client_panel_version,''), COALESCE(frpc_version,''),
		COALESCE(last_seen_at,''), registered_at FROM clients`
	var args []interface{}

	if role != "admin" {
		query += " WHERE owner_user_id = ?"
		args = append(args, userID)
	}
	query += " ORDER BY registered_at DESC"

	rows, err := h.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		h.logger.Error("failed to list devices", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to list devices")
		return
	}
	defer rows.Close()

	var devices []Device
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.ID, &d.UserID, &d.Name, &d.Status, &d.ClientPanelVersion, &d.FrpcVersion, &d.LastSeenAt, &d.RegisteredAt); err != nil {
			h.logger.Error("failed to scan device", "err", err)
			writeError(w, http.StatusInternalServerError, "failed to scan device")
			return
		}
		devices = append(devices, d)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"devices": devices})
}

// GetCurrent 获取当前设备信息。
func (h *Handler) GetCurrent(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r.Context())

	var d Device
	err := h.db.QueryRowContext(r.Context(), `
		SELECT id, owner_user_id, name, status, COALESCE(client_panel_version,''), COALESCE(frpc_version,''),
		COALESCE(last_seen_at,''), registered_at FROM clients WHERE owner_user_id = ? LIMIT 1
	`, userID).Scan(&d.ID, &d.UserID, &d.Name, &d.Status, &d.ClientPanelVersion, &d.FrpcVersion, &d.LastSeenAt, &d.RegisteredAt)

	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusOK, map[string]interface{}{"device": nil})
		return
	}
	if err != nil {
		h.logger.Error("failed to get device", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to get device")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"device": d})
}

// RotateToken 轮换设备 Token。
func (h *Handler) RotateToken(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r.Context())

	newToken := crypto.RandomToken(32)
	newFrpToken := crypto.RandomToken(32)

	h.audit.Log(r.Context(), audit.Entry{
		RequestID: auth.GetRequestIDFromContext(r.Context()),
		UserID:    userID,
		Action:    "device.rotate_token",
		TargetType: "device",
		IP:        r.RemoteAddr,
	})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"device_token":     newToken,
		"frp_device_token": newFrpToken,
	})
}

// Unbind 解绑设备。
func (h *Handler) Unbind(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r.Context())

	result, err := h.db.ExecContext(r.Context(), `
		UPDATE clients SET status = 'unbound', unbound_at = datetime('now') WHERE owner_user_id = ? AND status = 'bound'
	`, userID)
	if err != nil {
		h.logger.Error("failed to unbind device", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to unbind device")
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		writeError(w, http.StatusNotFound, "no bound device found")
		return
	}

	h.audit.Log(r.Context(), audit.Entry{
		RequestID: auth.GetRequestIDFromContext(r.Context()),
		UserID:    userID,
		Action:    "device.unbind",
		TargetType: "device",
		IP:        r.RemoteAddr,
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "unbound"})
}

// Delete 删除设备（管理员）。
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	deviceID := r.URL.Query().Get("id")
	if deviceID == "" {
		writeError(w, http.StatusBadRequest, "device id required")
		return
	}

	result, err := h.db.ExecContext(r.Context(), "DELETE FROM clients WHERE id = ?", deviceID)
	if err != nil {
		h.logger.Error("failed to delete device", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to delete device")
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}

	h.audit.Log(r.Context(), audit.Entry{
		RequestID: auth.GetRequestIDFromContext(r.Context()),
		UserID:    auth.GetUserIDFromContext(r.Context()),
		Action:    "device.delete",
		TargetType: "device",
		TargetID:   deviceID,
		IP:        r.RemoteAddr,
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// 辅助函数
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
