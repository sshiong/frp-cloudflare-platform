// Package signing 提供 Ed25519 配置签名功能。
// 用于对 FRP 配置快照进行签名和验证，防止配置篡改。
package signing

import (
	"crypto/ed25519"
	"database/sql"
	"encoding/base64"
	"fmt"
	"log/slog"

	"github.com/frp-panel/server-panel/internal/crypto"
)

// KeyPair 签名密钥对。
type KeyPair struct {
	ID         string `json:"id"`
	PublicKey  string `json:"public_key"`  // base64
	PrivateKey string `json:"private_key"` // AES-256-GCM 加密后的 base64
	Label      string `json:"label"`
	Active     bool   `json:"active"`
	CreatedAt  string `json:"created_at"`
}

// Signer 配置签名器。
type Signer struct {
	db      *sql.DB
	logger  *slog.Logger
	encKey  []byte // AES-256 密钥，用于加密/解密私钥
}

// NewSigner 创建签名器。encKey 必须是 32 字节。
func NewSigner(db *sql.DB, logger *slog.Logger, encKey []byte) *Signer {
	return &Signer{db: db, logger: logger, encKey: encKey}
}

// GenerateKeyPair 生成新的 Ed25519 密钥对并存储。
func (s *Signer) GenerateKeyPair(label string) (*KeyPair, error) {
	pub, priv, err := crypto.GenerateEd25519KeyPair()
	if err != nil {
		return nil, fmt.Errorf("generate key pair: %w", err)
	}

	// 加密私钥
	encPriv, err := crypto.EncryptAES256GCM(s.encKey, priv)
	if err != nil {
		return nil, fmt.Errorf("encrypt private key: %w", err)
	}

	id := crypto.RandomToken(16)
	pubB64 := base64.StdEncoding.EncodeToString(pub)
	privB64 := base64.StdEncoding.EncodeToString(encPriv)

	// 将现有活跃密钥标记为非活跃
	_, _ = s.db.Exec("UPDATE signing_keys SET active = 0 WHERE active = 1")

	_, err = s.db.Exec(`
		INSERT INTO signing_keys (id, public_key, private_key, label, active)
		VALUES (?, ?, ?, ?, 1)
	`, id, pubB64, privB64, label)
	if err != nil {
		return nil, fmt.Errorf("store key pair: %w", err)
	}

	return &KeyPair{
		ID:        id,
		PublicKey: pubB64,
		Label:     label,
		Active:    true,
	}, nil
}

// Sign 使用当前活跃的私钥对数据签名。
func (s *Signer) Sign(data []byte) ([]byte, error) {
	priv, err := s.getActivePrivateKey()
	if err != nil {
		return nil, fmt.Errorf("get active private key: %w", err)
	}
	return crypto.SignEd25519(priv, data), nil
}

// SignBase64 使用当前活跃的私钥对数据签名，返回 base64 编码。
func (s *Signer) SignBase64(data []byte) (string, error) {
	sig, err := s.Sign(data)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

// Verify 使用指定公钥验证签名。
func (s *Signer) Verify(pubKeyB64 string, data, sig []byte) (bool, error) {
	pub, err := base64.StdEncoding.DecodeString(pubKeyB64)
	if err != nil {
		return false, fmt.Errorf("decode public key: %w", err)
	}
	if len(pub) != ed25519.PublicKeySize {
		return false, fmt.Errorf("invalid public key size: %d", len(pub))
	}
	return crypto.VerifyEd25519(pub, data, sig), nil
}

// VerifyBase64 验证 base64 编码的签名。
func (s *Signer) VerifyBase64(pubKeyB64 string, data []byte, sigB64 string) (bool, error) {
	sig, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		return false, fmt.Errorf("decode signature: %w", err)
	}
	return s.Verify(pubKeyB64, data, sig)
}

// GetActiveKeyPair 获取当前活跃的密钥对。
func (s *Signer) GetActiveKeyPair() (*KeyPair, error) {
	var kp KeyPair
	err := s.db.QueryRow(`
		SELECT id, public_key, private_key, label, active, created_at
		FROM signing_keys WHERE active = 1 LIMIT 1
	`).Scan(&kp.ID, &kp.PublicKey, &kp.PrivateKey, &kp.Label, &kp.Active, &kp.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &kp, nil
}

// RotateKey 轮换签名密钥。
func (s *Signer) RotateKey(label string) (*KeyPair, error) {
	return s.GenerateKeyPair(label)
}

// ListKeyPairs 列出所有密钥对。
func (s *Signer) ListKeyPairs() ([]KeyPair, error) {
	rows, err := s.db.Query(`
		SELECT id, public_key, label, active, created_at
		FROM signing_keys ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []KeyPair
	for rows.Next() {
		var kp KeyPair
		if err := rows.Scan(&kp.ID, &kp.PublicKey, &kp.Label, &kp.Active, &kp.CreatedAt); err != nil {
			return nil, err
		}
		keys = append(keys, kp)
	}
	return keys, nil
}

// getActivePrivateKey 获取并解密当前活跃的私钥。
func (s *Signer) getActivePrivateKey() (ed25519.PrivateKey, error) {
	var encPrivB64 string
	err := s.db.QueryRow(`
		SELECT private_key FROM signing_keys WHERE active = 1 LIMIT 1
	`).Scan(&encPrivB64)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no active signing key found")
	}
	if err != nil {
		return nil, fmt.Errorf("query active key: %w", err)
	}

	encPriv, err := base64.StdEncoding.DecodeString(encPrivB64)
	if err != nil {
		return nil, fmt.Errorf("decode encrypted private key: %w", err)
	}

	priv, err := crypto.DecryptAES256GCM(s.encKey, encPriv)
	if err != nil {
		return nil, fmt.Errorf("decrypt private key: %w", err)
	}

	return ed25519.PrivateKey(priv), nil
}
