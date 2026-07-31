package health

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"
)

// ProbeResult 探测结果
type ProbeResult struct {
	Success  bool
	Message  string
	Protocol string
	Latency  time.Duration
}

// ProbeType 探测类型
type ProbeType string

const (
	ProbeTCP  ProbeType = "tcp"
	ProbeUDP  ProbeType = "udp"
	ProbeHTTP ProbeType = "http"
)

// Prober 本地服务健康探测器
type Prober struct {
	logger *slog.Logger
}

// NewProber 创建探测器
func NewProber(logger *slog.Logger) *Prober {
	return &Prober{logger: logger}
}

// Probe 执行探测
func (p *Prober) Probe(ctx context.Context, probeType ProbeType, ip string, port int, opts *ProbeOptions) *ProbeResult {
	switch probeType {
	case ProbeTCP:
		return p.probeTCP(ctx, ip, port, opts)
	case ProbeUDP:
		return p.probeUDP(ip, port, opts)
	case ProbeHTTP:
		return p.probeHTTP(ctx, ip, port, opts)
	default:
		return &ProbeResult{
			Success: false,
			Message: fmt.Sprintf("不支持的探测类型: %s", probeType),
		}
	}
}

// ProbeOptions 探测选项
type ProbeOptions struct {
	Timeout        time.Duration // 超时时间，默认 2s
	MaxRetries     int           // 最大重试次数，默认 2
	HTTPPath       string        // HTTP 探测路径，默认 /
	HTTPStatusMin  int           // HTTP 成功状态码下限，默认 200
	HTTPStatusMax  int           // HTTP 成功状态码上限，默认 299
	MaxBodyRead    int64         // HTTP 响应体最大读取字节数，默认 4KB
	MaxRedirects   int           // HTTP 最大重定向次数，默认 3
}

// DefaultProbeOptions 默认探测选项
func DefaultProbeOptions() *ProbeOptions {
	return &ProbeOptions{
		Timeout:       2 * time.Second,
		MaxRetries:    2,
		HTTPPath:      "/",
		HTTPStatusMin: 200,
		HTTPStatusMax: 299,
		MaxBodyRead:   4096,
		MaxRedirects:  3,
	}
}

// probeTCP TCP 探测
// 连接 local_ip:local_port，超时 2s，最多重试 2 次
// 不发送应用层数据
func (p *Prober) probeTCP(ctx context.Context, ip string, port int, opts *ProbeOptions) *ProbeResult {
	if opts == nil {
		opts = DefaultProbeOptions()
	}

	addr := net.JoinHostPort(ip, strconv.Itoa(port))
	start := time.Now()

	for attempt := 0; attempt <= opts.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(500 * time.Millisecond)
		}

		dialer := &net.Dialer{Timeout: opts.Timeout}
		conn, err := dialer.DialContext(ctx, "tcp", addr)
		if err != nil {
			if attempt == opts.MaxRetries {
				return &ProbeResult{
					Success:  false,
					Protocol: "tcp",
					Message:  fmt.Sprintf("TCP 连接失败: %s", err),
					Latency:  time.Since(start),
				}
			}
			continue
		}
		conn.Close()

		return &ProbeResult{
			Success:  true,
			Protocol: "tcp",
			Message:  "TCP 连接成功",
			Latency:  time.Since(start),
		}
	}

	return &ProbeResult{
		Success:  false,
		Protocol: "tcp",
		Message:  "TCP 探测失败（已用完重试次数）",
		Latency:  time.Since(start),
	}
}

// probeUDP UDP 探测
// UDP 没有可靠的通用"监听成功"握手
// 仅验证格式，不发送数据
func (p *Prober) probeUDP(ip string, port int, opts *ProbeOptions) *ProbeResult {
	// 验证 IP 和端口格式
	if net.ParseIP(ip) == nil {
		return &ProbeResult{
			Success:  false,
			Protocol: "udp",
			Message:  fmt.Sprintf("无效的 IP 地址: %s", ip),
		}
	}

	if port <= 0 || port > 65535 {
		return &ProbeResult{
			Success:  false,
			Protocol: "udp",
			Message:  fmt.Sprintf("无效的端口号: %d", port),
		}
	}

	// UDP 不做实际连接测试
	return &ProbeResult{
		Success:  true,
		Protocol: "udp",
		Message:  "UDP 端口格式正确（UDP 无法通过通用方式确认监听状态）",
	}
}

// probeHTTP HTTP 探测
// GET /，可配置状态码范围
// 限制响应体读取、重定向次数
// 清理代理环境变量
func (p *Prober) probeHTTP(ctx context.Context, ip string, port int, opts *ProbeOptions) *ProbeResult {
	if opts == nil {
		opts = DefaultProbeOptions()
	}

	// 构造 URL
	scheme := "http"
	if port == 443 {
		scheme = "https"
	}
	targetURL := fmt.Sprintf("%s://%s:%d%s", scheme, ip, port, opts.HTTPPath)

	start := time.Now()

	// 创建不使用代理的 Transport
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: opts.Timeout,
		}).DialContext,
		DisableKeepAlives: true,
	}

	// 限制重定向次数
	client := &http.Client{
		Transport: transport,
		Timeout:   opts.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= opts.MaxRedirects {
				return fmt.Errorf("超过最大重定向次数 %d", opts.MaxRedirects)
			}
			// 不跟随到非本机的重定向
			redirectHost := req.URL.Hostname()
			if redirectHost != ip && redirectHost != "localhost" && redirectHost != "127.0.0.1" {
				return fmt.Errorf("禁止跟随到非本机重定向: %s", redirectHost)
			}
			return nil
		},
	}

	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return &ProbeResult{
			Success:  false,
			Protocol: "http",
			Message:  fmt.Sprintf("创建请求失败: %s", err),
			Latency:  time.Since(start),
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return &ProbeResult{
			Success:  false,
			Protocol: "http",
			Message:  fmt.Sprintf("HTTP 请求失败: %s", err),
			Latency:  time.Since(start),
		}
	}
	defer resp.Body.Close()

	// 限制读取响应体
	io.ReadAll(io.LimitReader(resp.Body, opts.MaxBodyRead))

	// 检查状态码范围
	if resp.StatusCode < opts.HTTPStatusMin || resp.StatusCode > opts.HTTPStatusMax {
		return &ProbeResult{
			Success:  false,
			Protocol: "http",
			Message:  fmt.Sprintf("HTTP 状态码不在预期范围: %d（期望 %d-%d）", resp.StatusCode, opts.HTTPStatusMin, opts.HTTPStatusMax),
			Latency:  time.Since(start),
		}
	}

	return &ProbeResult{
		Success:  true,
		Protocol: "http",
		Message:  fmt.Sprintf("HTTP 探测成功（状态码 %d）", resp.StatusCode),
		Latency:  time.Since(start),
	}
}

// ValidateIPPort 验证 IP 和端口格式
func ValidateIPPort(ip string, port int) error {
	if net.ParseIP(ip) == nil {
		return fmt.Errorf("无效的 IP 地址: %s", ip)
	}
	if port <= 0 || port > 65535 {
		return fmt.Errorf("无效的端口号: %d（应为 1-65535）", port)
	}
	return nil
}

// isValidURLPath 验证 URL 路径安全性（防止 SSRF）
var validPathRegex = regexp.MustCompile(`^/[a-zA-Z0-9/_.\-~]*$`)

func isValidURLPath(path string) bool {
	if path == "" {
		return true
	}
	return validPathRegex.MatchString(path)
}
