package wgrok

import (
	"context"
	"fmt"
	"strings"

	wmh "github.com/3rg0n/webex-message-handler/go"
)

// WebexListener wraps the webex-message-handler and normalizes messages.
type WebexListener struct {
	token    string
	logger   wmh.Logger
	handler  *wmh.WebexMessageHandler
	callback MessageCallback
}

// NewWebexListener creates a new Webex listener.
func NewWebexListener(token string, logger wmh.Logger) *WebexListener {
	return &WebexListener{
		token:  token,
		logger: logger,
	}
}

// OnMessage registers a callback for incoming messages.
func (l *WebexListener) OnMessage(callback MessageCallback) {
	l.callback = callback
}

// Connect establishes the connection to Webex.
func (l *WebexListener) Connect(ctx context.Context) error {
	h, err := wmh.New(wmh.Config{
		Token:  l.token,
		Logger: l.logger,
	})
	if err != nil {
		return fmt.Errorf("create handler: %w", err)
	}
	l.handler = h

	h.OnMessageCreated(func(msg wmh.DecryptedMessage) {
		if l.callback != nil {
			incoming := IncomingMessage{
				Sender:   msg.PersonEmail,
				Text:     strings.TrimSpace(msg.Text),
				MsgID:    msg.ID,
				Platform: "webex",
				Cards:    []interface{}{},
			}
			l.callback(incoming)
		}
	})

	if err := h.Connect(ctx); err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	l.logger.Info("Webex listener connected")
	return nil
}

// Disconnect closes the connection to Webex.
func (l *WebexListener) Disconnect(ctx context.Context) error {
	if l.handler != nil {
		if err := l.handler.Disconnect(ctx); err != nil {
			return fmt.Errorf("disconnect: %w", err)
		}
		l.handler = nil
	}
	l.logger.Info("Webex listener disconnected")
	return nil
}
