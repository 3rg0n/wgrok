package wgrok

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	wmh "github.com/3rg0n/webex-message-handler/go"
)

type routerBotCases struct {
	Config struct {
		Domains []string `json:"domains"`
	} `json:"config"`
	Routes map[string]string `json:"routes"`
	Cases  []struct {
		Name              string        `json:"name"`
		Sender            string        `json:"sender"`
		Text              string        `json:"text"`
		Cards             []interface{} `json:"cards"`
		UseRoutes         bool          `json:"use_routes"`
		ExpectSend        bool          `json:"expect_send"`
		ExpectedReplyTo   string        `json:"expected_reply_to"`
		ExpectedReplyText string        `json:"expected_reply_text"`
		ExpectedReplyCard interface{}   `json:"expected_reply_card"`
	} `json:"cases"`
}

func loadRouterBotCases(t *testing.T) routerBotCases {
	t.Helper()
	data, err := os.ReadFile("../tests/router_bot_cases.json")
	if err != nil {
		t.Fatalf("load router bot cases: %v", err)
	}
	var cases routerBotCases
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatalf("parse router bot cases: %v", err)
	}
	return cases
}

func TestWgrokRouterBot(t *testing.T) {
	tc := loadRouterBotCases(t)

	for _, c := range tc.Cases {
		t.Run(c.Name, func(t *testing.T) {
			var capturedBody map[string]interface{}
			sendCalled := false

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				sendCalled = true
				body, _ := io.ReadAll(r.Body)
				json.Unmarshal(body, &capturedBody)
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"id":"msg-1"}`))
			}))
			defer srv.Close()
			overrideMessagesURL(t, srv.URL)

			config := &BotConfig{
				WebexToken: "fake-token",
				Domains:    tc.Config.Domains,
				PlatformTokens: map[string][]string{
					"webex": {"fake-token"},
				},
			}

			// Apply routes if test case specifies use_routes
			if c.UseRoutes {
				config.Routes = tc.Routes
			}

			bot := NewRouterBot(config)
			bot.client = srv.Client()

			msg := wmh.DecryptedMessage{
				PersonEmail: c.Sender,
				Text:        c.Text,
				ID:          "test-msg-id",
			}

			// Override fetchCards to return test data
			origFetch := bot.fetchCards
			_ = origFetch
			bot.onMessageWithCards(msg, c.Cards)

			if c.ExpectSend && !sendCalled {
				t.Error("expected send to be called, but it wasn't")
			}
			if !c.ExpectSend && sendCalled {
				t.Error("expected send NOT to be called, but it was")
			}

			if c.ExpectSend && sendCalled {
				if capturedBody["toPersonEmail"] != c.ExpectedReplyTo {
					t.Errorf("reply to = %v, want %v", capturedBody["toPersonEmail"], c.ExpectedReplyTo)
				}
				if capturedBody["text"] != c.ExpectedReplyText {
					t.Errorf("reply text = %v, want %v", capturedBody["text"], c.ExpectedReplyText)
				}

				attachments, hasAtt := capturedBody["attachments"].([]interface{})
				if c.ExpectedReplyCard != nil {
					if !hasAtt || len(attachments) == 0 {
						t.Error("expected card in reply, got none")
					}
				} else {
					if hasAtt && len(attachments) > 0 {
						t.Error("expected no card in reply, got one")
					}
				}
			}
		})
	}
}
