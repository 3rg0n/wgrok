package wgrok

import (
	"encoding/json"
	"os"
	"testing"
)

type ircConnectionStringCase struct {
	Name          string     `json:"name"`
	Input         string     `json:"input"`
	Expected      *IRCParams `json:"expected"`
	ExpectedError bool       `json:"expected_error"`
}

type ircTestCases struct {
	ParseConnectionString []ircConnectionStringCase `json:"parse_connection_string"`
	SendMessage           struct {
		Description string `json:"description"`
	} `json:"send_message"`
}

func loadIRCTestCases(t *testing.T) ircTestCases {
	t.Helper()
	data, err := os.ReadFile("../tests/irc_cases.json")
	if err != nil {
		t.Fatalf("load irc cases: %v", err)
	}
	var cases ircTestCases
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatalf("parse irc cases: %v", err)
	}
	return cases
}

func TestParseIRCConnectionString(t *testing.T) {
	cases := loadIRCTestCases(t)
	for _, tc := range cases.ParseConnectionString {
		t.Run(tc.Name, func(t *testing.T) {
			result, err := ParseIRCConnectionString(tc.Input)

			if tc.ExpectedError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tc.Expected == nil {
				t.Fatal("test case missing expected value")
			}

			if result.Nick != tc.Expected.Nick {
				t.Errorf("nick = %q, want %q", result.Nick, tc.Expected.Nick)
			}
			if result.Password != tc.Expected.Password {
				t.Errorf("password = %q, want %q", result.Password, tc.Expected.Password)
			}
			if result.Server != tc.Expected.Server {
				t.Errorf("server = %q, want %q", result.Server, tc.Expected.Server)
			}
			if result.Port != tc.Expected.Port {
				t.Errorf("port = %d, want %d", result.Port, tc.Expected.Port)
			}
			if result.Channel != tc.Expected.Channel {
				t.Errorf("channel = %q, want %q", result.Channel, tc.Expected.Channel)
			}
		})
	}
}

func TestSendIRCMessage(t *testing.T) {
	result, err := SendIRCMessage("wgrok-bot:pass@irc.libera.chat:6697/#wgrok", "#wgrok", "test message")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["status"] != "sent" {
		t.Errorf("status = %v, want sent", result["status"])
	}
	if result["target"] != "#wgrok" {
		t.Errorf("target = %v, want #wgrok", result["target"])
	}
}

func TestSendIRCMessageInvalidConnStr(t *testing.T) {
	_, err := SendIRCMessage("invalid-no-at-sign", "#wgrok", "test message")
	if err == nil {
		t.Fatal("expected error for invalid connection string")
	}
}

func TestSendIRCCard(t *testing.T) {
	// IRC cards should fall back to text message
	card := map[string]interface{}{"type": "card"}
	result, err := SendIRCCard("wgrok-bot:pass@irc.libera.chat:6697/#wgrok", "#wgrok", "test message", card)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["status"] != "sent" {
		t.Errorf("status = %v, want sent", result["status"])
	}
}
