package configrender

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ServerConfig 服务端下发的配置
type ServerConfig struct {
	ServerAddr     string        `json:"server_addr"`
	ServerPort     int           `json:"server_port"`
	FRPUser        string        `json:"frp_user"`
	FRPToken       string        `json:"frp_token_encrypted"`
	Transport      TransportConf `json:"transport"`
	Proxies        []ProxyConfig `json:"proxies"`
	AdminPort      int           `json:"admin_port"`
	AdminUser      string        `json:"admin_user"`
	AdminPass      string        `json:"admin_pass"`
	LogLevel       string        `json:"log_level"`
	LogMaxDays     int           `json:"log_max_days"`
}

// TransportConf 传输配置
type TransportConf struct {
	Protocol         string `json:"protocol"`          // tcp, kcp, quic, wss, websocket
	PoolCount        int    `json:"pool_count"`
	TCPMux           bool   `json:"tcp_mux"`
	HeartbeatTimeout int    `json:"heartbeat_timeout"`
	DialServerTimeout int   `json:"dial_server_timeout"`
}

// ProxyConfig 代理配置
type ProxyConfig struct {
	MappingUUID  string   `json:"mapping_uuid"`
	Name         string   `json:"name"`
	Type         string   `json:"type"` // tcp, udp, http, https, stcp, xtcp
	LocalIP      string   `json:"local_ip"`
	LocalPort    int      `json:"local_port"`
	RemotePort   int      `json:"remote_port"`
	CustomDomains []string `json:"custom_domains"`
	SubDomain    string   `json:"sub_domain"`
	Locations    []string `json:"locations"`
	HTTPUser     string   `json:"http_user"`
	HTTPPassword string   `json:"http_password"`
	UseEncryption bool    `json:"use_encryption"`
	UseCompression bool   `json:"use_compression"`
	Enabled      bool     `json:"enabled"`
	MappingID    string   `json:"mapping_id"`
	MappingRevision int64 `json:"mapping_revision"`
}

// RenderResult 渲染结果
type RenderResult struct {
	MainConfig   string // frpc.toml 主配置
	ProxyConfig  string // conf.d/proxies.toml 代理配置
	NeedRestart  bool   // 是否需要 restart（vs reload）
	ChangedParts []string // 变更的部分
}

// Renderer FRPC 配置渲染器
// 将服务端下发的配置渲染为 frpc.toml 格式
// 采用 main config + conf.d/proxies.toml 结构
type Renderer struct{}

// NewRenderer 创建渲染器
func NewRenderer() *Renderer {
	return &Renderer{}
}

// Render 渲染完整配置
func (r *Renderer) Render(cfg *ServerConfig) (*RenderResult, error) {
	if cfg == nil {
		return nil, fmt.Errorf("配置为空")
	}

	// 渲染主配置
	mainConfig, err := r.renderMainConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("渲染主配置失败: %w", err)
	}

	// 渲染代理配置
	proxyConfig, err := r.renderProxyConfig(cfg.Proxies)
	if err != nil {
		return nil, fmt.Errorf("渲染代理配置失败: %w", err)
	}

	return &RenderResult{
		MainConfig:  mainConfig,
		ProxyConfig: proxyConfig,
		NeedRestart: true, // 首次渲染总是需要 restart
	}, nil
}

// RenderProxyOnly 仅渲染代理配置（用于 reload 场景）
func (r *Renderer) RenderProxyOnly(proxies []ProxyConfig) (string, error) {
	return r.renderProxyConfig(proxies)
}

