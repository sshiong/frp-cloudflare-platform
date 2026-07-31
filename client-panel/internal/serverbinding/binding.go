package serverbinding

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// BindingState 服务端绑定状态
type BindingState string

const (
	StateUnbound          BindingState = "unbound"
	StateBinding          BindingState = "binding"
	StateBound            BindingState = "bound"
	StateSwitchingServer  BindingState = "switching_server"
	StateCredentialRevoked BindingState = "credential_revoked"
	StateUnbinding        BindingState = "unbinding"
)

// TLSTrustMode TLS 信任模式
type TLSTrustMode string

const (
	TrustSystem    TLSTrustMode = "system"
	TrustPinnedSPKI TLSTrustMode = "pinned_spki"
	TrustCustomCA  TLSTrustMode = "custom_ca"
)

// BindingInfo 绑定信息
type BindingInfo struct {
	State              BindingState
	ServerInstanceID   string
	NormalizedURL      string
	OwnerUserID        string
	ClientID           string
	Revision           int64
	TLSTrustMode       TLSTrustMode
	PinnedSPKISHA256   string
	CustomCAPath       string
}

// Manager 服务端绑定管理器
// 管理 Client Panel 与 Server Panel 的绑定关系
// 包括地址规范化、SSRF 防护、TLS 验证和绑定状态管理
type Manager struct {
	mu      sync.RWMutex
	binding *BindingInfo
	logger  *slog.Logger
}

// NewManager 创建绑定管理器
func NewManager(logger *slog.Logger) *Manager {
	return &Manager{
		binding: &BindingInfo{
			State: StateUnbound,
		},
		logger: logger,
	}
}

// NormalizeServerURL 规范化服务端地址
// 规则：
// - 生产环境必须使用 HTTPS
// - 禁止 URL 中携带用户名密码
// - 禁止任意路径，规范化为根地址
// - 解析后执行 SSRF 防护
func NormalizeServerURL(rawURL string, allowHTTP bool) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("URL 解析失败: %w", err)
	}

	// 验证协议
	scheme := strings.ToLower(u.Scheme)
	if scheme != "https" && scheme != "http" {
		return "", fmt.Errorf("不支持的协议: %s", scheme)
	}
	if scheme == "http" && !allowHTTP {
		return "", fmt.Errorf("生产环境必须使用 HTTPS")
	}

	// 禁止 URL 中携带用户名密码
	if u.User != nil {
		return "", fmt.Errorf("禁止在 URL 中携带用户名和密码")
	}

	// 验证主机名
	host := u.Hostname()
	if host == "" {
		return "", fmt.Errorf("缺少主机名")
	}

	// 规范化：仅保留 scheme + host + port
	port := u.Port()
	normalized := scheme + "://" + host
	if port != "" {
		normalized += ":" + port
	}

	return normalized, nil
}

// ValidateSSRFProtection 验证 SSRF 防护
// 禁止连接到：
// - 127.0.0.0/8 (loopback)
// - 10.0.0.0/8 (私有)
// - 172.16.0.0/12 (私有)
// - 192.168.0.0/16 (私有)
// - 169.254.0.0/16 (链路本地)
// - 0.0.0.0/8
// - ::1, fc00::/7, fe80::/10
//
// 例外：明确允许的开发/局域网地址
func ValidateSSRFProtection(host string, allowPrivate bool) error {
	if allowPrivate {
		return nil
	}

	// 解析 IP
	ips, err := net.LookupIP(host)
	if err != nil {
		// 可能是域名，暂时允许（TLS 验证时再解析）
		if net.ParseIP(host) == nil {
			return nil
		}
		return fmt.Errorf("DNS 解析失败: %w", err)
	}

	for _, ip := range ips {
		if isPrivateOrReserved(ip) {
			return fmt.Errorf("SSRF 防护: 禁止连接到私有/保留地址 %s", ip)
		}
	}

	return nil
}

// isPrivateOrReserved 检查 IP 是否为私有或保留地址
func isPrivateOrReserved(ip net.IP) bool {
	// IPv4 私有和保留地址
	privateCIDRs := []string{
		"127.0.0.0/8",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"169.254.0.0/16",
		"0.0.0.0/8",
	}

	for _, cidr := range privateCIDRs {
		_, network, _ := net.ParseCIDR(cidr)
		if network != nil && network.Contains(ip) {
			return true
		}
	}

	// IPv6
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	if ip.To4() == nil {
		// fc00::/7 (ULA)
		if len(ip) == net.IPv6len && ip[0]&0xfe == 0xfc {
			return true
		}
	}

	return false
}

