package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/frp-panel/client-panel/internal/hmacsigner"
)

// Event WebSocket 事件类型
type Event struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// 服务端推送事件类型
const (
	EventConfigUpdate   = "config_update"
	EventTokenChange    = "token_change"
	EventUserDisabled   = "user_disabled"
	EventDeviceRevoked  = "device_revoked"
	EventForceSync      = "force_sync"
	EventMappingUpdate  = "mapping_update"
	EventPing           = "ping"
)

// 重连退避参数
var reconnectDelays = []time.Duration{
	1 * time.Second,
	2 * time.Second,
	4 * time.Second,
	8 * time.Second,
	16 * time.Second,
	30 * time.Second,
	60 * time.Second,
}

// EventHandler 事件处理函数
type EventHandler func(event Event)

// Client WebSocket 客户端
// HMAC 认证的 WebSocket 连接到服务端
// 支持指数退避重连和全量同步
type Client struct {
	serverURL   string
	signer      *hmacsigner.Signer
	conn        *websocket.Conn
	logger      *slog.Logger
	handler     EventHandler
	mu          sync.Mutex
	ctx         context.Context
	cancel      context.CancelFunc
	isConnected bool
	reconnectIdx int
	stopCh      chan struct{}
}

// NewClient 创建 WebSocket 客户端
func NewClient(serverURL string, signer *hmacsigner.Signer, handler EventHandler, logger *slog.Logger) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		serverURL: serverURL,
		signer:    signer,
		handler:   handler,
		logger:    logger,
		ctx:       ctx,
		cancel:    cancel,
		stopCh:    make(chan struct{}),
	}
}

// Connect 建立 WebSocket 连接
func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.isConnected {
		return nil
	}

	// 构造 WebSocket URL
	u, err := url.Parse(c.serverURL)
	if err != nil {
		return fmt.Errorf("解析服务端 URL 失败: %w", err)
	}

	// 切换到 ws/wss 协议
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	case "http":
		u.Scheme = "ws"
	}
	u.Path = "/api/v1/client/ws"

	// 创建带 HMAC 签名的请求头
	header := http.Header{}
	if c.signer != nil {
		signResult, err := c.signer.SignRequest(&http.Request{
			Method: "GET",
			URL:    u,
		}, nil)
		if err != nil {
			return fmt.Errorf("签名失败: %w", err)
		}
		header.Set("X-Client-ID", signResult.ClientID)
		header.Set("X-Device-Token-Version", signResult.TokenVersion)
		header.Set("X-Request-Timestamp", signResult.Timestamp)
		header.Set("X-Request-Nonce", signResult.Nonce)
		header.Set("X-Content-SHA256", signResult.BodySHA256)
		header.Set("Authorization", signResult.Authorization)
	}

	// 建立连接
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), header)
	if err != nil {
		return fmt.Errorf("WebSocket 连接失败: %w", err)
	}

	c.conn = conn
	c.isConnected = true
	c.reconnectIdx = 0

	c.logger.Info("WebSocket 已连接", "url", u.String())

	// 启动读取和重连协程
	go c.readPump()
	go c.keepAlive()

	return nil
}

// Disconnect 断开连接
func (c *Client) Disconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case <-c.stopCh:
		// 已停止
	default:
		close(c.stopCh)
	}

	c.cancel()

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.isConnected = false

	c.logger.Info("WebSocket 已断开")
}

// IsConnected 检查连接状态
func (c *Client) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.isConnected
}

// readPump 持续读取消息
func (c *Client) readPump() {
	defer func() {
		c.mu.Lock()
		c.isConnected = false
		c.mu.Unlock()
		c.scheduleReconnect()
	}()

	c.conn.SetReadLimit(1 << 20) // 1MB
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				c.logger.Warn("WebSocket 异常关闭", "error", err)
			}
			return
		}

		// 解析事件
		var event Event
		if err := json.Unmarshal(message, &event); err != nil {
			c.logger.Warn("解析 WebSocket 消息失败", "error", err)
			continue
		}

		// 处理 ping
		if event.Type == EventPing {
			c.handlePing()
			continue
		}

		c.logger.Debug("收到 WebSocket 事件", "type", event.Type)

		// 调用事件处理器
		if c.handler != nil {
			c.handler(event)
		}
	}
}

// keepAlive 保持连接活跃（发送 ping）
func (c *Client) keepAlive() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.mu.Lock()
			if c.conn != nil {
				c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					c.logger.Warn("发送 ping 失败", "error", err)
					c.mu.Unlock()
					return
				}
			}
			c.mu.Unlock()
		}
	}
}

// handlePing 处理服务端 ping
func (c *Client) handlePing() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		pong := Event{Type: "pong"}
		data, _ := json.Marshal(pong)
		c.conn.WriteMessage(websocket.TextMessage, data)
	}
}

// scheduleReconnect 调度重连
func (c *Client) scheduleReconnect() {
	select {
	case <-c.stopCh:
		return
	case <-c.ctx.Done():
		return
	default:
	}

	// 指数退避 + 随机抖动
	delay := reconnectDelays[c.reconnectIdx]
	if c.reconnectIdx < len(reconnectDelays)-1 {
		c.reconnectIdx++
	}

	// 添加 ±20% 随机抖动
	jitter := time.Duration(rand.Float64()*0.4-0.2) * delay
	delay = delay + jitter

	c.logger.Info("计划重连", "delay", delay.String(), "attempt", c.reconnectIdx)

	select {
	case <-c.stopCh:
		return
	case <-c.ctx.Done():
		return
	case <-time.After(delay):
	}

	if err := c.Connect(); err != nil {
		c.logger.Error("重连失败", "error", err)
		c.scheduleReconnect()
		return
	}

	c.logger.Info("重连成功")
}

// Send 发送消息到服务端
func (c *Client) Send(event Event) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.isConnected || c.conn == nil {
		return fmt.Errorf("WebSocket 未连接")
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("序列化事件失败: %w", err)
	}

	c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return c.conn.WriteMessage(websocket.TextMessage, data)
}

// 退避策略计算
func calculateBackoff(attempt int) time.Duration {
	if attempt >= len(reconnectDelays) {
		return reconnectDelays[len(reconnectDelays)-1]
	}
	return reconnectDelays[attempt]
}

// 指数退避（带最大限制）
func exponentialBackoff(base time.Duration, attempt int, max time.Duration) time.Duration {
	delay := base * time.Duration(math.Pow(2, float64(attempt)))
	if delay > max {
		delay = max
	}
	return delay
}
