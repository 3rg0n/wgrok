package wgrok

import (
	"fmt"
	"net/http"

	wmh "github.com/3rg0n/webex-message-handler/go"
)

// WgrokSender wraps payloads in the echo protocol and sends via Webex.
type WgrokSender struct {
	config *SenderConfig
	client *http.Client
	logger wmh.Logger
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
func (s *WgrokSender) Send(payload string, card interface{}) (map[string]interface{}, error) {
	text := FormatEcho(s.config.Slug, payload)
	s.logger.Info(fmt.Sprintf("Sending to %s: %s", s.config.Target, text))
	if card != nil {
		s.logger.Info("Including adaptive card attachment")
		return SendCard(s.config.WebexToken, s.config.Target, text, card, s.client)
	}
	return SendMessage(s.config.WebexToken, s.config.Target, text, s.client)
}
