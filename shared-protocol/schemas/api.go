package schemas

// ========== Auth API ==========

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Success          bool   `json:"success"`
	MustChangePassword bool `json:"must_change_password"`
	UserID           string `json:"user_id,omitempty"`
	Username         string `json:"username,omitempty"`
	Role             string `json:"role,omitempty"`
}

type SessionInfo struct {
	UserID     string `json:"user_id"`
	Username   string `json:"username"`
	Role       string `json:"role"`
	ClientID   string `json:"client_id,omitempty"`
	ExpiresAt  int64  `json:"expires_at"`
}

// ========== Device API ==========

type DeviceRegisterRequest struct {
	InstallationInstanceID string `json:"installation_instance_id"`
	DeviceName            string `json:"device_name"`
	ClientPanelVersion    string `json:"client_panel_version"`
	FrpcVersion           string `json:"frpc_version"`
	ProtocolVersion       int    `json:"protocol_version"`
	ConfigSchemaVersion   int    `json:"config_schema_version"`
	OS                    string `json:"os"`
	Arch                  string `json:"arch"`
}

type DeviceRegisterResponse struct {
	ClientID              string `json:"client_id"`
	DeviceToken           string `json:"device_token"`
	FrpDeviceToken        string `json:"frp_device_token"`
	ConfigSigningPublicKey string `json:"config_signing_public_key"`
	ConfigSigningKeyID    string `json:"config_signing_key_id"`
}

type DeviceInfo struct {
	ClientID           string `json:"client_id"`
	Name               string `json:"name"`
	Status             string `json:"status"`
	OwnerUserID        string `json:"owner_user_id"`
	ClientPanelVersion string `json:"client_panel_version"`
	FrpcVersion        string `json:"frpc_version"`
	LastSeenAt         int64  `json:"last_seen_at"`
	RegisteredAt       int64  `json:"registered_at"`
}

// ========== Mapping API ==========

type CreateMappingRequest struct {
	Name             string `json:"name"`
	ProxyType        string `json:"proxy_type"` // tcp, udp, http
	LocalIP          string `json:"local_ip"`
	LocalPort        int    `json:"local_port"`
	RemotePort       int    `json:"remote_port,omitempty"` // for tcp/udp
	ExpectedVersion  int64  `json:"expected_config_version"`
	IdempotencyKey   string `json:"idempotency_key"`
}

type UpdateMappingRequest struct {
	Name             string `json:"name,omitempty"`
	LocalIP          string `json:"local_ip,omitempty"`
	LocalPort        int    `json:"local_port,omitempty"`
	RemotePort       int    `json:"remote_port,omitempty"`
	ExpectedVersion  int64  `json:"expected_config_version"`
	MappingRevision  int    `json:"mapping_revision"`
	IdempotencyKey   string `json:"idempotency_key"`
}

type MappingInfo struct {
	ID               string `json:"id"`
	UserID           string `json:"user_id"`
	ClientID         string `json:"client_id"`
	Name             string `json:"name"`
	ProxyType        string `json:"proxy_type"`
	LocalIP          string `json:"local_ip"`
	LocalPort        int    `json:"local_port"`
	RemotePort       int    `json:"remote_port,omitempty"`
	LifecycleStatus  string `json:"lifecycle_status"`
	DesiredState     string `json:"desired_state"`
	ObservedState    string `json:"observed_state"`
	ActiveRevision   int    `json:"active_revision"`
	PendingRevision  int    `json:"pending_revision,omitempty"`
	ConfigVersion    int64  `json:"config_version"`
	CreatedAt        int64  `json:"created_at"`
	UpdatedAt        int64  `json:"updated_at"`
}

// ========== Domain API ==========

type CreateDomainRequest struct {
	Hostname         string `json:"hostname"`
	MappingID        string `json:"mapping_id"`
	HTTPsMode        string `json:"https_mode"` // auto_certificate, cloudflare_proxy, http_only
	HTTPRedirect     bool   `json:"http_redirect"`
	ExpectedVersion  int64  `json:"expected_config_version"`
	IdempotencyKey   string `json:"idempotency_key"`
}

