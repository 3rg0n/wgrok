package wgrok

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	wmh "github.com/3rg0n/webex-message-handler/go"
)

// WgrokRouterBot listens for messages, validates allowlist, strips prefix, relays back.
type WgrokRouterBot struct {
	config     *BotConfig
	allowlist  *Allowlist
	logger     wmh.Logger
	listeners  map[string]PlatformListener
	client     *http.Client
	cancel     context.CancelFunc
	routes     map[string]string
	// Deprecated: handler is kept for backward compatibility with existing tests
	handler    *wmh.WebexMessageHandler
}

// NewRouterBot creates a new WgrokRouterBot.
func NewRouterBot(config *BotConfig) *WgrokRouterBot {
	return &WgrokRouterBot{
		config:    config,
		allowlist: NewAllowlist(config.Domains),
		logger:    GetLogger(config.Debug, "wgrok.router_bot"),
		listeners: make(map[string]PlatformListener),
		client:    &http.Client{},
		routes:    config.Routes,
	}
}

// Run connects to configured platforms and listens for messages. Blocks until ctx is cancelled or Stop is called.
func (b *WgrokRouterBot) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	b.cancel = cancel

	// If no platform tokens configured, fall back to webex for backward compatibility
	platformTokens := b.config.PlatformTokens
	if len(platformTokens) == 0 && b.config.WebexToken != "" {
		platformTokens = map[string][]string{
			"webex": {b.config.WebexToken},
		}
	}

	// Create and connect listeners for each platform
	for platform, tokens := range platformTokens {
		if len(tokens) == 0 {
			continue
		}

		listener, err := CreateListener(platform, tokens[0], b.logger)
		if err != nil {
			cancel()
			return fmt.Errorf("create %s listener: %w", platform, err)
		}

		// Register message callback
		listener.OnMessage(func(msg IncomingMessage) {
			b.onMessageFromListener(msg)
		})

		// Connect listener
		if err := listener.Connect(ctx); err != nil {
			cancel()
			return fmt.Errorf("connect %s listener: %w", platform, err)
		}

		b.listeners[platform] = listener
	}

	b.logger.Info("Router bot starting")
	b.logger.Info(fmt.Sprintf("Router bot connected to %d platform(s)", len(b.listeners)))

	<-ctx.Done()
	return ctx.Err()
}

// Stop disconnects the router bot.
func (b *WgrokRouterBot) Stop(ctx context.Context) {
	if b.cancel != nil {
		b.cancel()
	}
	for platform, listener := range b.listeners {
		_ = listener.Disconnect(ctx)
		b.logger.Debug(fmt.Sprintf("Disconnected %s listener", platform))
	}
	b.listeners = make(map[string]PlatformListener)
	// For backward compatibility, also disconnect handler if it exists
	if b.handler != nil {
		_ = b.handler.Disconnect(ctx)
		b.handler = nil
	}
	b.logger.Info("Router bot stopped")
}

// resolveTarget returns the target email for a slug.
// If slug is in routes, return the routed target; otherwise return sender (Mode C).
func (b *WgrokRouterBot) resolveTarget(slug, sender string) string {
	if target, ok := b.routes[slug]; ok {
		return target
	}
	return sender
}

// getSendPlatformToken returns the platform and token to use for sending.
// Prefers webex if available, otherwise returns the first available platform.
func (b *WgrokRouterBot) getSendPlatformToken() (platform, token string, err error) {
	if len(b.config.PlatformTokens) == 0 {
		return "", "", fmt.Errorf("no platform tokens configured")
	}

	// Prefer webex
	if tokens, ok := b.config.PlatformTokens["webex"]; ok && len(tokens) > 0 {
		return "webex", tokens[0], nil
	}

	// Otherwise return first available platform
	for p, tokens := range b.config.PlatformTokens {
		if len(tokens) > 0 {
			return p, tokens[0], nil
		}
	}

	return "", "", fmt.Errorf("no valid tokens found in platform tokens")
}

