package store

import (
	"encoding/hex"
	"testing"
)

func TestSM3Deterministic(t *testing.T) {
	// SM3("abc") should always produce the same result
	h1 := NewSM3()
	h1.Write([]byte("abc"))
	hash1 := h1.Sum(nil)

	h2 := NewSM3()
	h2.Write([]byte("abc"))
	hash2 := h2.Sum(nil)

	if hex.EncodeToString(hash1) != hex.EncodeToString(hash2) {
		t.Errorf("SM3 is not deterministic: %x vs %x", hash1, hash2)
	}
}

func TestSM3MultiWriteConsistency(t *testing.T) {
	h1 := NewSM3()
	h1.Write([]byte("abcdef"))

	h2 := NewSM3()
	h2.Write([]byte("abc"))
	h2.Write([]byte("def"))

	if hex.EncodeToString(h1.Sum(nil)) != hex.EncodeToString(h2.Sum(nil)) {
		t.Errorf("SM3 multi-write mismatch")
	}
}

func TestSM3NonEmpty(t *testing.T) {
	h := NewSM3()
	h.Write([]byte("abc"))
	hash := h.Sum(nil)
	t.Logf("SM3(\"abc\") = %s", hex.EncodeToString(hash))
}

func TestSM3HelloWorld(t *testing.T) {
	h := NewSM3()
	h.Write([]byte("hello world"))
	hash := h.Sum(nil)
	t.Logf("SM3(\"hello world\") = %s", hex.EncodeToString(hash))
}

func TestSM3Empty(t *testing.T) {
	h := NewSM3()
	hash := h.Sum(nil)
	t.Logf("SM3(\"\") = %s", hex.EncodeToString(hash))
}

func TestSM3LongMsg(t *testing.T) {
	h := NewSM3()
	// 64-byte message (one full block)
	msg := []byte("abcdbcdecdefdefgefghfghighijhijkijkljklmklmnlmnomnopnopq")
	h.Write(msg)
	hash := h.Sum(nil)
	t.Logf("SM3(64-byte) = %s", hex.EncodeToString(hash))
}

func TestSM3DifferentInputs(t *testing.T) {
	// Different inputs should produce different hashes
	h1 := NewSM3()
	h1.Write([]byte("abc"))
	h2 := NewSM3()
	h2.Write([]byte("abd"))

	h1h := hex.EncodeToString(h1.Sum(nil))
	h2h := hex.EncodeToString(h2.Sum(nil))

	if h1h == h2h {
		t.Errorf("SM3 collision: abc and abd produced same hash %s", h1h)
	}
}

func TestSM4WrongKeyLength(t *testing.T) {
	// Key must be exactly 16 bytes
	_, err := NewSM4Cipher([]byte("0123456789abcdeffedcba9876543210")) // 32 bytes
	if err == nil {
		t.Errorf("Expected error for 32-byte SM4 key")
	}
}

func TestSM4EncryptDecrypt(t *testing.T) {
	key := []byte("0123456789abcdef")       // 16 bytes for SM4
	plaintext := []byte("0123456789abcdef") // exactly 16 bytes

	block, err := NewSM4Cipher(key)
	if err != nil {
		t.Fatalf("NewSM4Cipher failed: %v", err)
	}

	dst := make([]byte, 16)
	block.Encrypt(dst, plaintext)
	t.Logf("SM4 encrypt: %x", dst)

	decrypted := make([]byte, 16)
	block.Decrypt(decrypted, dst)
	if string(decrypted) != string(plaintext) {
		t.Errorf("SM4 decrypt failed: got %x, want %x", decrypted, plaintext)
	}
}

func TestSM4GCMRoundtrip(t *testing.T) {
	key := []byte("0123456789abcdef")
	plaintext := []byte("Hello, SM4-GCM! This is a test message.")

	ciphertext, err := encryptField(key, plaintext, "sm4")
	if err != nil {
		t.Fatalf("encryptField failed: %v", err)
	}

	decrypted, err := decryptField(key, ciphertext, "sm4")
	if err != nil {
		t.Fatalf("decryptField failed: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("SM4-GCM roundtrip: got %s, want %s", decrypted, plaintext)
	}
}
