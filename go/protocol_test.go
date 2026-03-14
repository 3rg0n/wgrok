package wgrok

import (
	"encoding/json"
	"os"
	"testing"
)

type protocolCases struct {
	EchoPrefix string `json:"echo_prefix"`
	FormatEcho []struct {
		Slug     string `json:"slug"`
		Payload  string `json:"payload"`
		Expected string `json:"expected"`
	} `json:"format_echo"`
	ParseEcho struct {
		Valid []struct {
			Input   string `json:"input"`
			Slug    string `json:"slug"`
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
		Slug     string `json:"slug"`
		Payload  string `json:"payload"`
		Expected string `json:"expected"`
	} `json:"format_response"`
	ParseResponse struct {
		Valid []struct {
			Input   string `json:"input"`
			Slug    string `json:"slug"`
			Payload string `json:"payload"`
		} `json:"valid"`
		Errors []struct {
			Input         string `json:"input"`
			ErrorContains string `json:"error_contains"`
		} `json:"errors"`
	} `json:"parse_response"`
	Roundtrips struct {
		Echo []struct {
			Slug    string `json:"slug"`
			Payload string `json:"payload"`
		} `json:"echo"`
		Response []struct {
			Slug    string `json:"slug"`
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
		t.Run(tc.Slug+"_"+tc.Payload, func(t *testing.T) {
			got := FormatEcho(tc.Slug, tc.Payload)
			if got != tc.Expected {
				t.Errorf("FormatEcho(%q, %q) = %q, want %q", tc.Slug, tc.Payload, got, tc.Expected)
			}
		})
	}
}

func TestParseEcho(t *testing.T) {
	cases := loadProtocolCases(t)
	for _, tc := range cases.ParseEcho.Valid {
		t.Run(tc.Input, func(t *testing.T) {
			slug, payload, err := ParseEcho(tc.Input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if slug != tc.Slug {
				t.Errorf("slug = %q, want %q", slug, tc.Slug)
			}
			if payload != tc.Payload {
				t.Errorf("payload = %q, want %q", payload, tc.Payload)
			}
		})
	}
	for _, tc := range cases.ParseEcho.Errors {
		t.Run(tc.Input, func(t *testing.T) {
			_, _, err := ParseEcho(tc.Input)
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
		t.Run(tc.Slug+"_"+tc.Payload, func(t *testing.T) {
			got := FormatResponse(tc.Slug, tc.Payload)
			if got != tc.Expected {
				t.Errorf("FormatResponse(%q, %q) = %q, want %q", tc.Slug, tc.Payload, got, tc.Expected)
			}
		})
	}
}

func TestParseResponse(t *testing.T) {
	cases := loadProtocolCases(t)
	for _, tc := range cases.ParseResponse.Valid {
		t.Run(tc.Input, func(t *testing.T) {
			slug, payload, err := ParseResponse(tc.Input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if slug != tc.Slug {
				t.Errorf("slug = %q, want %q", slug, tc.Slug)
			}
			if payload != tc.Payload {
				t.Errorf("payload = %q, want %q", payload, tc.Payload)
			}
		})
	}
	for _, tc := range cases.ParseResponse.Errors {
		t.Run(tc.Input, func(t *testing.T) {
			_, _, err := ParseResponse(tc.Input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !containsInsensitive(err.Error(), tc.ErrorContains) {
				t.Errorf("error %q should contain %q", err.Error(), tc.ErrorContains)
			}
		})
	}
}

func TestEchoRoundtrip(t *testing.T) {
	cases := loadProtocolCases(t)
	for _, tc := range cases.Roundtrips.Echo {
		t.Run(tc.Slug, func(t *testing.T) {
			text := FormatEcho(tc.Slug, tc.Payload)
			slug, payload, err := ParseEcho(text)
			if err != nil {
				t.Fatalf("roundtrip error: %v", err)
			}
			if slug != tc.Slug || payload != tc.Payload {
				t.Errorf("roundtrip failed: got (%q, %q), want (%q, %q)", slug, payload, tc.Slug, tc.Payload)
			}
		})
	}
}

func TestResponseRoundtrip(t *testing.T) {
	cases := loadProtocolCases(t)
	for _, tc := range cases.Roundtrips.Response {
		t.Run(tc.Slug, func(t *testing.T) {
			text := FormatResponse(tc.Slug, tc.Payload)
			slug, payload, err := ParseResponse(text)
			if err != nil {
				t.Fatalf("roundtrip error: %v", err)
			}
			if slug != tc.Slug || payload != tc.Payload {
				t.Errorf("roundtrip failed: got (%q, %q), want (%q, %q)", slug, payload, tc.Slug, tc.Payload)
			}
		})
	}
}