// onMessageFromListener processes an IncomingMessage from a listener.
func (b *WgrokRouterBot) onMessageFromListener(msg IncomingMessage) {
	sender := msg.Sender
	text := msg.Text
	cards := msg.Cards

	if !b.allowlist.IsAllowed(sender) {
		b.logger.Warn(fmt.Sprintf("Rejected message from %s: not in allowlist", sender))
		return
	}

	if !IsEcho(text) {
		b.logger.Debug(fmt.Sprintf("Ignoring non-echo message from %s", sender))
		return
	}

	to, from, flags, payload, err := ParseEcho(text)
	if err != nil {
		b.logger.Error(fmt.Sprintf("Failed to parse echo message: %v", err))
		return
	}

	response := FormatResponse(to, from, flags, payload)
	replyTo := b.resolveTarget(to, sender)

	platform, token, err := b.getSendPlatformToken()
	if err != nil {
		b.logger.Error(fmt.Sprintf("Failed to get send platform token: %v", err))
		return
	}

	if len(cards) > 0 {
		b.logger.Info(fmt.Sprintf("Relaying to %s: %s (with %d card(s))", replyTo, response, len(cards)))
		_, err = PlatformSendCard(platform, token, replyTo, response, cards[0], b.client)
	} else {
		b.logger.Info(fmt.Sprintf("Relaying to %s: %s", replyTo, response))
		_, err = PlatformSendMessage(platform, token, replyTo, response, b.client)
	}
	if err != nil {
		b.logger.Error(fmt.Sprintf("Failed to relay message: %v", err))
	}
}

// onMessageWithCards is used by tests to inject card data without HTTP fetches.
func (b *WgrokRouterBot) onMessageWithCards(msg wmh.DecryptedMessage, cards []interface{}) {
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

	to, from, flags, payload, err := ParseEcho(text)
	if err != nil {
		b.logger.Error(fmt.Sprintf("Failed to parse echo message: %v", err))
		return
	}

	response := FormatResponse(to, from, flags, payload)
	replyTo := b.resolveTarget(to, sender)

	platform, token, err := b.getSendPlatformToken()
	if err != nil {
		b.logger.Error(fmt.Sprintf("Failed to get send platform token: %v", err))
		return
	}

	if len(cards) > 0 {
		b.logger.Info(fmt.Sprintf("Relaying to %s: %s (with %d card(s))", replyTo, response, len(cards)))
		_, err = PlatformSendCard(platform, token, replyTo, response, cards[0], b.client)
	} else {
		b.logger.Info(fmt.Sprintf("Relaying to %s: %s", replyTo, response))
		_, err = PlatformSendMessage(platform, token, replyTo, response, b.client)
	}
	if err != nil {
		b.logger.Error(fmt.Sprintf("Failed to relay message: %v", err))
	}
}

func (b *WgrokRouterBot) onMessage(msg wmh.DecryptedMessage) {
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

	to, from, flags, payload, err := ParseEcho(text)
	if err != nil {
		b.logger.Error(fmt.Sprintf("Failed to parse echo message: %v", err))
		return
	}

	response := FormatResponse(to, from, flags, payload)
	replyTo := b.resolveTarget(to, sender)

	platform, token, err := b.getSendPlatformToken()
	if err != nil {
		b.logger.Error(fmt.Sprintf("Failed to get send platform token: %v", err))
		return
	}

	// Check for card attachments on the original message (only for webex)
	cards := b.fetchCards(msg.ID, "webex")

	if len(cards) > 0 {
		b.logger.Info(fmt.Sprintf("Relaying to %s: %s (with %d card(s))", replyTo, response, len(cards)))
		_, err = PlatformSendCard(platform, token, replyTo, response, cards[0], b.client)
	} else {
		b.logger.Info(fmt.Sprintf("Relaying to %s: %s", replyTo, response))
		_, err = PlatformSendMessage(platform, token, replyTo, response, b.client)
	}
	if err != nil {
		b.logger.Error(fmt.Sprintf("Failed to relay message: %v", err))
	}
}

func (b *WgrokRouterBot) fetchCards(messageID, platform string) []interface{} {
	if messageID == "" || platform != "webex" {
		return nil
	}
	fullMsg, err := GetMessage(b.config.WebexToken, messageID, b.client)
	if err != nil {
		b.logger.Debug(fmt.Sprintf("Could not fetch message attachments: %v", err))
		return nil
	}
	return ExtractCards(fullMsg)
}
