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
		Token     string `json:"token"`
		Target    string `json:"target"`
		Slug      string `json:"slug"`
		FromSlug  string `json:"from_slug"`
	} `json:"config"`
	Cases []struct {
		Name             string      `json:"name"`
		Payload          string      `json:"payload"`
		Card             interface{} `json:"card"`
		Compress         bool        `json:"compress"`
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
			if result.MessageID != "msg-1" {
				t.Errorf("result MessageID = %v, want msg-1", result.MessageID)
			}
			if result.Buffered {
				t.Error("result should not be buffered")
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

func TestSenderPause(t *testing.T) {
	sendCount := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sendCount++
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"msg-1"}`))
	}))
	defer srv.Close()
	overrideMessagesURL(t, srv.URL)

	sender := NewSender(&SenderConfig{
		WebexToken: "fake-token",
		Target:     "target@webex.bot",
		Slug:       "myslug",
		Platform:   "webex",
	})
	sender.client = srv.Client()

	// Pause without notification
	err := sender.Pause(false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Send should buffer, not send
	result, err := sender.Send("hello", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Buffered {
		t.Error("expected result to indicate Buffered=true")
	}
	if result.MessageID != "" {
		t.Errorf("expected empty MessageID for buffered send, got %s", result.MessageID)
	}

	// No sends should have happened
	if sendCount != 0 {
		t.Errorf("expected 0 sends, got %d", sendCount)
	}

	// Check sender is paused
	sender.pauseMu.Lock()
	if !sender.paused {
		t.Error("expected sender to be paused")
	}
	if len(sender.buffer) != 1 {
		t.Errorf("expected 1 buffered message, got %d", len(sender.buffer))
	}
	sender.pauseMu.Unlock()
}

func TestSenderResume(t *testing.T) {
	sendCount := 0
	var capturedTexts []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sendCount++
		var body map[string]interface{}
		var buffer [4096]byte
		n, _ := r.Body.Read(buffer[:])
		json.Unmarshal(buffer[:n], &body)
		if text, ok := body["text"].(string); ok {
			capturedTexts = append(capturedTexts, text)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"msg-1"}`))
	}))
	defer srv.Close()
	overrideMessagesURL(t, srv.URL)

	sender := NewSender(&SenderConfig{
		WebexToken: "fake-token",
		Target:     "target@webex.bot",
		Slug:       "myslug",
		Platform:   "webex",
	})
	sender.client = srv.Client()

	// Pause without notification
	sender.Pause(false)

	// Buffer a message
	sender.Send("msg1", nil)
	sender.Send("msg2", nil)

	// Resume with notification
	err := sender.Resume(true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have sent resume + 2 buffered messages = 3 sends
	if sendCount != 3 {
		t.Errorf("expected 3 sends (resume + 2 buffered), got %d", sendCount)
	}

	// First send should be the resume command
	if sendCount > 0 && capturedTexts[0] != "./resume" {
		t.Errorf("first send expected ./resume, got %s", capturedTexts[0])
	}

	// Check sender is no longer paused
	sender.pauseMu.Lock()
	if sender.paused {
		t.Error("expected sender to not be paused after resume")
	}
	if len(sender.buffer) != 0 {
		t.Errorf("expected 0 buffered messages after resume, got %d", len(sender.buffer))
	}
	sender.pauseMu.Unlock()
}

func TestSenderPauseNotify(t *testing.T) {
	sendCount := 0
	var capturedText string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sendCount++
		var body map[string]interface{}
		var buffer [4096]byte
		n, _ := r.Body.Read(buffer[:])
		json.Unmarshal(buffer[:n], &body)
		if text, ok := body["text"].(string); ok {
			capturedText = text
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"msg-1"}`))
	}))
	defer srv.Close()
	overrideMessagesURL(t, srv.URL)

	sender := NewSender(&SenderConfig{
		WebexToken: "fake-token",
		Target:     "target@webex.bot",
		Slug:       "myslug",
		Platform:   "webex",
	})
	sender.client = srv.Client()

	// Pause with notification
	err := sender.Pause(true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have sent pause command
	if sendCount != 1 {
		t.Errorf("expected 1 send for pause notification, got %d", sendCount)
	}

	if capturedText != "./pause" {
		t.Errorf("expected pause notification text ./pause, got %s", capturedText)
	}
}

func TestSenderResumeNotify(t *testing.T) {
	sendCount := 0
	var capturedTexts []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sendCount++
		var body map[string]interface{}
		var buffer [4096]byte
		n, _ := r.Body.Read(buffer[:])
		json.Unmarshal(buffer[:n], &body)
		if text, ok := body["text"].(string); ok {
			capturedTexts = append(capturedTexts, text)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"msg-1"}`))
	}))
	defer srv.Close()
	overrideMessagesURL(t, srv.URL)

	sender := NewSender(&SenderConfig{
		WebexToken: "fake-token",
		Target:     "target@webex.bot",
		Slug:       "myslug",
		Platform:   "webex",
	})
	sender.client = srv.Client()

	// Pause and buffer
	sender.Pause(false)
	sender.Send("msg1", nil)

	// Resume without notification
	err := sender.Resume(false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have only 1 send (buffered message, no resume notification)
	if sendCount != 1 {
		t.Errorf("expected 1 send without notification, got %d", sendCount)
	}
}
