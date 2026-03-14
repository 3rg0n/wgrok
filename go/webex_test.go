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

type webexCases struct {
	ExtractCards []struct {
		Name     string                 `json:"name"`
		Message  map[string]interface{} `json:"message"`
		Expected []interface{}          `json:"expected"`
	} `json:"extract_cards"`
	RetryAfter struct {
		MaxRetries int `json:"max_retries"`
		Cases      []struct {
			Name      string `json:"name"`
			Responses []struct {
				Status     int                    `json:"status"`
				RetryAfter interface{}            `json:"retry_after"`
				Body       map[string]interface{} `json:"body"`
			} `json:"responses"`
			ExpectedResult       map[string]interface{} `json:"expected_result"`
			ExpectedAttempts     int                    `json:"expected_attempts"`
			ExpectedError        string                 `json:"expected_error_contains"`
			ExpectedSleepSeconds []int                  `json:"expected_sleep_seconds"`
		} `json:"cases"`
	} `json:"retry_after"`
}

func loadWebexCases(t *testing.T) webexCases {
	t.Helper()
	data, err := os.ReadFile("../tests/webex_cases.json")
	if err != nil {
		t.Fatalf("load webex cases: %v", err)
	}
	var cases webexCases
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatalf("parse webex cases: %v", err)
	}
	return cases
}

func overrideMessagesURL(t *testing.T, url string) {
	t.Helper()
	orig := WebexMessagesURL
	WebexMessagesURL = url
	t.Cleanup(func() { WebexMessagesURL = orig })
}

func overrideAttachmentActionsURL(t *testing.T, url string) {
	t.Helper()
	orig := WebexAttachmentActionsURL
	WebexAttachmentActionsURL = url
	t.Cleanup(func() { WebexAttachmentActionsURL = orig })
}

func TestExtractCards(t *testing.T) {
	cases := loadWebexCases(t)
	for _, tc := range cases.ExtractCards {
		t.Run(tc.Name, func(t *testing.T) {
			got := ExtractCards(tc.Message)
			if len(tc.Expected) == 0 {
				if len(got) != 0 {
					t.Errorf("expected empty, got %d cards", len(got))
				}
				return
			}
			if len(got) != len(tc.Expected) {
				t.Fatalf("got %d cards, want %d", len(got), len(tc.Expected))
			}
			gotJSON, _ := json.Marshal(got)
			expJSON, _ := json.Marshal(tc.Expected)
			if string(gotJSON) != string(expJSON) {
				t.Errorf("cards mismatch:\ngot:  %s\nwant: %s", gotJSON, expJSON)
			}
		})
	}
}

func TestSendMessageHTTP(t *testing.T) {
	var capturedBody map[string]interface{}
	var capturedAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"msg-1"}`))
	}))
	defer srv.Close()
	overrideMessagesURL(t, srv.URL)

	result, err := SendMessage("tok123", "user@example.com", "hello", srv.Client())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["id"] != "msg-1" {
		t.Errorf("result id = %v, want msg-1", result["id"])
	}
	if capturedAuth != "Bearer tok123" {
		t.Errorf("auth = %q, want %q", capturedAuth, "Bearer tok123")
	}
	if capturedBody["toPersonEmail"] != "user@example.com" {
		t.Errorf("toPersonEmail = %v, want user@example.com", capturedBody["toPersonEmail"])
	}
	if capturedBody["text"] != "hello" {
		t.Errorf("text = %v, want hello", capturedBody["text"])
	}
}

func TestSendMessageHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte(`{"message":"unauthorized"}`))
	}))
	defer srv.Close()
	overrideMessagesURL(t, srv.URL)

	_, err := SendMessage("badtoken", "user@example.com", "hello", srv.Client())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSendCardHTTP(t *testing.T) {
	var capturedBody map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"card-1"}`))
	}))
	defer srv.Close()
	overrideMessagesURL(t, srv.URL)

	card := map[string]interface{}{"type": "AdaptiveCard", "body": []interface{}{}}
	result, err := SendCard("tok", "user@x.com", "fallback", card, srv.Client())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["id"] != "card-1" {
		t.Errorf("result id = %v, want card-1", result["id"])
	}

	attachments, ok := capturedBody["attachments"].([]interface{})
	if !ok || len(attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %v", capturedBody["attachments"])
	}
	att := attachments[0].(map[string]interface{})
	if att["contentType"] != AdaptiveCardContentType {
		t.Errorf("contentType = %v, want %v", att["contentType"], AdaptiveCardContentType)
	}
}

func TestGetMessageHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"msg-1","text":"hello"}`))
	}))
	defer srv.Close()
	overrideMessagesURL(t, srv.URL)

	result, err := GetMessage("tok", "msg-1", srv.Client())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["id"] != "msg-1" {
		t.Errorf("result id = %v, want msg-1", result["id"])
	}
}

func TestGetAttachmentActionHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"act-1","type":"submit","inputs":{"name":"test"}}`))
	}))
	defer srv.Close()
	overrideAttachmentActionsURL(t, srv.URL)

	result, err := GetAttachmentAction("tok", "act-1", srv.Client())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["id"] != "act-1" {
		t.Errorf("result id = %v, want act-1", result["id"])
	}
	if result["type"] != "submit" {
		t.Errorf("result type = %v, want submit", result["type"])
	}
}

func TestRetryAfter(t *testing.T) {
	cases := loadWebexCases(t)
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
			overrideMessagesURL(t, srv.URL)

			result, err := SendMessage("tok", "user@example.com", "test", srv.Client())

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
