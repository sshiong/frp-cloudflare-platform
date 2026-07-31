package hmacsigner

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	// TimestampTolerance 时间戳容差，默认 ±300 秒
	TimestampTolerance = 300
	// NonceSize 128-bit Nonce
	NonceSize = 16
)

// Signer 设备 HMAC 签名器
// 使用 HKDF-SHA256 从 device_token 派生签名密钥
// 构造规范化请求串并生成 HMAC-SHA256 签名
type Signer struct {
	clientID     string
	signingKey   []byte
	tokenVersion int
}

// NewSigner 创建签名器
// signingKey 应通过 DeriveHMACKey 从 device_token 派生
func NewSigner(clientID string, signingKey []byte, tokenVersion int) *Signer {
	return &Signer{
		clientID:     clientID,
		signingKey:   signingKey,
		tokenVersion: tokenVersion,
	}
}

// RequestSignResult 签名结果
type RequestSignResult struct {
	ClientID          string
	TokenVersion      string
	Timestamp         string
	Nonce             string
	BodySHA256        string
	Authorization     string
	IdempotencyKey    string
}

// SignRequest 为 HTTP 请求生成 HMAC 签名头
// 规范签名串格式：
//
//	client_id + "\n" +
//	token_version + "\n" +
//	timestamp + "\n" +
//	nonce + "\n" +
//	METHOD + "\n" +
//	path + "\n" +
//	body_sha256
func (s *Signer) SignRequest(req *http.Request, body []byte) (*RequestSignResult, error) {
	// 生成 128-bit 随机 Nonce
	nonceBytes := make([]byte, NonceSize)
	if _, err := io.ReadFull(rand.Reader, nonceBytes); err != nil {
		return nil, fmt.Errorf("生成 Nonce 失败: %w", err)
	}
	nonce := base64.RawURLEncoding.EncodeToString(nonceBytes)

	// Unix UTC 秒级时间戳
	timestamp := strconv.FormatInt(time.Now().UTC().Unix(), 10)

	// 计算 Body SHA-256
	bodyHash := sha256.Sum256(body)
	bodySHA256 := hex.EncodeToString(bodyHash[:])

	// 规范化路径（包含排序后的查询参数）
	normalizedPath := normalizePath(req.URL)

	// 构造规范化签名串
	canonicalRequest := strings.Join([]string{
		s.clientID,
		strconv.Itoa(s.tokenVersion),
		timestamp,
		nonce,
		strings.ToUpper(req.Method),
		normalizedPath,
		bodySHA256,
	}, "\n")

	// HMAC-SHA256 签名
	mac := hmac.New(sha256.New, s.signingKey)
	mac.Write([]byte(canonicalRequest))
	signature := hex.EncodeToString(mac.Sum(nil))

	// 构造 Authorization 头
	authorization := fmt.Sprintf("Device-HMAC-SHA256 Signature=%s", signature)

	// 生成幂等键（变更请求使用）
	idempotencyKey := uuid.New().String()

	return &RequestSignResult{
		ClientID:       s.clientID,
		TokenVersion:   strconv.Itoa(s.tokenVersion),
		Timestamp:      timestamp,
		Nonce:          nonce,
		BodySHA256:     bodySHA256,
		Authorization:  authorization,
		IdempotencyKey: idempotencyKey,
	}, nil
}

// ApplyToRequest 将签名结果应用到 HTTP 请求头
func (r *RequestSignResult) ApplyToRequest(req *http.Request) {
	req.Header.Set("X-Client-ID", r.ClientID)
	req.Header.Set("X-Device-Token-Version", r.TokenVersion)
	req.Header.Set("X-Request-Timestamp", r.Timestamp)
	req.Header.Set("X-Request-Nonce", r.Nonce)
	req.Header.Set("X-Content-SHA256", r.BodySHA256)
	req.Header.Set("Authorization", r.Authorization)
	req.Header.Set("Idempotency-Key", r.IdempotencyKey)
}

// VerifySignature 验证 HMAC 签名（服务端使用）
func VerifySignature(signingKey []byte, canonicalRequest, signature string) bool {
	mac := hmac.New(sha256.New, signingKey)
	mac.Write([]byte(canonicalRequest))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

// VerifyTimestamp 验证时间戳在容差范围内
func VerifyTimestamp(timestampStr string, toleranceSeconds int) (bool, int64) {
	ts, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return false, 0
	}
	now := time.Now().UTC().Unix()
	diff := now - ts
	if diff < 0 {
		diff = -diff
	}
	return diff <= int64(toleranceSeconds), diff
}

// NormalizeCanonicalRequest 规范化请求串（供验证使用）
func NormalizeCanonicalRequest(clientID string, tokenVersion int, timestamp, nonce, method, pathAndQuery, bodySHA256 string) string {
	return strings.Join([]string{
		clientID,
		strconv.Itoa(tokenVersion),
		timestamp,
		nonce,
		strings.ToUpper(method),
		normalizePathFromString(pathAndQuery),
		bodySHA256,
	}, "\n")
}

// normalizePath 规范化 URL 路径和查询参数
// - 路径必须以 / 开头
// - 查询参数按键排序
// - 百分号编码标准化
func normalizePath(u *url.URL) string {
	path := u.Path
	if path == "" {
		path = "/"
	}

	// 排序查询参数
	query := u.Query()
	if len(query) == 0 {
		return path
	}

	// 按键排序
	keys := make([]string, 0, len(query))
	for k := range query {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		vals := query[k]
		sort.Strings(vals)
		for _, v := range vals {
			parts = append(parts, fmt.Sprintf("%s=%s",
				url.QueryEscape(k),
				url.QueryEscape(v)))
		}
	}

	return path + "?" + strings.Join(parts, "&")
}

// normalizePathFromString 从字符串规范化路径
func normalizePathFromString(pathAndQuery string) string {
	u, err := url.Parse(pathAndQuery)
	if err != nil {
		return pathAndQuery
	}
	return normalizePath(u)
}

// GenerateNonce 生成 128-bit 随机 Nonce（base64url 编码）
func GenerateNonce() (string, error) {
	b := make([]byte, NonceSize)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// GenerateIdempotencyKey 生成幂等键（UUID v4）
func GenerateIdempotencyKey() string {
	return uuid.New().String()
}
