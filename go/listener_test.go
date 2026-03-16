package wgrok

import (
	"testing"
)

func TestCreateListener(t *testing.T) {
	logger := GetLogger(false, "test")

	tests := []struct {
		name      string
		platform  string
		token     string
		expectErr bool
	}{
		{"webex", "webex", "fake-token", false},
		{"slack", "slack", "xapp-fake-token", false},
		{"discord", "discord", "fake-token", false},
		{"irc valid", "irc", "nick:pass@irc.libera.chat:6697/#test", false},
		{"irc invalid", "irc", "invalid", true},
		{"unsupported", "unknown", "token", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			listener, err := CreateListener(tt.platform, tt.token, logger)
			if tt.expectErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tt.expectErr && listener == nil {
				t.Error("expected listener, got nil")
			}
		})
	}
}

func TestIncomingMessage(t *testing.T) {
	msg := IncomingMessage{
		Sender:   "user@example.com",
		Text:     "test message",
		MsgID:    "msg-123",
		Platform: "webex",
		Cards:    []interface{}{},
	}

	if msg.Sender != "user@example.com" {
		t.Errorf("Sender = %q, want %q", msg.Sender, "user@example.com")
	}
	if msg.Text != "test message" {
		t.Errorf("Text = %q, want %q", msg.Text, "test message")
	}
	if msg.MsgID != "msg-123" {
		t.Errorf("MsgID = %q, want %q", msg.MsgID, "msg-123")
	}
	if msg.Platform != "webex" {
		t.Errorf("Platform = %q, want %q", msg.Platform, "webex")
	}
	if len(msg.Cards) != 0 {
		t.Errorf("Cards length = %d, want 0", len(msg.Cards))
	}
}

func TestWebexListener(t *testing.T) {
	logger := GetLogger(false, "test")
	listener := NewWebexListener("fake-token", logger)

	if listener == nil {
		t.Error("NewWebexListener returned nil")
	}

	// Test OnMessage callback registration
	listener.OnMessage(func(msg IncomingMessage) {
		// callback body
	})

	// Test that callback is set
	if listener.callback == nil {
		t.Error("callback not set")
	}
}

func TestSlackListener(t *testing.T) {
	logger := GetLogger(false, "test")
	listener := NewSlackListener("xapp-fake-token", logger)

	if listener == nil {
		t.Error("NewSlackListener returned nil")
	}

	// Test OnMessage callback registration
	listener.OnMessage(func(msg IncomingMessage) {
		// callback body
	})

	// Test that callback is set
	if listener.callback == nil {
		t.Error("callback not set")
	}
}

func TestDiscordListener(t *testing.T) {
	logger := GetLogger(false, "test")
	listener := NewDiscordListener("fake-token", logger)

	if listener == nil {
		t.Error("NewDiscordListener returned nil")
	}

	// Test OnMessage callback registration
	listener.OnMessage(func(msg IncomingMessage) {
		// callback body
	})

	// Test that callback is set
	if listener.callback == nil {
		t.Error("callback not set")
	}
}

func TestIrcListener(t *testing.T) {
	logger := GetLogger(false, "test")
	listener := NewIrcListener("nick:pass@irc.libera.chat:6697/#test", logger)

	if listener == nil {
		t.Error("NewIrcListener returned nil")
	}

	// Test OnMessage callback registration
	listener.OnMessage(func(msg IncomingMessage) {
		// callback body
	})

	// Test that callback is set
	if listener.callback == nil {
		t.Error("callback not set")
	}
}
