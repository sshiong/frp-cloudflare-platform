// Package mappings 提供映射管理 HTTP 处理器。
// 包括 TCP/UDP/HTTP 映射的 CRUD、端口分配、租约管理和配置版本控制。
package mappings

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/frp-panel/server-panel/internal/audit"
	"github.com/frp-panel/server-panel/internal/auth"
	"github.com/google/uuid"
)

// Handler 映射管理处理器。
type Handler struct {
	db     *sql.DB
	logger *slog.Logger
	audit  *audit.Logger
}

// NewHandler 创建映射管理处理器。
func NewHandler(db *sql.DB, logger *slog.Logger, auditLog *audit.Logger) *Handler {
	return &Handler{db: db, logger: logger, audit: auditLog}
}

// Mapping 映射实体。
type Mapping struct {
	ID           string `json:"id"`
	UserID       string `json:"user_id"`
	DeviceID     string `json:"device_id,omitempty"`
	Name         string `json:"name"`
	Protocol     string `json:"protocol"`
	LocalIP      string `json:"local_ip"`
	LocalPort    int    `json:"local_port"`
	RemotePort   int    `json:"remote_port,omitempty"`
	CustomDomain string `json:"custom_domain,omitempty"`
	ProxyName    string `json:"proxy_name"`
	ConfigVersion int   `json:"config_version"`
	Enabled      bool   `json:"enabled"`
	Status       string `json:"status"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// CreateRequest 创建映射请求。
type CreateRequest struct {
	DeviceID     string `json:"device_id"`
	Name         string `json:"name"`
	Protocol     string `json:"protocol"`
	LocalIP      string `json:"local_ip"`
	LocalPort    int    `json:"local_port"`
	RemotePort   int    `json:"remote_port,omitempty"` // 0 = 自动分配
	CustomDomain string `json:"custom_domain,omitempty"`
}

// Create 创建新映射。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// 参数验证
	if req.LocalPort <= 0 || req.LocalPort > 65535 {
		writeError(w, http.StatusBadRequest, "invalid local_port")
		return
	}
	if req.Protocol == "" {
		req.Protocol = "tcp"
	}
	validProtocols := map[string]bool{"tcp": true, "udp": true, "http": true, "https": true, "stcp": true, "xtcp": true}
	if !validProtocols[req.Protocol] {
		writeError(w, http.StatusBadRequest, "invalid protocol")
		return
	}
	if req.LocalIP == "" {
		req.LocalIP = "127.0.0.1"
	}

	userID := auth.GetUserIDFromContext(r.Context())

	// 验证设备属于当前用户
	if req.DeviceID != "" {
		var ownerID string
		if err := h.db.QueryRowContext(r.Context(), "SELECT user_id FROM devices WHERE id = ?", req.DeviceID).Scan(&ownerID); err != nil {
			writeError(w, http.StatusBadRequest, "device not found")
			return
		}
		if ownerID != userID {
			writeError(w, http.StatusForbidden, "device does not belong to you")
			return
		}
	}

	// 端口分配
	remotePort := req.RemotePort
	if remotePort == 0 && (req.Protocol == "tcp" || req.Protocol == "udp") {
		port, err := h.allocatePort(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to allocate port: "+err.Error())
			return
		}
		remotePort = port
	}

	// 生成 proxy_name
	proxyName := fmt.Sprintf("%s_%s_%s_%d", userID[:8], req.Protocol, req.Name, time.Now().Unix()%10000)
	if req.Name != "" {
		proxyName = fmt.Sprintf("%s_%s", userID[:8], req.Name)
	}

	id := uuid.New().String()
	_, err := h.db.ExecContext(r.Context(), `
		INSERT INTO mappings (id, user_id, device_id, name, protocol, local_ip, local_port, remote_port, custom_domain, proxy_name)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, userID, nullString(req.DeviceID), req.Name, req.Protocol, req.LocalIP, req.LocalPort,
		nullInt(remotePort), nullString(req.CustomDomain), proxyName)
	if err != nil {
		h.logger.Error("failed to create mapping", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to create mapping")
		return
	}

	// 创建端口租约
	if remotePort > 0 {
		_, _ = h.db.ExecContext(r.Context(), `
			INSERT OR REPLACE INTO port_leases (port, mapping_id, user_id, lease_type)
			VALUES (?, ?, ?, 'active')
		`, remotePort, id, userID)
	}

	h.audit.Log(r.Context(), audit.Entry{
		RequestID:  auth.GetRequestIDFromContext(r.Context()),
		UserID:     userID,
		Action:     "mapping.create",
		TargetType: "mapping",
		TargetID:   id,
		Detail:     map[string]interface{}{"protocol": req.Protocol, "remote_port": remotePort},
		IP:         r.RemoteAddr,
	})

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":           id,
		"remote_port":  remotePort,
		"proxy_name":   proxyName,
	})
}

