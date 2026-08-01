// Package certificates 提供证书管理 HTTP 处理器。
// 包括证书列表、签发、续期和状态查询。
package certificates

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/frp-panel/server-panel/internal/audit"
	"github.com/frp-panel/server-panel/internal/auth"
)

// Handler 证书管理处理器。
type Handler struct {
	db     *sql.DB
	logger *slog.Logger
	audit  *audit.Logger
	encKey []byte
}

// NewHandler 创建证书管理处理器。
func NewHandler(db *sql.DB, logger *slog.Logger, auditLog *audit.Logger, encKey []byte) *Handler {
	return &Handler{db: db, logger: logger, audit: auditLog, encKey: encKey}
}

// Certificate 证书实体。
type Certificate struct {
	ID              string `json:"id"`
	DomainBindingID string `json:"domain_binding_id"`
	Provider        string `json:"provider"`
	Status          string `json:"status"`
	NotBefore       string `json:"not_before,omitempty"`
	NotAfter        string `json:"not_after,omitempty"`
	RenewAfter      string `json:"renew_after,omitempty"`
	ErrorCode       string `json:"error_code,omitempty"`
	ErrorMessage    string `json:"error_message,omitempty"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

// List 获取证书列表。
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT id, domain_binding_id, provider, status,
		COALESCE(not_before,''), COALESCE(not_after,''), COALESCE(renew_after,''),
		COALESCE(last_error_code,''), COALESCE(last_error_message,''), created_at, updated_at
		FROM certificates ORDER BY created_at DESC
	`)
	if err != nil {
		h.logger.Error("failed to list certificates", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to list certificates")
		return
	}
	defer rows.Close()

	var certs []Certificate
	for rows.Next() {
		var c Certificate
		if err := rows.Scan(&c.ID, &c.DomainBindingID, &c.Provider, &c.Status,
			&c.NotBefore, &c.NotAfter, &c.RenewAfter, &c.ErrorCode, &c.ErrorMessage,
			&c.CreatedAt, &c.UpdatedAt); err != nil {
			h.logger.Error("failed to scan certificate", "err", err)
			writeError(w, http.StatusInternalServerError, "failed to scan certificate")
			return
		}
		certs = append(certs, c)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"certificates": certs})
}

// Issue 签发证书。
func (h *Handler) Issue(w http.ResponseWriter, r *http.Request) {
	domainID := r.URL.Query().Get("domain_id")
	if domainID == "" {
		writeError(w, http.StatusBadRequest, "domain_id required")
		return
	}

	// 检查是否已有证书
	var count int
	h.db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM certificates WHERE domain_binding_id = ?", domainID).Scan(&count)
	if count > 0 {
		writeError(w, http.StatusConflict, "certificate already exists")
		return
	}

	id := generateID()
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := h.db.ExecContext(r.Context(), `
		INSERT INTO certificates (id, domain_binding_id, provider, status, created_at, updated_at)
		VALUES (?, ?, 'lets_encrypt', 'pending', ?, ?)
	`, id, domainID, now, now)
	if err != nil {
		h.logger.Error("failed to create certificate", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to create certificate")
		return
	}

	h.audit.Log(r.Context(), audit.Entry{
		RequestID:  auth.GetRequestIDFromContext(r.Context()),
		UserID:     auth.GetUserIDFromContext(r.Context()),
		Action:     "certificate.issue",
		TargetType: "certificate",
		TargetID:   id,
		IP:         r.RemoteAddr,
	})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":      id,
		"status":  "pending",
		"message": "certificate issuance started",
	})
}

// Get 获取证书详情。
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id required")
		return
	}

	var c Certificate
	err := h.db.QueryRowContext(r.Context(), `
		SELECT id, domain_binding_id, provider, status,
		COALESCE(not_before,''), COALESCE(not_after,''), COALESCE(renew_after,''),
		COALESCE(last_error_code,''), COALESCE(last_error_message,''), created_at, updated_at
		FROM certificates WHERE id = ?
	`, id).Scan(&c.ID, &c.DomainBindingID, &c.Provider, &c.Status,
		&c.NotBefore, &c.NotAfter, &c.RenewAfter, &c.ErrorCode, &c.ErrorMessage,
		&c.CreatedAt, &c.UpdatedAt)

	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "certificate not found")
		return
	}
	if err != nil {
		h.logger.Error("failed to get certificate", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to get certificate")
		return
	}
	writeJSON(w, http.StatusOK, c)
}

// Renew 续期证书。
func (h *Handler) Renew(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id required")
		return
	}

	_, err := h.db.ExecContext(r.Context(), `
		UPDATE certificates SET status = 'renewing', updated_at = ? WHERE id = ?
	`, time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		h.logger.Error("failed to renew certificate", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to renew certificate")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "renewing"})
}

// GetStatus 获取证书状态。
func (h *Handler) GetStatus(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id required")
		return
	}

	var status string
	err := h.db.QueryRowContext(r.Context(), "SELECT status FROM certificates WHERE id = ?", id).Scan(&status)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "certificate not found")
		return
	}
	if err != nil {
		h.logger.Error("failed to get certificate status", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to get certificate status")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": status})
}

// GetCertPEM 获取证书 PEM。
func (h *Handler) GetCertPEM(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"pem":     "",
		"message": "certificate PEM not available",
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
