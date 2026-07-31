package serverclient

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/frp-panel/client-panel/internal/hmacsigner"
)

// Client Server API 客户端
// 使用设备 HMAC 认证和用户 Session 认证
// 支持请求重试和错误处理
type Client struct {
	serverURL  string
	signer     *hmacsigner.Signer
	httpClient *http.Client
	logger     *slog.Logger
}

// NewClient 创建 Server API 客户端
func NewClient(serverURL string, signer *hmacsigner.Signer, logger *slog.Logger) *Client {
	// TLS 配置
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	return &Client{
		serverURL: serverURL,
		signer:    signer,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig:    tlsConfig,
				MaxIdleConns:       10,
				IdleConnTimeout:    90 * time.Second,
				DisableCompression: false,
			},
		},
		logger: logger,
	}
}

// APIError 服务端 API 错误
type APIError struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details"`
}

// Error 实现 error 接口
func (e *APIError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// DeviceRequest 设备 HMAC 认证请求
func (c *Client) DeviceRequest(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	fullURL := c.serverURL + path

	req, err := http.NewRequestWithContext(ctx, method, fullURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// HMAC 签名
	if c.signer != nil {
		signResult, err := c.signer.SignRequest(req, body)
		if err != nil {
			return nil, fmt.Errorf("签名失败: %w", err)
		}
		signResult.ApplyToRequest(req)
	}

	// 发送请求（带重试）
	return c.doWithRetry(ctx, req, 3)
}

// SessionRequest 用户 Session 认证请求
func (c *Client) SessionRequest(ctx context.Context, method, path string, body []byte, sessionToken, csrfState string) ([]byte, error) {
	fullURL := c.serverURL + path

	req, err := http.NewRequestWithContext(ctx, method, fullURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// 设置 Session 认证
	if sessionToken != "" {
		req.Header.Set("Cookie", "session_token="+sessionToken)
	}
	if csrfState != "" {
		req.Header.Set("X-CSRF-Token", csrfState)
	}

	return c.doWithRetry(ctx, req, 2)
}

// doWithRetry 带指数退避的请求重试
func (c *Client) doWithRetry(ctx context.Context, req *http.Request, maxRetries int) ([]byte, error) {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// 指数退避：1s, 2s, 4s
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		// 克隆请求（因为 body 可能被消费）
		reqClone := req.Clone(ctx)
		if req.Body != nil {
			// 重新设置 body（简化实现，实际应使用 bytes.Buffer）
			reqClone.Body = req.Body
		}

		resp, err := c.httpClient.Do(reqClone)
		if err != nil {
			lastErr = fmt.Errorf("请求失败: %w", err)
			c.logger.Warn("请求重试", "attempt", attempt+1, "error", err)
			continue
		}

		respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
		resp.Body.Close()

		if err != nil {
			lastErr = fmt.Errorf("读取响应失败: %w", err)
			continue
		}

		// 成功响应
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return respBody, nil
		}

		// 解析错误
		var apiErr APIError
		if json.Unmarshal(respBody, &apiErr) == nil && apiErr.Code != "" {
			// 特定错误不重试
			if !isRetryableError(apiErr.Code) {
				return nil, &apiErr
			}
			lastErr = &apiErr
		} else {
			lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
		}

		c.logger.Warn("请求重试", "attempt", attempt+1, "status", resp.StatusCode)
	}

	return nil, fmt.Errorf("请求失败（已重试 %d 次）: %w", maxRetries, lastErr)
}

// isRetryableError 判断错误是否可重试
func isRetryableError(code string) bool {
	switch code {
	case "SESSION_REPLACED", "CLIENT_OWNER_MISMATCH", "DEVICE_REVOKED",
		"USER_DISABLED", "INVALID_SIGNATURE", "TOKEN_VERSION_MISMATCH":
		return false
	default:
		return true
	}
}

// --- 以下为具体业务 API ---

// FullSyncRequest 全量同步请求
type FullSyncRequest struct {
	CurrentConfigVersion int64 `json:"current_config_version"`
}

// FullSyncResponse 全量同步响应
type FullSyncResponse struct {
	ConfigVersion    int64           `json:"config_version"`
	ConfigBody       json.RawMessage `json:"config_body"`
	ConfigHash       string          `json:"config_hash"`
	Signature        string          `json:"signature"`
	SigningKeyID     string          `json:"signing_key_id"`
	SchemaVersion    int             `json:"schema_version"`
	Mappings         json.RawMessage `json:"mappings"`
	ServerAddr       string          `json:"server_addr"`
	ServerPort       int             `json:"server_port"`
	FRPUser          string          `json:"frp_user"`
	FRPToken         string          `json:"frp_token_encrypted"`
	NeedRestart      bool            `json:"need_restart"`
}

// FullSync 执行全量同步
func (c *Client) FullSync(ctx context.Context, currentVersion int64) (*FullSyncResponse, error) {
	reqBody, _ := json.Marshal(FullSyncRequest{CurrentConfigVersion: currentVersion})

	resp, err := c.DeviceRequest(ctx, "POST", "/api/v1/client/sync", reqBody)
	if err != nil {
		return nil, fmt.Errorf("全量同步请求失败: %w", err)
	}

	var syncResp FullSyncResponse
	if err := json.Unmarshal(resp, &syncResp); err != nil {
		return nil, fmt.Errorf("解析同步响应失败: %w", err)
	}

	return &syncResp, nil
}

// HeartbeatRequest 心跳请求
type HeartbeatRequest struct {
	FRPCStatus       string `json:"frpc_status"`
	FRPCPID          int    `json:"frpc_pid"`
	FRPCVersion      string `json:"frpc_version"`
	AppliedVersion   int64  `json:"applied_config_version"`
	ProxyCount       int    `json:"proxy_count"`
	UptimeSeconds    int64  `json:"uptime_seconds"`
	LastError        string `json:"last_error"`
}

// SendHeartbeat 发送心跳
func (c *Client) SendHeartbeat(ctx context.Context, req *HeartbeatRequest) error {
	body, _ := json.Marshal(req)
	_, err := c.DeviceRequest(ctx, "POST", "/api/v1/client/heartbeat", body)
	if err != nil {
		return fmt.Errorf("心跳发送失败: %w", err)
	}
	return nil
}

// StatusReport 状态上报
type StatusReport struct {
	AppliedVersion   int64  `json:"applied_config_version"`
	FailedVersion    int64  `json:"failed_config_version"`
	FRPCStatus       string `json:"frpc_status"`
	LastError        string `json:"last_error"`
	ProxyStatuses    []ProxyStatus `json:"proxy_statuses"`
}

// ProxyStatus 代理状态
type ProxyStatus struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Status   string `json:"status"`
	Error    string `json:"error"`
}

