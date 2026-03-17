package wgrok

import (
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

// Compress gzip-compresses data and returns a base64 string (no prefix).
func Compress(data string) (string, error) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write([]byte(data)); err != nil {
		return "", fmt.Errorf("gzip write: %w", err)
	}
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("gzip close: %w", err)
	}
	b64 := base64.StdEncoding.EncodeToString(buf.Bytes())
	return b64, nil
}

// Decompress attempts to base64-decode and gzip-decompress data.
// If decompression fails, returns the data unchanged (passthrough).
func Decompress(data string) (string, error) {
	compressed, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		// Not valid base64, passthrough
		return data, nil
	}
	r, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		// Not valid gzip, passthrough
		return data, nil
	}
	defer r.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		// Gzip read failed, passthrough
		return data, nil
	}
	return string(out), nil
}

// Encrypt encrypts data using AES-256-GCM with a random 12-byte IV.
// Returns base64(IV || ciphertext_with_tag).
func Encrypt(data string, key []byte) (string, error) {
	if len(key) != 32 {
		return "", fmt.Errorf("encryption key must be 32 bytes, got %d", len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("aes cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("gcm: %w", err)
	}

	// Generate 12-byte random IV (nonce)
	iv := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(iv); err != nil {
		return "", fmt.Errorf("rand iv: %w", err)
	}

	// Seal appends the 16-byte authentication tag automatically
	ciphertext := gcm.Seal(iv, iv, []byte(data), nil)

	// Encode to base64: IV || ciphertext || tag
	b64 := base64.StdEncoding.EncodeToString(ciphertext)
	return b64, nil
}

// Decrypt decrypts data encrypted with Encrypt.
// Expects base64(IV || ciphertext_with_tag).
func Decrypt(data string, key []byte) (string, error) {
	if len(key) != 32 {
		return "", fmt.Errorf("encryption key must be 32 bytes, got %d", len(key))
	}

	// Decode from base64
	ciphertext, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("aes cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("gcm: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	// Extract IV and ciphertext_with_tag
	iv := ciphertext[:nonceSize]
	sealed := ciphertext[nonceSize:]

	// Open verifies the tag and decrypts
	plaintext, err := gcm.Open(nil, iv, sealed, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}

	return string(plaintext), nil
}

// Chunk splits payload into raw chunks of at most maxSize bytes each.
// Returns just the raw chunk data (no N/T: prefix).
func Chunk(payload string, maxSize int) ([]string, error) {
	if maxSize <= 0 {
		return nil, fmt.Errorf("maxSize must be positive")
	}
	total := (len(payload) + maxSize - 1) / maxSize
	if total == 0 {
		total = 1
	}
	chunks := make([]string, 0, total)
	for i := 0; i < total; i++ {
		start := i * maxSize
		end := start + maxSize
		if end > len(payload) {
			end = len(payload)
		}
		chunkData := payload[start:end]
		chunks = append(chunks, chunkData)
	}
	return chunks, nil
}
