// Package errors defines shared error codes for the FRP Panel platform.
// Both Server Panel and Client Panel use these codes for consistent error handling.
package errors

// Error codes for API responses
const (
	// Authentication & Authorization
	CodeUnauthorized          = "UNAUTHORIZED"
	CodeSessionExpired        = "SESSION_EXPIRED"
	CodeSessionReplaced       = "SESSION_REPLACED"
	CodeCSRFTokenInvalid      = "CSRF_TOKEN_INVALID"
	CodeClientOwnerMismatch   = "CLIENT_OWNER_MISMATCH"
	CodeUserDisabled          = "USER_DISABLED"
	CodeDeviceRevoked         = "DEVICE_REVOKED"
	CodeMustChangePassword    = "MUST_CHANGE_PASSWORD"
	CodeInvalidCredentials    = "INVALID_CREDENTIALS"
	CodeReauthRequired        = "REAUTH_REQUIRED"
	CodeReauthTicketInvalid   = "REAUTH_TICKET_INVALID"
	CodeReauthTicketExpired   = "REAUTH_TICKET_EXPIRED"

	// HMAC & Device Auth
	CodeHMACSignatureInvalid  = "HMAC_SIGNATURE_INVALID"
	CodeTimestampExpired      = "TIMESTAMP_EXPIRED"
	CodeNonceReused           = "NONCE_REUSED"
	CodeBodyHashMismatch      = "BODY_HASH_MISMATCH"
	CodeTokenVersionMismatch  = "TOKEN_VERSION_MISMATCH"
	CodeDeviceNotBound        = "DEVICE_NOT_BOUND"

	// Configuration
	CodeConfigVersionConflict     = "CONFIG_VERSION_CONFLICT"
	CodeResourceRevisionConflict  = "RESOURCE_REVISION_CONFLICT"
	CodeMappingRevisionConflict   = "MAPPING_REVISION_CONFLICT"
	CodeUnsupportedConfigSchema   = "UNSUPPORTED_CONFIG_SCHEMA"
	CodeConfigSignatureInvalid    = "CONFIG_SIGNATURE_INVALID"
	CodeConfigVersionRollback     = "CONFIG_VERSION_ROLLBACK"
	CodeConfigSchemaIncompatible  = "CONFIG_SCHEMA_INCOMPATIBLE"
	CodeClientIdMismatch          = "CLIENT_ID_MISMATCH"
	CodeSigningKeyIdMismatch      = "SIGNING_KEY_ID_MISMATCH"

	// Resources
	CodePortAlreadyReserved       = "PORT_ALREADY_RESERVED"
	CodePortOutOfRange            = "PORT_OUT_OF_RANGE"
	CodePortForbidden             = "PORT_FORBIDDEN"
	CodeDomainAlreadyExists       = "DOMAIN_ALREADY_EXISTS"
	CodeDomainInvalid             = "DOMAIN_INVALID"
	CodeDomainConflictAdmin       = "DOMAIN_CONFLICT_ADMIN"
	CodeMappingNotFound           = "MAPPING_NOT_FOUND"
	CodeDomainNotFound            = "DOMAIN_NOT_FOUND"
	CodeClientNotFound            = "CLIENT_NOT_FOUND"
	CodeUserNotFound              = "USER_NOT_FOUND"
	CodeOperationNotFound         = "OPERATION_NOT_FOUND"

	// Quotas
	CodeQuotaExceededClients      = "QUOTA_EXCEEDED_CLIENTS"
	CodeQuotaExceededMappings     = "QUOTA_EXCEEDED_MAPPINGS"
	CodeQuotaExceededDomains      = "QUOTA_EXCEEDED_DOMAINS"
	CodeQuotaExceededPending      = "QUOTA_EXCEEDED_PENDING"

	// Cloudflare
	CodeCloudflareTokenInvalid    = "CLOUDFLARE_TOKEN_INVALID"
	CodeCloudflareTokenMissing    = "CLOUDFLARE_TOKEN_MISSING"
	CodeCloudflarePermissionDenied = "CLOUDFLARE_PERMISSION_DENIED"
	CodeCloudflareZoneNotFound    = "CLOUDFLARE_ZONE_NOT_FOUND"
	CodeCloudflareAPIError        = "CLOUDFLARE_API_ERROR"
	CodeCloudflareTokenPending    = "CLOUDFLARE_TOKEN_PENDING"

	// Certificates
	CodeCertificateError          = "CERTIFICATE_ERROR"
	CodeACMEChallengeFailed       = "ACME_CHALLENGE_FAILED"
	CodeACMERateLimited           = "ACME_RATE_LIMITED"
	CodeCertificateNotReady       = "CERTIFICATE_NOT_READY"
	CodeRenewalCooldown           = "RENEWAL_COOLDOWN"

	// Router
	CodeRouterSnapshotInvalid     = "ROUTER_SNAPSHOT_INVALID"
	CodeRouterApplyFailed         = "ROUTER_APPLY_FAILED"

	// Operations
	CodeOperationNotCancelable    = "OPERATION_NOT_CANCELABLE"
	CodeOperationPhaseInvalid     = "OPERATION_PHASE_INVALID"
	CodeIdempotencyKeyReused      = "IDEMPOTENCY_KEY_REUSED"

	// FRP
	CodeFRPCNotRunning            = "FRPC_NOT_RUNNING"
	CodeFRPCVerifyFailed          = "FRPC_VERIFY_FAILED"
	CodeFRPCApplyFailed           = "FRPC_APPLY_FAILED"
	CodeFRPSLoginRejected         = "FRPS_LOGIN_REJECTED"
	CodeFRPSProxyRejected         = "FRPS_PROXY_REJECTED"

	// Server
	CodeServerInstanceMismatch    = "SERVER_INSTANCE_MISMATCH"
	CodeServerUnreachable         = "SERVER_UNREACHABLE"
	CodeTLSVerificationFailed     = "TLS_VERIFICATION_FAILED"
	CodeBackupPasswordInvalid     = "BACKUP_PASSWORD_INVALID"
	CodeVersionIncompatible       = "VERSION_INCOMPATIBLE"

	// Generic
	CodeBadRequest                = "BAD_REQUEST"
	CodeNotFound                  = "NOT_FOUND"
	CodeConflict                  = "CONFLICT"
	CodeInternalError             = "INTERNAL_ERROR"
	CodeRateLimited               = "RATE_LIMITED"
	CodeForbidden                 = "FORBIDDEN"
	CodeRequestTooLarge           = "REQUEST_TOO_LARGE"
)

