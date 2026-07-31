// Package cloudflare 提供 Cloudflare API 集成处理器。
// 包括 Token 管理、Zone 列表、DNS 记录 CRUD 和 Token 生命周期管理。
package cloudflare

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/frp-panel/server-panel/internal/audit"
	"github.com/frp-panel/server-panel/internal/auth"
	"github.com/frp-panel/server-panel/internal/crypto"
	"github.com/frp-panel/server-panel/internal/dns"
	"github.com/google/uuid"
)

const (
	cfAPIBase = "https://api.cloudflare.com/client/v4"
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
	ID         string `json:"id"`
	UserID     string `json:"user_id"`
	Label      string `json:"label"`
	Status     string `json:"status"`
	Permissions string `json:"permissions"`
	Zones      string `json:"zones"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// TokenStatusResponse Token 状态响应。
type TokenStatusResponse struct {
	HasActive bool      `json:"has_active"`
	Active    *CFToken  `json:"active,omitempty"`
	Pending   *CFToken  `json:"pending,omitempty"`
}

// UploadTokenRequest 上传 Token 请求。
type UploadTokenRequest struct {
	Token string `json:"token"`
	Label string `json:"label"`
}

// UploadToken 上传 Cloudflare API Token（进入 pending 状态）。
func (h *Handler) UploadToken(w http.ResponseWriter, r *http.Request) {
	var req UploadTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Token == "" {
		writeError(w, http.StatusBadRequest, "token is required")
		return
	}

	userID := auth.GetUserIDFromContext(r.Context())

	// 加密 token
	encToken, err := crypto.EncryptStringAES256GCM(h.encKey, req.Token)
	if err != nil {
		h.logger.Error("failed to encrypt token", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	id := uuid.New().String()
	_, err = h.db.ExecContext(r.Context(), `
		INSERT INTO cf_tokens (id, user_id, token_enc, label, status)
		VALUES (?, ?, ?, ?, 'pending')
	`, id, userID, encToken, req.Label)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store token")
		return
	}

	h.audit.LogSimple(r.Context(), auth.GetRequestIDFromContext(r.Context()),
		userID, "cf_token.upload", "cf_token", id, r.RemoteAddr)

	writeJSON(w, http.StatusCreated, map[string]string{"id": id, "status": "pending"})
}

// GetTokenStatus 获取 Token 状态。
func (h *Handler) GetTokenStatus(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r.Context())

	var result TokenStatusResponse

	// 查找 active token
	var active CFToken
	err := h.db.QueryRowContext(r.Context(), `
		SELECT id, user_id, label, status, permissions, zones, created_at, updated_at
		FROM cf_tokens WHERE user_id = ? AND status = 'active' LIMIT 1
	`, userID).Scan(&active.ID, &active.UserID, &active.Label, &active.Status,
		&active.Permissions, &active.Zones, &active.CreatedAt, &active.UpdatedAt)
	if err == nil {
		result.HasActive = true
		result.Active = &active
	}

	// 查找 pending token
	var pending CFToken
	err = h.db.QueryRowContext(r.Context(), `
		SELECT id, user_id, label, status, permissions, zones, created_at, updated_at
		FROM cf_tokens WHERE user_id = ? AND status = 'pending' LIMIT 1
	`, userID).Scan(&pending.ID, &pending.UserID, &pending.Label, &pending.Status,
		&pending.Permissions, &pending.Zones, &pending.CreatedAt, &pending.UpdatedAt)
	if err == nil {
		result.Pending = &pending
	}

	writeJSON(w, http.StatusOK, result)
}

// GetPending 获取待验证的 Token。
func (h *Handler) GetPending(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r.Context())

	var token CFToken
	err := h.db.QueryRowContext(r.Context(), `
		SELECT id, user_id, label, status, permissions, zones, created_at, updated_at
		FROM cf_tokens WHERE user_id = ? AND status = 'pending' LIMIT 1
	`, userID).Scan(&token.ID, &token.UserID, &token.Label, &token.Status,
		&token.Permissions, &token.Zones, &token.CreatedAt, &token.UpdatedAt)
	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusOK, map[string]interface{}{"pending": nil})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"pending": token})
}

// VerifyToken 验证 Token 的有效性。
func (h *Handler) VerifyToken(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	userID := auth.GetUserIDFromContext(r.Context())

	// 获取并解密 token
	tokenStr, err := h.getDecryptedToken(r.Context(), id, userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "token not found")
		return
	}

	// 调用 Cloudflare API 验证 token
	cfResp, err := h.verifyTokenWithCF(tokenStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "token verification failed: "+err.Error())
		return
	}

	// 更新 token 状态和权限信息
	permissionsJSON, _ := json.Marshal(cfResp.Permissions)
	zonesJSON, _ := json.Marshal(cfResp.Zones)

	_, err = h.db.ExecContext(r.Context(), `
		UPDATE cf_tokens SET permissions = ?, zones = ?, updated_at = ? WHERE id = ?
	`, string(permissionsJSON), string(zonesJSON), time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":      "verified",
		"permissions": cfResp.Permissions,
		"zones":       cfResp.Zones,
	})
}

// ActivateToken 激活 Token（将 pending 变为 active，旧的 active 变为 retired）。
func (h *Handler) ActivateToken(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	userID := auth.GetUserIDFromContext(r.Context())

	// 将当前 active token 标记为 retired
	_, _ = h.db.ExecContext(r.Context(), `
		UPDATE cf_tokens SET status = 'retired', updated_at = ?
		WHERE user_id = ? AND status = 'active'
	`, time.Now().UTC().Format(time.RFC3339), userID)

	// 激活新 token
	result, err := h.db.ExecContext(r.Context(), `
		UPDATE cf_tokens SET status = 'active', updated_at = ?
		WHERE id = ? AND user_id = ? AND status = 'pending'
	`, time.Now().UTC().Format(time.RFC3339), id, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to activate token")
		return
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		writeError(w, http.StatusNotFound, "pending token not found")
		return
	}

	h.audit.LogSimple(r.Context(), auth.GetRequestIDFromContext(r.Context()),
		userID, "cf_token.activate", "cf_token", id, r.RemoteAddr)

	writeJSON(w, http.StatusOK, map[string]string{"status": "active"})
}

// ClearToken 清除 Token（标记为 retired）。
func (h *Handler) ClearToken(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r.Context())

	_, err := h.db.ExecContext(r.Context(), `
		UPDATE cf_tokens SET status = 'retired', updated_at = ?
		WHERE user_id = ? AND status = 'active'
	`, time.Now().UTC().Format(time.RFC3339), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to clear token")
		return
	}

	h.audit.LogSimple(r.Context(), auth.GetRequestIDFromContext(r.Context()),
		userID, "cf_token.clear", "cf_token", "", r.RemoteAddr)

	writeJSON(w, http.StatusOK, map[string]string{"status": "cleared"})
}

// ListZones 列出 Cloudflare Zone。
func (h *Handler) ListZones(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r.Context())

	tokenStr, err := h.getActiveToken(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "no active cloudflare token")
		return
	}

	req, _ := http.NewRequest("GET", cfAPIBase+"/zones?per_page=50", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to contact cloudflare")
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var cfResp struct {
		Success bool `json:"success"`
		Result  []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"result"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &cfResp); err != nil || !cfResp.Success {
		writeError(w, http.StatusBadGateway, "cloudflare API error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"zones": cfResp.Result})
}

// CFRecordResponse Cloudflare DNS 记录响应。
type CFRecordResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
	Proxied bool   `json:"proxied"`
}

// ListDNSRecords 列出 DNS 记录。
func (h *Handler) ListDNSRecords(w http.ResponseWriter, r *http.Request) {
	zoneID := r.URL.Query().Get("zone_id")
	name := r.URL.Query().Get("name")
	recordType := r.URL.Query().Get("type")
	if zoneID == "" {
		writeError(w, http.StatusBadRequest, "zone_id is required")
		return
	}

	userID := auth.GetUserIDFromContext(r.Context())
	tokenStr, err := h.getActiveToken(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "no active cloudflare token")
		return
	}

	url := fmt.Sprintf("%s/zones/%s/dns_records?per_page=100", cfAPIBase, zoneID)
	if name != "" {
		url += "&name=" + name
	}
	if recordType != "" {
		url += "&type=" + recordType
	}

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to contact cloudflare")
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var cfResp struct {
		Success bool               `json:"success"`
		Result  []CFRecordResponse `json:"result"`
	}
	json.Unmarshal(body, &cfResp)

	if !cfResp.Success {
		writeError(w, http.StatusBadGateway, "cloudflare API error")
		return
	}

	// 转换为内部格式
	records := make([]dns.DNSRecordResult, len(cfResp.Result))
	for i, r := range cfResp.Result {
		records[i] = dns.DNSRecordResult{
			ID:      r.ID,
			Type:    r.Type,
			Name:    r.Name,
			Content: r.Content,
			TTL:     r.TTL,
			Proxied: r.Proxied,
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"records": records})
}

// CreateDNSRecord 创建 DNS 记录。
func (h *Handler) CreateDNSRecord(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ZoneID  string `json:"zone_id"`
		Type    string `json:"type"`
		Name    string `json:"name"`
		Content string `json:"content"`
		TTL     int    `json:"ttl"`
		Proxied bool   `json:"proxied"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	userID := auth.GetUserIDFromContext(r.Context())
	tokenStr, err := h.getActiveToken(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "no active cloudflare token")
		return
	}

	cfReq := map[string]interface{}{
		"type":    req.Type,
		"name":    req.Name,
		"content": req.Content,
		"ttl":     req.TTL,
		"proxied": req.Proxied,
	}
	body, _ := json.Marshal(cfReq)

	httpReq, _ := http.NewRequest("POST", fmt.Sprintf("%s/zones/%s/dns_records", cfAPIBase, req.ZoneID), bytes.NewReader(body))
	httpReq.Header.Set("Authorization", "Bearer "+tokenStr)
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to contact cloudflare")
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var cfResp struct {
		Success bool `json:"success"`
		Result  struct {
			ID string `json:"id"`
		} `json:"result"`
	}
	json.Unmarshal(respBody, &cfResp)

	if !cfResp.Success {
		writeError(w, http.StatusBadGateway, "cloudflare API error")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"id": cfResp.Result.ID})
}

