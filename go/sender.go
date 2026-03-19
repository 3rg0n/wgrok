package wgrok

import (
	"fmt"
	"net/http"
	"sync"

	wmh "github.com/3rg0n/webex-message-handler/go"
)

// PlatformLimits defines message size limits per platform.
var PlatformLimits = map[string]int{
	"webex":   7439,
	"slack":   4000,
	"discord": 2000,
	"irc":     400,
}

// bufferedSend represents a message buffered during pause.
type bufferedSend struct {
	payload  string
	card     interface{}
	compress bool
}

// WgrokSender wraps payloads in the echo protocol and sends via Webex.
type WgrokSender struct {
	config  *SenderConfig
	client  *http.Client
	logger  wmh.Logger
	paused  bool
	buffer  []bufferedSend
	pauseMu sync.Mutex
}

// NewSender creates a new WgrokSender.
func NewSender(config *SenderConfig) *WgrokSender {
	return &WgrokSender{
		config: config,
		client: &http.Client{},
		logger: GetLogger(config.Debug, "wgrok.sender"),
	}
}

// Send formats payload as an echo message and sends to the configured target.
// If card is non-nil, it is attached as an adaptive card.
// If compress is true, the payload is gzip+base64 encoded before sending.
func (s *WgrokSender) Send(payload string, card interface{}) (map[string]interface{}, error) {
	return s.SendWithOptions(payload, card, false)
}

// SendWithOptions is like Send but with an explicit compress flag.
func (s *WgrokSender) SendWithOptions(payload string, card interface{}, compress bool) (map[string]interface{}, error) {
	s.pauseMu.Lock()
	if s.paused {
		if len(s.buffer) >= 1000 {
			s.logger.Warn("Pause buffer full (1000), dropping oldest message")
			s.buffer = s.buffer[1:]
		}
		s.buffer = append(s.buffer, bufferedSend{
			payload:  payload,
			card:     card,
			compress: compress,
		})
		s.pauseMu.Unlock()
		s.logger.Info("Sender is paused, buffering message")
		return map[string]interface{}{"buffered": true}, nil
	}
	s.pauseMu.Unlock()

	encrypted := s.config.EncryptKey != nil

	if compress {
		encoded, err := Compress(payload)
		if err != nil {
			return nil, fmt.Errorf("compress payload: %w", err)
		}
		payload = encoded
	}

	if encrypted {
		encoded, err := Encrypt(payload, s.config.EncryptKey)
		if err != nil {
			return nil, fmt.Errorf("encrypt payload: %w", err)
		}
		payload = encoded
	}

	// Use slug as both to and from (for sender context)
	to := s.config.Slug
	from := s.config.Slug
	flags := FormatFlags(compress, encrypted, 0, 0)

	text := FormatEcho(to, from, flags, payload)
	limit, ok := PlatformLimits[s.config.Platform]
	if !ok {
		limit = 7439
	}
	if len([]byte(text)) > limit && card == nil {
		// Estimate overhead for chunked format
		// Worst case for flags in chunking: "ze999/999" = 9 chars, we'll use 11 for safety
		flagOverhead := 11
		overhead := len([]byte(EchoPrefix)) + len([]byte(to)) + 1 + len([]byte(from)) + 1 + flagOverhead + 1
		maxPayload := limit - overhead
		chunks, err := Chunk(payload, maxPayload)
		if err != nil {
			return nil, fmt.Errorf("chunk payload: %w", err)
		}
		s.logger.Info(fmt.Sprintf("Payload exceeds %dB limit, sending %d chunks to %s", limit, len(chunks), s.config.Target))
		var lastResult map[string]interface{}
		for i, ch := range chunks {
			chunkFlags := FormatFlags(compress, encrypted, i+1, len(chunks))
			chunkText := FormatEcho(to, from, chunkFlags, ch)
			result, err := PlatformSendMessage(s.config.Platform, s.config.WebexToken, s.config.Target, chunkText, s.client)
			if err != nil {
				return nil, err
			}
			lastResult = result
		}
		return lastResult, nil
	}
	s.logger.Info(fmt.Sprintf("Sending to %s: %s", s.config.Target, text))
	if card != nil {
		s.logger.Info("Including adaptive card attachment")
		return PlatformSendCard(s.config.Platform, s.config.WebexToken, s.config.Target, text, card, s.client)
	}
	return PlatformSendMessage(s.config.Platform, s.config.WebexToken, s.config.Target, text, s.client)
}

// Pause pauses message delivery. If notify is true, sends a pause control message to the target.
func (s *WgrokSender) Pause(notify bool) error {
	s.pauseMu.Lock()
	s.paused = true
	s.pauseMu.Unlock()

	if notify {
		s.logger.Info(fmt.Sprintf("Sending pause notification to %s", s.config.Target))
		_, err := PlatformSendMessage(s.config.Platform, s.config.WebexToken, s.config.Target, PauseCmd, s.client)
		if err != nil {
			return fmt.Errorf("send pause message: %w", err)
		}
	}
	return nil
}

// Resume resumes message delivery. If notify is true, sends a resume control message to the target.
// Then flushes any buffered messages.
func (s *WgrokSender) Resume(notify bool) error {
	s.pauseMu.Lock()
	s.paused = false
	buffer := s.buffer
	s.buffer = []bufferedSend{}
	s.pauseMu.Unlock()

	if notify {
		s.logger.Info(fmt.Sprintf("Sending resume notification to %s", s.config.Target))
		_, err := PlatformSendMessage(s.config.Platform, s.config.WebexToken, s.config.Target, ResumeCmd, s.client)
		if err != nil {
			return fmt.Errorf("send resume message: %w", err)
		}
	}

	s.logger.Info(fmt.Sprintf("Resumed, flushing %d buffered message(s)", len(buffer)))
	for _, buffSend := range buffer {
		_, err := s.SendWithOptions(buffSend.payload, buffSend.card, buffSend.compress)
		if err != nil {
			s.logger.Error(fmt.Sprintf("Failed to send buffered message: %v", err))
		}
	}
	return nil
}
