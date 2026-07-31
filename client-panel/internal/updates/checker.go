package updates

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// UpdateInfo 更新信息
type UpdateInfo struct {
	HasUpdate     bool   `json:"has_update"`
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version"`
	MinVersion     string `json:"min_version"`
	DownloadURL    string `json:"download_url"`
	ReleaseNotes   string `json:"release_notes"`
	MustUpgrade    bool   `json:"must_upgrade"`
}

// Checker 更新检查器
// 检查 Client Panel 和 FRPC 的版本更新
// 支持最低版本强制升级策略
type Checker struct {
	currentVersion string
	clientID       string
	httpClient     *http.Client
	logger         *slog.Logger
	serverURL      string
}

// NewChecker 创建更新检查器
func NewChecker(currentVersion, clientID, serverURL string, logger *slog.Logger) *Checker {
	return &Checker{
		currentVersion: currentVersion,
		clientID:       clientID,
		serverURL:      serverURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
	}
}

// CheckUpdate 检查更新
func (c *Checker) CheckUpdate(ctx context.Context) (*UpdateInfo, error) {
	if c.serverURL == "" {
		return nil, fmt.Errorf("服务端地址未配置")
	}

	url := fmt.Sprintf("%s/api/v1/client/version?current=%s", c.serverURL, c.currentVersion)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("版本检查失败: HTTP %d", resp.StatusCode)
	}

	var info UpdateInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("解析版本信息失败: %w", err)
	}

	info.CurrentVersion = c.currentVersion

	// 判断是否有更新
	info.HasUpdate = isNewer(c.currentVersion, info.LatestVersion)
	info.MustUpgrade = isNewer(c.currentVersion, info.MinVersion)

	return &info, nil
}

// isNewer 比较版本号
// 支持语义化版本格式：v1.2.3 或 1.2.3
func isNewer(current, target string) bool {
	if target == "" {
		return false
	}

	// 移除 v 前缀
	current = strings.TrimPrefix(current, "v")
	target = strings.TrimPrefix(target, "v")

	// 分割版本号
	currentParts := strings.Split(current, ".")
	targetParts := strings.Split(target, ".")

	// 逐段比较
	maxLen := len(currentParts)
	if len(targetParts) > maxLen {
		maxLen = len(targetParts)
	}

	for i := 0; i < maxLen; i++ {
		var c, t int
		if i < len(currentParts) {
			fmt.Sscanf(currentParts[i], "%d", &c)
		}
		if i < len(targetParts) {
			fmt.Sscanf(targetParts[i], "%d", &t)
		}

		if t > c {
			return true
		}
		if t < c {
			return false
		}
	}

	return false
}

// ValidateDownloadURL 验证下载 URL 安全性
func ValidateDownloadURL(downloadURL string) error {
	if downloadURL == "" {
		return fmt.Errorf("下载 URL 为空")
	}

	// 必须使用 HTTPS
	if !strings.HasPrefix(downloadURL, "https://") {
		return fmt.Errorf("下载 URL 必须使用 HTTPS")
	}

	// 禁止包含认证信息
	if strings.Contains(downloadURL, "@") {
		return fmt.Errorf("下载 URL 禁止包含认证信息")
	}

	return nil
}
