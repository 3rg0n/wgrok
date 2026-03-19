package wgrok

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"nhooyr.io/websocket"
	wmh "github.com/3rg0n/webex-message-handler/go"
)

const (
	discordGatewayAPI = "https://discord.com/api/v10/gateway"

	// Discord Gateway opcodes
	opDispatch     = 0
	opHeartbeat    = 1
	opIdentify     = 2
	opHello        = 10
	opHeartbeatAck = 11

	// Intents: GUILD_MESSAGES (1 << 9) + MESSAGE_CONTENT (1 << 15)
	intents = (1 << 9) | (1 << 15)
)

// DiscordListener listens for Discord messages via Gateway WebSocket.
type DiscordListener struct {
	token          string
	logger         wmh.Logger
	callback       MessageCallback
	ws             *websocket.Conn
	running        bool
	heartbeatDone  chan struct{}
	stopHeartbeat  chan struct{}
	sequence       int
	hasSequence    bool
	seqMu          sync.Mutex
}

// NewDiscordListener creates a new Discord listener.
func NewDiscordListener(token string, logger wmh.Logger) *DiscordListener {
	return &DiscordListener{
		token:         token,
		logger:        logger,
		heartbeatDone: make(chan struct{}),
		stopHeartbeat: make(chan struct{}),
	}
}

// OnMessage registers a callback for incoming messages.
func (l *DiscordListener) OnMessage(callback MessageCallback) {
	l.callback = callback
}

// Connect establishes the WebSocket connection to Discord Gateway.
func (l *DiscordListener) Connect(ctx context.Context) error {
	// Get gateway URL
	gwURL, err := l.getGatewayURL()
	if err != nil {
		return fmt.Errorf("get gateway url: %w", err)
	}

	wsURL := gwURL + "/?v=10&encoding=json"
	ws, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("connect to discord gateway: %w", err)
	}
	l.ws = ws
	l.running = true

	// Wait for Hello (opcode 10)
	var helloMsg map[string]interface{}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	_, data, err := ws.Read(ctx)
	cancel()
	if err != nil {
		l.ws.Close(websocket.StatusNormalClosure, "")
		l.ws = nil
		l.running = false
		return fmt.Errorf("read hello: %w", err)
	}

	if err := json.Unmarshal(data, &helloMsg); err != nil {
		l.ws.Close(websocket.StatusNormalClosure, "")
		l.ws = nil
		l.running = false
		return fmt.Errorf("parse hello: %w", err)
	}

	op, ok := helloMsg["op"].(float64)
	if !ok || int(op) != opHello {
		l.ws.Close(websocket.StatusNormalClosure, "")
		l.ws = nil
		l.running = false
		return fmt.Errorf("expected Hello (op 10), got op %v", op)
	}

	// Extract heartbeat interval
	d, ok := helloMsg["d"].(map[string]interface{})
	if !ok {
		l.ws.Close(websocket.StatusNormalClosure, "")
		l.ws = nil
		l.running = false
		return fmt.Errorf("no data in hello message")
	}

	hbInterval, ok := d["heartbeat_interval"].(float64)
	if !ok {
		l.ws.Close(websocket.StatusNormalClosure, "")
		l.ws = nil
		l.running = false
		return fmt.Errorf("no heartbeat_interval in hello")
	}

	// Send Identify
	identify := map[string]interface{}{
		"op": opIdentify,
		"d": map[string]interface{}{
			"token": l.token,
			"intents": intents,
			"properties": map[string]interface{}{
				"os":      "linux",
				"browser": "wgrok",
				"device":  "wgrok",
			},
		},
	}
	identifyData, err := marshalJSON(identify)
	if err != nil {
		l.ws.Close(websocket.StatusNormalClosure, "")
		l.ws = nil
		l.running = false
		return fmt.Errorf("marshal identify: %w", err)
	}
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	err = ws.Write(ctx, websocket.MessageText, identifyData)
	cancel()
	if err != nil {
		l.ws.Close(websocket.StatusNormalClosure, "")
		l.ws = nil
		l.running = false
		return fmt.Errorf("send identify: %w", err)
	}

	l.logger.Info("Discord Gateway connected")

	// Start heartbeat and read loops in background
	go l.heartbeatLoop(time.Duration(hbInterval) * time.Millisecond)
	go l.readLoop()

	return nil
}