// renderMainConfig 渲染主配置
// 包含：server 地址、认证、admin API、includes、日志
func (r *Renderer) renderMainConfig(cfg *ServerConfig) (string, error) {
	var b strings.Builder

	b.WriteString("# FRPC 主配置 - 由 Client Panel 自动生成\n")
	b.WriteString("# 警告：请勿手动编辑此文件\n\n")

	// 服务端连接
	b.WriteString(fmt.Sprintf("serverAddr = %q\n", cfg.ServerAddr))
	b.WriteString(fmt.Sprintf("serverPort = %d\n\n", cfg.ServerPort))

	// FRP 用户认证
	b.WriteString(fmt.Sprintf("user = %q\n\n", cfg.FRPUser))
	// 注意：FRP Token 通过受保护文件注入，不写入明文配置
	// auth.token 通过 includes 或环境变量注入

	// 传输配置
	if cfg.Transport.Protocol != "" {
		b.WriteString("[transport]\n")
		b.WriteString(fmt.Sprintf("protocol = %q\n", cfg.Transport.Protocol))
		if cfg.Transport.PoolCount > 0 {
			b.WriteString(fmt.Sprintf("poolCount = %d\n", cfg.Transport.PoolCount))
		}
		b.WriteString(fmt.Sprintf("tcpMux = %v\n", cfg.Transport.TCPMux))
		if cfg.Transport.HeartbeatTimeout > 0 {
			b.WriteString(fmt.Sprintf("heartbeatTimeout = %d\n", cfg.Transport.HeartbeatTimeout))
		}
		if cfg.Transport.DialServerTimeout > 0 {
			b.WriteString(fmt.Sprintf("dialServerTimeout = %d\n", cfg.Transport.DialServerTimeout))
		}
		b.WriteString("\n")
	}

	// Admin API（仅本机访问）
	if cfg.AdminPort > 0 {
		b.WriteString("[webServer]\n")
		b.WriteString("addr = \"127.0.0.1\"\n")
		b.WriteString(fmt.Sprintf("port = %d\n", cfg.AdminPort))
		b.WriteString(fmt.Sprintf("user = %q\n", cfg.AdminUser))
		b.WriteString(fmt.Sprintf("password = %q\n", cfg.AdminPass))
		b.WriteString("\n")
	}

	// 日志配置
	b.WriteString("[log]\n")
	if cfg.LogLevel != "" {
		b.WriteString(fmt.Sprintf("level = %q\n", cfg.LogLevel))
	} else {
		b.WriteString("level = \"info\"\n")
	}
	b.WriteString("maxDays = 7\n")
	if cfg.LogMaxDays > 0 {
		b.WriteString(fmt.Sprintf("maxDays = %d\n", cfg.LogMaxDays))
	}
	b.WriteString("\n")

	// 包含代理配置
	b.WriteString("[include]\n")
	b.WriteString("includes = [\"conf.d/*.toml\"]\n")

	return b.String(), nil
}

// renderProxyConfig 渲染代理配置
// 代理名称格式：m_<mapping_uuid_without_dash>
// 禁止使用用户自由输入的名称作为内部唯一键
func (r *Renderer) renderProxyConfig(proxies []ProxyConfig) (string, error) {
	var b strings.Builder

	b.WriteString("# FRPC 代理配置 - 由 Client Panel 自动生成\n")
	b.WriteString("# 警告：请勿手动编辑此文件\n\n")

	for _, proxy := range proxies {
		// 代理名称：m_<mapping_uuid_without_dash>
		proxyName := formatProxyName(proxy.MappingUUID, proxy.Name)

		// 根据类型渲染
		switch strings.ToLower(proxy.Type) {
		case "tcp":
			r.renderTCPProxy(&b, proxyName, proxy)
		case "udp":
			r.renderUDPProxy(&b, proxyName, proxy)
		case "http":
			r.renderHTTPProxy(&b, proxyName, proxy)
		default:
			return "", fmt.Errorf("不支持的代理类型: %s", proxy.Type)
		}

		b.WriteString("\n")
	}

	return b.String(), nil
}

// renderTCPProxy 渲染 TCP 代理配置
func (r *Renderer) renderTCPProxy(b *strings.Builder, name string, proxy ProxyConfig) {
	b.WriteString(fmt.Sprintf("[[proxies]]\n"))
	b.WriteString(fmt.Sprintf("name = %q\n", name))
	b.WriteString("type = \"tcp\"\n")
	b.WriteString(fmt.Sprintf("localIP = %q\n", proxy.LocalIP))
	b.WriteString(fmt.Sprintf("localPort = %d\n", proxy.LocalPort))
	if proxy.RemotePort > 0 {
		b.WriteString(fmt.Sprintf("remotePort = %d\n", proxy.RemotePort))
	}
	r.renderCommonProxyFields(b, proxy)
}

// renderUDPProxy 渲染 UDP 代理配置
func (r *Renderer) renderUDPProxy(b *strings.Builder, name string, proxy ProxyConfig) {
	b.WriteString(fmt.Sprintf("[[proxies]]\n"))
	b.WriteString(fmt.Sprintf("name = %q\n", name))
	b.WriteString("type = \"udp\"\n")
	b.WriteString(fmt.Sprintf("localIP = %q\n", proxy.LocalIP))
	b.WriteString(fmt.Sprintf("localPort = %d\n", proxy.LocalPort))
	if proxy.RemotePort > 0 {
		b.WriteString(fmt.Sprintf("remotePort = %d\n", proxy.RemotePort))
	}
	r.renderCommonProxyFields(b, proxy)
}

