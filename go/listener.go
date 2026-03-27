package wgrok

import (
	"context"
	"fmt"

	wmh "github.com/3rg0n/webex-message-handler/go"
)

// IncomingMessage is the normalized incoming message from any platform.
type IncomingMessage struct {
	Sender   string
	Text     string
	HTML     string
	RoomID   string
	RoomType string
	MsgID    string
	Platform string
	Cards    []interface{}
}

// MessageCallback is the callback type for received messages.
type MessageCallback func(IncomingMessage)

// PlatformListener defines the interface for platform-specific listeners.
type PlatformListener interface {
	OnMessage(callback MessageCallback)
	Connect(ctx context.Context) error
	Disconnect(ctx context.Context) error
}

// CreateListener creates the right listener for a platform.
// For IRC, the token parameter should be the connection string (nick:password@server:port/channel).
func CreateListener(platform, token string, logger wmh.Logger) (PlatformListener, error) {
	switch platform {
	case "webex":
		return NewWebexListener(token, logger), nil
	case "slack":
		return NewSlackListener(token, logger), nil
	case "discord":
		return NewDiscordListener(token, logger), nil
	case "irc":
		conn, err := NewIrcConnection(token)
		if err != nil {
			return nil, fmt.Errorf("create irc connection: %w", err)
		}
		listener := NewIrcListener(token, logger)
		listener.conn = conn
		return listener, nil
	default:
		return nil, fmt.Errorf("unsupported platform: %s", platform)
	}
}
