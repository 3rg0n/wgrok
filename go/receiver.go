package wgrok

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	wmh "github.com/3rg0n/webex-message-handler/go"
)

// MessageHandler is the callback type for received messages.
// It receives the slug, payload, cards, and fromSlug (sender identifier).
type MessageHandler func(slug, payload string, cards []interface{}, fromSlug string)

// ControlHandler is the callback type for control messages (pause/resume).
// It receives the control command name.
type ControlHandler func(cmd string)

// chunkKey identifies a chunk stream by sender + slug.
type chunkKey struct {
	sender string
	slug   string
}

// WgrokReceiver listens for response messages, matches slug, invokes handler callback.
type WgrokReceiver struct {
	config          *ReceiverConfig
	allowlist       *Allowlist
	handler         MessageHandler
	OnControl       ControlHandler
	logger          wmh.Logger
	listener        PlatformListener
	client          *http.Client
	cancel          context.CancelFunc
	chunkBuffer     map[chunkKey]map[int]string
	chunkTimestamps map[chunkKey]time.Time
	chunkMu         sync.Mutex
	// Deprecated: wsHandler is kept for backward compatibility with existing tests
	wsHandler *wmh.WebexMessageHandler
}

// NewReceiver creates a new WgrokReceiver.
func NewReceiver(config *ReceiverConfig, handler MessageHandler) *WgrokReceiver {
	return &WgrokReceiver{
		config:          config,
		allowlist:       NewAllowlist(config.Domains),
		handler:         handler,
		logger:          GetLogger(config.Debug, "wgrok.receiver"),
		client:          &http.Client{},
		chunkBuffer:     make(map[chunkKey]map[int]string),
		chunkTimestamps: make(map[chunkKey]time.Time),
	}
}