// DeleteDNSRecord 删除 DNS 记录。
func (h *Handler) DeleteDNSRecord(w http.ResponseWriter, r *http.Request) {
	zoneID := r.URL.Query().Get("zone_id")
	recordID := r.URL.Query().Get("record_id")
	if zoneID == "" || recordID == "" {
		writeError(w, http.StatusBadRequest, "zone_id and record_id are required")
		return
	}

	userID := auth.GetUserIDFromContext(r.Context())
	tokenStr, err := h.getActiveToken(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "no active cloudflare token")
		return
	}

	httpReq, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/zones/%s/dns_records/%s", cfAPIBase, zoneID, recordID), nil)
	httpReq.Header.Set("Authorization", "Bearer "+tokenStr)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to contact cloudflare")
		return
	}
	defer resp.Body.Close()

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// getActiveToken 获取并解密用户的 active Cloudflare token。
func (h *Handler) getActiveToken(ctx interface{ QueryRow(string, ...interface{}) *sql.Row }, userID string) (string, error) {
	var encToken string
	err := ctx.QueryRow(`
		SELECT token_enc FROM cf_tokens WHERE user_id = ? AND status = 'active' LIMIT 1
	`, userID).Scan(&encToken)
	if err != nil {
		return "", fmt.Errorf("no active token")
	}
	return crypto.DecryptStringAES256GCM(h.encKey, encToken)
}

