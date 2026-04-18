package wgrok

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	wmh "github.com/3rg0n/webex-message-handler/go"
)

type receiverCases struct {
	Config struct {
		Slug    string   `json:"slug"`
		Domains []string `json:"domains"`
	} `json:"config"`
	Cases []struct {
		Name            string        `json:"name"`
		Sender          string        `json:"sender"`
		Text            string        `json:"text"`
		Cards           []interface{} `json:"cards"`
		ExpectHandler   bool          `json:"expect_handler"`
		ExpectedSlug    string        `json:"expected_slug"`
		ExpectedPayload string        `json:"expected_payload"`
		ExpectedFrom    string        `json:"expected_from"`
		ExpectedCards   []interface{} `json:"expected_cards"`
	} `json:"cases"`
}

func loadReceiverCases(t *testing.T) receiverCases {
	t.Helper()
	data, err := os.ReadFile("../tests/receiver_cases.json")
	if err != nil {
		t.Fatalf("load receiver cases: %v", err)
	}
	var cases receiverCases
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatalf("parse receiver cases: %v", err)
	}
	return cases
}

func TestWgrokReceiver(t *testing.T) {
	tc := loadReceiverCases(t)

	for _, c := range tc.Cases {
		t.Run(c.Name, func(t *testing.T) {
			handlerCalled := false
			var gotSlug, gotPayload, gotFrom string
			var gotCards []interface{}

			// Dummy HTTP server for card fetches (returns empty)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"id":"msg-1"}`))
			}))
			defer srv.Close()
			overrideMessagesURL(t, srv.URL)

			var gotCtx MessageContext
			handler := func(slug, payload string, cards []interface{}, fromSlug string, ctx MessageContext) {
				handlerCalled = true
				gotSlug = slug
				gotPayload = payload
				gotCards = cards
				gotFrom = fromSlug
				gotCtx = ctx
			}

			receiver := NewReceiver(&ReceiverConfig{
				WebexToken: "fake-token",
				Slug:       tc.Config.Slug,
				Domains:    tc.Config.Domains,
			}, handler)
			receiver.client = srv.Client()

			msg := wmh.DecryptedMessage{
				PersonEmail: c.Sender,
				Text:        c.Text,
				ID:          "test-msg-id",
			}

			// Override fetchCards to return test data
			receiver.onMessageWithCards(msg, c.Cards)

			if c.ExpectHandler && !handlerCalled {
				t.Error("expected handler to be called, but it wasn't")
			}
			if !c.ExpectHandler && handlerCalled {
				t.Error("expected handler NOT to be called, but it was")
			}

			if c.ExpectHandler && handlerCalled {
				if gotSlug != c.ExpectedSlug {
					t.Errorf("slug = %q, want %q", gotSlug, c.ExpectedSlug)
				}
				if gotPayload != c.ExpectedPayload {
					t.Errorf("payload = %q, want %q", gotPayload, c.ExpectedPayload)
				}
				if gotFrom != c.ExpectedFrom {
					t.Errorf("from = %q, want %q", gotFrom, c.ExpectedFrom)
				}
				gotCardsJSON, _ := json.Marshal(gotCards)
				expCardsJSON, _ := json.Marshal(c.ExpectedCards)
				if string(gotCardsJSON) != string(expCardsJSON) {
					t.Errorf("cards = %s, want %s", gotCardsJSON, expCardsJSON)
				}
				if gotCtx.MsgID != "test-msg-id" {
					t.Errorf("ctx.MsgID = %q, want test-msg-id", gotCtx.MsgID)
				}
				if gotCtx.Sender != c.Sender {
					t.Errorf("ctx.Sender = %q, want %q", gotCtx.Sender, c.Sender)
				}
			}
		})
	}
}

func TestReceiverPauseControl(t *testing.T) {
	controlCalled := false
	var controlCmd string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"msg-1"}`))
	}))
	defer srv.Close()
	overrideMessagesURL(t, srv.URL)

	handler := func(slug, payload string, cards []interface{}, fromSlug string, ctx MessageContext) {
		t.Error("handler should not be called for control messages")
	}

	controlHandler := func(cmd string) {
		controlCalled = true
		controlCmd = cmd
	}

	receiver := NewReceiver(&ReceiverConfig{
		WebexToken: "fake-token",
		Slug:       "myslug",
		Domains:    []string{"webex.bot"},
	}, handler)
	receiver.OnControl = controlHandler
	receiver.client = srv.Client()

	msg := wmh.DecryptedMessage{
		PersonEmail: "sender@webex.bot",
		Text:        "./pause",
		ID:          "pause-msg-id",
	}

	receiver.onMessageWithCards(msg, []interface{}{})

	if !controlCalled {
		t.Error("expected OnControl to be called for pause message")
	}
	if controlCmd != "pause" {
		t.Errorf("expected control cmd 'pause', got %q", controlCmd)
	}
}

func TestReceiverResumeControl(t *testing.T) {
	controlCalled := false
	var controlCmd string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"msg-1"}`))
	}))
	defer srv.Close()
	overrideMessagesURL(t, srv.URL)

	handler := func(slug, payload string, cards []interface{}, fromSlug string, ctx MessageContext) {
		t.Error("handler should not be called for control messages")
	}

	controlHandler := func(cmd string) {
		controlCalled = true
		controlCmd = cmd
	}

	receiver := NewReceiver(&ReceiverConfig{
		WebexToken: "fake-token",
		Slug:       "myslug",
		Domains:    []string{"webex.bot"},
	}, handler)
	receiver.OnControl = controlHandler
	receiver.client = srv.Client()

	msg := wmh.DecryptedMessage{
		PersonEmail: "sender@webex.bot",
		Text:        "./resume",
		ID:          "resume-msg-id",
	}

	receiver.onMessageWithCards(msg, []interface{}{})

	if !controlCalled {
		t.Error("expected OnControl to be called for resume message")
	}
	if controlCmd != "resume" {
		t.Errorf("expected control cmd 'resume', got %q", controlCmd)
	}
}

func TestReceiverControlNoCallback(t *testing.T) {
	// Test that receiver handles control messages gracefully even without a callback

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"msg-1"}`))
	}))
	defer srv.Close()
	overrideMessagesURL(t, srv.URL)

	handler := func(slug, payload string, cards []interface{}, fromSlug string, ctx MessageContext) {
		t.Error("handler should not be called for control messages")
	}

	receiver := NewReceiver(&ReceiverConfig{
		WebexToken: "fake-token",
		Slug:       "myslug",
		Domains:    []string{"webex.bot"},
	}, handler)
	receiver.OnControl = nil // No callback set
	receiver.client = srv.Client()

	msg := wmh.DecryptedMessage{
		PersonEmail: "sender@webex.bot",
		Text:        "./pause",
		ID:          "pause-msg-id",
	}

	// Should not panic
	receiver.onMessageWithCards(msg, []interface{}{})
}
