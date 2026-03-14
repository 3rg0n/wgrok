package wgrok

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

type platformDispatchCase struct {
	Name           string `json:"name"`
	Platform       string `json:"platform"`
	ExpectedModule string `json:"expected_module"`
	ExpectedError  bool   `json:"expected_error"`
}

type platformTestCases struct {
	Description string                 `json:"description"`
	Cases       []platformDispatchCase `json:"cases"`
}

func loadPlatformTestCases(t *testing.T) platformTestCases {
	t.Helper()
	data, err := os.ReadFile("../tests/platform_dispatch_cases.json")
	if err != nil {
		t.Fatalf("load platform cases: %v", err)
	}
	var cases platformTestCases
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatalf("parse platform cases: %v", err)
	}
	return cases
}

func TestPlatformSendMessageDispatch(t *testing.T) {
	cases := loadPlatformTestCases(t)

	for _, tc := range cases.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			// Skip card tests for send_message tests
			if tc.ExpectedModule == "" && !tc.ExpectedError {
				t.Skip()
			}

			var capturedPlatform string

			// Create mock servers for different platforms
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"id":"msg-1"}`))
				_ = body
			}))
			defer srv.Close()

			// Setup overrides based on platform
			switch tc.Platform {
			case "webex":
				capturedPlatform = "webex"
				overrideMessagesURL(t, srv.URL)
			case "slack":
				capturedPlatform = "slack"
				overrideSlackPostMessageURL(t, srv.URL)
			case "discord":
				capturedPlatform = "discord"
				overrideDiscordChannelMessagesURL(t, func(channelID string) string {
					return srv.URL
				})
			case "irc":
				capturedPlatform = "irc"
			}

			// Dispatch based on platform
			token := "test-token"
			if tc.Platform == "irc" {
				token = "wgrok-bot@irc.libera.chat:6697/#test"
			}
			result, err := PlatformSendMessage(tc.Platform, token, "test-target", "test message", srv.Client())

			if tc.ExpectedError {
				if err == nil {
					t.Errorf("expected error for platform %q, got nil", tc.Platform)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error for platform %q: %v", tc.Platform, err)
				}
				if result == nil {
					t.Errorf("expected result for platform %q, got nil", tc.Platform)
				}
				if capturedPlatform != tc.ExpectedModule {
					t.Errorf("platform = %q, want %q", capturedPlatform, tc.ExpectedModule)
				}
			}
		})
	}
}

func TestPlatformSendCardDispatch(t *testing.T) {
	// Test that PlatformSendCard routes to correct implementations
	platforms := []string{"webex", "slack", "discord", "irc"}

	for _, platform := range platforms {
		t.Run(platform, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"id":"msg-1"}`))
			}))
			defer srv.Close()

			// Setup overrides
			switch platform {
			case "webex":
				overrideMessagesURL(t, srv.URL)
			case "slack":
				overrideSlackPostMessageURL(t, srv.URL)
			case "discord":
				overrideDiscordChannelMessagesURL(t, func(channelID string) string {
					return srv.URL
				})
			}

			card := map[string]interface{}{"type": "test"}
			token := "test-token"
			if platform == "irc" {
				token = "wgrok-bot@irc.libera.chat:6697/#test"
			}
			result, err := PlatformSendCard(platform, token, "test-target", "test message", card, srv.Client())

			if err != nil {
				t.Errorf("unexpected error for platform %q: %v", platform, err)
			}
			if result == nil {
				t.Errorf("expected result for platform %q, got nil", platform)
			}
		})
	}
}

func TestPlatformUnsupportedPlatform(t *testing.T) {
	_, err := PlatformSendMessage("unsupported", "token", "target", "text", nil)
	if err == nil {
		t.Fatal("expected error for unsupported platform")
	}
	if !strings.Contains(err.Error(), "unsupported platform") {
		t.Errorf("error = %v, want to contain 'unsupported platform'", err)
	}

	_, err = PlatformSendCard("unsupported", "token", "target", "text", nil, nil)
	if err == nil {
		t.Fatal("expected error for unsupported platform")
	}
	if !strings.Contains(err.Error(), "unsupported platform") {
		t.Errorf("error = %v, want to contain 'unsupported platform'", err)
	}
}
