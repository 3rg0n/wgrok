package wgrok

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

type webexCases struct {
	ExtractCards []struct {
		Name     string                 `json:"name"`
		Message  map[string]interface{} `json:"message"`
		Expected []interface{}          `json:"expected"`
	} `json:"extract_cards"`
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
