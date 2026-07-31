package crypto

import (
	"testing"
)

func TestRandomToken(t *testing.T) {
	token1 := RandomToken(32)
	token2 := RandomToken(32)

	if len(token1) == 0 {
		t.Error("RandomToken returned empty string")
	}

	if token1 == token2 {
		t.Error("RandomToken returned same value twice")
	}
}

func TestSHA256Hex(t *testing.T) {
	hash := SHA256Hex([]byte("test"))
	if len(hash) == 0 {
		t.Error("SHA256Hex returned empty string")
	}

	// Same input should produce same hash
	hash2 := SHA256Hex([]byte("test"))
	if hash != hash2 {
		t.Error("SHA256Hex returned different values for same input")
	}
}

func TestHMACSHA256Hex(t *testing.T) {
	key := []byte("test-key-123456789012345678901234") // 32 bytes
	message := []byte("test message")

	sig1 := HMACSHA256Hex(key, message)
	sig2 := HMACSHA256Hex(key, message)

	if sig1 != sig2 {
		t.Error("HMACSHA256Hex returned different values for same input")
	}

	if len(sig1) == 0 {
		t.Error("HMACSHA256Hex returned empty string")
	}
}

func TestVerifyHMACSHA256(t *testing.T) {
	key := []byte("test-key-123456789012345678901234")
	message := []byte("test message")

	mac := HMACSHA256(key, message)

	if !VerifyHMACSHA256(key, message, mac) {
		t.Error("VerifyHMACSHA256 failed for valid MAC")
	}

	if VerifyHMACSHA256(key, []byte("wrong message"), mac) {
		t.Error("VerifyHMACSHA256 succeeded for wrong message")
	}
}

func TestEncryptDecryptAES256GCM(t *testing.T) {
	key := make([]byte, 32) // AES-256
	for i := range key {
		key[i] = byte(i)
	}

	plaintext := []byte("Hello, World!")

	ciphertext, err := EncryptAES256GCM(key, plaintext)
	if err != nil {
		t.Fatalf("EncryptAES256GCM failed: %v", err)
	}

	if len(ciphertext) == 0 {
		t.Error("EncryptAES256GCM returned empty ciphertext")
	}

	decrypted, err := DecryptAES256GCM(key, ciphertext)
	if err != nil {
		t.Fatalf("DecryptAES256GCM failed: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("DecryptAES256GCM returned wrong plaintext: got %q, want %q", string(decrypted), string(plaintext))
	}
}

func TestGenerateEd25519KeyPair(t *testing.T) {
	pub, priv, err := GenerateEd25519KeyPair()
	if err != nil {
		t.Fatalf("GenerateEd25519KeyPair failed: %v", err)
	}

	if len(pub) == 0 {
		t.Error("GenerateEd25519KeyPair returned empty public key")
	}

	if len(priv) == 0 {
		t.Error("GenerateEd25519KeyPair returned empty private key")
	}
}

func TestSignVerifyEd25519(t *testing.T) {
	pub, priv, err := GenerateEd25519KeyPair()
	if err != nil {
		t.Fatalf("GenerateEd25519KeyPair failed: %v", err)
	}

	message := []byte("test message to sign")

	sig := SignEd25519(priv, message)
	if len(sig) == 0 {
		t.Error("SignEd25519 returned empty signature")
	}

	if !VerifyEd25519(pub, message, sig) {
		t.Error("VerifyEd25519 failed for valid signature")
	}

	if VerifyEd25519(pub, []byte("wrong message"), sig) {
		t.Error("VerifyEd25519 succeeded for wrong message")
	}
}

func TestHashPasswordArgon2id(t *testing.T) {
	password := "SecurePassword123!"

	hash, err := HashPasswordArgon2id(password)
	if err != nil {
		t.Fatalf("HashPasswordArgon2id failed: %v", err)
	}

	if len(hash) == 0 {
		t.Error("HashPasswordArgon2id returned empty hash")
	}

	ok, err := VerifyPasswordArgon2id(password, hash)
	if err != nil {
		t.Fatalf("VerifyPasswordArgon2id failed: %v", err)
	}
	if !ok {
		t.Error("VerifyPasswordArgon2id failed for correct password")
	}

	ok, err = VerifyPasswordArgon2id("wrong-password", hash)
	if err != nil {
		t.Fatalf("VerifyPasswordArgon2id failed: %v", err)
	}
	if ok {
		t.Error("VerifyPasswordArgon2id succeeded for wrong password")
	}
}

func TestConstantTimeEqual(t *testing.T) {
	a := []byte("test123")
	b := []byte("test123")
	c := []byte("test456")

	if !ConstantTimeEqual(a, b) {
		t.Error("ConstantTimeEqual failed for equal slices")
	}

	if ConstantTimeEqual(a, c) {
		t.Error("ConstantTimeEqual succeeded for different slices")
	}
}