// renderHTTPProxy 渲染 HTTP 代理配置
func (r *Renderer) renderHTTPProxy(b *strings.Builder, name string, proxy ProxyConfig) {
	b.WriteString(fmt.Sprintf("[[proxies]]\n"))
	b.WriteString(fmt.Sprintf("name = %q\n", name))
	b.WriteString("type = \"http\"\n")
	b.WriteString(fmt.Sprintf("localIP = %q\n", proxy.LocalIP))
	b.WriteString(fmt.Sprintf("localPort = %d\n", proxy.LocalPort))

	// HTTP 代理使用 customDomains
	if len(proxy.CustomDomains) > 0 {
		b.WriteString("customDomains = [")
		for i, domain := range proxy.CustomDomains {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(fmt.Sprintf("%q", domain))
		}
		b.WriteString("]\n")
	}

	if proxy.SubDomain != "" {
		b.WriteString(fmt.Sprintf("subDomain = %q\n", proxy.SubDomain))
	}

	// Locations
	if len(proxy.Locations) > 0 {
		b.WriteString("locations = [")
		for i, loc := range proxy.Locations {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(fmt.Sprintf("%q", loc))
		}
		b.WriteString("]\n")
	}

	// HTTP Basic Auth
	if proxy.HTTPUser != "" {
		b.WriteString(fmt.Sprintf("httpUser = %q\n", proxy.HTTPUser))
	}
	if proxy.HTTPPassword != "" {
		b.WriteString(fmt.Sprintf("httpPassword = %q\n", proxy.HTTPPassword))
	}

	r.renderCommonProxyFields(b, proxy)
}

// renderCommonProxyFields 渲染通用代理字段
func (r *Renderer) renderCommonProxyFields(b *strings.Builder, proxy ProxyConfig) {
	if proxy.UseEncryption {
		b.WriteString("useEncryption = true\n")
	}
	if proxy.UseCompression {
		b.WriteString("useCompression = true\n")
	}
}

// formatProxyName 格式化代理名称
// 格式：m_<mapping_uuid_without_dash>
// 确保名称稳定、不可碰撞，不使用用户输入
func formatProxyName(mappingUUID, fallbackName string) string {
	if mappingUUID != "" {
		// 移除 UUID 中的连字符
		cleaned := strings.ReplaceAll(mappingUUID, "-", "")
		return "m_" + cleaned
	}
	// 备用名称（不应发生）
	if fallbackName != "" {
		return "m_" + strings.ReplaceAll(fallbackName, " ", "_")
	}
	return "m_unknown"
}

// DiffConfigs 比较两份代理配置，判断变化类型
// 返回：需要 restart 的变更和仅需 reload 的变更
func DiffConfigs(old, new []ProxyConfig) (needRestart bool, changedProxies []string) {
	// 构建旧配置映射
	oldMap := make(map[string]ProxyConfig)
	for _, p := range old {
		oldMap[p.MappingUUID] = p
	}

	for _, np := range new {
		op, exists := oldMap[np.MappingUUID]
		if !exists {
			// 新增代理 - reload
			changedProxies = append(changedProxies, np.MappingUUID)
			continue
		}

		// 比较字段变化
		if op.LocalIP != np.LocalIP || op.LocalPort != np.LocalPort ||
			op.RemotePort != np.RemotePort || op.Type != np.Type {
			// 本地地址/端口/类型变化 - 尝试 reload
			changedProxies = append(changedProxies, np.MappingUUID)
		}

		if !stringSliceEqual(op.CustomDomains, np.CustomDomains) {
			changedProxies = append(changedProxies, np.MappingUUID)
		}
	}

	// 检查删除的代理
	for _, op := range old {
		found := false
		for _, np := range new {
			if np.MappingUUID == op.MappingUUID {
				found = true
				break
			}
		}
		if !found {
			changedProxies = append(changedProxies, op.MappingUUID)
		}
	}

	return needRestart, changedProxies
}

// stringSliceEqual 比较字符串切片
func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ParseServerConfig 从 JSON 解析服务端配置
func ParseServerConfig(data json.RawMessage) (*ServerConfig, error) {
	var cfg ServerConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}
	return &cfg, nil
}
