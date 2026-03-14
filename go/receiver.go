package wgrok

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	wmh "github.com/3rg0n/webex-message-handler/go"
)

// MessageHandler is the callback type for received messages.
// It receives the slug, payload, and any adaptive card attachments.
type MessageHandler func(slug, payload string, cards []interface{})

// WgrokReceiver listens for response messages, matches slug, invokes handler callback.
type WgrokReceiver struct {
	config   *ReceiverConfig
	allowlist *Allowlist
	handler  MessageHandler
	logger   wmh.Logger
	wsHandler *wmh.WebexMessageHandler
	client   *http.Client
	cancel   context.CancelFunc
}

// NewReceiver creates a new WgrokReceiver.
func NewReceiver(config *ReceiverConfig, handler MessageHandler) *WgrokReceiver {
	return &WgrokReceiver{
		config:    config,
		allowlist: NewAllowlist(config.Domains),
		handler:   handler,
		logger:    GetLogger(config.Debug, "wgrok.receiver"),
		client:    &http.Client{},
	}
}

// Listen connects to Webex and listens for response messages matching the configured slug.
// Blocks until ctx is cancelled or Stop is called.
func (r *WgrokReceiver) Listen(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	r.cancel = cancel

	h, err := wmh.New(wmh.Config{
		Token:  r.config.WebexToken,
		Logger: r.logger,
	})
	if err != nil {
		cancel()
		return fmt.Errorf("create handler: %w", err)
	}
	r.wsHandler = h

	h.OnMessageCreated(func(msg wmh.DecryptedMessage) {
		r.onMessage(msg)
	})

	r.logger.Info(fmt.Sprintf("Receiver listening for slug: %s", r.config.Slug))
	if err := h.Connect(ctx); err != nil {
		cancel()
		return fmt.Errorf("connect: %w", err)
	}
	r.logger.Info("Receiver connected")

	<-ctx.Done()
	return ctx.Err()
}

// Stop disconnects the receiver.
func (r *WgrokReceiver) Stop(ctx context.Context) {
	if r.cancel != nil {
		r.cancel()
	}
	if r.wsHandler != nil {
		_ = r.wsHandler.Disconnect(ctx)
		r.wsHandler = nil
	}
	r.logger.Info("Receiver stopped")
}

// FetchAction fetches an attachment action (card form submission) by ID.
func (r *WgrokReceiver) FetchAction(actionID string) (map[string]interface{}, error) {
	return GetAttachmentAction(r.config.WebexToken, actionID, r.client)
}

// onMessageWithCards is used by tests to inject card data without HTTP fetches.
func (r *WgrokReceiver) onMessageWithCards(msg wmh.DecryptedMessage, cards []interface{}) {
	sender := msg.PersonEmail
	text := strings.TrimSpace(msg.Text)

	if !r.allowlist.IsAllowed(sender) {
		r.logger.Warn(fmt.Sprintf("Rejected message from %s: not in allowlist", sender))
		return
	}

	slug, payload, err := ParseResponse(text)
	if err != nil {
		r.logger.Debug(fmt.Sprintf("Ignoring unparseable message from %s", sender))
		return
	}

	if slug != r.config.Slug {
		r.logger.Debug(fmt.Sprintf("Ignoring message with slug %q (expected %q)", slug, r.config.Slug))
		return
	}

	if len(cards) > 0 {
		r.logger.Info(fmt.Sprintf("Received payload for slug %q from %s (with %d card(s))", slug, sender, len(cards)))
	} else {
		r.logger.Info(fmt.Sprintf("Received payload for slug %q from %s", slug, sender))
	}
	r.handler(slug, payload, cards)
}

func (r *WgrokReceiver) onMessage(msg wmh.DecryptedMessage) {
	sender := msg.PersonEmail
	text := strings.TrimSpace(msg.Text)

	if !r.allowlist.IsAllowed(sender) {
		r.logger.Warn(fmt.Sprintf("Rejected message from %s: not in allowlist", sender))
		return
	}

	slug, payload, err := ParseResponse(text)
	if err != nil {
		r.logger.Debug(fmt.Sprintf("Ignoring unparseable message from %s", sender))
		return
	}

	if slug != r.config.Slug {
		r.logger.Debug(fmt.Sprintf("Ignoring message with slug %q (expected %q)", slug, r.config.Slug))
		return
	}

	// Fetch card attachments from the full message
	cards := r.fetchCards(msg.ID)

	if len(cards) > 0 {
		r.logger.Info(fmt.Sprintf("Received payload for slug %q from %s (with %d card(s))", slug, sender, len(cards)))
	} else {
		r.logger.Info(fmt.Sprintf("Received payload for slug %q from %s", slug, sender))
	}
	r.handler(slug, payload, cards)
}

func (r *WgrokReceiver) fetchCards(messageID string) []interface{} {
	if messageID == "" {
		return nil
	}
	fullMsg, err := GetMessage(r.config.WebexToken, messageID, r.client)
	if err != nil {
		r.logger.Debug(fmt.Sprintf("Could not fetch message attachments: %v", err))
		return nil
	}
	return ExtractCards(fullMsg)
}