type DomainPreflightResult struct {
	DomainExists     bool              `json:"domain_exists"`
	CloudflareDNS    *CloudflareDNSInfo `json:"cloudflare_dns,omitempty"`
	ZoneID           string            `json:"zone_id,omitempty"`
	ZoneName         string            `json:"zone_name,omitempty"`
	HasPermission    bool              `json:"has_permission"`
	MissingPerms     []string          `json:"missing_permissions,omitempty"`
}

type CloudflareDNSInfo struct {
	RecordID  string `json:"record_id"`
	Type      string `json:"type"`
	Name      string `json:"name"`
	Content   string `json:"content"`
	TTL       int    `json:"ttl"`
	Proxied   bool   `json:"proxied"`
}

type DomainInfo struct {
	ID               string `json:"id"`
	UserID           string `json:"user_id"`
	ClientID         string `json:"client_id"`
	MappingID        string `json:"mapping_id"`
	Hostname         string `json:"hostname"`
	NormalizedDomain string `json:"normalized_domain"`
	ZoneID           string `json:"zone_id,omitempty"`
	HTTPsMode        string `json:"https_mode"`
	HTTPRedirect     bool   `json:"http_redirect"`
	Status           string `json:"status"`
	Revision         int    `json:"revision"`
	CreatedAt        int64  `json:"created_at"`
	UpdatedAt        int64  `json:"updated_at"`
}

// ========== Cloudflare API ==========

type CloudflareTokenStatus struct {
	HasToken     bool     `json:"has_token"`
	Status       string   `json:"status"`
	TokenVersion int      `json:"token_version,omitempty"`
	VerifiedAt   int64    `json:"verified_at,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
}

type CloudflareTokenVerifyResult struct {
	Valid        bool     `json:"valid"`
	Capabilities []string `json:"capabilities"`
	Zones        []Zone   `json:"zones"`
	MissingPerms []string `json:"missing_permissions,omitempty"`
}

type Zone struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ========== Certificate API ==========

type CertificateInfo struct {
	ID               string `json:"id"`
	DomainBindingID  string `json:"domain_binding_id"`
	Domain           string `json:"domain"`
	Provider         string `json:"provider"`
	Status           string `json:"status"`
	NotBefore        int64  `json:"not_before"`
	NotAfter         int64  `json:"not_after"`
	RenewAfter       int64  `json:"renew_after"`
	LastErrorCode    string `json:"last_error_code,omitempty"`
	LastErrorMessage string `json:"last_error_message,omitempty"`
	UpdatedAt        int64  `json:"updated_at"`
}

// ========== Operation API ==========

type OperationInfo struct {
	ID                 string `json:"id"`
	UserID             string `json:"user_id"`
	ClientID           string `json:"client_id"`
	ResourceType       string `json:"resource_type"`
	ResourceID         string `json:"resource_id"`
	OperationType      string `json:"operation_type"`
	Status             string `json:"status"`
	Phase              string `json:"phase"`
	Step               string `json:"step"`
	Cancelable         bool   `json:"cancelable"`
	ErrorCode          string `json:"error_code,omitempty"`
	ErrorMessage       string `json:"error_message,omitempty"`
	CreatedAt          int64  `json:"created_at"`
	UpdatedAt          int64  `json:"updated_at"`
	CompletedAt        int64  `json:"completed_at,omitempty"`
}

// ========== Admin API ==========

type CreateUserRequest struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	MaxClients  int `json:"max_clients"`
	MaxMappings int `json:"max_mappings"`
	MaxDomains  int `json:"max_domains"`
}

type CreateUserResponse struct {
	UserID          string `json:"user_id"`
	Username        string `json:"username"`
	InitialPassword string `json:"initial_password"`
}

type UserInfo struct {
	ID             string `json:"id"`
	Username       string `json:"username"`
	Role           string `json:"role"`
	Status         string `json:"status"`
	MustChangePass bool   `json:"must_change_password"`
	MaxClients     int    `json:"max_clients"`
	MaxMappings    int    `json:"max_mappings"`
	MaxDomains     int    `json:"max_domains"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
}

