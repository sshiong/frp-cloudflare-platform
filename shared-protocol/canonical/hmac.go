// Package canonical defines the canonical request format for HMAC-SHA256 device authentication.
// Both Server Panel and Client Panel must use this exact format for signature generation and verification.
package canonical

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

// CanonicalRequest builds the canonical request string for HMAC signing.
//
// Format:
//
//	client_id + "\n" +
//	token_version + "\n" +
//	timestamp + "\n" +
//	nonce + "\n" +
//	UPPERCASE(http_method) + "\n" +
//	normalized_path_and_query + "\n" +
//	body_sha256
func CanonicalRequest(clientID, tokenVersion, timestamp, nonce, httpMethod, path, query, bodySHA256 string) string {
	normalizedPath := NormalizePath(path)
	normalizedQuery := NormalizeQuery(query)
	pathAndQuery := normalizedPath
	if normalizedQuery != "" {
		pathAndQuery = normalizedPath + "?" + normalizedQuery
	}

	return fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s\n%s",
		clientID,
		tokenVersion,
		timestamp,
		nonce,
		strings.ToUpper(httpMethod),
		pathAndQuery,
		bodySHA256,
	)
}

// NormalizePath normalizes the URL path for canonical signing.
// - Ensures leading slash
// - Resolves . and .. segments
// - Collapses multiple slashes
// - Does NOT percent-encode (use raw path)
func NormalizePath(path string) string {
	if path == "" || path[0] != '/' {
		path = "/" + path
	}

	// Collapse multiple slashes
	for strings.Contains(path, "//") {
		path = strings.ReplaceAll(path, "//", "/")
	}

	// Simple path normalization (resolve . and ..)
	segments := strings.Split(path, "/")
	normalized := make([]string, 0, len(segments))
	for _, seg := range segments {
		switch seg {
		case ".", "":
			// Skip current dir and empty segments (except leading)
			if len(normalized) == 0 {
				normalized = append(normalized, "")
			}
		case "..":
			if len(normalized) > 1 {
				normalized = normalized[:len(normalized)-1]
			}
		default:
			normalized = append(normalized, seg)
		}
	}

	result := strings.Join(normalized, "/")
	if result == "" {
		result = "/"
	}
	return result
}

// NormalizeQuery normalizes query parameters for canonical signing.
// - Sorts by key, then by value
// - Applies standard percent encoding
// - Returns empty string if no query
func NormalizeQuery(query string) string {
	if query == "" {
		return ""
	}

	values, err := url.ParseQuery(query)
	if err != nil {
		return query
	}

	// Sort keys
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		vals := values[k]
		sort.Strings(vals)
		for _, v := range vals {
			parts = append(parts, percentEncode(k)+"="+percentEncode(v))
		}
	}

	return strings.Join(parts, "&")
}

// percentEncode applies RFC 3986 percent encoding.
func percentEncode(s string) string {
	encoded := url.QueryEscape(s)
	// QueryEscape uses + for spaces, we need %20
	encoded = strings.ReplaceAll(encoded, "+", "%20")
	// Unescape unreserved characters that QueryEscape unnecessarily encodes
	for _, c := range []string{"-", "_", ".", "~"} {
		encoded = strings.ReplaceAll(encoded, "%"+hex.EncodeToString([]byte(c)), c)
	}
	return encoded
}

// Sign generates HMAC-SHA256 signature for the canonical request.
func Sign(signingKey []byte, canonicalRequest string) string {
	mac := hmac.New(sha256.New, signingKey)
	mac.Write([]byte(canonicalRequest))
	return hex.EncodeToString(mac.Sum(nil))
}

// Verify verifies HMAC-SHA256 signature using constant-time comparison.
func Verify(signingKey []byte, canonicalRequest, expectedSignature string) bool {
	expected, err := hex.DecodeString(expectedSignature)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, signingKey)
	mac.Write([]byte(canonicalRequest))
	computed := mac.Sum(nil)
	return hmac.Equal(computed, expected)
}

// BodySHA256 computes SHA-256 hex digest of request body.
// For GET/HEAD with no body, pass empty byte slice.
func BodySHA256(body []byte) string {
	h := sha256.Sum256(body)
	return hex.EncodeToString(h[:])
}

// EmptyBodyHash is the SHA-256 of empty bytes.
const EmptyBodyHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

// BuildAuthHeader builds the Authorization header value.
func BuildAuthHeader(signature string) string {
	return "Device-HMAC-SHA256 Signature=" + signature
}
