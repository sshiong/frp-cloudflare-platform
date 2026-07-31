// Package crypto 提供平台所需的全部密码学工具。
// 所有函数均为无状态的纯函数，不依赖数据库或外部存储。
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/hkdf"
)

// ---------------------------------------------------------------------------
// AES-256-GCM 加解密
// ---------------------------------------------------------------------------

// EncryptAES256GCM 使用 AES-256-GCM 加密明文。
// key 必须是 32 字节。返回 nonce || ciphertext || tag 的拼接。
func EncryptAES256GCM(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes.NewCipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cipher.NewGCM: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}
	// seal 格式: nonce || ciphertext+tag
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// DecryptAES256GCM 使用 AES-256-GCM 解密密文。
// 输入格式: nonce || ciphertext+tag。
func DecryptAES256GCM(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes.NewCipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cipher.NewGCM: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}
	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("gcm.Open: %w", err)
	}
	return plaintext, nil
}

// EncryptStringAES256GCM 加密字符串并返回 base64 编码结果。
func EncryptStringAES256GCM(key []byte, plaintext string) (string, error) {
	ct, err := EncryptAES256GCM(key, []byte(plaintext))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(ct), nil
}

// DecryptStringAES256GCM 解密 base64 编码的密文。
func DecryptStringAES256GCM(key []byte, b64ciphertext string) (string, error) {
	ct, err := base64.StdEncoding.DecodeString(b64ciphertext)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}
	pt, err := DecryptAES256GCM(key, ct)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}

// ---------------------------------------------------------------------------
// Ed25519 签名
// ---------------------------------------------------------------------------

// GenerateEd25519KeyPair 生成 Ed25519 密钥对。
func GenerateEd25519KeyPair() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("ed25519.GenerateKey: %w", err)
	}
	return pub, priv, nil
}

// SignEd25519 使用 Ed25519 私钥对消息签名。
func SignEd25519(priv ed25519.PrivateKey, message []byte) []byte {
	return ed25519.Sign(priv, message)
}

// VerifyEd25519 使用 Ed25519 公钥验证签名。
func VerifyEd25519(pub ed25519.PublicKey, message, sig []byte) bool {
	return ed25519.Verify(pub, message, sig)
}

// ---------------------------------------------------------------------------
// HMAC-SHA256
// ---------------------------------------------------------------------------

// HMACSHA256 计算 HMAC-SHA256。
func HMACSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

// HMACSHA256Hex 返回 HMAC-SHA256 的十六进制字符串。
func HMACSHA256Hex(key, data []byte) string {
	return hex.EncodeToString(HMACSHA256(key, data))
}

// VerifyHMACSHA256 常量时间比较 HMAC-SHA256。
func VerifyHMACSHA256(key, data, expectedMAC []byte) bool {
	mac := HMACSHA256(key, data)
	return subtle.ConstantTimeCompare(mac, expectedMAC) == 1
}

// ---------------------------------------------------------------------------
// HKDF-SHA256 密钥派生
// ---------------------------------------------------------------------------

// HKDFSHA256 使用 HKDF-SHA256 从 master key 派生子密钥。
func HKDFSHA256(masterKey, salt, info []byte, length int) ([]byte, error) {
	hkdfReader := hkdf.New(sha256.New, masterKey, salt, info)
	key := make([]byte, length)
	if _, err := io.ReadFull(hkdfReader, key); err != nil {
		return nil, fmt.Errorf("hkdf read: %w", err)
	}
	return key, nil
}

// ---------------------------------------------------------------------------
// 随机密码/Token 生成
// ---------------------------------------------------------------------------

const (
	// CharSetAlphaNum 包含大小写字母和数字，去掉了容易混淆的字符（0/O, 1/l/I）
	CharSetAlphaNum = "ABCDEFGHJKMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz23456789"
	// CharSetHex 是十六进制字符集
	CharSetHex = "0123456789abcdef"
)

// RandomPassword 生成指定长度的随机密码。
// 使用 crypto/rand 保证密码学安全性。
func RandomPassword(length int) string {
	return RandomFromCharset(length, CharSetAlphaNum)
}

// RandomToken 生成指定长度的随机 hex token。
func RandomToken(byteLen int) string {
	b := make([]byte, byteLen)
	_, _ = io.ReadFull(rand.Reader, b)
	return hex.EncodeToString(b)
}