type AuditLogEntry struct {
	ID                  string `json:"id"`
	ActorType           string `json:"actor_type"`
	ActorID             string `json:"actor_id"`
	Action              string `json:"action"`
	ResourceType        string `json:"resource_type"`
	ResourceID          string `json:"resource_id"`
	Result              string `json:"result"`
	RequestID           string `json:"request_id"`
	BrowserSourceIP     string `json:"browser_source_ip"`
	CreatedAt           int64  `json:"created_at"`
}

type SystemStatus struct {
	ServerInstanceID string `json:"server_instance_id"`
	FRPSStatus       string `json:"frps_status"`
	RouterStatus     string `json:"router_status"`
	OnlineClients    int    `json:"online_clients"`
	TotalClients     int    `json:"total_clients"`
	TotalUsers       int    `json:"total_users"`
	TotalMappings    int    `json:"total_mappings"`
	TotalDomains     int    `json:"total_domains"`
	DBSize           int64  `json:"db_size"`
	WALSize          int64  `json:"wal_size"`
	RouterVersion    int64  `json:"router_version"`
	RouterApplied    int64  `json:"router_applied_version"`
}

// ========== Client Bootstrap ==========

type ClientBootstrap struct {
	ClientID               string              `json:"client_id"`
	ServerInstanceID       string              `json:"server_instance_id"`
	OwnerUserID            string              `json:"owner_user_id"`
	DesiredConfigVersion   int64               `json:"desired_config_version"`
	AppliedConfigVersion   int64               `json:"applied_config_version"`
	FrpUsername            string              `json:"frp_username"`
	FrpServerAddr         string              `json:"frp_server_addr"`
	FrpServerPort         int                 `json:"frp_server_port"`
	ConfigSigningKeyID     string              `json:"config_signing_key_id"`
	ConfigSigningPublicKey string              `json:"config_signing_public_key"`
	Mappings               []MappingInfo       `json:"mappings"`
	Domains                []DomainInfo        `json:"domains"`
	VersionInfo            ServerVersionInfo   `json:"version_info"`
}

// ========== Config Snapshot ==========

type ConfigSnapshot struct {
	ClientID           string      `json:"client_id"`
	ConfigVersion      int64       `json:"config_version"`
	SchemaVersion      int         `json:"schema_version"`
	GeneratedAt        string      `json:"generated_at"`
	ConfigBody         interface{} `json:"config_body"`
	ConfigHash         string      `json:"config_hash"`
	ConfigSigningKeyID string      `json:"config_signing_key_id"`
	ConfigSignature    string      `json:"config_signature"`
}

type ConfigApplyResult struct {
	ConfigVersion  int64  `json:"config_version"`
	Success        bool   `json:"success"`
	ErrorCode      string `json:"error_code,omitempty"`
	ErrorMessage   string `json:"error_message,omitempty"`
	FrpcPID        int    `json:"frpc_pid,omitempty"`
	AppliedAt      int64  `json:"applied_at"`
}

// ========== Heartbeat ==========

type HeartbeatRequest struct {
	ClientPanelVersion string `json:"client_panel_version"`
	FrpcVersion        string `json:"frpc_version"`
	FrpcRunning        bool   `json:"frpc_running"`
	FrpcPID            int    `json:"frpc_pid,omitempty"`
	AppliedVersion     int64  `json:"applied_config_version"`
	OS                 string `json:"os"`
	Arch               string `json:"arch"`
}

type StatusReport struct {
	FrpcRunning     bool           `json:"frpc_running"`
	FrpcPID         int            `json:"frpc_pid,omitempty"`
	FrpcVersion     string         `json:"frpc_version"`
	AppliedVersion  int64          `json:"applied_config_version"`
	ProxyStates     []ProxyState   `json:"proxy_states"`
	LocalHealth     []HealthCheck  `json:"local_health"`
	ErrorSummary    string         `json:"error_summary,omitempty"`
}

type ProxyState struct {
	ProxyName  string `json:"proxy_name"`
	Type       string `json:"type"`
	Status     string `json:"status"`
	RemoteAddr string `json:"remote_addr"`
	Error      string `json:"error,omitempty"`
}

type HealthCheck struct {
	MappingID string `json:"mapping_id"`
	LocalAddr string `json:"local_addr"`
	Status    string `json:"status"` // reachable, unreachable, unknown
	Latency   int    `json:"latency_ms,omitempty"`
}