// Get 获取映射详情。
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	mapping, err := h.getByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get mapping")
		return
	}
	if mapping == nil {
		writeError(w, http.StatusNotFound, "mapping not found")
		return
	}

	// 权限检查
	userID := auth.GetUserIDFromContext(r.Context())
	role := auth.GetUserRoleFromContext(r.Context())
	if mapping.UserID != userID && role != "admin" {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	writeJSON(w, http.StatusOK, mapping)
}

// List 列出映射。
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r.Context())
	role := auth.GetUserRoleFromContext(r.Context())

	query := `SELECT id, user_id, COALESCE(device_id,''), name, protocol, local_ip, local_port,
		COALESCE(remote_port,0), COALESCE(custom_domain,''), COALESCE(proxy_name,''),
		config_version, enabled, status, created_at, updated_at
		FROM mappings`
	var args []interface{}

	if role != "admin" {
		query += " WHERE user_id = ?"
		args = append(args, userID)
	}
	query += " ORDER BY created_at DESC"

	rows, err := h.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list mappings")
		return
	}
	defer rows.Close()

	var mappings []Mapping
	for rows.Next() {
		var m Mapping
		var enabled int
		if err := rows.Scan(&m.ID, &m.UserID, &m.DeviceID, &m.Name, &m.Protocol, &m.LocalIP, &m.LocalPort,
			&m.RemotePort, &m.CustomDomain, &m.ProxyName, &m.ConfigVersion, &enabled, &m.Status, &m.CreatedAt, &m.UpdatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to scan mapping")
			return
		}
		m.Enabled = enabled == 1
		mappings = append(mappings, m)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"mappings": mappings})
}

