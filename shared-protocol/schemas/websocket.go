package schemas

// WebSocket event types for Server -> Client notifications.
const (
	// Config events
	WSEventConfigUpdate     = "config_update"
	WSEventConfigVersion    = "config_version_change"
	WSEventForceSync        = "force_sync"

	// Token events
	WSEventFRPTokenReset    = "frp_token_reset"
	WSEventCFTokenChanged   = "cloudflare_token_changed"
	WSEventCFTokenCleared   = "cloudflare_token_cleared"

	// User events
	WSEventUserDisabled     = "user_disabled"
	WSEventUserDeleted      = "user_deleted"
	WSEventPasswordChanged  = "password_changed"

	// Device events
	WSEventDeviceRevoked    = "device_revoked"
	WSEventDeviceDisabled   = "device_disabled"

	// Mapping events
	WSEventMappingCreated   = "mapping_created"
	WSEventMappingUpdated   = "mapping_updated"
	WSEventMappingDeleted   = "mapping_deleted"
	WSEventMappingEnabled   = "mapping_enabled"
	WSEventMappingDisabled  = "mapping_disabled"

	// Domain events
	WSEventDomainCreated    = "domain_created"
	WSEventDomainUpdated    = "domain_updated"
	WSEventDomainDeleted    = "domain_deleted"

	// Certificate events
	WSEventCertIssued       = "certificate_issued"
	WSEventCertRenewed      = "certificate_renewed"
	WSEventCertError        = "certificate_error"

	// Router events
	WSEventRouterUpdated    = "router_updated"

	// System events
	WSEventSystemMaintenance = "system_maintenance"
	WSEventPing             = "ping"
	WSEventPong             = "pong"
)

// WSEvent represents a WebSocket event message.
type WSEvent struct {
	Type      string      `json:"type"`
	Payload   interface{} `json:"payload,omitempty"`
	Timestamp int64       `json:"timestamp"`
	RequestID string      `json:"request_id,omitempty"`
}

// WSConfigUpdatePayload is the payload for config_update events.
type WSConfigUpdatePayload struct {
	ConfigVersion int64  `json:"config_version"`
	Action        string `json:"action"` // "full_sync", "reload", "restart"
}

// WSErrorPayload is the payload for error events.
type WSErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// WSPongResponse is the response to a ping.
type WSPongResponse struct {
	ServerTime int64 `json:"server_time"`
}