// Listen connects to the configured platform and listens for response messages matching the configured slug.
// Blocks until ctx is cancelled or Stop is called.
func (r *WgrokReceiver) Listen(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	r.cancel = cancel

	// Create listener based on platform configuration
	token := r.config.WebexToken
	if r.config.Platform != "" && r.config.Platform != "webex" {
		// For non-webex platforms, we'd need a different token source
		// For now, use WebexToken as fallback (would be overridden in real usage)
	}

	listener, err := CreateListener(r.config.Platform, token, r.logger)
	if err != nil {
		cancel()
		return fmt.Errorf("create listener: %w", err)
	}
	r.listener = listener

	// Register message callback
	listener.OnMessage(func(msg IncomingMessage) {
		r.onMessageFromListener(msg)
	})

	r.logger.Info(fmt.Sprintf("Receiver listening for slug: %s on platform %s", r.config.Slug, r.config.Platform))

	// Connect listener
	if err := listener.Connect(ctx); err != nil {
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
	if r.listener != nil {
		_ = r.listener.Disconnect(ctx)
		r.listener = nil
	}
	// For backward compatibility, also disconnect wsHandler if it exists
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

// onMessageFromListener processes an IncomingMessage from a listener.
func (r *WgrokReceiver) onMessageFromListener(msg IncomingMessage) {
	sender := msg.Sender
	text := msg.Text
	text = StripBotMention(text, msg.HTML)
	cards := msg.Cards

	if !r.allowlist.IsAllowed(sender) {
		r.logger.Warn(fmt.Sprintf("Rejected message from %s: not in allowlist", sender))
		return
	}

	// Check for control messages
	if IsPause(text) {
		r.logger.Info(fmt.Sprintf("Received pause from %s", sender))
		if r.OnControl != nil {
			r.OnControl("pause")
		}
		return
	}

	if IsResume(text) {
		r.logger.Info(fmt.Sprintf("Received resume from %s", sender))
		if r.OnControl != nil {
			r.OnControl("resume")
		}
		return
	}

	to, from, flags, payload, err := ParseResponse(text)
	if err != nil {
		r.logger.Debug(fmt.Sprintf("Ignoring unparseable message from %s", sender))
		return
	}

	if to != r.config.Slug {
		r.logger.Debug(fmt.Sprintf("Ignoring message with slug %q (expected %q)", to, r.config.Slug))
		return
	}

	// Parse flags to extract compression, encryption, and chunking info
	compressed, encrypted, chunkSeq, chunkTotal, _ := ParseFlags(flags)

	// Handle chunking
	if chunkSeq > 0 {
		if chunkTotal > 100 || chunkSeq > chunkTotal || chunkSeq < 1 {
			r.logger.Warn(fmt.Sprintf("Invalid chunk %d/%d from %s", chunkSeq, chunkTotal, sender))
			return
		}
		key := chunkKey{sender: sender, slug: to}
		r.chunkMu.Lock()

		// Check for chunk timeout (5 minutes = 300 seconds)
		now := time.Now()
		if ts, exists := r.chunkTimestamps[key]; exists {
			if now.Sub(ts) > 5*time.Minute {
				r.logger.Warn(fmt.Sprintf("Discarding incomplete chunk set for %v (timeout after 5 minutes)", key))
				delete(r.chunkBuffer, key)
				delete(r.chunkTimestamps, key)
				r.chunkMu.Unlock()
				return
			}
		} else {
			// First chunk for this key â record timestamp
			r.chunkTimestamps[key] = now
		}

		if r.chunkBuffer[key] == nil {
			r.chunkBuffer[key] = make(map[int]string)
		}
		r.chunkBuffer[key][chunkSeq] = payload
		if len(r.chunkBuffer[key]) < chunkTotal {
			r.chunkMu.Unlock()
			r.logger.Debug(fmt.Sprintf("Buffered chunk %d/%d for slug %q from %s", chunkSeq, chunkTotal, to, sender))
			return
		}

		// Verify all indices 1..chunkTotal are present before reassembly
		allPresent := true
		for i := 1; i <= chunkTotal; i++ {
			if _, exists := r.chunkBuffer[key][i]; !exists {
				allPresent = false
				break
			}
		}
		if !allPresent {
			r.logger.Warn(fmt.Sprintf("Incomplete chunk set for %v: missing indices, discarding", key))
			delete(r.chunkBuffer, key)
			delete(r.chunkTimestamps, key)
			r.chunkMu.Unlock()
			return
		}

		// All chunks received and verified â reassemble
		var assembled strings.Builder
		for i := 1; i <= chunkTotal; i++ {
			assembled.WriteString(r.chunkBuffer[key][i])
		}
		delete(r.chunkBuffer, key)
		delete(r.chunkTimestamps, key)
		r.chunkMu.Unlock()
		payload = assembled.String()
		r.logger.Debug(fmt.Sprintf("Reassembled %d chunks for slug %q from %s", chunkTotal, to, sender))
	}

	// Decrypt if needed
	if encrypted {
		if r.config.EncryptKey == nil {
			r.logger.Warn("Received encrypted message but no key configured, skipping decryption")
			return
		}
		decoded, err := Decrypt(payload, r.config.EncryptKey)
		if err != nil {
			r.logger.Warn(fmt.Sprintf("Decrypt failed: %v", err))
			return
		}
		payload = decoded
	}

	// Decompress if needed
	if compressed {
		decoded, err := Decompress(payload)
		if err != nil {
			r.logger.Warn(fmt.Sprintf("Decompress failed: %v", err))
			return
		}
		payload = decoded
	}

	if len(cards) > 0 {
		r.logger.Info(fmt.Sprintf("Received payload for slug %q from %s (with %d card(s))", to, sender, len(cards)))
	} else {
		r.logger.Info(fmt.Sprintf("Received payload for slug %q from %s", to, sender))
	}
	r.handler(to, payload, cards, from)
}

// onMessageWithCards is used by tests to inject card data without HTTP fetches.
func (r *WgrokReceiver) onMessageWithCards(msg wmh.DecryptedMessage, cards []interface{}) {
	sender := msg.PersonEmail
	text := strings.TrimSpace(msg.Text)
	text = StripBotMention(text, "")

	if !r.allowlist.IsAllowed(sender) {
		r.logger.Warn(fmt.Sprintf("Rejected message from %s: not in allowlist", sender))
		return
	}

	// Check for control messages
	if IsPause(text) {
		r.logger.Info(fmt.Sprintf("Received pause from %s", sender))
		if r.OnControl != nil {
			r.OnControl("pause")
		}
		return
	}

	if IsResume(text) {
		r.logger.Info(fmt.Sprintf("Received resume from %s", sender))
		if r.OnControl != nil {
			r.OnControl("resume")
		}
		return
	}

	to, from, flags, payload, err := ParseResponse(text)
	if err != nil {
		r.logger.Debug(fmt.Sprintf("Ignoring unparseable message from %s", sender))
		return
	}

	if to != r.config.Slug {
		r.logger.Debug(fmt.Sprintf("Ignoring message with slug %q (expected %q)", to, r.config.Slug))
		return
	}

	// Parse flags to extract compression, encryption, and chunking info
	compressed, encrypted, chunkSeq, chunkTotal, _ := ParseFlags(flags)

	// Handle chunking
	if chunkSeq > 0 {
		if chunkTotal > 100 || chunkSeq > chunkTotal || chunkSeq < 1 {
			r.logger.Warn(fmt.Sprintf("Invalid chunk %d/%d from %s", chunkSeq, chunkTotal, sender))
			return
		}
		key := chunkKey{sender: sender, slug: to}
		r.chunkMu.Lock()

		// Check for chunk timeout (5 minutes = 300 seconds)
		now := time.Now()
		if ts, exists := r.chunkTimestamps[key]; exists {
			if now.Sub(ts) > 5*time.Minute {
				r.logger.Warn(fmt.Sprintf("Discarding incomplete chunk set for %v (timeout after 5 minutes)", key))
				delete(r.chunkBuffer, key)
				delete(r.chunkTimestamps, key)
				r.chunkMu.Unlock()
				return
			}
		} else {
			// First chunk for this key  record timestamp
			r.chunkTimestamps[key] = now
		}

		r.chunkMu.Lock()
		if r.chunkBuffer[key] == nil {
			r.chunkBuffer[key] = make(map[int]string)
		}
		r.chunkBuffer[key][chunkSeq] = payload
		if len(r.chunkBuffer[key]) < chunkTotal {
			r.chunkMu.Unlock()
			return
		}

  // Verify all indices 1..chunkTotal are present before reassembly
  allPresent := true
  for i := 1; i <= chunkTotal; i++ {
  	if _, exists := r.chunkBuffer[key][i]; !exists {
  		allPresent = false
  		break
  	}
  }
  if !allPresent {
  	r.logger.Warn(fmt.Sprintf("Incomplete chunk set for %v: missing indices, discarding", key))
  	delete(r.chunkBuffer, key)
  	delete(r.chunkTimestamps, key)
  	r.chunkMu.Unlock()
  	return
  }

  // All chunks received and verified  reassemble
		var assembled strings.Builder
		for i := 1; i <= chunkTotal; i++ {
			assembled.WriteString(r.chunkBuffer[key][i])
		}
		delete(r.chunkBuffer, key)
		r.chunkMu.Unlock()
		payload = assembled.String()
	}

	// Decrypt if needed
	if encrypted {
		if r.config.EncryptKey == nil {
			r.logger.Warn("Received encrypted message but no key configured, skipping decryption")
			return
		}
		decoded, err := Decrypt(payload, r.config.EncryptKey)
		if err != nil {
			r.logger.Warn(fmt.Sprintf("Decrypt failed: %v", err))
			return
		}
		payload = decoded
	}

	// Decompress if needed
	if compressed {
		decoded, decErr := Decompress(payload)
		if decErr != nil {
			r.logger.Warn(fmt.Sprintf("Decompress failed: %v", decErr))
			return
		}
		payload = decoded
	}

	if len(cards) > 0 {
		r.logger.Info(fmt.Sprintf("Received payload for slug %q from %s (with %d card(s))", to, sender, len(cards)))
	} else {
		r.logger.Info(fmt.Sprintf("Received payload for slug %q from %s", to, sender))
	}
	r.handler(to, payload, cards, from)
}

func (r *WgrokReceiver) onMessage(msg wmh.DecryptedMessage) {
	sender := msg.PersonEmail
	text := strings.TrimSpace(msg.Text)
	text = StripBotMention(text, "")

	if !r.allowlist.IsAllowed(sender) {
		r.logger.Warn(fmt.Sprintf("Rejected message from %s: not in allowlist", sender))
		return
	}

	// Check for control messages
	if IsPause(text) {
		r.logger.Info(fmt.Sprintf("Received pause from %s", sender))
		if r.OnControl != nil {
			r.OnControl("pause")
		}
		return
	}

	if IsResume(text) {
		r.logger.Info(fmt.Sprintf("Received resume from %s", sender))
		if r.OnControl != nil {
			r.OnControl("resume")
		}
		return
	}

	to, from, flags, payload, err := ParseResponse(text)
	if err != nil {
		r.logger.Debug(fmt.Sprintf("Ignoring unparseable message from %s", sender))
		return
	}

	if to != r.config.Slug {
		r.logger.Debug(fmt.Sprintf("Ignoring message with slug %q (expected %q)", to, r.config.Slug))
		return
	}

	// Parse flags to extract compression, encryption, and chunking info
	compressed, encrypted, chunkSeq, chunkTotal, _ := ParseFlags(flags)

	// Handle chunking
	if chunkSeq > 0 {
		if chunkTotal > 100 || chunkSeq > chunkTotal || chunkSeq < 1 {
			r.logger.Warn(fmt.Sprintf("Invalid chunk %d/%d from %s", chunkSeq, chunkTotal, sender))
			return
		}
		key := chunkKey{sender: sender, slug: to}
		r.chunkMu.Lock()
		if r.chunkBuffer[key] == nil {
			r.chunkBuffer[key] = make(map[int]string)
		}
		r.chunkBuffer[key][chunkSeq] = payload
		if len(r.chunkBuffer[key]) < chunkTotal {
			r.chunkMu.Unlock()
			return
		}

  // Verify all indices 1..chunkTotal are present before reassembly
  allPresent := true
  for i := 1; i <= chunkTotal; i++ {
  	if _, exists := r.chunkBuffer[key][i]; !exists {
  		allPresent = false
  		break
  	}
  }
  if !allPresent {
  	r.logger.Warn(fmt.Sprintf("Incomplete chunk set for %v: missing indices, discarding", key))
  	delete(r.chunkBuffer, key)
  	delete(r.chunkTimestamps, key)
  	r.chunkMu.Unlock()
  	return
  }

  // All chunks received and verified  reassemble
		var assembled strings.Builder
		for i := 1; i <= chunkTotal; i++ {
			assembled.WriteString(r.chunkBuffer[key][i])
		}
		delete(r.chunkBuffer, key)
		r.chunkMu.Unlock()
		payload = assembled.String()
	}

	// Decrypt if needed
	if encrypted {
		if r.config.EncryptKey == nil {
			r.logger.Warn("Received encrypted message but no key configured, skipping decryption")
			return
		}
		decoded, err := Decrypt(payload, r.config.EncryptKey)
		if err != nil {
			r.logger.Warn(fmt.Sprintf("Decrypt failed: %v", err))
			return
		}
		payload = decoded
	}

	// Decompress if needed
	if compressed {
		decoded, decErr := Decompress(payload)
		if decErr != nil {
			r.logger.Warn(fmt.Sprintf("Decompress failed: %v", decErr))
			return
		}
		payload = decoded
	}

	// Fetch card attachments from the full message
	cards := r.fetchCards(msg.ID)

	if len(cards) > 0 {
		r.logger.Info(fmt.Sprintf("Received payload for slug %q from %s (with %d card(s))", to, sender, len(cards)))
	} else {
		r.logger.Info(fmt.Sprintf("Received payload for slug %q from %s", to, sender))
	}
	r.handler(to, payload, cards, from)
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
