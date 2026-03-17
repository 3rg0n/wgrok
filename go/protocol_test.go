package wgrok

import (
	"encoding/json"
	"os"
	"testing"
)

type protocolCases struct {
	EchoPrefix string `json:"echo_prefix"`
	FormatEcho []struct {
		To       string `json:"to"`
		From     string `json:"from"`
		Flags    string `json:"flags"`
		Payload  string `json:"payload"`
		Expected string `json:"expected"`
	} `json:"format_echo"`
	ParseEcho struct {
		Valid []struct {
			Input   string `json:"input"`
			To      string `json:"to"`
			From    string `json:"from"`
			Flags   string `json:"flags"`
			Payload string `json:"payload"`
		} `json:"valid"`
		Errors []struct {
			Input         string `json:"input"`
			ErrorContains string `json:"error_contains"`
		} `json:"errors"`
	} `json:"parse_echo"`
	IsEcho []struct {
		Input    string `json:"input"`
		Expected bool   `json:"expected"`
	} `json:"is_echo"`
	FormatResponse []struct {
		To       string `json:"to"`
		From     string `json:"from"`
		Flags    string `json:"flags"`
		Payload  string `json:"payload"`
		Expected string `json:"expected"`
	} `json:"format_response"`
	ParseResponse struct {
		Valid []struct {
			Input   string `json:"input"`
			To      string `json:"to"`
			From    string `json:"from"`
			Flags   string `json:"flags"`
			Payload string `json:"payload"`
		} `json:"valid"`
		Errors []struct {
			Input         string `json:"input"`
			ErrorContains string `json:"error_contains"`
		} `json:"errors"`
	} `json:"parse_response"`
	ParseFlags []struct {
		Input       string `json:"input"`
		Compressed  bool   `json:"compressed"`
		ChunkSeq    *int   `json:"chunk_seq"`
		ChunkTotal  *int   `json:"chunk_total"`
	} `json:"parse_flags"`
	FormatFlags []struct {
		Compressed bool   `json:"compressed"`
		ChunkSeq   *int   `json:"chunk_seq"`
		ChunkTotal *int   `json:"chunk_total"`
		Expected   string `json:"expected"`
	} `json:"format_flags"`
	Roundtrips struct {
		Echo []struct {
			To      string `json:"to"`
			From    string `json:"from"`
			Flags   string `json:"flags"`
			Payload string `json:"payload"`
		} `json:"echo"`
		Response []struct {
			To      string `json:"to"`
			From    string `json:"from"`
			Flags   string `json:"flags"`
			Payload string `json:"payload"`
		} `json:"response"`
	} `json:"roundtrips"`
}

func loadProtocolCases(t *testing.T) protocolCases {
	t.Helper()
	data, err := os.ReadFile("../tests/protocol_cases.json")
	if err != nil {
		t.Fatalf("load protocol cases: %v", err)
	}
	var cases protocolCases
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatalf("parse protocol cases: %v", err)
	}
	return cases
}

func TestEchoPrefix(t *testing.T) {
	cases := loadProtocolCases(t)
	if EchoPrefix != cases.EchoPrefix {
		t.Errorf("EchoPrefix = %q, want %q", EchoPrefix, cases.EchoPrefix)
	}
}

func TestFormatEcho(t *testing.T) {
	cases := loadProtocolCases(t)
	for _, tc := range cases.FormatEcho {
		t.Run(tc.To+"_"+tc.From+"_"+tc.Flags, func(t *testing.T) {
			got := FormatEcho(tc.To, tc.From, tc.Flags, tc.Payload)
			if got != tc.Expected {
				t.Errorf("FormatEcho(%q, %q, %q, %q) = %q, want %q", tc.To, tc.From, tc.Flags, tc.Payload, got, tc.Expected)
			}
		})
	}
}

