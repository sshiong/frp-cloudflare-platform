// Package canonical defines configuration canonicalization rules.
// Config JSON must be canonicalized before hashing and signing.
package canonical

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

// ConfigEnvelope represents the signed config envelope.
type ConfigEnvelope struct {
	ClientID          string      `json:"client_id"`
	ConfigVersion     int64       `json:"config_version"`
	SchemaVersion     int         `json:"schema_version"`
	GeneratedAt       string      `json:"generated_at"`
	ConfigBody        interface{} `json:"config_body"`
	ConfigHash        string      `json:"config_hash"`
	ConfigSigningKeyID string     `json:"config_signing_key_id"`
	ConfigSignature   string      `json:"config_signature"`
}

// CanonicalizeJSON produces a deterministic JSON representation.
// Keys are sorted lexicographically at all levels.
// This ensures consistent hashing across different JSON encoders.
func CanonicalizeJSON(v interface{}) ([]byte, error) {
	normalized, err := normalizeValue(v)
	if err != nil {
		return nil, fmt.Errorf("canonicalize: %w", err)
	}
	data, err := json.Marshal(normalized)
	if err != nil {
		return nil, fmt.Errorf("marshal canonical: %w", err)
	}
	return data, nil
}

// normalizeValue recursively normalizes a value for canonical JSON.
func normalizeValue(v interface{}) (interface{}, error) {
	switch val := v.(type) {
	case map[string]interface{}:
		return normalizeMap(val)
	case []interface{}:
		return normalizeSlice(val)
	default:
		return val, nil
	}
}

// normalizeMap sorts map keys and recursively normalizes values.
func normalizeMap(m map[string]interface{}) (map[string]interface{}, error) {
	result := make(map[string]interface{}, len(m))
	for k, val := range m {
		normalized, err := normalizeValue(val)
		if err != nil {
			return nil, err
		}
		result[k] = normalized
	}
	return result, nil
}

// normalizeSlice recursively normalizes slice elements.
func normalizeSlice(s []interface{}) ([]interface{}, error) {
	result := make([]interface{}, len(s))
	for i, val := range s {
		normalized, err := normalizeValue(val)
		if err != nil {
			return nil, err
		}
		result[i] = normalized
	}
	return result, nil
}

// ConfigHash computes SHA-256 of canonical config body.
func ConfigHash(configBody interface{}) (string, error) {
	canonical, err := CanonicalizeJSON(configBody)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(canonical)
	return hex.EncodeToString(h[:]), nil
}

// CanonicalEnvelopeForSigning produces the canonical bytes used for Ed25519 signing.
// It covers: client_id, config_version, schema_version, config_hash.
func CanonicalEnvelopeForSigning(clientID string, configVersion int64, schemaVersion int, configHash string) []byte {
	// Use a simple deterministic concatenation
	s := fmt.Sprintf("frp-panel-config-signature-v1\nclient_id=%s\nconfig_version=%d\nschema_version=%d\nconfig_hash=%s",
		clientID, configVersion, schemaVersion, configHash)
	return []byte(s)
}

// SortMapKeys sorts the keys of a map for deterministic iteration.
func SortMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
