package wgrok

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"
)

type codecCases struct {
	Roundtrips []struct {
		Input string `json:"input"`
	} `json:"roundtrips"`
	EncryptRoundtrips []struct {
		Input string `json:"input"`
	} `json:"encrypt_roundtrips"`
	EncryptTestKey string `json:"encrypt_test_key"`
	Chunking []struct {
		Input          string   `json:"input"`
		MaxSize        int      `json:"max_size"`
		ExpectedCount  int      `json:"expected_count"`
		ExpectedChunks []string `json:"expected_chunks"`
		Description    string   `json:"description"`
	} `json:"chunking"`
}

func loadCodecCases(t *testing.T) codecCases {
	t.Helper()
	data, err := os.ReadFile("../tests/codec_cases.json")
	if err != nil {
		t.Fatalf("load codec cases: %v", err)
	}
	var cases codecCases
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatalf("parse codec cases: %v", err)
	}
	return cases
}

func TestCodecRoundtrip(t *testing.T) {
	tc := loadCodecCases(t)
	for _, c := range tc.Roundtrips {
		t.Run(c.Input, func(t *testing.T) {
			compressed, err := Compress(c.Input)
			if err != nil {
				t.Fatalf("compress error: %v", err)
			}
			decompressed, err := Decompress(compressed)
			if err != nil {
				t.Fatalf("decompress error: %v", err)
			}
			if decompressed != c.Input {
				t.Errorf("roundtrip: got %q, want %q", decompressed, c.Input)
			}
		})
	}
}

func TestCodecChunking(t *testing.T) {
	tc := loadCodecCases(t)
	for _, c := range tc.Chunking {
		t.Run(c.Description, func(t *testing.T) {
			chunks, err := Chunk(c.Input, c.MaxSize)
			if err != nil {
				t.Fatalf("chunk error: %v", err)
			}
			if len(chunks) != c.ExpectedCount {
				t.Errorf("chunk count: got %d, want %d", len(chunks), c.ExpectedCount)
			}
			for i, expected := range c.ExpectedChunks {
				if i < len(chunks) && chunks[i] != expected {
					t.Errorf("chunk[%d]: got %q, want %q", i, chunks[i], expected)
				}
			}
		})
	}
}

func TestEncryptRoundtrip(t *testing.T) {
	tc := loadCodecCases(t)
	if tc.EncryptTestKey == "" {
		t.Fatal("encrypt_test_key not found in test data")
	}
	key, err := base64.StdEncoding.DecodeString(tc.EncryptTestKey)
	if err != nil {
		t.Fatalf("decode test key: %v", err)
	}
	if len(key) != 32 {
		t.Fatalf("test key must be 32 bytes, got %d", len(key))
	}

	for _, c := range tc.EncryptRoundtrips {
		t.Run(c.Input, func(t *testing.T) {
			encrypted, err := Encrypt(c.Input, key)
			if err != nil {
				t.Fatalf("encrypt error: %v", err)
			}
			decrypted, err := Decrypt(encrypted, key)
			if err != nil {
				t.Fatalf("decrypt error: %v", err)
			}
			if decrypted != c.Input {
				t.Errorf("roundtrip: got %q, want %q", decrypted, c.Input)
			}
		})
	}
}

func TestEncryptWrongKey(t *testing.T) {
	tc := loadCodecCases(t)
	if tc.EncryptTestKey == "" {
		t.Fatal("encrypt_test_key not found in test data")
	}
	key, err := base64.StdEncoding.DecodeString(tc.EncryptTestKey)
	if err != nil {
		t.Fatalf("decode test key: %v", err)
	}

	encrypted, err := Encrypt("hello", key)
	if err != nil {
		t.Fatalf("encrypt error: %v", err)
	}

	// Create wrong key (all zeros)
	wrongKey := make([]byte, 32)
	decrypted, err := Decrypt(encrypted, wrongKey)
	if err == nil {
		t.Fatalf("decrypt with wrong key should fail, got: %q", decrypted)
	}
}
