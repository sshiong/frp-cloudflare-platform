package activesession

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Session 活动浏览器代理会话
// 每个 client_id 同一时间最多一个有效会话
// 仅存在于内存，不持久化
// Client 重启后会话清空，但设备后台同步继续
type Session struct {
	LocalProxySessionID string
	CookieSecretHash    string // SHA-256(cookie_secret)，用于验证
	ServerSessionToken  string // 远程 Server Session Token（内存中）
	ServerCSRFState     string // Server 端 CSRF 状态
	ServerSessionID     string
	UserID              string
	ClientID            string
	SourceIP            string
	UserAgent           string
	CreatedAt           time.Time
	LastSeenAt          time.Time
	ExpiresAt           time.Time
	SessionGeneration   int64
}

// Manager 活动会话管理器
// 保证每个 client_id 同一时间最多一个有效会话
type Manager struct {
	mu             sync.Mutex
	activeSession  *Session
	clientID       string
	sessionGen     int64
	logger         *slog.Logger
}

// NewManager 创建会话管理器
func NewManager(clientID string, logger *slog.Logger) *Manager {
	return &Manager{
		clientID: clientID,
		logger:   logger,
	}
}

// CreateSession 创建新的活动会话
// 如果已有旧会话，执行抢占流程：
// 1. 原子抢占锁
// 2. session_generation + 1
// 3. 生成新 Cookie Secret
// 4. 创建新会话
// 返回新会话和旧会话（如果有）
func (m *Manager) CreateSession(
	serverSessionToken string,
	serverSessionID string,
	serverCSRFState string,
	userID string,
	clientID string,
	sourceIP string,
	userAgent string,
) (newSession *Session, cookieSecret string, oldSession *Session, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查并保存旧会话
	oldSession = m.activeSession

	// 递增会话代际
	m.sessionGen++

	// 生成 256-bit 随机 Cookie Secret
	cookieSecretBytes := make([]byte, 32)
	if _, err := rand.Read(cookieSecretBytes); err != nil {
		return nil, "", nil, fmt.Errorf("生成 Cookie Secret 失败: %w", err)
	}
	cookieSecret = hex.EncodeToString(cookieSecretBytes)

	// 计算 Cookie Secret Hash（存储用于验证）
	hash := sha256.Sum256([]byte(cookieSecret))
	cookieSecretHash := hex.EncodeToString(hash[:])

	// 生成本地会话 ID
	sessionIDBytes := make([]byte, 16)
	if _, err := rand.Read(sessionIDBytes); err != nil {
		return nil, "", nil, fmt.Errorf("生成会话 ID 失败: %w", err)
	}

	now := time.Now()
	newSession = &Session{
		LocalProxySessionID: hex.EncodeToString(sessionIDBytes),
		CookieSecretHash:    cookieSecretHash,
		ServerSessionToken:  serverSessionToken,
		ServerCSRFState:     serverCSRFState,
		ServerSessionID:     serverSessionID,
		UserID:              userID,
		ClientID:            clientID,
		SourceIP:            sourceIP,
		UserAgent:           userAgent,
		CreatedAt:           now,
		LastSeenAt:          now,
		ExpiresAt:           now.Add(24 * time.Hour), // 24 小时过期
		SessionGeneration:   m.sessionGen,
	}

	m.activeSession = newSession

	m.logger.Info("新活动会话已创建",
		"session_id", newSession.LocalProxySessionID,
		"generation", m.sessionGen,
		"source_ip", sourceIP,
		"had_old_session", oldSession != nil,
	)

	return newSession, cookieSecret, oldSession, nil
}

// ValidateSession 验证请求的会话有效性
// 检查：
// 1. 是否有活动会话
// 2. Cookie Secret 是否匹配
// 3. 会话代际是否匹配
// 4. 会话是否过期
// 返回会话对象和验证结果
func (m *Manager) ValidateSession(cookieSecret string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.activeSession == nil {
		return nil, fmt.Errorf("NO_SESSION: 无活动会话")
	}

	// 检查过期
	if time.Now().After(m.activeSession.ExpiresAt) {
		m.activeSession = nil
		return nil, fmt.Errorf("SESSION_EXPIRED: 会话已过期")
	}

	// 验证 Cookie Secret
	hash := sha256.Sum256([]byte(cookieSecret))
	cookieHash := hex.EncodeToString(hash[:])

	if subtle.ConstantTimeCompare([]byte(cookieHash), []byte(m.activeSession.CookieSecretHash)) != 1 {
		return nil, fmt.Errorf("INVALID_COOKIE: Cookie 验证失败")
	}

	// 更新最后访问时间
	m.activeSession.LastSeenAt = time.Now()

	return m.activeSession, nil
}

// RevokeSession 撤销当前活动会话
func (m *Manager) RevokeSession() *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	old := m.activeSession
	m.activeSession = nil

	if old != nil {
		m.logger.Info("活动会话已撤销",
			"session_id", old.LocalProxySessionID,
			"generation", old.SessionGeneration,
		)
	}

	return old
}

// GetSession 获取当前活动会话（不验证 Cookie）
func (m *Manager) GetSession() *Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.activeSession
}

// GetGeneration 获取当前会话代际
func (m *Manager) GetGeneration() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sessionGen
}

// IsActive 检查是否有活动会话
func (m *Manager) IsActive() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.activeSession != nil && time.Now().Before(m.activeSession.ExpiresAt)
}

// UpdateServerSession 更新远程 Server Session 信息
// 用于 Server Session 刷新后的同步
func (m *Manager) UpdateServerSession(token, sessionID, csrfState string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.activeSession != nil {
		m.activeSession.ServerSessionToken = token
		m.activeSession.ServerSessionID = sessionID
		m.activeSession.ServerCSRFState = csrfState
	}
}

// Clear 清除所有会话状态
// 用于 Client 重启或设备解绑
func (m *Manager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.activeSession = nil
	m.sessionGen = 0
	m.logger.Info("会话管理器已清空")
}
