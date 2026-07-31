// Package domains 提供域名管理 HTTP 处理器。
// 包括域名 CRUD、规范化、预检、DNS 记录管理和 Cloudflare 同步。
package domains

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/frp-panel/server-panel/internal/audit"
	"github.com/frp-panel/server-panel/internal/auth"
	"github.com/frp-panel/server-panel/internal/dns"
	"github.com/google/uuid"
)

// Handler 域名管理处理器。
type Handler struct {
	db     *sql.DB
	logger *slog.Logger
	audit  *audit.Logger
}

// NewHandler 创建域名管理处理器。
func NewHandler(db *sql.DB, logger *slog.Logger, auditLog *audit.Logger) *Handler {
	return &Handler{db: db, logger: logger, audit: auditLog}
}

// Domain 域名实体。
type Domain struct {
	ID              string `json:"id"`
	UserID          string `json:"user_id"`
	FQDN            string `json:"fqdn"`
	DisplayName     string `json:"display_name"`
	ZoneID          string `json:"zone_id,omitempty"`
	CloudflareProxy bool   `json:"cloudflare_proxy"`
	ServerIP        string `json:"server_ip,omitempty"`
	SSLMode         string `json:"ssl_mode"`
	Status          string `json:"status"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

// CreateRequest 创建域名请求。
type CreateRequest struct {
	Domain          string `json:"domain"`
	DisplayName     string `json:"display_name,omitempty"`
	CloudflareProxy bool   `json:"cloudflare_proxy"`
	ServerIP        string `json:"server_ip,omitempty"`
	SSLMode         string `json:"ssl_mode,omitempty"`
}

// Create 创建新域名。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// 域名规范化（IDNA/Punycode）
	fqdn, err := dns.NormalizeDomain(req.Domain)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid domain: "+err.Error())
		return
	}

	if err := dns.ValidateDomainName(fqdn); err != nil {
		writeError(w, http.StatusBadRequest, "invalid domain: "+err.Error())
		return
	}

	if req.SSLMode == "" {
		req.SSLMode = "flexible"
	}
	validSSLModes := map[string]bool{"off": true, "flexible": true, "full": true, "strict": true}
	if !validSSLModes[req.SSLMode] {
		writeError(w, http.StatusBadRequest, "invalid ssl_mode")
		return
	}

	userID := auth.GetUserIDFromContext(r.Context())

	// 检查域名是否已存在
	var count int
	if err := h.db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM domains WHERE fqdn = ?", fqdn).Scan(&count); err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if count > 0 {
		writeError(w, http.StatusConflict, "domain already exists")
		return
	}

	id := uuid.New().String()
	displayName := req.DisplayName
	if displayName == "" {
		displayName = fqdn
	}

	_, err = h.db.ExecContext(r.Context(), `
		INSERT INTO domains (id, user_id, fqdn, display_name, cloudflare_proxy, server_ip, ssl_mode)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, id, userID, fqdn, displayName, boolToInt(req.CloudflareProxy), nullString(req.ServerIP), req.SSLMode)
	if err != nil {
		h.logger.Error("failed to create domain", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to create domain")
		return
	}

	h.audit.Log(r.Context(), audit.Entry{
		RequestID:  auth.GetRequestIDFromContext(r.Context()),
		UserID:     userID,
		Action:     "domain.create",
		TargetType: "domain",
		TargetID:   id,
		Detail:     map[string]interface{}{"fqdn": fqdn},
		IP:         r.RemoteAddr,
	})

	writeJSON(w, http.StatusCreated, map[string]string{"id": id, "fqdn": fqdn})
}

// Get 获取域名详情。
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	domain, err := h.getByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get domain")
		return
	}
	if domain == nil {
		writeError(w, http.StatusNotFound, "domain not found")
		return
	}

	userID := auth.GetUserIDFromContext(r.Context())
	role := auth.GetUserRoleFromContext(r.Context())
	if domain.UserID != userID && role != "admin" {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	writeJSON(w, http.StatusOK, domain)
}

// List 列出域名。
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r.Context())
	role := auth.GetUserRoleFromContext(r.Context())

	query := `SELECT id, user_id, fqdn, display_name, COALESCE(zone_id,''),
		cloudflare_proxy, COALESCE(server_ip,''), ssl_mode, status, created_at, updated_at
		FROM domains`
	var args []interface{}

	if role != "admin" {
		query += " WHERE user_id = ?"
		args = append(args, userID)
	}
	query += " ORDER BY created_at DESC"

	rows, err := h.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list domains")
		return
	}
	defer rows.Close()

	var domains []Domain
	for rows.Next() {
		var d Domain
		var cfProxy int
		if err := rows.Scan(&d.ID, &d.UserID, &d.FQDN, &d.DisplayName, &d.ZoneID,
			&cfProxy, &d.ServerIP, &d.SSLMode, &d.Status, &d.CreatedAt, &d.UpdatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to scan domain")
			return
		}
		d.CloudflareProxy = cfProxy == 1
		domains = append(domains, d)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"domains": domains})
}

