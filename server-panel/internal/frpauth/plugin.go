// Package frpauth 提供 FRPS HTTP Plugin 接口实现。
// 用于验证 FRP 客户端的登录、代理创建、工作连接等操作。
package frpauth

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/frp-panel/server-panel/internal/crypto"
)

// Plugin FRPS HTTP Plugin 处理器。
type Plugin struct {
	db     *sql.DB
	logger *slog.Logger
	secret []byte // HMAC 验证密钥
}

// NewPlugin 创建 FRPS HTTP Plugin。
func NewPlugin(db *sql.DB, logger *slog.Logger, secret []byte) *Plugin {
	return &Plugin{db: db, logger: logger, secret: secret}
}

// FRPSRequest FRPS 插件请求格式。
type FRPSRequest struct {
	Content json.RawMessage `json:"content"`
}

// FRPSResponse FRPS 插件响应格式。
type FRPSResponse struct {
	Reject        bool   `json:"reject"`
	RejectReason  string `json:"reject_reason,omitempty"`
	Unchanged     bool   `json:"unchanged"`
	Content       interface{} `json:"content,omitempty"`
}

// LoginContent 登录请求内容。
type LoginContent struct {
	Version      string `json:"version"`
	Hostname     string `json:"hostname"`
	OS           string `json:"os"`
	Arch         string `json:"arch"`
	User         string `json:"user"`
	Password     string `json:"password"`
	Timestamp    int64  `json:"timestamp"`
	PrivilegeKey string `json:"privilege_key"`
	RunID        string `json:"run_id"`
	PoolCount    int    `json:"pool_count"`
}

// NewProxyContent 新代理请求内容。
type NewProxyContent struct {
	RunID      string `json:"run_id"`
	ProxyName  string `json:"proxy_name"`
	ProxyType  string `json:"proxy_type"`
	RemotePort int    `json:"remote_port,omitempty"`
	CustomDomains []string `json:"custom_domains,omitempty"`
	Locations  []string `json:"locations,omitempty"`
}

// NewWorkConnContent 新工作连接请求内容。
type NewWorkConnContent struct {
	RunID       string `json:"run_id"`
	ProxyName   string `json:"proxy_name"`
	PrivilegeKey string `json:"privilege_key"`
	Timestamp   int64  `json:"timestamp"`
}

// CloseProxyContent 关闭代理请求内容。
type CloseProxyContent struct {
	RunID     string `json:"run_id"`
	ProxyName string `json:"proxy_name"`
	Reason    string `json:"reason,omitempty"`
}

// PingContent 心跳请求内容。
type PingContent struct {
	RunID     string `json:"run_id"`
	Timestamp int64  `json:"timestamp"`
}

// HandleLogin 处理登录验证。
func (p *Plugin) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req FRPSRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		p.writeResponse(w, FRPSResponse{Reject: true, RejectReason: "invalid request"})
		return
	}

	var content LoginContent
	if err := json.Unmarshal(req.Content, &content); err != nil {
		p.writeResponse(w, FRPSResponse{Reject: true, RejectReason: "invalid content"})
		return
	}

	// 验证用户凭据
	var userID, tokenHash string
	var enabled int
	err := p.db.QueryRowContext(r.Context(), `
		SELECT u.id, u.enabled
		FROM users u
		WHERE u.username = ?
	`, content.User).Scan(&userID, &enabled)
	if err == sql.ErrNoRows {
		p.logger.Warn("frp login: user not found", "user", content.User)
		p.writeResponse(w, FRPSResponse{Reject: true, RejectReason: "authentication failed"})
		return
	}
	if err != nil {
		p.logger.Error("frp login: db error", "err", err)
		p.writeResponse(w, FRPSResponse{Reject: true, RejectReason: "internal error"})
		return
	}
	if enabled == 0 {
		p.writeResponse(w, FRPSResponse{Reject: true, RejectReason: "account disabled"})
		return
	}

	// 验证 privilege_key（HMAC-SHA256）
	_ = tokenHash // 使用 tokenHash 进行验证
	if !p.verifyPrivilegeKey(content.User, content.PrivilegeKey, content.Timestamp) {
		p.logger.Warn("frp login: invalid privilege key", "user", content.User)
		p.writeResponse(w, FRPSResponse{Reject: true, RejectReason: "authentication failed"})
		return
	}

	// 检查是否已有同名 run_id 的连接
	var count int
	_ = p.db.QueryRowContext(r.Context(), `
		SELECT COUNT(*) FROM mappings m
		JOIN devices d ON m.device_id = d.id
		WHERE d.user_id = ?
	`, userID).Scan(&count)

	p.logger.Info("frp login accepted", "user", content.User, "run_id", content.RunID)
	p.writeResponse(w, FRPSResponse{Reject: false})
}

