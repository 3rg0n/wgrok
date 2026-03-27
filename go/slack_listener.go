package wgrok

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"nhooyr.io/websocket"
	wmh "github.com/3rg0n/webex-message-handler/go"
)

const slackSocketModeURL = "https://slack.com/api/apps.connections.open"

// SlackListener listens for Slack messages via Socket Mode WebSocket.
type SlackListener struct {
	token      string
	logger     wmh.Logger
	callback   MessageCallback
	ws         *websocket.Conn
	running    bool
	httpClient SimpleHTTPClient
	ctx        context.Context
	cancelCtx  context.CancelFunc
}

// SimpleHTTPClient interface for dependency injection.
type SimpleHTTPClient interface {
	PostJSON(url, token string, body interface{}) (map[string]interface{}, error)
}

// DefaultSlackHTTPClient is the default HTTP client for Slack API calls.
type DefaultSlackHTTPClient struct{}

// PostJSON makes a POST request and returns JSON response.
func (c *DefaultSlackHTTPClient) PostJSON(url, token string, body interface{}) (map[string]interface{}, error) {

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(data))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// NewSlackListener creates a new Slack listener.
func NewSlackListener(token string, logger wmh.Logger) *SlackListener {
	return &SlackListener{
		token:      token,
		logger:     logger,
		httpClient: &DefaultSlackHTTPClient{},
	}
}

// OnMessage registers a callback for incoming messages.
func (l *SlackListener) OnMessage(callback MessageCallback) {
	l.callback = callback
}

// Connect establishes the WebSocket connection to Slack Socket Mode.
func (l *SlackListener) Connect(ctx context.Context) error {
	// Get WebSocket URL from Socket Mode API
	wsURL, err := l.getSocketModeURL()
	if err != nil {
		return fmt.Errorf("get socket mode URL: %w", err)
	}

	// Connect to WebSocket
	ws, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("connect to slack socket mode: %w", err)
	}
	l.ws = ws
	l.running = true

	l.logger.Info("Slack Socket Mode connected")

	// Create a lifecycle context for background goroutines
	l.ctx, l.cancelCtx = context.WithCancel(context.Background())

	// Start reading messages in background
	go l.readLoop()

	return nil
}

// getSocketModeURL requests a WebSocket URL from Slack's Socket Mode API.
func (l *SlackListener) getSocketModeURL() (string, error) {
	resp, err := l.httpClient.PostJSON(slackSocketModeURL, l.token, nil)
	if err != nil {
		return "", err
	}

	if ok, _ := resp["ok"].(bool); !ok {
		errMsg := "unknown error"
		if e, ok := resp["error"].(string); ok {
			errMsg = e
		}
		return "", fmt.Errorf("slack apps.connections.open failed: %s", errMsg)
	}

	if url, ok := resp["url"].(string); ok {
		return url, nil
	}
	return "", fmt.Errorf("no url in socket mode response")
}

// readLoop reads messages from the WebSocket.
func (l *SlackListener) readLoop() {
	defer l.close()

	for l.running && l.ws != nil {
		ctx, cancel := context.WithTimeout(l.ctx, 5*time.Minute)
		messageType, data, err := l.ws.Read(ctx)
		cancel()

		if err != nil {
			l.logger.Debug(fmt.Sprintf("read from slack ws: %v", err))
			break
		}

		if messageType == websocket.MessageText {
			l.handleEvent(string(data))
		}
	}
}

// handleEvent processes a Socket Mode envelope.
func (l *SlackListener) handleEvent(raw string) {
	var envelope map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		return
	}

	// Acknowledge the envelope
	if envelopeID, ok := envelope["envelope_id"].(string); ok && l.ws != nil {
		ack := map[string]interface{}{"envelope_id": envelopeID}
		ackData, err := json.Marshal(ack)
		if err == nil {
			ctx, cancel := context.WithTimeout(l.ctx, 5*time.Second)
			_ = l.ws.Write(ctx, websocket.MessageText, ackData)
			cancel()
		}
	}

	// Check if this is an events_api envelope
	eventType, ok := envelope["type"].(string)
	if !ok || eventType != "events_api" {
		return
	}

	payload, ok := envelope["payload"].(map[string]interface{})
	if !ok {
		return
	}

	event, ok := payload["event"].(map[string]interface{})
	if !ok {
		return
	}

	// Check if this is a message event
	et, ok := event["type"].(string)
	if !ok || et != "message" {
		return
	}

	// Skip bot messages
	if _, hasBotID := event["bot_id"]; hasBotID {
		return
	}

	// Extract message info
	sender, _ := event["user"].(string)
	text, _ := event["text"].(string)
	msgID, _ := event["ts"].(string)

	if l.callback != nil {
		incoming := IncomingMessage{
			Sender:   sender,
			Text:     strings.TrimSpace(text),
			MsgID:    msgID,
			Platform: "slack",
			Cards:    []interface{}{},
		}
		l.callback(incoming)
	}
}

// close closes the WebSocket connection.
func (l *SlackListener) close() {
	l.running = false
	if l.cancelCtx != nil {
		l.cancelCtx()
	}
	if l.ws != nil {
		_ = l.ws.Close(websocket.StatusNormalClosure, "")
		l.ws = nil
	}
}

// Disconnect closes the connection to Slack.
func (l *SlackListener) Disconnect(ctx context.Context) error {
	l.close()
	l.logger.Info("Slack listener disconnected")
	return nil
}
