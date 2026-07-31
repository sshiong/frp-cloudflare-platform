// Package certificates 提供证书管理 HTTP 处理器。
// 包括 ACME 证书签发（DNS-01）、续期、状态查询和私钥加密存储。
package certificates

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/frp-panel/server-panel/internal/audit"
	"github.com/frp-panel/server-panel/internal/auth"
	"github.com/frp-panel/server-panel/internal/crypto"
	"github.com/frp-panel/server-panel/internal/dns"
	"github.com/google/uuid"
)

// Handler 证书管理处理器。
type Handler struct {
	db     *sql.DB
	logger *slog.Logger
	audit  *audit.Logger
	encKey []byte // AES-256 密钥，用于加密私钥
}

// NewHandler 创建证书管理处理器。
func NewHandler(db *sql.DB, logger *slog.Logger, auditLog *audit.Logger, encKey []byte) *Handler {
	return &Handler{db: db, logger: logger, audit: auditLog, encKey: encKey}
}

// Certificate 证书实体。
type Certificate struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	DomainID  string `json:"domain_id,omitempty"`
	FQDN      string `json:"fqdn"`
	Issuer    string `json:"issuer"`
	NotBefore string `json:"not_before,omitempty"`
	NotAfter  string `json:"not_after,omitempty"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// IssueRequest 签发证书请求。
type IssueRequest struct {
	DomainID string `json:"domain_id"`
	FQDN     string `json:"fqdn"`
	Issuer   string `json:"issuer"` // "letsencrypt" or "zerossl"
}

// Issue 签发新证书（ACME DNS-01）。
func (h *Handler) Issue(w http.ResponseWriter, r *http.Request) {
	var req IssueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.FQDN == "" {
		writeError(w, http.StatusBadRequest, "fqdn is required")
		return
	}

	// 规范化域名
	fqdn, err := dns.NormalizeDomain(req.FQDN)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid fqdn: "+err.Error())
		return
	}

	if req.Issuer == "" {
		req.Issuer = "letsencrypt"
	}
	if req.Issuer != "letsencrypt" && req.Issuer != "zerossl" {
		writeError(w, http.StatusBadRequest, "invalid issuer: must be letsencrypt or zerossl")
		return
	}

	userID := auth.GetUserIDFromContext(r.Context())

	// 检查是否已有活跃证书
	var count int
	if err := h.db.QueryRowContext(r.Context(), `
		SELECT COUNT(*) FROM certificates WHERE fqdn = ? AND status = 'active'
	`, fqdn).Scan(&count); err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if count > 0 {
		writeError(w, http.StatusConflict, "active certificate already exists for this domain")
		return
	}

	// 生成 ECDSA 私钥
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		h.logger.Error("failed to generate private key", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to generate key")
		return
	}

	// 序列化私钥
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: elliptic.Marshal(privateKey.Curve, privateKey.X, privateKey.Y),
	})

	// 加密私钥
	encKeyPEM, err := crypto.EncryptAES256GCM(h.encKey, keyPEM)
	if err != nil {
		h.logger.Error("failed to encrypt private key", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	id := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)

	// 注意：实际的 ACME 签发流程需要：
	// 1. 创建 ACME account
	// 2. 创建 order
	// 3. 获取 DNS-01 challenge
	// 4. 在 Cloudflare 添加 _acme-challenge TXT 记录
	// 5. 等待验证完成
	// 6. 下载证书
	// 此处存储初始状态，后续通过后台任务完成签发

	_, err = h.db.ExecContext(r.Context(), `
		INSERT INTO certificates (id, user_id, domain_id, fqdn, issuer, key_pem, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 'pending', ?, ?)
	`, id, userID, nullString(req.DomainID), fqdn, req.Issuer, string(encKeyPEM), now, now)
	if err != nil {
		h.logger.Error("failed to create certificate record", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to create certificate")
		return
	}

	h.audit.Log(r.Context(), audit.Entry{
		RequestID:  auth.GetRequestIDFromContext(r.Context()),
		UserID:     userID,
		Action:     "certificate.issue",
		TargetType: "certificate",
		TargetID:   id,
		Detail:     map[string]interface{}{"fqdn": fqdn, "issuer": req.Issuer},
		IP:         r.RemoteAddr,
	})

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":     id,
		"status": "pending",
		"fqdn":   fqdn,
	})
}

// List 列出证书。
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r.Context())
	role := auth.GetUserRoleFromContext(r.Context())

	query := `SELECT id, user_id, COALESCE(domain_id,''), fqdn, issuer,
		COALESCE(not_before,''), COALESCE(not_after,''), status, created_at, updated_at
		FROM certificates`
	var args []interface{}

	if role != "admin" {
		query += " WHERE user_id = ?"
		args = append(args, userID)
	}
	query += " ORDER BY created_at DESC"

	rows, err := h.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list certificates")
		return
	}
	defer rows.Close()

	var certs []Certificate
	for rows.Next() {
		var c Certificate
		if err := rows.Scan(&c.ID, &c.UserID, &c.DomainID, &c.FQDN, &c.Issuer,
			&c.NotBefore, &c.NotAfter, &c.Status, &c.CreatedAt, &c.UpdatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to scan certificate")
			return
		}
		certs = append(certs, c)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"certificates": certs})
}

// Get 获取证书详情。
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	var c Certificate
	err := h.db.QueryRowContext(r.Context(), `
		SELECT id, user_id, COALESCE(domain_id,''), fqdn, issuer,
		COALESCE(not_before,''), COALESCE(not_after,''), status, created_at, updated_at
		FROM certificates WHERE id = ?
	`, id).Scan(&c.ID, &c.UserID, &c.DomainID, &c.FQDN, &c.Issuer,
		&c.NotBefore, &c.NotAfter, &c.Status, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "certificate not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get certificate")
		return
	}

	userID := auth.GetUserIDFromContext(r.Context())
	role := auth.GetUserRoleFromContext(r.Context())
	if c.UserID != userID && role != "admin" {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	writeJSON(w, http.StatusOK, c)
}

// Renew 续期证书。
func (h *Handler) Renew(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	// 验证权限
	var c Certificate
	err := h.db.QueryRowContext(r.Context(), `
		SELECT id, user_id, fqdn, issuer, status FROM certificates WHERE id = ?
	`, id).Scan(&c.ID, &c.UserID, &c.FQDN, &c.Issuer, &c.Status)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "certificate not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	userID := auth.GetUserIDFromContext(r.Context())
	role := auth.GetUserRoleFromContext(r.Context())
	if c.UserID != userID && role != "admin" {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	// 重置状态为 pending
	_, err = h.db.ExecContext(r.Context(), `
		UPDATE certificates SET status = 'pending', updated_at = ? WHERE id = ?
	`, time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to renew certificate")
		return
	}

	h.audit.LogSimple(r.Context(), auth.GetRequestIDFromContext(r.Context()),
		userID, "certificate.renew", "certificate", id, r.RemoteAddr)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":     id,
		"status": "pending",
		"fqdn":   c.FQDN,
	})
}

// GetStatus 获取证书状态。
func (h *Handler) GetStatus(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	var status, fqdn, issuer, notAfter string
	err := h.db.QueryRowContext(r.Context(), `
		SELECT status, fqdn, issuer, COALESCE(not_after,'') FROM certificates WHERE id = ?
	`, id).Scan(&status, &fqdn, &issuer, &notAfter)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "certificate not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	// 计算剩余天数
	daysRemaining := 0
	if notAfter != "" {
		expiry, err := time.Parse(time.RFC3339, notAfter)
		if err == nil {
			daysRemaining = int(time.Until(expiry).Hours() / 24)
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":             id,
		"status":         status,
		"fqdn":           fqdn,
		"issuer":         issuer,
		"not_after":      notAfter,
		"days_remaining": daysRemaining,
	})
}

// GetCertPEM 获取证书 PEM 内容（含私钥）。
func (h *Handler) GetCertPEM(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	var certPEM, keyPEM, chainPEM sql.NullString
	var userID string
	err := h.db.QueryRowContext(r.Context(), `
		SELECT user_id, cert_pem, key_pem, chain_pem FROM certificates WHERE id = ?
	`, id).Scan(&userID, &certPEM, &keyPEM, &chainPEM)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "certificate not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	// 权限检查
	currentUser := auth.GetUserIDFromContext(r.Context())
	role := auth.GetUserRoleFromContext(r.Context())
	if userID != currentUser && role != "admin" {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	result := map[string]interface{}{}
	if certPEM.Valid && certPEM.String != "" {
		decrypted, err := crypto.DecryptStringAES256GCM(h.encKey, certPEM.String)
		if err == nil {
			result["cert_pem"] = decrypted
		}
	}
	if keyPEM.Valid && keyPEM.String != "" {
		decrypted, err := crypto.DecryptStringAES256GCM(h.encKey, keyPEM.String)
		if err == nil {
			result["key_pem"] = decrypted
		}
	}
	if chainPEM.Valid && chainPEM.String != "" {
		decrypted, err := crypto.DecryptStringAES256GCM(h.encKey, chainPEM.String)
		if err == nil {
			result["chain_pem"] = decrypted
		}
	}

	writeJSON(w, http.StatusOK, result)
}

func nullString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
