package wgrok

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	wmh "github.com/3rg0n/webex-message-handler/go"
)

var privmsgRegex = regexp.MustCompile(`^:([^!]+)![^ ]+ PRIVMSG ([^ ]+) :(.+)$`)

// IrcListener listens for IRC messages via persistent TCP/TLS connection.
type IrcListener struct {
	conn     *IrcConnection
	logger   wmh.Logger
	callback MessageCallback
	running  bool
	done     chan struct{}
}

// NewIrcListener creates a new IRC listener.
func NewIrcListener(connStr string, logger wmh.Logger) *IrcListener {
	return &IrcListener{
		logger: logger,
		done:   make(chan struct{}),
	}
}

// OnMessage registers a callback for incoming messages.
func (l *IrcListener) OnMessage(callback MessageCallback) {
	l.callback = callback
}

// Connect establishes the connection to IRC.
func (l *IrcListener) Connect(ctx context.Context) error {
	// Extract connection string from context or use a default
	// For now, we'll create a placeholder since the listener doesn't have the conn string
	// This will be set up properly when used with CreateListener
	if l.conn == nil {
		return fmt.Errorf("connection not initialized")
	}

	if err := l.conn.Connect(); err != nil {
		return fmt.Errorf("connect to irc: %w", err)
	}

	l.running = true
	l.logger.Info(fmt.Sprintf("IRC connected to %s@%s:%d/%s", l.conn.nick, l.conn.params.Server, l.conn.params.Port, l.conn.channel))

	// Start reading messages in background
	go l.readLoop()

	return nil
}

// readLoop reads lines from the IRC server and dispatches events.
func (l *IrcListener) readLoop() {
	defer close(l.done)

	for l.running {
		line, err := l.conn.ReadLine(300 * time.Second)
		if err != nil {
			if l.running {
				l.logger.Debug(fmt.Sprintf("read from irc: %v", err))
			}
			break
		}

		if line == "" {
			continue
		}

		// Handle server PING
		if strings.HasPrefix(line, "PING") {
			pongArg := ""
			if len(line) > 5 {
				pongArg = line[5:]
			}
			_ = l.conn.sendRaw(fmt.Sprintf("PONG %s", pongArg))
			continue
		}

		// Parse PRIVMSG
		match := privmsgRegex.FindStringSubmatch(line)
		if match != nil && len(match) >= 4 && l.callback != nil {
			nick := match[1]
			// target := match[2]  // We don't need this for now
			text := match[3]

			incoming := IncomingMessage{
				Sender:   nick,
				Text:     strings.TrimSpace(text),
				MsgID:    "",
				Platform: "irc",
				Cards:    []interface{}{},
			}
			l.callback(incoming)
		}
	}
}

// Disconnect closes the connection to IRC.
func (l *IrcListener) Disconnect(ctx context.Context) error {
	l.running = false

	if l.conn != nil {
		_ = l.conn.Disconnect()
	}

	// Wait for readLoop to finish
	select {
	case <-l.done:
	case <-ctx.Done():
	case <-time.After(5 * time.Second):
	}

	l.logger.Info("IRC listener disconnected")
	return nil
}