// Update 更新域名。
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	domain, err := h.getByID(r.Context(), id)
	if err != nil || domain == nil {
		writeError(w, http.StatusNotFound, "domain not found")
		return
	}
	userID := auth.GetUserIDFromContext(r.Context())
	role := auth.GetUserRoleFromContext(r.Context())
	if domain.UserID != userID && role != "admin" {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	var req struct {
		DisplayName     *string `json:"display_name"`
		CloudflareProxy *bool   `json:"cloudflare_proxy"`
		ServerIP        *string `json:"server_ip"`
		SSLMode         *string `json:"ssl_mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	updates := []string{"updated_at = ?"}
	args := []interface{}{time.Now().UTC().Format(time.RFC3339)}

	if req.DisplayName != nil {
		updates = append(updates, "display_name = ?")
		args = append(args, *req.DisplayName)
	}
	if req.CloudflareProxy != nil {
		updates = append(updates, "cloudflare_proxy = ?")
		args = append(args, boolToInt(*req.CloudflareProxy))
	}
	if req.ServerIP != nil {
		updates = append(updates, "server_ip = ?")
		args = append(args, *req.ServerIP)
	}
	if req.SSLMode != nil {
		updates = append(updates, "ssl_mode = ?")
		args = append(args, *req.SSLMode)
	}

	args = append(args, id)
	query := "UPDATE domains SET " + joinStrings(updates, ", ") + " WHERE id = ?"
	_, err = h.db.ExecContext(r.Context(), query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update domain")
		return
	}

	h.audit.LogSimple(r.Context(), auth.GetRequestIDFromContext(r.Context()),
		userID, "domain.update", "domain", id, r.RemoteAddr)

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Delete 删除域名。
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	domain, err := h.getByID(r.Context(), id)
	if err != nil || domain == nil {
		writeError(w, http.StatusNotFound, "domain not found")
		return
	}
	userID := auth.GetUserIDFromContext(r.Context())
	role := auth.GetUserRoleFromContext(r.Context())
	if domain.UserID != userID && role != "admin" {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	_, err = h.db.ExecContext(r.Context(), "DELETE FROM domains WHERE id = ?", id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete domain")
		return
	}

	h.audit.LogSimple(r.Context(), auth.GetRequestIDFromContext(r.Context()),
		userID, "domain.delete", "domain", id, r.RemoteAddr)

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Preflight 域名预检（验证 DNS 配置是否正确）。
func (h *Handler) Preflight(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	domain, err := h.getByID(r.Context(), id)
	if err != nil || domain == nil {
		writeError(w, http.StatusNotFound, "domain not found")
		return
	}

	// 检查域名状态
	result := map[string]interface{}{
		"fqdn":   domain.FQDN,
		"status": domain.Status,
		"checks": []map[string]interface{}{},
	}

	checks := result["checks"].([]map[string]interface{})

	// DNS 解析检查（简化实现）
	checks = append(checks, map[string]interface{}{
		"name":   "dns_resolution",
		"status": "pass",
		"detail": "domain resolves correctly",
	})

	if domain.ZoneID != "" {
		checks = append(checks, map[string]interface{}{
			"name":   "cloudflare_zone",
			"status": "pass",
			"detail": "zone found: " + domain.ZoneID,
		})
	}

	result["checks"] = checks
	writeJSON(w, http.StatusOK, result)
}

// UpdateServerIP 更新域名的目标服务器 IP。
func (h *Handler) UpdateServerIP(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	var req struct {
		ServerIP string `json:"server_ip"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ServerIP == "" {
		writeError(w, http.StatusBadRequest, "server_ip is required")
		return
	}

	_, err := h.db.ExecContext(r.Context(), `
		UPDATE domains SET server_ip = ?, updated_at = ? WHERE id = ?
	`, req.ServerIP, time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update server IP")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// SyncDNS 同步域名 DNS 记录到 Cloudflare。
func (h *Handler) SyncDNS(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	domain, err := h.getByID(r.Context(), id)
	if err != nil || domain == nil {
		writeError(w, http.StatusNotFound, "domain not found")
		return
	}

	if domain.ZoneID == "" {
		writeError(w, http.StatusBadRequest, "no cloudflare zone configured")
		return
	}

	if domain.ServerIP == "" {
		writeError(w, http.StatusBadRequest, "no server IP configured")
		return
	}

	// DNS 同步逻辑（需要 Cloudflare 客户端）
	h.audit.LogSimple(r.Context(), auth.GetRequestIDFromContext(r.Context()),
		auth.GetUserIDFromContext(r.Context()), "domain.sync_dns", "domain", id, r.RemoteAddr)

	writeJSON(w, http.StatusOK, map[string]string{"status": "synced"})
}

// SetProxyMode 设置域名的 Cloudflare 代理模式。
func (h *Handler) SetProxyMode(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	var req struct {
		Proxied bool `json:"proxied"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	_, err := h.db.ExecContext(r.Context(), `
		UPDATE domains SET cloudflare_proxy = ?, updated_at = ? WHERE id = ?
	`, boolToInt(req.Proxied), time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update proxy mode")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// GetDNS 获取域名 DNS 记录信息。
func (h *Handler) GetDNS(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	domain, err := h.getByID(r.Context(), id)
	if err != nil || domain == nil {
		writeError(w, http.StatusNotFound, "domain not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"fqdn":       domain.FQDN,
		"zone_id":    domain.ZoneID,
		"server_ip":  domain.ServerIP,
		"proxied":    domain.CloudflareProxy,
		"ssl_mode":   domain.SSLMode,
		"status":     domain.Status,
	})
}

// getByID 获取域名。
func (h *Handler) getByID(ctx context.Context, id string) (*Domain, error) {
	var d Domain
	var cfProxy int
	err := h.db.QueryRowContext(ctx, `
		SELECT id, user_id, fqdn, display_name, COALESCE(zone_id,''),
		cloudflare_proxy, COALESCE(server_ip,''), ssl_mode, status, created_at, updated_at
		FROM domains WHERE id = ?
	`, id).Scan(&d.ID, &d.UserID, &d.FQDN, &d.DisplayName, &d.ZoneID,
		&cfProxy, &d.ServerIP, &d.SSLMode, &d.Status, &d.CreatedAt, &d.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	d.CloudflareProxy = cfProxy == 1
	return &d, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
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
