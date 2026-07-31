// Package configsync 提供 FRP 配置同步功能。
// 负责配置版本管理、快照生成、Ed25519 签名和配置规范化。
package configsync

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/frp-panel/server-panel/internal/crypto"
	"github.com/frp-panel/server-panel/internal/signing"
)

// Syncer 配置同步器。
type Syncer struct {
	db       *sql.DB
	logger   *slog.Logger
	signer   *signing.Signer
}

// NewSyncer 创建配置同步器。
func NewSyncer(db *sql.DB, logger *slog.Logger, signer *signing.Signer) *Syncer {
	return &Syncer{db: db, logger: logger, signer: signer}
}

// FRPConfig FRP 客户端配置结构。
type FRPConfig struct {
	ServerAddr string              `json:"server_addr"`
	ServerPort int                 `json:"server_port"`
	Token      string              `json:"token,omitempty"`
	Proxies    []FRPProxyConfig    `json:"proxies"`
}

// FRPProxyConfig FRP 代理配置。
type FRPProxyConfig struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	LocalIP    string `json:"local_ip"`
	LocalPort  int    `json:"local_port"`
	RemotePort int    `json:"remote_port,omitempty"`
	CustomDomain string `json:"custom_domain,omitempty"`
}

// Snapshot 配置快照。
type Snapshot struct {
	ID        string `json:"id"`
	Version   int    `json:"version"`
	Config    string `json:"config_json"`
	Hash      string `json:"hash"`
	Signature string `json:"signature"`
	CreatedAt string `json:"created_at"`
}

// GetCurrentVersion 获取当前配置版本号。
func (s *Syncer) GetCurrentVersion(ctx context.Context) (int, error) {
	var version int
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(version), 0) FROM config_snapshots
	`).Scan(&version)
	return version, err
}

// GenerateSnapshot 生成新的配置快照。
func (s *Syncer) GenerateSnapshot(ctx context.Context, cfg FRPConfig) (*Snapshot, error) {
	// 规范化配置 JSON
	canonical, err := Canonicalize(cfg)
	if err != nil {
		return nil, fmt.Errorf("canonicalize config: %w", err)
	}

	hash := crypto.SHA256Hex(canonical)
	version, _ := s.GetCurrentVersion(ctx)
	version++

	// Ed25519 签名
	sig, err := s.signer.SignBase64(canonical)
	if err != nil {
		s.logger.Warn("failed to sign config, storing without signature", "err", err)
	}

	id := crypto.RandomToken(16)
	now := time.Now().UTC().Format(time.RFC3339)

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO config_snapshots (id, version, config_json, hash, signature, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, id, version, string(canonical), hash, sig, now)
	if err != nil {
		return nil, fmt.Errorf("insert snapshot: %w", err)
	}

	return &Snapshot{
		ID:        id,
		Version:   version,
		Config:    string(canonical),
		Hash:      hash,
		Signature: sig,
		CreatedAt: now,
	}, nil
}

// GetLatestSnapshot 获取最新配置快照。
func (s *Syncer) GetLatestSnapshot(ctx context.Context) (*Snapshot, error) {
	var snap Snapshot
	err := s.db.QueryRowContext(ctx, `
		SELECT id, version, config_json, hash, signature, created_at
		FROM config_snapshots ORDER BY version DESC LIMIT 1
	`).Scan(&snap.ID, &snap.Version, &snap.Config, &snap.Hash, &snap.Signature, &snap.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &snap, nil
}

// GetSnapshotByVersion 获取指定版本的配置快照。
func (s *Syncer) GetSnapshotByVersion(ctx context.Context, version int) (*Snapshot, error) {
	var snap Snapshot
	err := s.db.QueryRowContext(ctx, `
		SELECT id, version, config_json, hash, signature, created_at
		FROM config_snapshots WHERE version = ?
	`, version).Scan(&snap.ID, &snap.Version, &snap.Config, &snap.Hash, &snap.Signature, &snap.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &snap, nil
}

// VerifySnapshot 验证快照签名。
func (s *Syncer) VerifySnapshot(ctx context.Context, snap *Snapshot) (bool, error) {
	if snap.Signature == "" {
		return false, nil
	}
	kp, err := s.signer.GetActiveKeyPair()
	if err != nil || kp == nil {
		return false, fmt.Errorf("no active signing key")
	}
	return s.signer.VerifyBase64(kp.PublicKey, []byte(snap.Config), snap.Signature)
}

// ListSnapshots 列出配置快照。
func (s *Syncer) ListSnapshots(ctx context.Context, limit, offset int) ([]Snapshot, int, error) {
	if limit <= 0 {
		limit = 20
	}
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM config_snapshots").Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, version, config_json, hash, signature, created_at
		FROM config_snapshots ORDER BY version DESC LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var snapshots []Snapshot
	for rows.Next() {
		var snap Snapshot
		if err := rows.Scan(&snap.ID, &snap.Version, &snap.Config, &snap.Hash, &snap.Signature, &snap.CreatedAt); err != nil {
			return nil, 0, err
		}
		snapshots = append(snapshots, snap)
	}
	return snapshots, total, nil
}

// Canonicalize 将配置规范化为确定性 JSON。
// 保证相同内容始终产生相同的 JSON（排序键）。
func Canonicalize(v interface{}) ([]byte, error) {
	// 先序列化为通用结构
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	// 解码再重新编码（保证键排序）
	var generic interface{}
	if err := json.Unmarshal(data, &generic); err != nil {
		return nil, err
	}

	// 递归排序 map 键
	sorted := sortKeys(generic)

	result, err := json.Marshal(sorted)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// sortKeys 递归排序 map 的键。
func sortKeys(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		sorted := make(map[string]interface{}, len(val))
		for k, sv := range val {
			sorted[k] = sortKeys(sv)
		}
		return sorted
	case []interface{}:
		for i, sv := range val {
			val[i] = sortKeys(sv)
		}
		return val
	default:
		return v
	}
}