// RandomFromCharset 从指定字符集中随机选取字符生成字符串。
func RandomFromCharset(length int, charset string) string {
	b := make([]byte, length)
	n := uint32(len(charset))
	for i := range b {
		var rb [4]byte
		_, _ = io.ReadFull(rand.Reader, rb[:])
		// 使用拒绝采样避免模偏差（对 256 和 charset 长度非倍数的情况）
		val := uint32(rb[0])<<24 | uint32(rb[1])<<16 | uint32(rb[2])<<8 | uint32(rb[3])
		b[i] = charset[val%n]
	}
	return string(b)
}

// RandomBytes 生成指定长度的随机字节。
func RandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return nil, fmt.Errorf("generate random bytes: %w", err)
	}
	return b, nil
}

// ---------------------------------------------------------------------------
// SHA-256 哈希
// ---------------------------------------------------------------------------

// SHA256Hex 计算 SHA-256 并返回十六进制字符串。
func SHA256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// SHA256Bytes 计算 SHA-256 并返回字节。
func SHA256Bytes(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

// ---------------------------------------------------------------------------
// 常量时间比较
// ---------------------------------------------------------------------------

// ConstantTimeEqual 常量时间比较两个字节切片。
func ConstantTimeEqual(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}

// ConstantTimeEqualString 常量时间比较两个字符串。
func ConstantTimeEqualString(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// ---------------------------------------------------------------------------
// Argon2id 密码哈希
// ---------------------------------------------------------------------------

// Argon2id 参数（OWASP 2023 推荐值）
const (
	Argon2Time    = 3
	Argon2Memory  = 64 * 1024 // 64 MB
	Argon2Threads = 4
	Argon2KeyLen  = 32
	Argon2SaltLen = 16
)

// HashPasswordArgon2id 使用 argon2id 算法哈希密码。
// 返回格式: $argon2id$v=19$m=65536,t=3,p=4$<salt>$<hash>
func HashPasswordArgon2id(password string) (string, error) {
	salt, err := RandomBytes(Argon2SaltLen)
	if err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	hash := argon2.IDKey([]byte(password), salt, Argon2Time, Argon2Memory, Argon2Threads, Argon2KeyLen)
	// 编码为标准格式
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, Argon2Memory, Argon2Time, Argon2Threads, b64Salt, b64Hash), nil
}

// VerifyPasswordArgon2id 验证密码是否匹配 argon2id 哈希。
func VerifyPasswordArgon2id(password, encoded string) (bool, error) {
	parts, err := parseArgon2id(encoded)
	if err != nil {
		return false, err
	}
	hash := argon2.IDKey([]byte(password), parts.salt, parts.time, parts.memory, parts.threads, parts.keyLen)
	return ConstantTimeEqual(hash, parts.hash), nil
}

type argon2idParts struct {
	salt    []byte
	hash    []byte
	time    uint32
	memory  uint32
	threads uint8
	keyLen  uint32
}

func parseArgon2id(encoded string) (*argon2idParts, error) {
	// 格式: $argon2id$v=19$m=65536,t=3,p=4$<salt>$<hash>
	if !strings.HasPrefix(encoded, "$argon2id$") {
		return nil, errors.New("invalid argon2id hash format")
	}
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 {
		return nil, errors.New("invalid argon2id hash format: wrong number of fields")
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return nil, fmt.Errorf("parse version: %w", err)
	}
	if version != argon2.Version {
		return nil, fmt.Errorf("unsupported argon2 version: %d", version)
	}

	p := &argon2idParts{}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.memory, &p.time, &p.threads); err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}

	var err error
	p.salt, err = base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return nil, fmt.Errorf("decode salt: %w", err)
	}
	p.hash, err = base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return nil, fmt.Errorf("decode hash: %w", err)
	}
	p.keyLen = uint32(len(p.hash))

	return p, nil
}

// ---------------------------------------------------------------------------
// 编码辅助
// ---------------------------------------------------------------------------

// EncodeBase64 base64 编码。
func EncodeBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// DecodeBase64 base64 解码。
func DecodeBase64(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

// EncodeHex 十六进制编码。
func EncodeHex(data []byte) string {
	return hex.EncodeToString(data)
}

// DecodeHex 十六进制解码。
func DecodeHex(s string) ([]byte, error) {
	return hex.DecodeString(s)
}
