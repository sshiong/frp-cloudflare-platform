package configverify

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// ConfigEnvelope 配置信封
// Server Panel 签名的完整配置结构
type ConfigEnvelope struct {
	ClientID       string          `json:"client_id"`
	ConfigVersion  int64           `json:"config_version"`
	SchemaVersion  int             `json:"schema_version"`
	GeneratedAt    string          `json:"generated_at"`
	ConfigBody     json.RawMessage `json:"config_body"`
	ConfigHash     string          `json:"config_hash"`
	SigningKeyID   string          `json:"signing_key_id"`
	Signature      string          `json:"signature"`
}

// VerifyResult 验证结果
type VerifyResult struct {
	Valid           bool
	ConfigVersion   int64
	SchemaVersion   int
	KeyID           string
	Error           string
}

// Verifier 配置签名验证器
// 验证 Server Panel 使用 Ed25519 签名的配置
// 验证内容：签名、哈希、Key ID、Client ID、版本单调性、Schema 兼容性
type Verifier struct {
	publicKey      ed25519.PublicKey
	keyID          string
	clientID       string
	currentVersion int64
	schemaVersion  int
}

// NewVerifier 创建配置验证器
// publicKey: Client 首次绑定时固定的签名公钥
// keyID: 当前绑定的密钥 ID
// clientID: 当前 client_id
func NewVerifier(publicKey ed25519.PublicKey, keyID, clientID string, currentVersion int64, schemaVersion int) *Verifier {
	return &Verifier{
		publicKey:      publicKey,
		keyID:          keyID,
		clientID:       clientID,
		currentVersion: currentVersion,
		schemaVersion:  schemaVersion,
	}
}

// Verify 验证配置信封
// 验证步骤：
// 1. Client ID 匹配
// 2. Key ID 匹配
// 3. Schema 版本兼容
// 4. 配置版本单调递增
// 5. 配置哈希验证
// 6. Ed25519 签名验证
func (v *Verifier) Verify(envelope *ConfigEnvelope) *VerifyResult {
	result := &VerifyResult{
		ConfigVersion: envelope.ConfigVersion,
		SchemaVersion: envelope.SchemaVersion,
		KeyID:         envelope.SigningKeyID,
	}

	// 1. Client ID 匹配
	if envelope.ClientID != v.clientID {
		result.Error = fmt.Sprintf("CLIENT_ID_MISMATCH: 期望 %s, 实际 %s", v.clientID, envelope.ClientID)
		return result
	}

	// 2. Key ID 匹配
	if envelope.SigningKeyID != v.keyID {
		result.Error = fmt.Sprintf("KEY_ID_MISMATCH: 期望 %s, 实际 %s", v.keyID, envelope.SigningKeyID)
		return result
	}

	// 3. Schema 版本兼容
	if !isSchemaCompatible(v.schemaVersion, envelope.SchemaVersion) {
		result.Error = fmt.Sprintf("SCHEMA_INCOMPATIBLE: 本地 %d, 配置 %d", v.schemaVersion, envelope.SchemaVersion)
		return result
	}

	// 4. 配置版本单调递增
	if envelope.ConfigVersion <= v.currentVersion {
		result.Error = fmt.Sprintf("VERSION_NOT_MONOTONIC: 当前 %d, 配置 %d", v.currentVersion, envelope.ConfigVersion)
		return result
	}

	// 5. 配置哈希验证
	expectedHash := computeConfigHash(envelope.ConfigBody)
	if envelope.ConfigHash != expectedHash {
		result.Error = fmt.Sprintf("HASH_MISMATCH: 期望 %s, 实际 %s", expectedHash, envelope.ConfigHash)
		return result
	}

	// 6. Ed25519 签名验证
	canonicalEnvelope := buildCanonicalEnvelope(envelope)
	if !verifySignature(v.publicKey, canonicalEnvelope, envelope.Signature) {
		result.Error = "SIGNATURE_INVALID: Ed25519 签名验证失败"
		return result
	}

	result.Valid = true
	return result
}

// UpdateVersion 更新验证器的当前版本（应用成功后调用）
func (v *Verifier) UpdateVersion(version int64) {
	v.currentVersion = version
}

// UpdatePublicKey 更新签名公钥（密钥轮换时调用）
func (v *Verifier) UpdatePublicKey(publicKey ed25519.PublicKey, keyID string) {
	v.publicKey = publicKey
	v.keyID = keyID
}

// computeConfigHash 计算配置体 SHA-256 哈希
func computeConfigHash(configBody []byte) string {
	h := sha256.Sum256(configBody)
	return hex.EncodeToString(h[:])
}

// buildCanonicalEnvelope 构建规范化信封（用于签名验证）
// 签名覆盖：client_id, config_version, schema_version, config_hash
func buildCanonicalEnvelope(envelope *ConfigEnvelope) []byte {
	// 规范化格式：client_id + version + schema + hash
	canonical := fmt.Sprintf("%s\n%d\n%d\n%s",
		envelope.ClientID,
		envelope.ConfigVersion,
		envelope.SchemaVersion,
		envelope.ConfigHash,
	)
	return []byte(canonical)
}

// verifySignature 验证 Ed25519 签名
func verifySignature(publicKey ed25519.PublicKey, message []byte, signatureHex string) bool {
	signature, err := hex.DecodeString(signatureHex)
	if err != nil {
		return false
	}
	if len(signature) != ed25519.SignatureSize {
		return false
	}
	return ed25519.Verify(publicKey, message, signature)
}

// isSchemaCompatible 检查 Schema 版本兼容性
// 当前版本必须 >= 配置要求的版本
// 且差异不超过 1（主版本）
func isSchemaCompatible(local, remote int) bool {
	if remote > local {
		return false
	}
	// 允许向下兼容 1 个主版本
	if local-remote > 1 {
		return false
	}
	return true
}

// ParseEnvelope 从 JSON 解析配置信封
func ParseEnvelope(data []byte) (*ConfigEnvelope, error) {
	var envelope ConfigEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("解析配置信封失败: %w", err)
	}
	return &envelope, nil
}
