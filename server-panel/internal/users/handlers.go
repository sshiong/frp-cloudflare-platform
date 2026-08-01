// Package users 提供用户管理 HTTP 处理器。
// 包括 CRUD、密码修改、启用/禁用和删除操作。
package users

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/frp-panel/server-panel/internal/audit"
	"github.com/frp-panel/server-panel/internal/auth"
	"github.com/frp-panel/server-panel/internal/crypto"
	"github.com/frp-panel/server-panel/internal/operations"
	"github.com/google/uuid"
)

// Handler 用户管理处理器。
type Handler struct {
	db      *sql.DB
	logger  *slog.Logger
	audit   *audit.Logger
	ops     *operations.Manager
}

// NewHandler 创建用户管理处理器。
func NewHandler(db *sql.DB, logger *slog.Logger, auditLog *audit.Logger, ops *operations.Manager) *Handler {
	return &Handler{db: db, logger: logger, audit: auditLog, ops: ops}
}

// User 用户实体。
type User struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	Enabled   bool   `json:"enabled"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// List 列出所有用户（管理员）。
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT id, username, role, status, created_at, updated_at
		FROM users ORDER BY created_at DESC
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list users")
		return
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		var status string
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &status, &u.CreatedAt, &u.UpdatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to scan user")
			return
		}
		u.Enabled = status == "active"
		users = append(users, u)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"users": users})
}

// Get 获取单个用户信息。
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("id")
	if userID == "" {
		userID = auth.GetUserIDFromContext(r.Context())
	}

	var u User
	var status string
	err := h.db.QueryRowContext(r.Context(), `
		SELECT id, username, role, status, created_at, updated_at
		FROM users WHERE id = ?
	`, userID).Scan(&u.ID, &u.Username, &u.Role, &status, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get user")
		return
	}
	u.Enabled = status == "active"
	writeJSON(w, http.StatusOK, u)
}

// CreateRequest 创建用户请求。
type CreateRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// Create 创建新用户（管理员）。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Username == "" {
		writeError(w, http.StatusBadRequest, "username is required")
		return
	}

	// 如果没有提供密码，自动生成一个
	if req.Password == "" {
		req.Password = crypto.RandomPassword(12)
	}

	if req.Role == "" {
		req.Role = "user"
	}
	if req.Role != "admin" && req.Role != "user" {
		writeError(w, http.StatusBadRequest, "invalid role")
		return
	}
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	// 检查用户名是否已存在
	var count int
	if err := h.db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM users WHERE username = ?", req.Username).Scan(&count); err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if count > 0 {
		writeError(w, http.StatusConflict, "username already exists")
		return
	}

	// 哈希密码
	hash, err := crypto.HashPasswordArgon2id(req.Password)
	if err != nil {
		h.logger.Error("failed to hash password", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	id := uuid.New().String()
	_, err = h.db.ExecContext(r.Context(), `
		INSERT INTO users (id, username, password, role) VALUES (?, ?, ?, ?)
	`, id, req.Username, hash, req.Role)
	if err != nil {
		h.logger.Error("failed to create user", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	h.audit.Log(r.Context(), audit.Entry{
		RequestID:  auth.GetRequestIDFromContext(r.Context()),
		UserID:     auth.GetUserIDFromContext(r.Context()),
		Action:     "user.create",
		TargetType: "user",
		TargetID:   id,
		Detail:     map[string]interface{}{"username": req.Username, "role": req.Role},
		IP:         r.RemoteAddr,
	})

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":       id,
		"username": req.Username,
		"role":     req.Role,
	})
}

// UpdateRequest 更新用户请求。
type UpdateRequest struct {
	ID       string `json:"id"`
	Username string `json:"username,omitempty"`
	Role     string `json:"role,omitempty"`
}

// Update 更新用户信息（管理员）。
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	var req UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ID == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	updates := []string{}
	args := []interface{}{}
	if req.Username != "" {
		updates = append(updates, "username = ?")
		args = append(args, req.Username)
	}
	if req.Role != "" {
		if req.Role != "admin" && req.Role != "user" {
			writeError(w, http.StatusBadRequest, "invalid role")
			return
		}
		updates = append(updates, "role = ?")
		args = append(args, req.Role)
	}
	if len(updates) == 0 {
		writeError(w, http.StatusBadRequest, "no fields to update")
		return
	}

	args = append(args, req.ID)
	query := "UPDATE users SET " + joinStrings(updates, ", ") + " WHERE id = ?"
	result, err := h.db.ExecContext(r.Context(), query, args...)
	if err != nil {
		h.logger.Error("failed to update user", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to update user")
		return
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	h.audit.LogSimple(r.Context(), auth.GetRequestIDFromContext(r.Context()),
		auth.GetUserIDFromContext(r.Context()), "user.update", "user", req.ID, r.RemoteAddr)

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ChangePasswordRequest 修改密码请求。
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

// ChangePassword 修改当前用户密码。
func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var req ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.NewPassword == "" || len(req.NewPassword) < 8 {
		writeError(w, http.StatusBadRequest, "new password must be at least 8 characters")
		return
	}

	userID := auth.GetUserIDFromContext(r.Context())

	var storedHash string
	if err := h.db.QueryRowContext(r.Context(), "SELECT password FROM users WHERE id = ?", userID).Scan(&storedHash); err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	ok, err := crypto.VerifyPasswordArgon2id(req.OldPassword, storedHash)
	if err != nil || !ok {
		writeError(w, http.StatusUnauthorized, "old password is incorrect")
		return
	}

	newHash, err := crypto.HashPasswordArgon2id(req.NewPassword)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if _, err := h.db.ExecContext(r.Context(), "UPDATE users SET password = ? WHERE id = ?", newHash, userID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update password")
		return
	}

	h.audit.LogSimple(r.Context(), auth.GetRequestIDFromContext(r.Context()),
		userID, "user.change_password", "user", userID, r.RemoteAddr)

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// SetEnabled 启用/禁用用户（管理员）。
func (h *Handler) SetEnabled(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("id")
	enabled := r.URL.Query().Get("enabled")
	if userID == "" || enabled == "" {
		writeError(w, http.StatusBadRequest, "id and enabled are required")
		return
	}

	enabledVal := 0
	if enabled == "true" || enabled == "1" {
		enabledVal = 1
	}

	result, err := h.db.ExecContext(r.Context(), "UPDATE users SET enabled = ? WHERE id = ?", enabledVal, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update user")
		return
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	action := "user.disable"
	if enabledVal == 1 {
		action = "user.enable"
	}
	h.audit.LogSimple(r.Context(), auth.GetRequestIDFromContext(r.Context()),
		auth.GetUserIDFromContext(r.Context()), action, "user", userID, r.RemoteAddr)

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Delete 删除用户（管理员）。
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("id")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	// 不允许删除自己
	currentUser := auth.GetUserIDFromContext(r.Context())
	if userID == currentUser {
		writeError(w, http.StatusForbidden, "cannot delete yourself")
		return
	}

	// 创建删除操作记录
	op, err := h.ops.Create(r.Context(), currentUser, "user.delete", userID, "user", nil)
	if err != nil {
		h.logger.Error("failed to create delete operation", "err", err)
	}

	// 删除用户（级联删除会处理关联数据）
	result, err := h.db.ExecContext(r.Context(), "DELETE FROM users WHERE id = ?", userID)
	if err != nil {
		if op != nil {
			_ = h.ops.SetError(r.Context(), op.ID, err.Error())
		}
		writeError(w, http.StatusInternalServerError, "failed to delete user")
		return
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		if op != nil {
			_ = h.ops.SetError(r.Context(), op.ID, "user not found")
		}
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	if op != nil {
		_ = h.ops.UpdateState(r.Context(), op.ID, operations.StateSucceeded)
	}

	h.audit.LogSimple(r.Context(), auth.GetRequestIDFromContext(r.Context()),
		currentUser, "user.delete", "user", userID, r.RemoteAddr)

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// AdminSetPassword 管理员重置用户密码。
func (h *Handler) AdminSetPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID   string `json:"user_id"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.UserID == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "user_id and password are required")
		return
	}
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	hash, err := crypto.HashPasswordArgon2id(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	result, err := h.db.ExecContext(r.Context(), "UPDATE users SET password = ? WHERE id = ?", hash, req.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to reset password")
		return
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	h.audit.LogSimple(r.Context(), auth.GetRequestIDFromContext(r.Context()),
		auth.GetUserIDFromContext(r.Context()), "user.admin_set_password", "user", req.UserID, r.RemoteAddr)

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// InitAdmin 如果没有管理员用户，创建初始管理员。
// 返回用户名和密码（仅首次运行时）。
func InitAdmin(db *sql.DB, logger *slog.Logger) (username, password string, err error) {
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		return "", "", err
	}
	if count > 0 {
		return "", "", nil // 已有用户，不需要创建
	}

	username = "admin"
	password = crypto.RandomPassword(12)
	hash, err := crypto.HashPasswordArgon2id(password)
	if err != nil {
		return "", "", err
	}

	id := uuid.New().String()
	_, err = db.Exec("INSERT INTO users (id, username, password, role) VALUES (?, ?, ?, 'admin')",
		id, username, hash)
	if err != nil {
		return "", "", err
	}

	logger.Info("initial admin user created", "username", username)
	return username, password, nil
}

// --- 辅助函数 ---

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
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
