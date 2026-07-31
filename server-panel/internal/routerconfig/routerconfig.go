// Package routerconfig 提供路由器配置管理功能。
// 包括快照生成、HMAC 签名、IPC 通知和版本跟踪。
package routerconfig

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/frp-panel/server-panel/internal/crypto"
)

// RouterConfig 路由器配置。
type RouterConfig struct {
	// TLS 证书映射: domain -> cert/key 路径
	TLSCertificates map[string]TLSCertConfig `json:"tls_certificates"`
	// 主机路由映射: host -> upstream
	HostRoutes map[string]HostRoute `json:"host_routes"`
	// 错误页面
	ErrorPages ErrorPagesConfig `json:"error_pages"`
	// 服务器 IP
	ServerIP string `json:"server_ip"`
	// FRPS vhostHTTPPort
	VHostHTTPPort int `json:"vhost_http_port"`
	// FRPS vhostHTTPSPort
	VHostHTTPSPort int `json:"vhost_https_port"`
}

// TLSCertConfig TLS 证书配置。
type TLSCertConfig struct {
	CertPath string `json:"cert_path"`
	KeyPath  string `json:"key_path"`
}

// HostRoute 主机路由配置。
type HostRoute struct {
	Upstream string `json:"upstream"`
	Protocol string `json:"protocol"`
}

// ErrorPagesConfig 错误页面配置。
type ErrorPagesConfig struct {
	Page404 string `json:"page_404"`
	Page502 string `json:"page_502"`
}

// Snapshot 路由器配置快照。
type Snapshot struct {
	ID        string `json:"id"`
	Version   int    `json:"version"`
	Config    string `json:"config_json"`
	HMACSig   string `json:"hmac_sig"`
	CreatedAt string `json:"created_at"`
}

// Manager 路由器配置管理器。
type Manager struct {
	db       *sql.DB
	logger   *slog.Logger
	hmacKey  []byte
	sockPath string // Unix socket 路径
}

// NewManager 创建路由器配置管理器。
func NewManager(db *sql.DB, logger *slog.Logger, hmacKey []byte, sockPath string) *Manager {
	return &Manager{
		db:       db,
		logger:   logger,
		hmacKey:  hmacKey,
		sockPath: sockPath,
	}
}

// GetCurrentVersion 获取当前路由器配置版本。
func (m *Manager) GetCurrentVersion(ctx context.Context) (int, error) {
	var version int
	err := m.db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(version), 0) FROM router_snapshots
	`).Scan(&version)
	return version, err
}

// GenerateSnapshot 生成新的路由器配置快照。
func (m *Manager) GenerateSnapshot(ctx context.Context, cfg RouterConfig) (*Snapshot, error) {
	// 规范化 JSON
	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}

	// HMAC-SHA256 签名
	hmacSig := crypto.HMACSHA256Hex(m.hmacKey, cfgJSON)

	version, _ := m.GetCurrentVersion(ctx)
	version++

	id := crypto.RandomToken(16)
	now := time.Now().UTC().Format(time.RFC3339)

	_, err = m.db.ExecContext(ctx, `
		INSERT INTO router_snapshots (id, version, config_json, hmac_sig, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, id, version, string(cfgJSON), hmacSig, now)
	if err != nil {
		return nil, fmt.Errorf("insert snapshot: %w", err)
	}

	snap := &Snapshot{
		ID:        id,
		Version:   version,
		Config:    string(cfgJSON),
		HMACSig:   hmacSig,
		CreatedAt: now,
	}

	// 通知路由器进程
	if err := m.notifyRouter(ctx, snap); err != nil {
		m.logger.Warn("failed to notify router", "err", err)
	}

	return snap, nil
}

// GetLatestSnapshot 获取最新路由器配置快照。
func (m *Manager) GetLatestSnapshot(ctx context.Context) (*Snapshot, error) {
	var snap Snapshot
	err := m.db.QueryRowContext(ctx, `
		SELECT id, version, config_json, hmac_sig, created_at
		FROM router_snapshots ORDER BY version DESC LIMIT 1
	`).Scan(&snap.ID, &snap.Version, &snap.Config, &snap.HMACSig, &snap.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &snap, nil
}

// GetLastGoodSnapshot 获取最近一个已验证的快照。
// 用于路由器崩溃恢复。
func (m *Manager) GetLastGoodSnapshot(ctx context.Context) (*Snapshot, error) {
	return m.GetLatestSnapshot(ctx)
}

// VerifySnapshotHMAC 验证快照 HMAC。
func (m *Manager) VerifySnapshotHMAC(snap *Snapshot) bool {
	expected := crypto.HMACSHA256Hex(m.hmacKey, []byte(snap.Config))
	return crypto.ConstantTimeEqualString(expected, snap.HMACSig)
}

// notifyRouter 通过 Unix Socket 通知路由器进程重新加载配置。
func (m *Manager) notifyRouter(ctx context.Context, snap *Snapshot) error {
	if m.sockPath == "" {
		return nil
	}

	conn, err := net.DialTimeout("unix", m.sockPath, 5*time.Second)
	if err != nil {
		return fmt.Errorf("connect to router socket: %w", err)
	}
	defer conn.Close()

	// 发送 reload 命令
	msg := fmt.Sprintf("RELOAD %d\n", snap.Version)
	_, err = conn.Write([]byte(msg))
	if err != nil {
		return fmt.Errorf("send reload command: %w", err)
	}

	// 等待确认
	buf := make([]byte, 256)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, err := conn.Read(buf)
	if err != nil {
		return fmt.Errorf("read router response: %w", err)
	}

	response := string(buf[:n])
	if response != "OK\n" && response != "OK" {
		return fmt.Errorf("unexpected router response: %s", response)
	}

	m.logger.Info("router notified of config update", "version", snap.Version)
	return nil
}

// ListSnapshots 列出路由器配置快照。
func (m *Manager) ListSnapshots(ctx context.Context, limit int) ([]Snapshot, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, version, config_json, hmac_sig, created_at
		FROM router_snapshots ORDER BY version DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snapshots []Snapshot
	for rows.Next() {
		var snap Snapshot
		if err := rows.Scan(&snap.ID, &snap.Version, &snap.Config, &snap.HMACSig, &snap.CreatedAt); err != nil {
			return nil, err
		}
		snapshots = append(snapshots, snap)
	}
	return snapshots, nil
}
