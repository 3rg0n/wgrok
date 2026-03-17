package wgrok

import (
	"encoding/json"
	"os"
	"testing"
)

type codecCases struct {
	Roundtrips []struct {
		Input string `json:"input"`
	} `json:"roundtrips"`
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
