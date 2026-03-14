package wgrok

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

type senderCases struct {
	Config struct {
		Token  string `json:"token"`
		Target string `json:"target"`
		Slug   string `json:"slug"`
	} `json:"config"`
	Cases []struct {
		Name             string      `json:"name"`
		Payload          string      `json:"payload"`
		Card             interface{} `json:"card"`
		ExpectedText     string      `json:"expected_text"`
		ExpectedTarget   string      `json:"expected_target"`
		ExpectedUsesCard bool        `json:"expected_uses_card"`
	} `json:"cases"`
}

func loadSenderCases(t *testing.T) senderCases {
	t.Helper()
	data, err := os.ReadFile("../tests/sender_cases.json")
	if err != nil {
		t.Fatalf("load sender cases: %v", err)
	}
	var cases senderCases
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatalf("parse sender cases: %v", err)
	}
	return cases
}

func TestWgrokSender(t *testing.T) {
	tc := loadSenderCases(t)

	for _, c := range tc.Cases {
		t.Run(c.Name, func(t *testing.T) {
			var capturedBody map[string]interface{}

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				json.Unmarshal(body, &capturedBody)
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"id":"msg-1"}`))
			}))
			defer srv.Close()
			overrideMessagesURL(t, srv.URL)

			sender := NewSender(&SenderConfig{
				WebexToken: tc.Config.Token,
				Target:     tc.Config.Target,
				Slug:       tc.Config.Slug,
				Platform:   "webex",
			})
			sender.client = srv.Client()

			var card interface{}
			if c.Card != nil {
				card = c.Card
			}

			result, err := sender.Send(c.Payload, card)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result["id"] != "msg-1" {
				t.Errorf("result id = %v, want msg-1", result["id"])
			}
			if capturedBody["text"] != c.ExpectedText {
				t.Errorf("text = %v, want %v", capturedBody["text"], c.ExpectedText)
			}
			if capturedBody["toPersonEmail"] != c.ExpectedTarget {
				t.Errorf("toPersonEmail = %v, want %v", capturedBody["toPersonEmail"], c.ExpectedTarget)
			}

			attachments, hasAttachments := capturedBody["attachments"].([]interface{})
			if c.ExpectedUsesCard {
				if !hasAttachments || len(attachments) == 0 {
					t.Error("expected card attachment, got none")
				}
			} else {
				if hasAttachments && len(attachments) > 0 {
					t.Error("expected no card attachment, got one")
				}
			}
		})
	}
}
