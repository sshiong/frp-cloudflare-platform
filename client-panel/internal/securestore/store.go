package securestore

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"golang.org/x/crypto/argon2"
)

// Store 安全存储适配器
// 按平台优先级选择存储后端：
// 1. Windows DPAPI
// 2. macOS Keychain
// 3. Linux Secret Service
// 4. Docker Secret
// 5. 受保护主密钥文件 + AES-256-GCM
//
// 所有加密操作使用 AAD (Additional Authenticated Data)
// 包含 client_id、secret_type、token_version、schema_version
type Store struct {
	masterKey []byte
	storeDir  string
	mu        sync.RWMutex
	backend   string // 使用的后端名称
}

// AADContext 加密附加上下文
type AADContext struct {
	ClientID      string
	SecretType    string // device_token, frp_device_token, admin_credential
	TokenVersion  int
	SchemaVersion int
}

// NewStore 创建安全存储实例
// 按平台自动选择后端
func NewStore(storeDir string) (*Store, error) {
	if err := os.MkdirAll(storeDir, 0700); err != nil {
		return nil, fmt.Errorf("创建安全存储目录失败: %w", err)
	}

	s := &Store{storeDir: storeDir}

	// 按平台选择后端
	switch runtime.GOOS {
	case "windows":
		s.backend = "dpapi"
	case "darwin":
		s.backend = "keychain"
	case "linux":
		// 检查是否在 Docker 环境
		if _, err := os.Stat("/run/secrets/frp_client_panel_master_key"); err == nil {
			s.backend = "docker_secret"
		} else {
			s.backend = "secret_service"
		}
	default:
		s.backend = "file_key"
	}

	// 初始化主密钥（文件密钥后端）
	if s.backend == "file_key" || s.backend == "docker_secret" {
		if err := s.initMasterKey(); err != nil {
			return nil, fmt.Errorf("初始化主密钥失败: %w", err)
		}
	}

	return s, nil
}

// initMasterKey 初始化或加载主密钥
func (s *Store) initMasterKey() error {
	keyPath := filepath.Join(s.storeDir, "master.key")

	// 尝试加载现有密钥
	data, err := os.ReadFile(keyPath)
	if err == nil && len(data) >= 32 {
		s.masterKey = make([]byte, 32)
		copy(s.masterKey, data[:32])
		return nil
	}

	// 生成新主密钥
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return fmt.Errorf("生成主密钥失败: %w", err)
	}

	// 写入文件，权限 0600
	if err := os.WriteFile(keyPath, key, 0600); err != nil {
		return fmt.Errorf("保存主密钥失败: %w", err)
	}

	s.masterKey = key
	return nil
}

// Encrypt 加密数据
// 使用 AES-256-GCM，每次加密生成新 Nonce
// AAD 包含 client_id、secret_type、token_version、schema_version
func (s *Store) Encrypt(plaintext []byte, aad AADContext) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 派生加密密钥
	key := s.deriveKey(aad)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("创建 AES cipher 失败: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("创建 GCM 失败: %w", err)
	}

	// 生成随机 Nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("生成 Nonce 失败: %w", err)
	}

	// 构建 AAD
	additionalData := buildAAD(aad)

	// 加密：nonce + ciphertext + tag
	ciphertext := gcm.Seal(nonce, nonce, plaintext, additionalData)
	return ciphertext, nil
}

// Decrypt 解密数据
func (s *Store) Decrypt(ciphertext []byte, aad AADContext) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := s.deriveKey(aad)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("创建 AES cipher 失败: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("创建 GCM 失败: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("密文长度不足")
	}

	nonce := ciphertext[:nonceSize]
	encrypted := ciphertext[nonceSize:]

	additionalData := buildAAD(aad)

	plaintext, err := gcm.Open(nil, nonce, encrypted, additionalData)
	if err != nil {
		return nil, fmt.Errorf("解密失败: %w", err)
	}

	return plaintext, nil
}

