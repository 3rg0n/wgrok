package wgrok

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
)

type discordTestCases struct {
	SendMessage struct {
		ExpectedURLPattern    string   `json:"expected_url_pattern"`
		ExpectedPayloadFields []string `json:"expected_payload_fields"`
		ExpectedAuthPrefix    string   `json:"expected_auth_header_prefix"`
	} `json:"send_message"`
	SendCard struct {
		ExpectedExtraField string `json:"expected_extra_field"`
	} `json:"send_card"`
	RetryAfter struct {
		MaxRetries int `json:"max_retries"`
		Cases      []struct {
			Name      string `json:"name"`
			Responses []struct {
				Status     int                    `json:"status"`
				RetryAfter interface{}            `json:"retry_after"`
				Body       map[string]interface{} `json:"body"`
			} `json:"responses"`
			ExpectedResult   map[string]interface{} `json:"expected_result"`
			ExpectedAttempts int                    `json:"expected_attempts"`
			ExpectedError    string                 `json:"expected_error_contains"`
		} `json:"cases"`
	} `json:"retry_after"`
}

func loadDiscordTestCases(t *testing.T) discordTestCases {
	t.Helper()
	data, err := os.ReadFile("../tests/discord_cases.json")
	if err != nil {
		t.Fatalf("load discord cases: %v", err)
	}
	var cases discordTestCases
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatalf("parse discord cases: %v", err)
	}
	return cases
}

// We need to override the URL builder - create a test variable for it
var testDiscordChannelMessagesURL func(channelID string) string

func overrideDiscordChannelMessagesURL(t *testing.T, urlFunc func(channelID string) string) {
	t.Helper()
	orig := DiscordChannelMessagesURL
	DiscordChannelMessagesURL = urlFunc
	t.Cleanup(func() { DiscordChannelMessagesURL = orig })
}

func TestSendDiscordMessageHTTP(t *testing.T) {
	var capturedBody map[string]interface{}
	var capturedAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"msg-discord-1","content":"hello"}`))
	}))
	defer srv.Close()

	// Override URL builder to use test server
	overrideDiscordChannelMessagesURL(t, func(channelID string) string {
		return srv.URL
	})

	result, err := SendDiscordMessage("bot-token-123", "123456", "hello", srv.Client())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["id"] != "msg-discord-1" {
		t.Errorf("result id = %v, want msg-discord-1", result["id"])
	}
	if capturedAuth != "Bot bot-token-123" {
		t.Errorf("auth = %q, want %q", capturedAuth, "Bot bot-token-123")
	}
	if capturedBody["content"] != "hello" {
		t.Errorf("content = %v, want hello", capturedBody["content"])
	}
}

func TestSendDiscordMessageHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte(`{"message":"Unauthorized"}`))
	}))
	defer srv.Close()

	overrideDiscordChannelMessagesURL(t, func(channelID string) string {
		return srv.URL
	})

	_, err := SendDiscordMessage("badtoken", "123456", "hello", srv.Client())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSendDiscordCardHTTP(t *testing.T) {
	var capturedBody map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"msg-embed-1"}`))
	}))
	defer srv.Close()

	overrideDiscordChannelMessagesURL(t, func(channelID string) string {
		return srv.URL
	})

	embed := map[string]interface{}{"title": "Card Title", "description": "Card description"}
	result, err := SendDiscordCard("tok", "123456", "fallback", embed, srv.Client())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["id"] != "msg-embed-1" {
		t.Errorf("result id = %v, want msg-embed-1", result["id"])
	}

	embeds, ok := capturedBody["embeds"].([]interface{})
	if !ok || len(embeds) != 1 {
		t.Fatalf("expected 1 embed, got %v", capturedBody["embeds"])
	}
	embedMap := embeds[0].(map[string]interface{})
	if embedMap["title"] != "Card Title" {
		t.Errorf("embed title = %v, want Card Title", embedMap["title"])
	}
}

func TestDiscordRetryAfter(t *testing.T) {
	cases := loadDiscordTestCases(t)
	for _, tc := range cases.RetryAfter.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			responseIndex := 0
			var mu sync.Mutex

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()

				if responseIndex >= len(tc.Responses) {
					t.Errorf("unexpected request: response index %d >= %d", responseIndex, len(tc.Responses))
					w.WriteHeader(500)
					return
				}

				resp := tc.Responses[responseIndex]
				responseIndex++

				w.Header().Set("Content-Type", "application/json")
				if resp.RetryAfter != nil {
					w.Header().Set("Retry-After", resp.RetryAfter.(string))
				}
				w.WriteHeader(resp.Status)

				if resp.Body != nil {
					body, _ := json.Marshal(resp.Body)
					w.Write(body)
				} else {
					w.Write([]byte("{}"))
				}
			}))
			defer srv.Close()

			overrideDiscordChannelMessagesURL(t, func(channelID string) string {
				return srv.URL
			})

			result, err := SendDiscordMessage("tok", "123456", "test", srv.Client())

			if tc.ExpectedResult != nil {
				if err != nil {
					t.Errorf("expected success, got error: %v", err)
				}
				if result == nil || result["id"] != tc.ExpectedResult["id"] {
					t.Errorf("result id = %v, want %v", result["id"], tc.ExpectedResult["id"])
				}
			}

			if tc.ExpectedError != "" {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tc.ExpectedError)
				} else if !strings.Contains(err.Error(), tc.ExpectedError) {
					t.Errorf("error = %v, want to contain %q", err, tc.ExpectedError)
				}
			}

			if tc.ExpectedAttempts > 0 {
				if responseIndex != tc.ExpectedAttempts {
					t.Errorf("expected %d attempts, got %d", tc.ExpectedAttempts, responseIndex)
				}
			}
		})
	}
}
