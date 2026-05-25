package crypto

import (
	"bytes"
	"testing"
)

func TestEncryptDecryptRoundtrip(t *testing.T) {
	masterKey := make([]byte, 32)
	for i := range masterKey {
		masterKey[i] = byte(i)
	}

	key, err := DeriveKey(masterKey, "user-123")
	if err != nil {
		t.Fatalf("DeriveKey: %v", err)
	}

	plaintext := []byte(`{"tasks":[],"version":1}`)
	blob, err := Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	got, err := Decrypt(key, blob)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	if !bytes.Equal(got, plaintext) {
		t.Fatalf("roundtrip mismatch: got %q want %q", got, plaintext)
	}
}

func TestEncryptProducesUniqueNonces(t *testing.T) {
	masterKey := make([]byte, 32)
	key, _ := DeriveKey(masterKey, "user-abc")
	plaintext := []byte("hello")

	blob1, _ := Encrypt(key, plaintext)
	blob2, _ := Encrypt(key, plaintext)

	if bytes.Equal(blob1, blob2) {
		t.Fatal("expected different ciphertexts due to random nonce")
	}
}

func TestDeriveKeyDifferentUsersGetDifferentKeys(t *testing.T) {
	masterKey := make([]byte, 32)
	k1, _ := DeriveKey(masterKey, "user-1")
	k2, _ := DeriveKey(masterKey, "user-2")

	if bytes.Equal(k1, k2) {
		t.Fatal("different users should get different derived keys")
	}
}

func TestDecryptWrongKeyFails(t *testing.T) {
	masterKey := make([]byte, 32)
	key1, _ := DeriveKey(masterKey, "user-1")
	key2, _ := DeriveKey(masterKey, "user-2")

	blob, _ := Encrypt(key1, []byte("secret"))
	_, err := Decrypt(key2, blob)
	if err == nil {
		t.Fatal("expected decryption with wrong key to fail")
	}
}

func TestDecryptTooShortBlob(t *testing.T) {
	key := make([]byte, 32)
	_, err := Decrypt(key, []byte("short"))
	if err == nil {
		t.Fatal("expected error for short blob")
	}
}