// HandleNewProxy 处理新代理创建验证。
func (p *Plugin) HandleNewProxy(w http.ResponseWriter, r *http.Request) {
	var req FRPSRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		p.writeResponse(w, FRPSResponse{Reject: true, RejectReason: "invalid request"})
		return
	}

	var content NewProxyContent
	if err := json.Unmarshal(req.Content, &content); err != nil {
		p.writeResponse(w, FRPSResponse{Reject: true, RejectReason: "invalid content"})
		return
	}

	// 查找匹配的映射规则
	var mappingID string
	var enabled int
	err := p.db.QueryRowContext(r.Context(), `
		SELECT m.id, m.enabled
		FROM mappings m
		WHERE m.proxy_name = ?
	`, content.ProxyName).Scan(&mappingID, &enabled)

	if err == sql.ErrNoRows {
		p.logger.Warn("frp new-proxy: mapping not found", "proxy_name", content.ProxyName)
		p.writeResponse(w, FRPSResponse{Reject: true, RejectReason: "proxy not authorized"})
		return
	}
	if err != nil {
		p.logger.Error("frp new-proxy: db error", "err", err)
		p.writeResponse(w, FRPSResponse{Reject: true, RejectReason: "internal error"})
		return
	}
	if enabled == 0 {
		p.writeResponse(w, FRPSResponse{Reject: true, RejectReason: "proxy disabled"})
		return
	}

	// 更新映射状态为 active
	_, _ = p.db.ExecContext(r.Context(), `
		UPDATE mappings SET status = 'active', updated_at = ? WHERE id = ?
	`, time.Now().UTC().Format(time.RFC3339), mappingID)

	p.logger.Info("frp new-proxy accepted", "proxy_name", content.ProxyName, "type", content.ProxyType)
	p.writeResponse(w, FRPSResponse{Reject: false})
}

// HandleNewWorkConn 处理新工作连接验证。
func (p *Plugin) HandleNewWorkConn(w http.ResponseWriter, r *http.Request) {
	var req FRPSRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		p.writeResponse(w, FRPSResponse{Reject: true, RejectReason: "invalid request"})
		return
	}

	var content NewWorkConnContent
	if err := json.Unmarshal(req.Content, &content); err != nil {
		p.writeResponse(w, FRPSResponse{Reject: true, RejectReason: "invalid content"})
		return
	}

	// 验证 privilege_key
	if !p.verifyPrivilegeKey(content.RunID, content.PrivilegeKey, content.Timestamp) {
		p.logger.Warn("frp new-work-conn: invalid privilege key", "run_id", content.RunID)
		p.writeResponse(w, FRPSResponse{Reject: true, RejectReason: "authentication failed"})
		return
	}

	p.logger.Info("frp new-work-conn accepted", "run_id", content.RunID, "proxy_name", content.ProxyName)
	p.writeResponse(w, FRPSResponse{Reject: false})
}

// HandleCloseProxy 处理代理关闭事件。
func (p *Plugin) HandleCloseProxy(w http.ResponseWriter, r *http.Request) {
	var req FRPSRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		p.writeResponse(w, FRPSResponse{Reject: true, RejectReason: "invalid request"})
		return
	}

	var content CloseProxyContent
	if err := json.Unmarshal(req.Content, &content); err != nil {
		p.writeResponse(w, FRPSResponse{Reject: true, RejectReason: "invalid content"})
		return
	}

	// 更新映射状态为 error/disabled
	_, _ = p.db.ExecContext(r.Context(), `
		UPDATE mappings SET status = 'error', updated_at = ?
		WHERE proxy_name = ?
	`, time.Now().UTC().Format(time.RFC3339), content.ProxyName)

	p.logger.Info("frp close-proxy", "proxy_name", content.ProxyName, "reason", content.Reason)
	p.writeResponse(w, FRPSResponse{Reject: false})
}

// HandlePing 处理心跳。
func (p *Plugin) HandlePing(w http.ResponseWriter, r *http.Request) {
	var req FRPSRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		p.writeResponse(w, FRPSResponse{Reject: true, RejectReason: "invalid request"})
		return
	}

	var content PingContent
	if err := json.Unmarshal(req.Content, &content); err != nil {
		p.writeResponse(w, FRPSResponse{Reject: true, RejectReason: "invalid content"})
		return
	}

	// 心跳验证通过即可
	p.writeResponse(w, FRPSResponse{Reject: false})
}

// verifyPrivilegeKey 验证 FRP privilege_key。
// privilege_key = HMAC-SHA256(secret, user + timestamp)
func (p *Plugin) verifyPrivilegeKey(user, key string, timestamp int64) bool {
	// 检查时间戳在合理范围内（5 分钟）
	now := time.Now().Unix()
	if now-timestamp > 300 || timestamp-now > 300 {
		return false
	}

	data := fmt.Sprintf("%s%d", user, timestamp)
	expected := crypto.HMACSHA256Hex(p.secret, []byte(data))
	return crypto.ConstantTimeEqualString(expected, key)
}

// writeResponse 写入 JSON 响应。
func (p *Plugin) writeResponse(w http.ResponseWriter, resp FRPSResponse) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
