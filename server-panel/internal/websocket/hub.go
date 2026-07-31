// Package websocket 提供 WebSocket 连接管理和事件广播功能。
package websocket

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"nhooyr.io/websocket"
)

// Event WebSocket 事件。
type Event struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

// Client WebSocket 客户端。
type Client struct {
	ID     string
	UserID string
	SendCh chan []byte
	hub    *Hub
	conn   *websocket.Conn
}

// Hub WebSocket 连接管理中心。
type Hub struct {
	mu         sync.RWMutex
	clients    map[string]*Client   // clientID -> Client
	userClient map[string][]*Client // userID -> []*Client
	logger     *slog.Logger
	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte
	stopCh     chan struct{}
}

// NewHub 创建 WebSocket Hub。
func NewHub(logger *slog.Logger) *Hub {
	return &Hub{
		clients:    make(map[string]*Client),
		userClient: make(map[string][]*Client),
		logger:     logger,
		register:   make(chan *Client, 64),
		unregister: make(chan *Client, 64),
		broadcast:  make(chan []byte, 256),
		stopCh:     make(chan struct{}),
	}
}

// Start 启动 Hub 事件循环。
func (h *Hub) Start(ctx context.Context) {
	go h.loop(ctx)
}

// Stop 优雅停止 Hub。
func (h *Hub) Stop() {
	close(h.stopCh)

	h.mu.Lock()
	defer h.mu.Unlock()

	for _, client := range h.clients {
		close(client.SendCh)
	}
}

// Register 注册新客户端。
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister 注销客户端。
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

// BroadcastToUser 向指定用户的所有连接广播事件。
func (h *Hub) BroadcastToUser(userID string, event Event) {
	data, err := json.Marshal(event)
	if err != nil {
		h.logger.Error("marshal event failed", "err", err)
		return
	}

	h.mu.RLock()
	clients := h.userClient[userID]
	h.mu.RUnlock()

	for _, client := range clients {
		select {
		case client.SendCh <- data:
		default:
			// 客户端通道已满，跳过
		}
	}
}

// BroadcastAll 向所有连接广播事件。
func (h *Hub) BroadcastAll(event Event) {
	data, err := json.Marshal(event)
	if err != nil {
		h.logger.Error("marshal event failed", "err", err)
		return
	}
	h.broadcast <- data
}

// ClientCount 返回当前连接数。
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// loop 事件循环。
func (h *Hub) loop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-h.stopCh:
			return
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.ID] = client
			h.userClient[client.UserID] = append(h.userClient[client.UserID], client)
			h.mu.Unlock()
			h.logger.Info("ws client connected", "client_id", client.ID, "user_id", client.UserID)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.ID]; ok {
				delete(h.clients, client.ID)
				h.removeFromUserClients(client)
				close(client.SendCh)
			}
			h.mu.Unlock()
			h.logger.Info("ws client disconnected", "client_id", client.ID)

		case data := <-h.broadcast:
			h.mu.RLock()
			for _, client := range h.clients {
				select {
				case client.SendCh <- data:
				default:
				}
			}
			h.mu.RUnlock()
		}
	}
}

// removeFromUserClients 从用户客户端列表中移除客户端（需要持有写锁）。
func (h *Hub) removeFromUserClients(client *Client) {
	clients := h.userClient[client.UserID]
	for i, c := range clients {
		if c.ID == client.ID {
			h.userClient[client.UserID] = append(clients[:i], clients[i+1:]...)
			break
		}
	}
	if len(h.userClient[client.UserID]) == 0 {
		delete(h.userClient, client.UserID)
	}
}

// HandleConnection 处理 WebSocket 连接。
func (h *Hub) HandleConnection(conn *websocket.Conn, clientID, userID string) *Client {
	client := &Client{
		ID:     clientID,
		UserID: userID,
		SendCh: make(chan []byte, 64),
		hub:    h,
		conn:   conn,
	}

	h.Register(client)

	// 启动写协程
	go client.writePump()

	// 读协程（阻塞直到连接关闭）
	client.readPump()

	h.Unregister(client)
	return client
}

// readPump 读取客户端消息（主要检测连接关闭）。
func (c *Client) readPump() {
	ctx := context.Background()
	for {
		_, _, err := c.conn.Read(ctx)
		if err != nil {
			break
		}
	}
}

// writePump 将事件写入 WebSocket 连接。
func (c *Client) writePump() {
	ctx := context.Background()
	for {
		msg, ok := <-c.SendCh
		if !ok {
			// 通道已关闭
			c.conn.Close(websocket.StatusNormalClosure, "")
			return
		}

		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		err := c.conn.Write(ctx, websocket.MessageText, msg)
		cancel()
		if err != nil {
			return
		}
	}
}