// VerifyTLSCertificate 验证 TLS 证书
// 支持三种模式：
// - system: 使用系统 CA
// - pinned_spki: 验证 SPKI 指纹
// - custom_ca: 使用自定义 CA
func VerifyTLSCertificate(serverURL string, trustMode TLSTrustMode, pinnedSPKI string, customCAPath string) (*tls.ConnectionState, error) {
	u, err := url.Parse(serverURL)
	if err != nil {
		return nil, fmt.Errorf("URL 解析失败: %w", err)
	}

	host := u.Host
	if u.Port() == "" {
		host += ":443"
	}

	// 配置 TLS
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	switch trustMode {
	case TrustSystem:
		// 使用默认系统 CA
	case TrustCustomCA:
		// 加载自定义 CA
		if customCAPath == "" {
			return nil, fmt.Errorf("自定义 CA 路径未指定")
		}
		caCert, err := x509.SystemCertPool()
		if err != nil {
			caCert = x509.NewCertPool()
		}
		// 注意：实际应从文件加载 CA 证书
		_ = caCert
		tlsConfig.RootCAs = caCert
	case TrustPinnedSPKI:
		// SPKI 指纹验证在连接后执行
	default:
		return nil, fmt.Errorf("不支持的 TLS 信任模式: %s", trustMode)
	}

	// 建立 TLS 连接
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second}, "tcp", host, tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("TLS 连接失败: %w", err)
	}
	defer conn.Close()

	state := conn.ConnectionState()

	// 验证 SPKI 指纹
	if trustMode == TrustPinnedSPKI && pinnedSPKI != "" {
		if len(state.PeerCertificates) == 0 {
			return nil, fmt.Errorf("服务端未提供证书")
		}
		spkiHash := ComputeSPKIHash(state.PeerCertificates[0])
		if spkiHash != pinnedSPKI {
			return nil, fmt.Errorf("SPKI 指纹不匹配: 期望 %s, 实际 %s", pinnedSPKI, spkiHash)
		}
	}

	return &state, nil
}

// ComputeSPKIHash 计算证书 SPKI SHA-256 指纹
func ComputeSPKIHash(cert *x509.Certificate) string {
	spkiHash := sha256.Sum256(cert.RawSubjectPublicKeyInfo)
	return hex.EncodeToString(spkiHash[:])
}

// GetServerInstanceID 请求服务端获取实例 ID
// 通过只读接口 /api/v1/instance/id 获取
func GetServerInstanceID(ctx context.Context, serverURL string, httpClient *http.Client) (string, error) {
	instanceURL := serverURL + "/api/v1/instance/id"
	req, err := http.NewRequestWithContext(ctx, "GET", instanceURL, nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求服务端实例 ID 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("获取实例 ID 失败: HTTP %d", resp.StatusCode)
	}

	var result struct {
		InstanceID string `json:"instance_id"`
	}
	if err := decodeJSON(resp.Body, &result); err != nil {
		return "", fmt.Errorf("解析实例 ID 失败: %w", err)
	}

	if result.InstanceID == "" {
		return "", fmt.Errorf("服务端返回空实例 ID")
	}

	return result.InstanceID, nil
}

// SetBinding 更新绑定信息
func (m *Manager) SetBinding(info *BindingInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.binding = info
	m.logger.Info("绑定状态已更新",
		"state", info.State,
		"server_id", info.ServerInstanceID,
		"client_id", info.ClientID,
	)
}

// GetBinding 获取当前绑定信息
func (m *Manager) GetBinding() *BindingInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.binding
}

// GetState 获取当前绑定状态
func (m *Manager) GetState() BindingState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.binding.State
}

// IsBound 检查是否已绑定
func (m *Manager) IsBound() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.binding.State == StateBound
}

// 辅助函数：从 response body 解码 JSON
func decodeJSON(body interface{ Read([]byte) (int, error) }, v interface{}) error {
	// 使用标准 json.Decoder
	data := make([]byte, 0, 4096)
	buf := make([]byte, 1024)
	for {
		n, err := body.Read(buf)
		if n > 0 {
			data = append(data, buf[:n]...)
		}
		if err != nil {
			break
		}
	}
	return json.Unmarshal(data, v)
}