// getGatewayURL fetches the Discord Gateway URL.
func (l *DiscordListener) getGatewayURL() (string, error) {
	resp, err := http.Get(discordGatewayAPI)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Limit response body to 1MB to prevent memory exhaustion
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(data))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}

	url, ok := result["url"].(string)
	if !ok {
		return "", fmt.Errorf("no url in gateway response")
	}
	return url, nil
}

// heartbeatLoop sends periodic heartbeats to keep the connection alive.
func (l *DiscordListener) heartbeatLoop(interval time.Duration) {
	defer close(l.heartbeatDone)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-l.stopHeartbeat:
			return
		case <-ticker.C:
			if !l.running || l.ws == nil {
				return
			}

			l.seqMu.Lock()
			var seqVal interface{}
			if l.hasSequence {
				seqVal = l.sequence
			}
			heartbeat := map[string]interface{}{
				"op": opHeartbeat,
				"d":  seqVal,
			}
			l.seqMu.Unlock()
			hbData, err := marshalJSON(heartbeat)
			if err != nil {
				l.logger.Debug(fmt.Sprintf("marshal heartbeat: %v", err))
				continue
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_ = l.ws.Write(ctx, websocket.MessageText, hbData)
			cancel()
		}
	}
}

// readLoop reads messages from the WebSocket.
func (l *DiscordListener) readLoop() {
	defer l.close()

	for l.running && l.ws != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		_, data, err := l.ws.Read(ctx)
		cancel()

		if err != nil {
			l.logger.Debug(fmt.Sprintf("read from discord ws: %v", err))
			break
		}

		var msg map[string]interface{}
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}

		// Track sequence number for heartbeat (mutex-protected)
		if s, ok := msg["s"].(float64); ok {
			l.seqMu.Lock()
			l.sequence = int(s)
			l.hasSequence = true
			l.seqMu.Unlock()
		}

		// Check for dispatch events
		op, ok := msg["op"].(float64)
		if !ok {
			continue
		}

		if int(op) == opDispatch {
			t, ok := msg["t"].(string)
			if !ok {
				continue
			}

			if t == "MESSAGE_CREATE" {
				if d, ok := msg["d"].(map[string]interface{}); ok {
					l.handleMessageCreate(d)
				}
			}
		}
	}
}

// handleMessageCreate processes a MESSAGE_CREATE dispatch event.
func (l *DiscordListener) handleMessageCreate(event map[string]interface{}) {
	author, ok := event["author"].(map[string]interface{})
	if !ok {
		return
	}

	// Skip bot messages
	if isBot, ok := author["bot"].(bool); ok && isBot {
		return
	}

	// Extract message info
	senderID, _ := event["id"].(string)
	if authorID, ok := author["id"].(string); ok {
		senderID = authorID
	}

	content, _ := event["content"].(string)
	msgID, _ := event["id"].(string)

	// Extract embeds as cards
	var cards []interface{}
	if embeds, ok := event["embeds"].([]interface{}); ok {
		cards = embeds
	}

	if l.callback != nil {
		incoming := IncomingMessage{
			Sender:   senderID,
			Text:     strings.TrimSpace(content),
			MsgID:    msgID,
			Platform: "discord",
			Cards:    cards,
		}
		l.callback(incoming)
	}
}

// close closes the WebSocket connection.
func (l *DiscordListener) close() {
	l.running = false
	// Signal heartbeat to stop
	select {
	case l.stopHeartbeat <- struct{}{}:
	default:
	}
	if l.ws != nil {
		_ = l.ws.Close(websocket.StatusNormalClosure, "")
		l.ws = nil
	}
	// Wait for heartbeat loop to finish
	select {
	case <-l.heartbeatDone:
	case <-time.After(5 * time.Second):
	}
}

// Disconnect closes the connection to Discord.
func (l *DiscordListener) Disconnect(ctx context.Context) error {
	l.close()
	l.logger.Info("Discord listener disconnected")
	return nil
}

// marshalJSON marshals v to JSON, returning an error if serialization fails.
func marshalJSON(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}
