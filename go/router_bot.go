package wgrok

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	wmh "github.com/3rg0n/webex-message-handler/go"
)

// bufferedMsg represents a message buffered during pause.
type bufferedMsg struct {
	response string
	target   string
	cards    []interface{}
}

// WgrokRouterBot listens for messages, validates allowlist, strips prefix, relays back.
type WgrokRouterBot struct {
	config         *BotConfig
	allowlist      *Allowlist
	logger         wmh.Logger
	listeners      map[string]PlatformListener
	client         *http.Client
	cancel         context.CancelFunc
	routes         map[string]string
	pausedTargets  map[string]bool
	pauseBuffer    map[string][]bufferedMsg
	pauseMu        sync.Mutex
	// Deprecated: handler is kept for backward compatibility with existing tests
	handler        *wmh.WebexMessageHandler
}

// NewRouterBot creates a new WgrokRouterBot.
func NewRouterBot(config *BotConfig) *WgrokRouterBot {
	return &WgrokRouterBot{
		config:        config,
		allowlist:     NewAllowlist(config.Domains),
		logger:        GetLogger(config.Debug, "wgrok.router_bot"),
		listeners:     make(map[string]PlatformListener),
		client:        &http.Client{},
		routes:        config.Routes,
		pausedTargets: make(map[string]bool),
		pauseBuffer:   make(map[string][]bufferedMsg),
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
	text = StripBotMention(text, msg.HTML)
	cards := msg.Cards

	if !b.allowlist.IsAllowed(sender) {
		b.logger.Warn(fmt.Sprintf("Rejected message from %s: not in allowlist", sender))
		return
	}

	// Check for control messages before echo parsing
	if IsPause(text) {
		b.pauseMu.Lock()
		b.pausedTargets[sender] = true
		if b.pauseBuffer[sender] == nil {
			b.pauseBuffer[sender] = []bufferedMsg{}
		}
		b.pauseMu.Unlock()
		b.logger.Info(fmt.Sprintf("Paused message delivery for %s", sender))
		return
	}

	if IsResume(text) {
		b.pauseMu.Lock()
		delete(b.pausedTargets, sender)
		buffer := b.pauseBuffer[sender]
		delete(b.pauseBuffer, sender)
		b.pauseMu.Unlock()

		b.logger.Info(fmt.Sprintf("Resumed message delivery for %s, flushing %d buffered message(s)", sender, len(buffer)))
		b.flushBuffer(sender, buffer)
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

	// Check if target is paused
	b.pauseMu.Lock()
	if b.pausedTargets[replyTo] {
		b.logger.Info(fmt.Sprintf("Target %s is paused, buffering message", replyTo))
		if len(b.pauseBuffer[replyTo]) >= 1000 {
			b.logger.Warn(fmt.Sprintf("Pause buffer full for %s, dropping oldest message", replyTo))
			b.pauseBuffer[replyTo] = b.pauseBuffer[replyTo][1:]
		}
		b.pauseBuffer[replyTo] = append(b.pauseBuffer[replyTo], bufferedMsg{
			response: response,
			target:   replyTo,
			cards:    cards,
		})
		b.pauseMu.Unlock()
		return
	}
	b.pauseMu.Unlock()

	// Always use roomID when available — works for both 1:1 and group rooms
	if msg.RoomID != "" {
		if len(cards) > 0 {
			b.logger.Info(fmt.Sprintf("Relaying to room %s: %s (with %d card(s))", msg.RoomID, response, len(cards)))
			_, err = PlatformSendCardToRoom(platform, token, msg.RoomID, response, cards[0], b.client)
		} else {
			b.logger.Info(fmt.Sprintf("Relaying to room %s: %s", msg.RoomID, response))
			_, err = PlatformSendMessageToRoom(platform, token, msg.RoomID, response, b.client)
		}
	} else {
		if len(cards) > 0 {
			b.logger.Info(fmt.Sprintf("Relaying to %s: %s (with %d card(s))", replyTo, response, len(cards)))
			_, err = PlatformSendCard(platform, token, replyTo, response, cards[0], b.client)
		} else {
			b.logger.Info(fmt.Sprintf("Relaying to %s: %s", replyTo, response))
			_, err = PlatformSendMessage(platform, token, replyTo, response, b.client)
		}
	}
	if err != nil {
		b.logger.Error(fmt.Sprintf("Failed to relay message: %v", err))
	}
}

// onMessageWithCards is used by tests to inject card data without HTTP fetches.
func (b *WgrokRouterBot) onMessageWithCards(msg wmh.DecryptedMessage, cards []interface{}) {
	sender := msg.PersonEmail
	text := strings.TrimSpace(msg.Text)
	text = StripBotMention(text, "")

	if !b.allowlist.IsAllowed(sender) {
		b.logger.Warn(fmt.Sprintf("Rejected message from %s: not in allowlist", sender))
		return
	}

	// Check for control messages before echo parsing
	if IsPause(text) {
		b.pauseMu.Lock()
		b.pausedTargets[sender] = true
		if b.pauseBuffer[sender] == nil {
			b.pauseBuffer[sender] = []bufferedMsg{}
		}
		b.pauseMu.Unlock()
		b.logger.Info(fmt.Sprintf("Paused message delivery for %s", sender))
		return
	}

	if IsResume(text) {
		b.pauseMu.Lock()
		delete(b.pausedTargets, sender)
		buffer := b.pauseBuffer[sender]
		delete(b.pauseBuffer, sender)
		b.pauseMu.Unlock()

		b.logger.Info(fmt.Sprintf("Resumed message delivery for %s, flushing %d buffered message(s)", sender, len(buffer)))
		b.flushBuffer(sender, buffer)
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

	roomID := msg.RoomID

	// Check if target is paused
	b.pauseMu.Lock()
	if b.pausedTargets[replyTo] {
		b.logger.Info(fmt.Sprintf("Target %s is paused, buffering message", replyTo))
		if len(b.pauseBuffer[replyTo]) >= 1000 {
			b.logger.Warn(fmt.Sprintf("Pause buffer full for %s, dropping oldest message", replyTo))
			b.pauseBuffer[replyTo] = b.pauseBuffer[replyTo][1:]
		}
		b.pauseBuffer[replyTo] = append(b.pauseBuffer[replyTo], bufferedMsg{
			response: response,
			target:   replyTo,
			cards:    cards,
		})
		b.pauseMu.Unlock()
		return
	}
	b.pauseMu.Unlock()

	// Always use roomID when available — works for both 1:1 and group rooms
	if roomID != "" {
		if len(cards) > 0 {
			b.logger.Info(fmt.Sprintf("Relaying to room %s: %s (with %d card(s))", roomID, response, len(cards)))
			_, err = PlatformSendCardToRoom(platform, token, roomID, response, cards[0], b.client)
		} else {
			b.logger.Info(fmt.Sprintf("Relaying to room %s: %s", roomID, response))
			_, err = PlatformSendMessageToRoom(platform, token, roomID, response, b.client)
		}
	} else {
		if len(cards) > 0 {
			b.logger.Info(fmt.Sprintf("Relaying to %s: %s (with %d card(s))", replyTo, response, len(cards)))
			_, err = PlatformSendCard(platform, token, replyTo, response, cards[0], b.client)
		} else {
			b.logger.Info(fmt.Sprintf("Relaying to %s: %s", replyTo, response))
			_, err = PlatformSendMessage(platform, token, replyTo, response, b.client)
		}
	}
	if err != nil {
		b.logger.Error(fmt.Sprintf("Failed to relay message: %v", err))
	}
}

func (b *WgrokRouterBot) onMessage(msg wmh.DecryptedMessage) {
	sender := msg.PersonEmail
	text := strings.TrimSpace(msg.Text)
	text = StripBotMention(text, "")

	if !b.allowlist.IsAllowed(sender) {
		b.logger.Warn(fmt.Sprintf("Rejected message from %s: not in allowlist", sender))
		return
	}

	// Check for control messages before echo parsing
	if IsPause(text) {
		b.pauseMu.Lock()
		b.pausedTargets[sender] = true
		if b.pauseBuffer[sender] == nil {
			b.pauseBuffer[sender] = []bufferedMsg{}
		}
		b.pauseMu.Unlock()
		b.logger.Info(fmt.Sprintf("Paused message delivery for %s", sender))
		return
	}

	if IsResume(text) {
		b.pauseMu.Lock()
		delete(b.pausedTargets, sender)
		buffer := b.pauseBuffer[sender]
		delete(b.pauseBuffer, sender)
		b.pauseMu.Unlock()

		b.logger.Info(fmt.Sprintf("Resumed message delivery for %s, flushing %d buffered message(s)", sender, len(buffer)))
		b.flushBuffer(sender, buffer)
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

	roomID := msg.RoomID

	// Check if target is paused
	b.pauseMu.Lock()
	if b.pausedTargets[replyTo] {
		b.logger.Info(fmt.Sprintf("Target %s is paused, buffering message", replyTo))
		cards := b.fetchCards(msg.ID, "webex")
		if len(b.pauseBuffer[replyTo]) >= 1000 {
			b.logger.Warn(fmt.Sprintf("Pause buffer full for %s, dropping oldest message", replyTo))
			b.pauseBuffer[replyTo] = b.pauseBuffer[replyTo][1:]
		}
		b.pauseBuffer[replyTo] = append(b.pauseBuffer[replyTo], bufferedMsg{
			response: response,
			target:   replyTo,
			cards:    cards,
		})
		b.pauseMu.Unlock()
		return
	}
	b.pauseMu.Unlock()

	// Check for card attachments on the original message (only for webex)
	cards := b.fetchCards(msg.ID, "webex")

	// Always use roomID when available — works for both 1:1 and group rooms
	if roomID != "" {
		if len(cards) > 0 {
			b.logger.Info(fmt.Sprintf("Relaying to room %s: %s (with %d card(s))", roomID, response, len(cards)))
			_, err = PlatformSendCardToRoom(platform, token, roomID, response, cards[0], b.client)
		} else {
			b.logger.Info(fmt.Sprintf("Relaying to room %s: %s", roomID, response))
			_, err = PlatformSendMessageToRoom(platform, token, roomID, response, b.client)
		}
	} else {
		if len(cards) > 0 {
			b.logger.Info(fmt.Sprintf("Relaying to %s: %s (with %d card(s))", replyTo, response, len(cards)))
			_, err = PlatformSendCard(platform, token, replyTo, response, cards[0], b.client)
		} else {
			b.logger.Info(fmt.Sprintf("Relaying to %s: %s", replyTo, response))
			_, err = PlatformSendMessage(platform, token, replyTo, response, b.client)
		}
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

// flushBuffer sends all buffered messages for a target and removes them from the buffer.
func (b *WgrokRouterBot) flushBuffer(target string, buffer []bufferedMsg) {
	platform, token, err := b.getSendPlatformToken()
	if err != nil {
		b.logger.Error(fmt.Sprintf("Failed to get send platform token for flush: %v", err))
		return
	}

	for _, buffMsg := range buffer {
		if len(buffMsg.cards) > 0 {
			b.logger.Info(fmt.Sprintf("Flushing buffered message to %s (with %d card(s))", buffMsg.target, len(buffMsg.cards)))
			_, err = PlatformSendCard(platform, token, buffMsg.target, buffMsg.response, buffMsg.cards[0], b.client)
		} else {
			b.logger.Info(fmt.Sprintf("Flushing buffered message to %s", buffMsg.target))
			_, err = PlatformSendMessage(platform, token, buffMsg.target, buffMsg.response, b.client)
		}
		if err != nil {
			b.logger.Error(fmt.Sprintf("Failed to send buffered message: %v", err))
		}
	}
}

// Pause sends a pause control message to all Mode C route targets.
func (b *WgrokRouterBot) Pause() error {
	platform, token, err := b.getSendPlatformToken()
	if err != nil {
		return fmt.Errorf("get send platform token: %w", err)
	}

	for _, target := range b.routes {
		b.logger.Info(fmt.Sprintf("Sending pause to route target %s", target))
		_, err = PlatformSendMessage(platform, token, target, PauseCmd, b.client)
		if err != nil {
			b.logger.Error(fmt.Sprintf("Failed to send pause to %s: %v", target, err))
		}
	}
	return nil
}

// Resume sends a resume control message to all Mode C route targets.
func (b *WgrokRouterBot) Resume() error {
	platform, token, err := b.getSendPlatformToken()
	if err != nil {
		return fmt.Errorf("get send platform token: %w", err)
	}

	for _, target := range b.routes {
		b.logger.Info(fmt.Sprintf("Sending resume to route target %s", target))
		_, err = PlatformSendMessage(platform, token, target, ResumeCmd, b.client)
		if err != nil {
			b.logger.Error(fmt.Sprintf("Failed to send resume to %s: %v", target, err))
		}
	}
	return nil
}