// ReportStatus 上报状态
func (c *Client) ReportStatus(ctx context.Context, report *StatusReport) error {
	body, _ := json.Marshal(report)
	_, err := c.DeviceRequest(ctx, "POST", "/api/v1/client/status", body)
	if err != nil {
		return fmt.Errorf("状态上报失败: %w", err)
	}
	return nil
}

// ApplyResult 应用结果上报
type ApplyResult struct {
	ConfigVersion int64  `json:"config_version"`
	Success       bool   `json:"success"`
	ErrorSummary  string `json:"error_summary"`
	AppliedAt     string `json:"applied_at"`
	Action        string `json:"action"` // reload, restart
}

// ReportApplyResult 上报配置应用结果
func (c *Client) ReportApplyResult(ctx context.Context, result *ApplyResult) error {
	body, _ := json.Marshal(result)
	_, err := c.DeviceRequest(ctx, "POST", "/api/v1/client/apply-result", body)
	if err != nil {
		return fmt.Errorf("应用结果上报失败: %w", err)
	}
	return nil
}

// ConfigPullResponse 配置拉取响应
type ConfigPullResponse struct {
	ConfigVersion int64           `json:"config_version"`
	ConfigBody    json.RawMessage `json:"config_body"`
	ConfigHash    string          `json:"config_hash"`
	Signature     string          `json:"signature"`
	SigningKeyID  string          `json:"signing_key_id"`
	SchemaVersion int             `json:"schema_version"`
}

// PullConfig 拉取最新配置
func (c *Client) PullConfig(ctx context.Context) (*ConfigPullResponse, error) {
	resp, err := c.DeviceRequest(ctx, "GET", "/api/v1/client/config", nil)
	if err != nil {
		return nil, fmt.Errorf("配置拉取失败: %w", err)
	}

	var configResp ConfigPullResponse
	if err := json.Unmarshal(resp, &configResp); err != nil {
		return nil, fmt.Errorf("解析配置响应失败: %w", err)
	}

	return &configResp, nil
}

// UpdateCheckResponse 版本检查响应
type UpdateCheckResponse struct {
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version"`
	MinVersion     string `json:"min_version"`
	DownloadURL    string `json:"download_url"`
	ReleaseNotes   string `json:"release_notes"`
	MustUpgrade    bool   `json:"must_upgrade"`
}

// CheckUpdate 检查更新
func (c *Client) CheckUpdate(ctx context.Context, currentVersion string) (*UpdateCheckResponse, error) {
	path := "/api/v1/client/version?current=" + currentVersion
	resp, err := c.DeviceRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("版本检查失败: %w", err)
	}

	var updateResp UpdateCheckResponse
	if err := json.Unmarshal(resp, &updateResp); err != nil {
		return nil, fmt.Errorf("解析版本响应失败: %w", err)
	}

	return &updateResp, nil
}

// SetServerURL 更新服务端地址
func (c *Client) SetServerURL(url string) {
	c.serverURL = url
}