func TestParseEcho(t *testing.T) {
	cases := loadProtocolCases(t)
	for _, tc := range cases.ParseEcho.Valid {
		t.Run(tc.Input, func(t *testing.T) {
			to, from, flags, payload, err := ParseEcho(tc.Input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if to != tc.To {
				t.Errorf("to = %q, want %q", to, tc.To)
			}
			if from != tc.From {
				t.Errorf("from = %q, want %q", from, tc.From)
			}
			if flags != tc.Flags {
				t.Errorf("flags = %q, want %q", flags, tc.Flags)
			}
			if payload != tc.Payload {
				t.Errorf("payload = %q, want %q", payload, tc.Payload)
			}
		})
	}
	for _, tc := range cases.ParseEcho.Errors {
		t.Run(tc.Input, func(t *testing.T) {
			_, _, _, _, err := ParseEcho(tc.Input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !containsInsensitive(err.Error(), tc.ErrorContains) {
				t.Errorf("error %q should contain %q", err.Error(), tc.ErrorContains)
			}
		})
	}
}

func TestIsEcho(t *testing.T) {
	cases := loadProtocolCases(t)
	for _, tc := range cases.IsEcho {
		t.Run(tc.Input, func(t *testing.T) {
			got := IsEcho(tc.Input)
			if got != tc.Expected {
				t.Errorf("IsEcho(%q) = %v, want %v", tc.Input, got, tc.Expected)
			}
		})
	}
}

func TestFormatResponse(t *testing.T) {
	cases := loadProtocolCases(t)
	for _, tc := range cases.FormatResponse {
		t.Run(tc.To+"_"+tc.From+"_"+tc.Flags, func(t *testing.T) {
			got := FormatResponse(tc.To, tc.From, tc.Flags, tc.Payload)
			if got != tc.Expected {
				t.Errorf("FormatResponse(%q, %q, %q, %q) = %q, want %q", tc.To, tc.From, tc.Flags, tc.Payload, got, tc.Expected)
			}
		})
	}
}

func TestParseResponse(t *testing.T) {
	cases := loadProtocolCases(t)
	for _, tc := range cases.ParseResponse.Valid {
		t.Run(tc.Input, func(t *testing.T) {
			to, from, flags, payload, err := ParseResponse(tc.Input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if to != tc.To {
				t.Errorf("to = %q, want %q", to, tc.To)
			}
			if from != tc.From {
				t.Errorf("from = %q, want %q", from, tc.From)
			}
			if flags != tc.Flags {
				t.Errorf("flags = %q, want %q", flags, tc.Flags)
			}
			if payload != tc.Payload {
				t.Errorf("payload = %q, want %q", payload, tc.Payload)
			}
		})
	}
	for _, tc := range cases.ParseResponse.Errors {
		t.Run(tc.Input, func(t *testing.T) {
			_, _, _, _, err := ParseResponse(tc.Input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !containsInsensitive(err.Error(), tc.ErrorContains) {
				t.Errorf("error %q should contain %q", err.Error(), tc.ErrorContains)
			}
		})
	}
}

func TestParseFlags(t *testing.T) {
	cases := loadProtocolCases(t)
	for _, tc := range cases.ParseFlags {
		t.Run(tc.Input, func(t *testing.T) {
			compressed, chunkSeq, chunkTotal, err := ParseFlags(tc.Input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if compressed != tc.Compressed {
				t.Errorf("compressed = %v, want %v", compressed, tc.Compressed)
			}
			var expectedSeq, expectedTotal int
			if tc.ChunkSeq != nil {
				expectedSeq = *tc.ChunkSeq
			}
			if tc.ChunkTotal != nil {
				expectedTotal = *tc.ChunkTotal
			}
			if chunkSeq != expectedSeq {
				t.Errorf("chunkSeq = %d, want %d", chunkSeq, expectedSeq)
			}
			if chunkTotal != expectedTotal {
				t.Errorf("chunkTotal = %d, want %d", chunkTotal, expectedTotal)
			}
		})
	}
}

func TestFormatFlags(t *testing.T) {
	cases := loadProtocolCases(t)
	for _, tc := range cases.FormatFlags {
		var seq, total int
		if tc.ChunkSeq != nil {
			seq = *tc.ChunkSeq
		}
		if tc.ChunkTotal != nil {
			total = *tc.ChunkTotal
		}
		t.Run(tc.Expected, func(t *testing.T) {
			got := FormatFlags(tc.Compressed, seq, total)
			if got != tc.Expected {
				t.Errorf("FormatFlags(%v, %d, %d) = %q, want %q", tc.Compressed, seq, total, got, tc.Expected)
			}
		})
	}
}

func TestEchoRoundtrip(t *testing.T) {
	cases := loadProtocolCases(t)
	for _, tc := range cases.Roundtrips.Echo {
		t.Run(tc.To, func(t *testing.T) {
			text := FormatEcho(tc.To, tc.From, tc.Flags, tc.Payload)
			to, from, flags, payload, err := ParseEcho(text)
			if err != nil {
				t.Fatalf("roundtrip error: %v", err)
			}
			if to != tc.To || from != tc.From || flags != tc.Flags || payload != tc.Payload {
				t.Errorf("roundtrip failed: got (%q, %q, %q, %q), want (%q, %q, %q, %q)", to, from, flags, payload, tc.To, tc.From, tc.Flags, tc.Payload)
			}
		})
	}
}

func TestResponseRoundtrip(t *testing.T) {
	cases := loadProtocolCases(t)
	for _, tc := range cases.Roundtrips.Response {
		t.Run(tc.To, func(t *testing.T) {
			text := FormatResponse(tc.To, tc.From, tc.Flags, tc.Payload)
			to, from, flags, payload, err := ParseResponse(text)
			if err != nil {
				t.Fatalf("roundtrip error: %v", err)
			}
			if to != tc.To || from != tc.From || flags != tc.Flags || payload != tc.Payload {
				t.Errorf("roundtrip failed: got (%q, %q, %q, %q), want (%q, %q, %q, %q)", to, from, flags, payload, tc.To, tc.From, tc.Flags, tc.Payload)
			}
		})
	}
}