// APIError represents a structured API error response.
type APIError struct {
	Code       string                 `json:"code"`
	Message    string                 `json:"message"`
	Details    map[string]interface{} `json:"details,omitempty"`
	RequestID  string                 `json:"request_id,omitempty"`
}

// Error implements the error interface.
func (e *APIError) Error() string {
	return e.Code + ": " + e.Message
}

// New creates a new APIError.
func New(code, message string) *APIError {
	return &APIError{
		Code:    code,
		Message: message,
	}
}

// NewWithDetails creates a new APIError with additional details.
func NewWithDetails(code, message string, details map[string]interface{}) *APIError {
	return &APIError{
		Code:    code,
		Message: message,
		Details: details,
	}
}

// WithRequestID adds a request ID to the error.
func (e *APIError) WithRequestID(requestID string) *APIError {
	e.RequestID = requestID
	return e
}

// ErrorMessages maps error codes to default Chinese messages.
var ErrorMessages = map[string]string{
	CodeUnauthorized:              "未授权，请先登录",
	CodeSessionExpired:            "会话已过期，请重新登录",
	CodeSessionReplaced:           "当前账号已在另一台设备登录，本次会话已失效",
	CodeCSRFTokenInvalid:          "CSRF令牌无效",
	CodeClientOwnerMismatch:       "设备归属不匹配",
	CodeUserDisabled:              "用户已被停用",
	CodeDeviceRevoked:             "设备已被撤销",
	CodeMustChangePassword:        "首次登录必须修改密码",
	CodeInvalidCredentials:        "用户名或密码错误",
	CodeReauthRequired:            "敏感操作需要二次认证",
	CodeHMACSignatureInvalid:      "HMAC签名无效",
	CodeTimestampExpired:          "请求时间戳超出允许范围",
	CodeNonceReused:               "请求Nonce已被使用",
	CodeBodyHashMismatch:          "请求体哈希不匹配",
	CodeConfigVersionConflict:     "配置版本冲突，请刷新后重试",
	CodeResourceRevisionConflict:  "资源版本冲突，请刷新后重试",
	CodePortAlreadyReserved:       "远程端口已被占用",
	CodePortOutOfRange:            "端口超出允许范围",
	CodeDomainAlreadyExists:       "域名已被使用",
	CodeDomainInvalid:             "域名格式无效",
	CodeQuotaExceededMappings:     "映射配额已满",
	CodeQuotaExceededDomains:      "域名配额已满",
	CodeCloudflareTokenInvalid:    "Cloudflare Token无效",
	CodeCloudflareTokenMissing:    "未配置Cloudflare Token",
	CodeCloudflarePermissionDenied:"Cloudflare Token权限不足",
	CodeCertificateError:          "证书错误",
	CodeACMERateLimited:           "证书申请频率限制，请稍后重试",
	CodeOperationNotCancelable:    "当前操作阶段不可取消",
	CodeIdempotencyKeyReused:      "幂等键已被使用",
	CodeFRPCNotRunning:            "FRPC未运行",
	CodeServerUnreachable:         "服务端不可达",
	CodeInternalError:             "服务器内部错误",
	CodeRateLimited:               "请求过于频繁，请稍后重试",
	CodeBadRequest:                "请求参数错误",
	CodeNotFound:                  "资源不存在",
	CodeConflict:                  "操作冲突",
}
