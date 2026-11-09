package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"

	"golang.org/x/crypto/hkdf"
)

const nonceSize = 12

// DeriveKey derives a per-user AES-256 key from the master key and user ID using HKDF-SHA256.
func DeriveKey(masterKey []byte, userID string) ([]byte, error) {
	reader := hkdf.New(sha256.New, masterKey, []byte(userID), []byte("daily-tasks-user-data"))
	key := make([]byte, 32)
	if _, err := io.ReadFull(reader, key); err != nil {
		return nil, err
	}
	return key, nil
}

// Encrypt encrypts plaintext with AES-256-GCM using the given key.
// Output format: [12-byte nonce][ciphertext+tag].
func Encrypt(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts a blob produced by Encrypt.
func Decrypt(key, blob []byte) ([]byte, error) {
	if len(blob) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce, ciphertext := blob[:nonceSize], blob[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}
