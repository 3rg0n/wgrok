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

type slackTestCases struct {
	SendMessage struct {
		ExpectedURL           string   `json:"expected_url"`
		ExpectedPayloadFields []string `json:"expected_payload_fields"`
		ExpectedAuthPrefix    string   `json:"expected_auth_header_prefix"`
	} `json:"send_message"`
	SendCard struct {
		ExpectedURL        string `json:"expected_url"`
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

func loadSlackTestCases(t *testing.T) slackTestCases {
	t.Helper()
	data, err := os.ReadFile("../tests/slack_cases.json")
	if err != nil {
		t.Fatalf("load slack cases: %v", err)
	}
	var cases slackTestCases
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatalf("parse slack cases: %v", err)
	}
	return cases
}

func overrideSlackPostMessageURL(t *testing.T, url string) {
	t.Helper()
	orig := SlackPostMessageURL
	SlackPostMessageURL = url
	t.Cleanup(func() { SlackPostMessageURL = orig })
}

func TestSendSlackMessageHTTP(t *testing.T) {
	var capturedBody map[string]interface{}
	var capturedAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true,"ts":"1234.5678"}`))
	}))
	defer srv.Close()
	overrideSlackPostMessageURL(t, srv.URL)

	result, err := SendSlackMessage("tok-slack-123", "C123456", "hello", srv.Client())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result["ok"].(bool) {
		t.Errorf("result ok = %v, want true", result["ok"])
	}
	if capturedAuth != "Bearer tok-slack-123" {
		t.Errorf("auth = %q, want %q", capturedAuth, "Bearer tok-slack-123")
	}
	if capturedBody["channel"] != "C123456" {
		t.Errorf("channel = %v, want C123456", capturedBody["channel"])
	}
	if capturedBody["text"] != "hello" {
		t.Errorf("text = %v, want hello", capturedBody["text"])
	}
}

func TestSendSlackMessageHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte(`{"ok":false,"error":"invalid_auth"}`))
	}))
	defer srv.Close()
	overrideSlackPostMessageURL(t, srv.URL)

	_, err := SendSlackMessage("badtoken", "C123456", "hello", srv.Client())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSendSlackCardHTTP(t *testing.T) {
	var capturedBody map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true,"ts":"1234.5678"}`))
	}))
	defer srv.Close()
	overrideSlackPostMessageURL(t, srv.URL)

	card := map[string]interface{}{"type": "section", "text": map[string]interface{}{"type": "mrkdwn", "text": "card text"}}
	result, err := SendSlackCard("tok", "C123456", "fallback", card, srv.Client())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result["ok"].(bool) {
		t.Errorf("result ok = %v, want true", result["ok"])
	}

	blocks, ok := capturedBody["blocks"].([]interface{})
	if !ok || len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %v", capturedBody["blocks"])
	}
	blockMap := blocks[0].(map[string]interface{})
	if blockMap["type"] != "section" {
		t.Errorf("block type = %v, want section", blockMap["type"])
	}
}

func TestSlackRetryAfter(t *testing.T) {
	cases := loadSlackTestCases(t)
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
			overrideSlackPostMessageURL(t, srv.URL)

			result, err := SendSlackMessage("tok", "C123456", "test", srv.Client())

			if tc.ExpectedResult != nil {
				if err != nil {
					t.Errorf("expected success, got error: %v", err)
				}
				if result == nil || result["ts"] != tc.ExpectedResult["ts"] {
					t.Errorf("result ts = %v, want %v", result["ts"], tc.ExpectedResult["ts"])
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