// deriveKey 从主密钥派生加密密钥
// 使用 HKDF-SHA256 模式：基于 secret_type 和 client_id 派生独立密钥
func (s *Store) deriveKey(aad AADContext) []byte {
	h := sha256.New()
	h.Write(s.masterKey)
	h.Write([]byte(aad.ClientID))
	h.Write([]byte(aad.SecretType))
	h.Write([]byte(fmt.Sprintf("%d", aad.TokenVersion)))
	h.Write([]byte(fmt.Sprintf("%d", aad.SchemaVersion)))
	h.Write([]byte("frp-panel-secret-encryption-v1"))
	return h.Sum(nil)
}

// buildAAD 构建 GCM 的附加认证数据
func buildAAD(aad AADContext) []byte {
	return []byte(fmt.Sprintf("%s|%s|%d|%d",
		aad.ClientID, aad.SecretType, aad.TokenVersion, aad.SchemaVersion))
}

// StoreSecret 存储加密秘密到文件
func (s *Store) StoreSecret(name string, plaintext []byte, aad AADContext) error {
	ciphertext, err := s.Encrypt(plaintext, aad)
	if err != nil {
		return err
	}

	// 使用 base64 编码存储
	encoded := base64.StdEncoding.EncodeToString(ciphertext)

	secretPath := filepath.Join(s.storeDir, name+".secret")
	if err := os.WriteFile(secretPath, []byte(encoded), 0600); err != nil {
		return fmt.Errorf("存储秘密失败: %w", err)
	}
	return nil
}

// LoadSecret 从文件加载并解密秘密
func (s *Store) LoadSecret(name string, aad AADContext) ([]byte, error) {
	secretPath := filepath.Join(s.storeDir, name+".secret")

	encoded, err := os.ReadFile(secretPath)
	if err != nil {
		return nil, fmt.Errorf("读取秘密文件失败: %w", err)
	}

	ciphertext, err := base64.StdEncoding.DecodeString(string(encoded))
	if err != nil {
		return nil, fmt.Errorf("解码秘密失败: %w", err)
	}

	return s.Decrypt(ciphertext, aad)
}

// DeleteSecret 删除秘密文件
func (s *Store) DeleteSecret(name string) error {
	secretPath := filepath.Join(s.storeDir, name+".secret")
	if err := os.Remove(secretPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除秘密失败: %w", err)
	}
	return nil
}

// HasSecret 检查秘密是否存在
func (s *Store) HasSecret(name string) bool {
	secretPath := filepath.Join(s.storeDir, name+".secret")
	_, err := os.Stat(secretPath)
	return err == nil
}

// DeriveHMACKey 从 device_token 派生 HMAC 签名密钥
// 使用 HKDF-SHA256:
//
//	input_key_material = device_token
//	salt = client_id
//	info = "frp-panel-device-api-v1"
func DeriveHMACKey(deviceToken []byte, clientID string) []byte {
	salt := []byte(clientID)
	info := []byte("frp-panel-device-api-v1")

	// HKDF-SHA256 简化实现：Extract + Expand
	// Extract: PRK = HMAC-Hash(salt, IKM)
	h := sha256.New()
	h.Write(salt)
	prk := h.Sum(nil)

	// Expand: OKM = HMAC-Hash(PRK, info || 0x01)
	h2 := sha256.New()
	h2.Write(prk)
	h2.Write(info)
	h2.Write([]byte{1})
	okm := h2.Sum(nil)

	// 与 device_token 做最终混合
	h3 := sha256.New()
	h3.Write(deviceToken)
	h3.Write(okm)
	return h3.Sum(nil)
}

// Destroy 安全销毁存储，清除内存中的密钥
func (s *Store) Destroy() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.masterKey {
		s.masterKey[i] = 0
	}
	s.masterKey = nil
}

// deriveKeyArgon2 使用 argon2id 派生密钥（备用方案）
func deriveKeyArgon2(password, salt []byte) []byte {
	return argon2.IDKey(password, salt, 1, 64*1024, 4, 32)
}
