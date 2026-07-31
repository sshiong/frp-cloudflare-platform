// Package schemas defines version and compatibility information shared between panels.
package schemas

// Current protocol and schema versions.
const (
	ProtocolVersion      = 1
	ConfigSchemaVersion  = 1
	RouterSchemaVersion  = 1
	APIVersion           = "v1"
)

// VersionInfo contains version information exchanged between server and client.
type VersionInfo struct {
	ClientPanelVersion string `json:"client_panel_version"`
	FrpcVersion        string `json:"frpc_version"`
	ProtocolVersion    int    `json:"protocol_version"`
	ConfigSchemaVersion int   `json:"config_schema_version"`
	OS                 string `json:"os"`
	Arch               string `json:"arch"`
}

// ServerVersionInfo contains server version information returned to clients.
type ServerVersionInfo struct {
	MinimumClientVersion    string `json:"minimum_client_version"`
	LatestClientVersion     string `json:"latest_client_version"`
	MinimumFrpcVersion      string `json:"minimum_frpc_version"`
	RecommendedFrpcVersion  string `json:"recommended_frpc_version"`
	ProtocolVersion         int    `json:"protocol_version"`
	SupportedConfigSchema   []int  `json:"supported_config_schema_versions"`
	UpgradePolicy           string `json:"upgrade_policy"`
}

// IsCompatible checks if client versions are compatible with server.
func IsCompatible(client *VersionInfo, server *ServerVersionInfo) (bool, string) {
	if client.ProtocolVersion != server.ProtocolVersion {
		return false, "protocol version incompatible"
	}

	supported := false
	for _, v := range server.SupportedConfigSchema {
		if v == client.ConfigSchemaVersion {
			supported = true
			break
		}
	}
	if !supported {
		return false, "config schema version not supported"
	}

	return true, ""
}

// MappingStatus represents the lifecycle status of a mapping.
type MappingStatus string

const (
	MappingStatusReserved    MappingStatus = "reserved"
	MappingStatusPendingApply MappingStatus = "pending_apply"
	MappingStatusRunning     MappingStatus = "running"
	MappingStatusOffline     MappingStatus = "offline"
	MappingStatusConfigError MappingStatus = "config_error"
	MappingStatusDisabled    MappingStatus = "disabled"
	MappingStatusDeleting    MappingStatus = "deleting"
)

// DomainStatus represents the status of a domain binding.
type DomainStatus string

const (
	DomainStatusReserved          DomainStatus = "reserved"
	DomainStatusPendingDNS        DomainStatus = "pending_dns"
	DomainStatusPendingCert       DomainStatus = "pending_certificate"
	DomainStatusActive            DomainStatus = "active"
	DomainStatusOffline           DomainStatus = "offline"
	DomainStatusDNSError          DomainStatus = "dns_error"
	DomainStatusCertError         DomainStatus = "certificate_error"
	DomainStatusDisabled          DomainStatus = "disabled"
	DomainStatusDeleting          DomainStatus = "deleting"
)

// CertificateStatus represents certificate status.
type CertificateStatus string

const (
	CertStatusPending         CertificateStatus = "pending"
	CertStatusValid           CertificateStatus = "valid"
	CertStatusRenewing        CertificateStatus = "renewing"
	CertStatusExpired         CertificateStatus = "expired"
	CertStatusBlockedNoToken  CertificateStatus = "blocked_missing_token"
	CertStatusBlockedBadToken CertificateStatus = "blocked_invalid_token"
	CertStatusError           CertificateStatus = "error"
)

// CloudflareTokenStatus represents CF token status.
type CloudflareTokenStatus string

const (
	CFTokenStatusMissing   CloudflareTokenStatus = "missing"
	CFTokenStatusValid     CloudflareTokenStatus = "valid"
	CFTokenStatusInvalid   CloudflareTokenStatus = "invalid"
	CFTokenStatusPermDenied CloudflareTokenStatus = "permission_denied"
)

// OperationStatus represents operation status.
type OperationStatus string

const (
	OpStatusPending         OperationStatus = "pending"
	OpStatusRunning         OperationStatus = "running"
	OpStatusWaitingClient   OperationStatus = "waiting_client"
	OpStatusWaitingExternal OperationStatus = "waiting_external"
	OpStatusSucceeded       OperationStatus = "succeeded"
	OpStatusFailed          OperationStatus = "failed"
	OpStatusCancelled       OperationStatus = "cancelled"
)

// HTTPSMode represents the HTTPS mode for a domain.
type HTTPSMode string

const (
	HTTPSAutoCert       HTTPSMode = "auto_certificate"
	HTTPSCloudflareProxy HTTPSMode = "cloudflare_proxy"
	HTTPSHTTPOnly       HTTPSMode = "http_only"
)

// IsValidHTTPSCheckMode checks if the HTTPS mode + proxied combination is valid.
func IsValidHTTPSCheckMode(mode HTTPSMode, proxied bool) bool {
	switch mode {
	case HTTPSAutoCert:
		return !proxied
	case HTTPSCloudflareProxy:
		return proxied
	case HTTPSHTTPOnly:
		return !proxied
	default:
		return false
	}
}

// ProxyType represents the proxy protocol type.
type ProxyType string

const (
	ProxyTypeTCP  ProxyType = "tcp"
	ProxyTypeUDP  ProxyType = "udp"
	ProxyTypeHTTP ProxyType = "http"
)

// UserRole represents user roles.
type UserRole string

const (
	RoleAdmin UserRole = "admin"
	RoleUser  UserRole = "user"
)

// UserStatus represents user status.
type UserStatus string

const (
	UserStatusActive   UserStatus = "active"
	UserStatusDisabled UserStatus = "disabled"
)

// ClientStatus represents client device status.
type ClientStatus string

const (
	ClientStatusOnline         ClientStatus = "online"
	ClientStatusOffline        ClientStatus = "offline"
	ClientStatusDisabled       ClientStatus = "disabled"
	ClientStatusOutdated       ClientStatus = "outdated"
	ClientStatusConfigPending  ClientStatus = "config_pending"
	ClientStatusConfigError    ClientStatus = "config_error"
)

// BindingStatus represents client binding status.
type BindingStatus string

const (
	BindingStatusUnbound         BindingStatus = "unbound"
	BindingStatusBinding         BindingStatus = "binding"
	BindingStatusBound           BindingStatus = "bound"
	BindingStatusSwitchingServer BindingStatus = "switching_server"
	BindingStatusCredentialRevoked BindingStatus = "credential_revoked"
	BindingStatusUnbinding       BindingStatus = "unbinding"
)

// SessionStatus represents browser session status.
type SessionStatus string

const (
	SessionStatusActive            SessionStatus = "active"
	SessionStatusServerUnreachable SessionStatus = "server_unreachable_readonly"
	SessionStatusExpired           SessionStatus = "expired"
	SessionStatusRevoked           SessionStatus = "revoked"
)
