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

func TestRouterBotPause(t *testing.T) {
	sendCount := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sendCount++
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"msg-1"}`))
	}))
	defer srv.Close()
	overrideMessagesURL(t, srv.URL)

	config := &BotConfig{
		WebexToken: "fake-token",
		Domains:    []string{"webex.bot"},
		PlatformTokens: map[string][]string{
			"webex": {"fake-token"},
		},
		Routes: map[string]string{"myslug": "target@webex.bot"},
	}

	bot := NewRouterBot(config)
	bot.client = srv.Client()

	// Pause target manually
	bot.pauseMu.Lock()
	bot.pausedTargets["target@webex.bot"] = true
	bot.pauseBuffer["target@webex.bot"] = []bufferedMsg{}
	bot.pauseMu.Unlock()

	// Send echo to registered slug (should be buffered)
	echoMsg := wmh.DecryptedMessage{
		PersonEmail: "sender@webex.bot",
		Text:        "./echo:myslug:sender:-:hello",
		ID:          "echo-msg-id",
	}
	bot.onMessageWithCards(echoMsg, []interface{}{})

	// Should have been buffered, not sent
	if sendCount != 0 {
		t.Errorf("expected 0 sends (buffered), got %d", sendCount)
	}

	// Check buffer
	bot.pauseMu.Lock()
	if len(bot.pauseBuffer["target@webex.bot"]) != 1 {
		t.Errorf("expected 1 buffered message, got %d", len(bot.pauseBuffer["target@webex.bot"]))
	}
	bot.pauseMu.Unlock()
}

func TestRouterBotResume(t *testing.T) {
	var capturedBodies []map[string]interface{}
	sendCount := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sendCount++
		var body map[string]interface{}
		io.ReadAll(r.Body)
		capturedBodies = append(capturedBodies, body)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"msg-1"}`))
	}))
	defer srv.Close()
	overrideMessagesURL(t, srv.URL)

	config := &BotConfig{
		WebexToken: "fake-token",
		Domains:    []string{"webex.bot"},
		PlatformTokens: map[string][]string{
			"webex": {"fake-token"},
		},
	}

	bot := NewRouterBot(config)
	bot.client = srv.Client()

	// Pause the target
	pauseMsg := wmh.DecryptedMessage{
		PersonEmail: "sender@webex.bot",
		Text:        "./pause",
		ID:          "pause-msg-id",
	}
	bot.onMessageWithCards(pauseMsg, []interface{}{})

	// Buffer a message
	echoMsg := wmh.DecryptedMessage{
		PersonEmail: "other@webex.bot",
		Text:        "./echo:myslug:other:-:hello",
		ID:          "echo-msg-id",
	}
	bot.onMessageWithCards(echoMsg, []interface{}{})

	// Resume
	resumeMsg := wmh.DecryptedMessage{
		PersonEmail: "sender@webex.bot",
		Text:        "./resume",
		ID:          "resume-msg-id",
	}
	bot.onMessageWithCards(resumeMsg, []interface{}{})

	// The buffered message should now be sent
	if sendCount != 1 {
		t.Errorf("expected 1 send (flushed), got %d", sendCount)
	}

	// Check that the target is no longer paused
	bot.pauseMu.Lock()
	if bot.pausedTargets["sender@webex.bot"] {
		t.Error("expected sender@webex.bot to not be paused after resume")
	}
	bot.pauseMu.Unlock()
}

func TestRouterBotPauseDoesNotAffectOtherTargets(t *testing.T) {
	sendCount := 0
	var targets []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sendCount++
		var body map[string]interface{}
		var buffer [4096]byte
		n, _ := r.Body.Read(buffer[:])
		json.Unmarshal(buffer[:n], &body)
		if target, ok := body["toPersonEmail"].(string); ok {
			targets = append(targets, target)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"msg-1"}`))
	}))
	defer srv.Close()
	overrideMessagesURL(t, srv.URL)

	config := &BotConfig{
		WebexToken: "fake-token",
		Domains:    []string{"webex.bot"},
		PlatformTokens: map[string][]string{
			"webex": {"fake-token"},
		},
		Routes: map[string]string{
			"slug1": "target1@webex.bot",
			"slug2": "target2@webex.bot",
		},
	}

	bot := NewRouterBot(config)
	bot.client = srv.Client()

	// Pause target1
	bot.pauseMu.Lock()
	bot.pausedTargets["target1@webex.bot"] = true
	bot.pauseBuffer["target1@webex.bot"] = []bufferedMsg{}
	bot.pauseMu.Unlock()

	// Send echo to slug1 (should be buffered because target1 is paused)
	echo1Msg := wmh.DecryptedMessage{
		PersonEmail: "sender@webex.bot",
		Text:        "./echo:slug1:sender:-:msg1",
		ID:          "echo-msg-id-1",
	}
	bot.onMessageWithCards(echo1Msg, []interface{}{})

	// Send echo to slug2 (should go through because target2 is not paused)
	echo2Msg := wmh.DecryptedMessage{
		PersonEmail: "sender@webex.bot",
		Text:        "./echo:slug2:sender:-:msg2",
		ID:          "echo-msg-id-2",
	}
	bot.onMessageWithCards(echo2Msg, []interface{}{})

	// Only one send should have happened (to target2)
	if sendCount != 1 {
		t.Errorf("expected 1 send, got %d", sendCount)
	}
	if len(targets) > 0 && targets[0] != "target2@webex.bot" {
		t.Errorf("expected send to target2@webex.bot, got %s", targets[0])
	}

	// Verify the buffered message
	bot.pauseMu.Lock()
	if len(bot.pauseBuffer["target1@webex.bot"]) != 1 {
		t.Errorf("expected 1 buffered message for target1, got %d", len(bot.pauseBuffer["target1@webex.bot"]))
	}
	bot.pauseMu.Unlock()
}
