// Package domains 提供域名管理 HTTP 处理器。
// 包括域名 CRUD、规范化、预检、DNS 记录管理和 Cloudflare 同步。
package domains

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/frp-panel/server-panel/internal/audit"
	"github.com/frp-panel/server-panel/internal/auth"
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
	ClientID        string `json:"client_id"`
	MappingID       string `json:"mapping_id"`
	Hostname        string `json:"hostname"`
	NormalizedDomain string `json:"normalized_domain"`
	ZoneID          string `json:"zone_id,omitempty"`
	HTTPsMode       string `json:"https_mode"`
	HTTPRedirect    bool   `json:"http_redirect"`
	Status          string `json:"status"`
	Revision        int    `json:"revision"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

// CreateRequest 创建域名请求。
type CreateRequest struct {
	Hostname   string `json:"hostname"`
	MappingID  string `json:"mapping_id"`
	HTTPsMode  string `json:"https_mode"`
	HTTPRedirect bool `json:"http_redirect"`
}

// Create 创建域名绑定。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Hostname == "" {
		writeError(w, http.StatusBadRequest, "hostname required")
		return
	}

	// 规范化域名
	normalizedDomain := normalizeDomain(req.Hostname)

	// 检查是否已存在
	var exists int
	err := h.db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM domain_bindings WHERE normalized_domain = ?", normalizedDomain).Scan(&exists)
	if err != nil {
		h.logger.Error("failed to check domain", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to check domain")
		return
	}
	if exists > 0 {
		writeError(w, http.StatusConflict, "domain already exists")
		return
	}

	if req.HTTPsMode == "" {
		req.HTTPsMode = "http_only"
	}

	id := uuid.New().String()
	clientID := "default" // TODO: 从请求或session获取
	mappingID := req.MappingID
	if mappingID == "" {
		mappingID = uuid.New().String()
	}

	_, err = h.db.ExecContext(r.Context(), `
		INSERT INTO domain_bindings (id, user_id, client_id, mapping_id, hostname, normalized_domain, https_mode, http_redirect, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'reserved')
	`, id, userID, clientID, mappingID, req.Hostname, normalizedDomain, req.HTTPsMode, boolToInt(req.HTTPRedirect))
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
		IP:         r.RemoteAddr,
	})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":      id,
		"status":  "reserved",
		"message": "domain created successfully",
	})
}

// List 获取域名列表。
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r.Context())
	role := auth.GetUserRoleFromContext(r.Context())

	query := `SELECT id, user_id, client_id, mapping_id, hostname, normalized_domain,
		COALESCE(zone_id,''), https_mode, http_redirect, status, revision, created_at, updated_at
		FROM domain_bindings`
	var args []interface{}

	if role != "admin" {
		query += " WHERE user_id = ?"
		args = append(args, userID)
	}
	query += " ORDER BY created_at DESC"

	rows, err := h.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		h.logger.Error("failed to list domains", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to list domains")
		return
	}
	defer rows.Close()

	var domains []Domain
	for rows.Next() {
		var d Domain
		var httpRedirect int
		if err := rows.Scan(&d.ID, &d.UserID, &d.ClientID, &d.MappingID, &d.Hostname, &d.NormalizedDomain,
			&d.ZoneID, &d.HTTPsMode, &httpRedirect, &d.Status, &d.Revision, &d.CreatedAt, &d.UpdatedAt); err != nil {
			h.logger.Error("failed to scan domain", "err", err)
			writeError(w, http.StatusInternalServerError, "failed to scan domain")
			return
		}
		d.HTTPRedirect = httpRedirect == 1
		domains = append(domains, d)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"domains": domains})
}

// Get 获取单个域名。
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	domainID := r.URL.Query().Get("id")
	if domainID == "" {
		writeError(w, http.StatusBadRequest, "domain id required")
		return
	}

	var d Domain
	var httpRedirect int
	err := h.db.QueryRowContext(r.Context(), `
		SELECT id, user_id, client_id, mapping_id, hostname, normalized_domain,
		COALESCE(zone_id,''), https_mode, http_redirect, status, revision, created_at, updated_at
		FROM domain_bindings WHERE id = ?
	`, domainID).Scan(&d.ID, &d.UserID, &d.ClientID, &d.MappingID, &d.Hostname, &d.NormalizedDomain,
		&d.ZoneID, &d.HTTPsMode, &httpRedirect, &d.Status, &d.Revision, &d.CreatedAt, &d.UpdatedAt)

	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "domain not found")
		return
	}
	if err != nil {
		h.logger.Error("failed to get domain", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to get domain")
		return
	}
	d.HTTPRedirect = httpRedirect == 1
	writeJSON(w, http.StatusOK, d)
}

// Update 更新域名。
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	domainID := r.URL.Query().Get("id")
	if domainID == "" {
		writeError(w, http.StatusBadRequest, "domain id required")
		return
	}

	var req struct {
		HTTPsMode    string `json:"https_mode"`
		HTTPRedirect *bool  `json:"http_redirect"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.HTTPsMode != "" {
		_, err := h.db.ExecContext(r.Context(), "UPDATE domain_bindings SET https_mode = ?, updated_at = ? WHERE id = ?",
			req.HTTPsMode, time.Now().UTC().Format(time.RFC3339), domainID)
		if err != nil {
			h.logger.Error("failed to update domain", "err", err)
			writeError(w, http.StatusInternalServerError, "failed to update domain")
			return
		}
	}

	if req.HTTPRedirect != nil {
		_, err := h.db.ExecContext(r.Context(), "UPDATE domain_bindings SET http_redirect = ?, updated_at = ? WHERE id = ?",
			boolToInt(*req.HTTPRedirect), time.Now().UTC().Format(time.RFC3339), domainID)
		if err != nil {
			h.logger.Error("failed to update domain", "err", err)
			writeError(w, http.StatusInternalServerError, "failed to update domain")
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// Delete 删除域名。
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	domainID := r.URL.Query().Get("id")
	if domainID == "" {
		writeError(w, http.StatusBadRequest, "domain id required")
		return
	}

	result, err := h.db.ExecContext(r.Context(), "DELETE FROM domain_bindings WHERE id = ?", domainID)
	if err != nil {
		h.logger.Error("failed to delete domain", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to delete domain")
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		writeError(w, http.StatusNotFound, "domain not found")
		return
	}

	h.audit.Log(r.Context(), audit.Entry{
		RequestID:  auth.GetRequestIDFromContext(r.Context()),
		UserID:     auth.GetUserIDFromContext(r.Context()),
		Action:     "domain.delete",
		TargetType: "domain",
		TargetID:   domainID,
		IP:         r.RemoteAddr,
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// Preflight 域名预检。
func (h *Handler) Preflight(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Hostname string `json:"hostname"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Hostname == "" {
		writeError(w, http.StatusBadRequest, "hostname required")
		return
	}

	normalizedDomain := normalizeDomain(req.Hostname)

	// 检查是否已存在
	var exists int
	h.db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM domain_bindings WHERE normalized_domain = ?", normalizedDomain).Scan(&exists)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"hostname":    req.Hostname,
		"normalized":  normalizedDomain,
		"available":   exists == 0,
		"zone_id":     "",
		"has_cf_token": false,
	})
}

// SyncDNS 同步 DNS。
func (h *Handler) SyncDNS(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "not_implemented",
		"message": "Cloudflare integration not configured",
	})
}

// SetProxyMode 设置代理模式。
func (h *Handler) SetProxyMode(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "not_implemented",
		"message": "Cloudflare integration not configured",
	})
}

// UpdateServerIP 更新服务器 IP。
func (h *Handler) UpdateServerIP(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "not_implemented",
		"message": "Cloudflare integration not configured",
	})
}

// GetDNS 获取 DNS 记录。
func (h *Handler) GetDNS(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"records": []interface{}{},
	})
}

// 辅助函数
func normalizeDomain(domain string) string {
	// 简单的域名规范化
	domain = strings.ToLower(domain)
	domain = strings.TrimSuffix(domain, ".")
	return domain
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
