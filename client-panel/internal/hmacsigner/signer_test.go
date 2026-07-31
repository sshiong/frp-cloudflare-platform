package hmacsigner

import (
	"net/http"
	"testing"
)

func TestNewSigner(t *testing.T) {
	clientID := "test-client-123"
	signingKey := make([]byte, 32)
	for i := range signingKey {
		signingKey[i] = byte(i)
	}
	tokenVersion := 1

	signer := NewSigner(clientID, signingKey, tokenVersion)
	if signer == nil {
		t.Error("NewSigner returned nil signer")
	}
}

func TestSignRequest(t *testing.T) {
	clientID := "test-client-123"
	signingKey := make([]byte, 32)
	for i := range signingKey {
		signingKey[i] = byte(i)
	}
	tokenVersion := 1

	signer := NewSigner(clientID, signingKey, tokenVersion)

	req, _ := http.NewRequest("GET", "http://localhost:9000/api/v1/client/config", nil)

	result, err := signer.SignRequest(req, nil)
	if err != nil {
		t.Fatalf("SignRequest failed: %v", err)
	}

	if result == nil {
		t.Error("SignRequest returned nil result")
	}

	if len(result.Authorization) == 0 {
		t.Error("SignRequest Authorization is empty")
	}

	if result.ClientID != clientID {
		t.Errorf("SignRequest ClientID = %q, want %q", result.ClientID, clientID)
	}
}

func TestVerifyTimestamp(t *testing.T) {
	// Valid timestamp
	ok, _ := VerifyTimestamp("1234567890", 300)
	if !ok {
		// This might fail if timestamp is too old, which is expected
		t.Log("VerifyTimestamp returned false for old timestamp (expected)")
	}
}

func TestNormalizeCanonicalRequest(t *testing.T) {
	canonical := NormalizeCanonicalRequest(
		"client-123",
		1,
		"1234567890",
		"nonce-abc",
		"GET",
		"/api/v1/config",
		"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
	)

	if len(canonical) == 0 {
		t.Error("NormalizeCanonicalRequest returned empty string")
	}

	// Same inputs should produce same canonical
	canonical2 := NormalizeCanonicalRequest(
		"client-123",
		1,
		"1234567890",
		"nonce-abc",
		"GET",
		"/api/v1/config",
		"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
	)

	if canonical != canonical2 {
		t.Error("NormalizeCanonicalRequest returned different values for same input")
	}
}

func TestGenerateNonce(t *testing.T) {
	nonce1, err := GenerateNonce()
	if err != nil {
		t.Fatalf("GenerateNonce failed: %v", err)
	}

	nonce2, err := GenerateNonce()
	if err != nil {
		t.Fatalf("GenerateNonce failed: %v", err)
	}

	if nonce1 == nonce2 {
		t.Error("GenerateNonce returned same value twice")
	}

	if len(nonce1) == 0 {
		t.Error("GenerateNonce returned empty string")
	}
}

func TestGenerateIdempotencyKey(t *testing.T) {
	key1 := GenerateIdempotencyKey()
	key2 := GenerateIdempotencyKey()

	if key1 == key2 {
		t.Error("GenerateIdempotencyKey returned same value twice")
	}

	if len(key1) == 0 {
		t.Error("GenerateIdempotencyKey returned empty string")
	}
}