// Update 更新映射。
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	// 验证权限
	mapping, err := h.getByID(r.Context(), id)
	if err != nil || mapping == nil {
		writeError(w, http.StatusNotFound, "mapping not found")
		return
	}
	userID := auth.GetUserIDFromContext(r.Context())
	role := auth.GetUserRoleFromContext(r.Context())
	if mapping.UserID != userID && role != "admin" {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	var req struct {
		Name         string `json:"name,omitempty"`
		LocalIP      string `json:"local_ip,omitempty"`
		LocalPort    int    `json:"local_port,omitempty"`
		CustomDomain string `json:"custom_domain,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// 构建更新语句
	updates := []string{"updated_at = ?"}
	args := []interface{}{time.Now().UTC().Format(time.RFC3339)}
	if req.Name != "" {
		updates = append(updates, "name = ?")
		args = append(args, req.Name)
	}
	if req.LocalIP != "" {
		updates = append(updates, "local_ip = ?")
		args = append(args, req.LocalIP)
	}
	if req.LocalPort > 0 {
		updates = append(updates, "local_port = ?")
		args = append(args, req.LocalPort)
	}
	if req.CustomDomain != "" {
		updates = append(updates, "custom_domain = ?")
		args = append(args, req.CustomDomain)
	}

	// 递增配置版本
	updates = append(updates, "config_version = config_version + 1")
	args = append(args, id)

	query := "UPDATE mappings SET " + joinStrings(updates, ", ") + " WHERE id = ?"
	_, err = h.db.ExecContext(r.Context(), query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update mapping")
		return
	}

	h.audit.LogSimple(r.Context(), auth.GetRequestIDFromContext(r.Context()),
		userID, "mapping.update", "mapping", id, r.RemoteAddr)

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Delete 删除映射。
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	mapping, err := h.getByID(r.Context(), id)
	if err != nil || mapping == nil {
		writeError(w, http.StatusNotFound, "mapping not found")
		return
	}
	userID := auth.GetUserIDFromContext(r.Context())
	role := auth.GetUserRoleFromContext(r.Context())
	if mapping.UserID != userID && role != "admin" {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	// 释放端口租约
	if mapping.RemotePort > 0 {
		_, _ = h.db.ExecContext(r.Context(), "DELETE FROM port_leases WHERE port = ?", mapping.RemotePort)
	}

	_, err = h.db.ExecContext(r.Context(), "DELETE FROM mappings WHERE id = ?", id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete mapping")
		return
	}

	h.audit.LogSimple(r.Context(), auth.GetRequestIDFromContext(r.Context()),
		userID, "mapping.delete", "mapping", id, r.RemoteAddr)

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// SetEnabled 启用/禁用映射。
func (h *Handler) SetEnabled(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	enabled := r.URL.Query().Get("enabled")
	if id == "" || enabled == "" {
		writeError(w, http.StatusBadRequest, "id and enabled are required")
		return
	}

	enabledVal := 0
	status := "disabled"
	if enabled == "true" || enabled == "1" {
		enabledVal = 1
		status = "pending"
	}

	_, err := h.db.ExecContext(r.Context(), `
		UPDATE mappings SET enabled = ?, status = ?, config_version = config_version + 1, updated_at = ?
		WHERE id = ?
	`, enabledVal, status, time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update mapping")
		return
	}

	action := "mapping.disable"
	if enabledVal == 1 {
		action = "mapping.enable"
	}
	h.audit.LogSimple(r.Context(), auth.GetRequestIDFromContext(r.Context()),
		auth.GetUserIDFromContext(r.Context()), action, "mapping", id, r.RemoteAddr)

	writeJSON(w, http.StatusOK, map[string]string{"status": status})
}

// ForceDelete 强制删除映射（忽略错误状态）。
func (h *Handler) ForceDelete(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	_, _ = h.db.ExecContext(r.Context(), "DELETE FROM port_leases WHERE mapping_id = ?", id)
	_, err := h.db.ExecContext(r.Context(), "DELETE FROM mappings WHERE id = ?", id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to force delete mapping")
		return
	}

	h.audit.LogSimple(r.Context(), auth.GetRequestIDFromContext(r.Context()),
		auth.GetUserIDFromContext(r.Context()), "mapping.force_delete", "mapping", id, r.RemoteAddr)

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// getByID 获取映射。
func (h *Handler) getByID(ctx context.Context, id string) (*Mapping, error) {
	var m Mapping
	var enabled int
	err := h.db.QueryRowContext(ctx, `
		SELECT id, user_id, COALESCE(device_id,''), name, protocol, local_ip, local_port,
		COALESCE(remote_port,0), COALESCE(custom_domain,''), COALESCE(proxy_name,''),
		config_version, enabled, status, created_at, updated_at
		FROM mappings WHERE id = ?
	`, id).Scan(&m.ID, &m.UserID, &m.DeviceID, &m.Name, &m.Protocol, &m.LocalIP, &m.LocalPort,
		&m.RemotePort, &m.CustomDomain, &m.ProxyName, &m.ConfigVersion, &enabled, &m.Status, &m.CreatedAt, &m.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	m.Enabled = enabled == 1
	return &m, nil
}

// allocatePort 自动分配可用端口。
// 从 10000 开始查找未被占用的端口。
func (h *Handler) allocatePort(ctx context.Context, userID string) (int, error) {
	// 查找所有已分配的端口
	rows, err := h.db.QueryContext(ctx, "SELECT port FROM port_leases ORDER BY port")
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	usedPorts := make(map[int]bool)
	for rows.Next() {
		var port int
		if err := rows.Scan(&port); err != nil {
			return 0, err
		}
		usedPorts[port] = true
	}

	// 从 10000 开始查找可用端口
	for port := 10000; port <= 60000; port++ {
		if !usedPorts[port] {
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available ports")
}

func nullString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func nullInt(n int) interface{} {
	if n == 0 {
		return nil
	}
	return n
}

func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for _, s := range strs[1:] {
		result += sep + s
	}
	return result
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
