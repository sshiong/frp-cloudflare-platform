package sessionproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// Proxy 远程会话代理
// 代理浏览器登录请求到 Server Panel
// 保存 Server Session Token 在内存中
// 管理 CSRF 状态
type Proxy struct {
	mu              sync.RWMutex
	serverURL       string
	sessionToken    string // Server Web Session Token
	sessionID       string
	csrfState       string
	userID          string
	clientID        string
	httpClient      *http.Client
	logger          *slog.Logger
	isAuthenticated bool
}

// NewProxy 创建会话代理
func NewProxy(serverURL string, logger *slog.Logger) *Proxy {
	return &Proxy{
		serverURL: serverURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	SessionToken string `json:"session_token"`
	SessionID    string `json:"session_id"`
	CSRFState    string `json:"csrf_state"`
	UserID       string `json:"user_id"`
	Username     string `json:"username"`
	Role         string `json:"role"`
	MustChange   bool   `json:"must_change_password"`
}

// ServerErrorResponse 服务端错误响应
type ServerErrorResponse struct {
	Error struct {
		Code    string      `json:"code"`
		Message string      `json:"message"`
		Details interface{} `json:"details"`
	} `json:"error"`
}

// Login 代理登录请求到 Server Panel
// 1. 发送登录请求到 Server
// 2. 保存 Server Session Token
// 3. 更新 CSRF 状态
// 4. 记录用户信息
func (p *Proxy) Login(ctx context.Context, username, password string) (*LoginResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.serverURL == "" {
		return nil, fmt.Errorf("SERVER_NOT_CONFIGURED: 未配置服务端地址")
	}

	// 构造登录请求
	loginReq := LoginRequest{
		Username: username,
		Password: password,
	}
	body, err := json.Marshal(loginReq)
	if err != nil {
		return nil, fmt.Errorf("序列化登录请求失败: %w", err)
	}

	// 发送到 Server Panel
	loginURL := p.serverURL + "/api/v1/auth/login"
	req, err := http.NewRequestWithContext(ctx, "POST", loginURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建登录请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("SERVER_UNREACHABLE: 服务端不可达: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 限制 1MB
	if err != nil {
		return nil, fmt.Errorf("读取登录响应失败: %w", err)
	}

	// 处理错误响应
	if resp.StatusCode != http.StatusOK {
		var errResp ServerErrorResponse
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Error.Code != "" {
			return nil, fmt.Errorf("%s: %s", errResp.Error.Code, errResp.Error.Message)
		}
		return nil, fmt.Errorf("登录失败: HTTP %d", resp.StatusCode)
	}

	// 解析成功响应
	var loginResp LoginResponse
	if err := json.Unmarshal(respBody, &loginResp); err != nil {
		return nil, fmt.Errorf("解析登录响应失败: %w", err)
	}

	// 保存 Server Session 信息
	p.sessionToken = loginResp.SessionToken
	p.sessionID = loginResp.SessionID
	p.csrfState = loginResp.CSRFState
	p.userID = loginResp.UserID
	p.isAuthenticated = true

	p.logger.Info("Server Session 已建立",
		"user_id", loginResp.UserID,
		"session_id", loginResp.SessionID,
	)

	return &loginResp, nil
}

// Logout 撤销 Server Session
func (p *Proxy) Logout(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.isAuthenticated {
		return nil
	}

	// 向 Server 发送注销请求
	logoutURL := p.serverURL + "/api/v1/auth/logout"
	req, err := http.NewRequestWithContext(ctx, "POST", logoutURL, nil)
	if err != nil {
		p.logger.Warn("创建注销请求失败", "error", err)
	} else {
		p.setAuthHeaders(req)
		resp, err := p.httpClient.Do(req)
		if err != nil {
			p.logger.Warn("发送注销请求失败", "error", err)
		} else {
			resp.Body.Close()
		}
	}

	// 清除本地状态
	p.clearSession()

	p.logger.Info("Server Session 已注销")
	return nil
}

// ForwardToServer 将请求转发到 Server Panel（使用当前 Session）
// 用于代理浏览器的写操作请求
func (p *Proxy) ForwardToServer(ctx context.Context, method, path string, body []byte) (*http.Response, []byte, error) {
	p.mu.RLock()
	if !p.isAuthenticated {
		p.mu.RUnlock()
		return nil, nil, fmt.Errorf("NOT_AUTHENTICATED: 未登录")
	}
	serverURL := p.serverURL
	p.mu.RUnlock()

	fullURL := serverURL + path
	req, err := http.NewRequestWithContext(ctx, method, fullURL, bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("创建转发请求失败: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	p.setAuthHeaders(req)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("转发请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 限制 10MB
	if err != nil {
		return nil, nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// 检查 Session 是否被撤销
	if resp.StatusCode == http.StatusUnauthorized {
		var errResp ServerErrorResponse
		if json.Unmarshal(respBody, &errResp) == nil {
			if errResp.Error.Code == "SESSION_REPLACED" || errResp.Error.Code == "SESSION_EXPIRED" {
				p.clearSession()
				return resp, respBody, fmt.Errorf("SESSION_REPLACED: %s", errResp.Error.Message)
			}
		}
	}

	return resp, respBody, nil
}

// GetSessionToken 获取当前 Server Session Token
func (p *Proxy) GetSessionToken() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.sessionToken
}

// GetCSRFState 获取当前 CSRF 状态
func (p *Proxy) GetCSRFState() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.csrfState
}

// GetUserID 获取当前用户 ID
func (p *Proxy) GetUserID() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.userID
}

// IsAuthenticated 检查是否已认证
func (p *Proxy) IsAuthenticated() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.isAuthenticated
}

// SetServerURL 更新服务端地址（未认证时允许）
func (p *Proxy) SetServerURL(url string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.isAuthenticated {
		return // 已认证时不允许修改
	}
	p.serverURL = url
}

// GetServerURL 获取服务端地址
func (p *Proxy) GetServerURL() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.serverURL
}

// setAuthHeaders 设置认证头（调用者需持锁）
func (p *Proxy) setAuthHeaders(req *http.Request) {
	if p.sessionToken != "" {
		req.Header.Set("Cookie", "session_token="+p.sessionToken)
	}
	if p.csrfState != "" {
		req.Header.Set("X-CSRF-Token", p.csrfState)
	}
}

// clearSession 清除会话状态（调用者需持锁）
func (p *Proxy) clearSession() {
	p.sessionToken = ""
	p.sessionID = ""
	p.csrfState = ""
	p.userID = ""
	p.isAuthenticated = false
}
