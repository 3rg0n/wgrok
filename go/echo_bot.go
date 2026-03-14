package wgrok

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	wmh "github.com/3rg0n/webex-message-handler/go"
)

// WgrokEchoBot listens for echo messages, validates allowlist, strips prefix, relays back.
type WgrokEchoBot struct {
	config    *BotConfig
	allowlist *Allowlist
	logger    wmh.Logger
	handler   *wmh.WebexMessageHandler
	client    *http.Client
	cancel    context.CancelFunc
}

// NewEchoBot creates a new WgrokEchoBot.
func NewEchoBot(config *BotConfig) *WgrokEchoBot {
	return &WgrokEchoBot{
		config:    config,
		allowlist: NewAllowlist(config.Domains),
		logger:    GetLogger(config.Debug, "wgrok.echo_bot"),
		client:    &http.Client{},
	}
}

// Run connects to Webex and listens for echo messages. Blocks until ctx is cancelled or Stop is called.
func (b *WgrokEchoBot) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	b.cancel = cancel

	h, err := wmh.New(wmh.Config{
		Token:  b.config.WebexToken,
		Logger: b.logger,
	})
	if err != nil {
		cancel()
		return fmt.Errorf("create handler: %w", err)
	}
	b.handler = h

	h.OnMessageCreated(func(msg wmh.DecryptedMessage) {
		b.onMessage(msg)
	})

	b.logger.Info("Echo bot starting")
	if err := h.Connect(ctx); err != nil {
		cancel()
		return fmt.Errorf("connect: %w", err)
	}
	b.logger.Info("Echo bot connected")

	<-ctx.Done()
	return ctx.Err()
}

// Stop disconnects the echo bot.
func (b *WgrokEchoBot) Stop(ctx context.Context) {
	if b.cancel != nil {
		b.cancel()
	}
	if b.handler != nil {
		_ = b.handler.Disconnect(ctx)
		b.handler = nil
	}
	b.logger.Info("Echo bot stopped")
}

// onMessageWithCards is used by tests to inject card data without HTTP fetches.
func (b *WgrokEchoBot) onMessageWithCards(msg wmh.DecryptedMessage, cards []interface{}) {
	sender := msg.PersonEmail
	text := strings.TrimSpace(msg.Text)

	if !b.allowlist.IsAllowed(sender) {
		b.logger.Warn(fmt.Sprintf("Rejected message from %s: not in allowlist", sender))
		return
	}

	if !IsEcho(text) {
		b.logger.Debug(fmt.Sprintf("Ignoring non-echo message from %s", sender))
		return
	}

	slug, payload, err := ParseEcho(text)
	if err != nil {
		b.logger.Error(fmt.Sprintf("Failed to parse echo message: %v", err))
		return
	}

	response := FormatResponse(slug, payload)

	if len(cards) > 0 {
		b.logger.Info(fmt.Sprintf("Relaying to %s: %s (with %d card(s))", sender, response, len(cards)))
		_, err = SendCard(b.config.WebexToken, sender, response, cards[0], b.client)
	} else {
		b.logger.Info(fmt.Sprintf("Relaying to %s: %s", sender, response))
		_, err = SendMessage(b.config.WebexToken, sender, response, b.client)
	}
	if err != nil {
		b.logger.Error(fmt.Sprintf("Failed to relay message: %v", err))
	}
}

func (b *WgrokEchoBot) onMessage(msg wmh.DecryptedMessage) {
	sender := msg.PersonEmail
	text := strings.TrimSpace(msg.Text)

	if !b.allowlist.IsAllowed(sender) {
		b.logger.Warn(fmt.Sprintf("Rejected message from %s: not in allowlist", sender))
		return
	}

	if !IsEcho(text) {
		b.logger.Debug(fmt.Sprintf("Ignoring non-echo message from %s", sender))
		return
	}

	slug, payload, err := ParseEcho(text)
	if err != nil {
		b.logger.Error(fmt.Sprintf("Failed to parse echo message: %v", err))
		return
	}

	response := FormatResponse(slug, payload)

	// Check for card attachments on the original message
	cards := b.fetchCards(msg.ID)

	if len(cards) > 0 {
		b.logger.Info(fmt.Sprintf("Relaying to %s: %s (with %d card(s))", sender, response, len(cards)))
		_, err = SendCard(b.config.WebexToken, sender, response, cards[0], b.client)
	} else {
		b.logger.Info(fmt.Sprintf("Relaying to %s: %s", sender, response))
		_, err = SendMessage(b.config.WebexToken, sender, response, b.client)
	}
	if err != nil {
		b.logger.Error(fmt.Sprintf("Failed to relay message: %v", err))
	}
}

func (b *WgrokEchoBot) fetchCards(messageID string) []interface{} {
	if messageID == "" {
		return nil
	}
	fullMsg, err := GetMessage(b.config.WebexToken, messageID, b.client)
	if err != nil {
		b.logger.Debug(fmt.Sprintf("Could not fetch message attachments: %v", err))
		return nil
	}
	return ExtractCards(fullMsg)
}