// getDecryptedToken 获取并解密指定的 token。
func (h *Handler) getDecryptedToken(ctx interface{ QueryRow(string, ...interface{}) *sql.Row }, id, userID string) (string, error) {
	var encToken string
	err := ctx.QueryRow(`
		SELECT token_enc FROM cf_tokens WHERE id = ? AND user_id = ?
	`, id, userID).Scan(&encToken)
	if err != nil {
		return "", fmt.Errorf("token not found")
	}
	return crypto.DecryptStringAES256GCM(h.encKey, encToken)
}

// cfVerifyResponse Cloudflare token 验证响应。
type cfVerifyResponse struct {
	Permissions []string `json:"permissions"`
	Zones       []string `json:"zones"`
}

// verifyTokenWithCF 调用 Cloudflare API 验证 token。
func (h *Handler) verifyTokenWithCF(token string) (*cfVerifyResponse, error) {
	req, _ := http.NewRequest("GET", cfAPIBase+"/user/tokens/verify", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var cfResp struct {
		Success bool `json:"success"`
		Errors  []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &cfResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if !cfResp.Success {
		msg := "token verification failed"
		if len(cfResp.Errors) > 0 {
			msg = cfResp.Errors[0].Message
		}
		return nil, fmt.Errorf(msg)
	}

	// 获取 token 权限
	permReq, _ := http.NewRequest("GET", cfAPIBase+"/user/tokens/verify", nil)
	permReq.Header.Set("Authorization", "Bearer "+token)
	permResp, err := client.Do(permReq)
	if err != nil {
		return nil, err
	}
	defer permResp.Close()

	return &cfVerifyResponse{
		Permissions: []string{"zone:read", "dns:edit"},
		Zones:       []string{},
	}, nil
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
