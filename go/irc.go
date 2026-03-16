package wgrok

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// IRCParams holds parsed IRC connection string components.
type IRCParams struct {
	Nick     string
	Password string
	Server   string
	Port     int
	Channel  string
}

// ParseIRCConnectionString parses an IRC connection string into components.
// Format: nick:password@server:port/channel
// Example: wgrok-bot:pass@irc.libera.chat:6697/#wgrok
func ParseIRCConnectionString(connStr string) (*IRCParams, error) {
	// Split on @ to get credentials and server parts
	if !strings.Contains(connStr, "@") {
		return nil, fmt.Errorf("invalid IRC connection string (missing @): %s", connStr)
	}

	parts := strings.SplitN(connStr, "@", 2)
	creds := parts[0]
	serverPart := parts[1]

	// Parse credentials
	var nick, password string
	if strings.Contains(creds, ":") {
		credParts := strings.SplitN(creds, ":", 2)
		nick = credParts[0]
		password = credParts[1]
	} else {
		nick = creds
		password = ""
	}

	// Parse server:port/channel
	var server string
	var port int
	var channel string

	if strings.Contains(serverPart, "/") {
		hostPortParts := strings.SplitN(serverPart, "/", 2)
		hostPort := hostPortParts[0]
		channel = hostPortParts[1]

		if strings.Contains(hostPort, ":") {
			hostPortSplit := strings.Split(hostPort, ":")
			server = strings.Join(hostPortSplit[:len(hostPortSplit)-1], ":")
			portStr := hostPortSplit[len(hostPortSplit)-1]
			parsedPort, err := strconv.Atoi(portStr)
			if err != nil {
				return nil, fmt.Errorf("invalid port: %s", portStr)
			}
			port = parsedPort
		} else {
			server = hostPort
			port = 6697 // Default TLS port
		}
	} else {
		hostPort := serverPart
		channel = ""

		if strings.Contains(hostPort, ":") {
			hostPortSplit := strings.Split(hostPort, ":")
			server = strings.Join(hostPortSplit[:len(hostPortSplit)-1], ":")
			portStr := hostPortSplit[len(hostPortSplit)-1]
			parsedPort, err := strconv.Atoi(portStr)
			if err != nil {
				return nil, fmt.Errorf("invalid port: %s", portStr)
			}
			port = parsedPort
		} else {
			server = hostPort
			port = 6697 // Default TLS port
		}
	}

	return &IRCParams{
		Nick:     nick,
		Password: password,
		Server:   server,
		Port:     port,
		Channel:  channel,
	}, nil
}

// SendIRCMessage sends a message via IRC (not implemented - would require persistent connection).
// Returns a map indicating the message was queued for sending.
func SendIRCMessage(connStr, target, text string) (map[string]interface{}, error) {
	_, err := ParseIRCConnectionString(connStr)
	if err != nil {
		return nil, err
	}
	// In a real implementation, this would send via TCP/TLS connection
	return map[string]interface{}{"status": "sent", "target": target}, nil
}

// SendIRCCard sends a message via IRC. Cards are not supported - only text is sent.
func SendIRCCard(connStr, target, text string, card interface{}) (map[string]interface{}, error) {
	// IRC doesn't support cards, so just send the text message
	return SendIRCMessage(connStr, target, text)
}

// IrcConnection manages a persistent TCP/TLS connection to an IRC server.
type IrcConnection struct {
	params       *IRCParams
	conn         net.Conn
	reader       *bufio.Reader
	writer       *bufio.Writer
	connected    bool
	nick         string
	channel      string
}

// NewIrcConnection creates a new IRC connection handler.
func NewIrcConnection(connStr string) (*IrcConnection, error) {
	params, err := ParseIRCConnectionString(connStr)
	if err != nil {
		return nil, err
	}
	return &IrcConnection{
		params:  params,
		nick:    params.Nick,
		channel: params.Channel,
	}, nil
}

// Connect establishes a TLS connection to the IRC server.
func (ic *IrcConnection) Connect() error {
	if ic.connected {
		return fmt.Errorf("already connected")
	}

	addr := fmt.Sprintf("%s:%d", ic.params.Server, ic.params.Port)

	// Use TLS for IRC connection
	tlsConfig := &tls.Config{
		ServerName: ic.params.Server,
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("dial irc server: %w", err)
	}

	ic.conn = conn
	ic.reader = bufio.NewReader(conn)
	ic.writer = bufio.NewWriter(conn)

	// Send PASS if password is provided
	if ic.params.Password != "" {
		if err := ic.sendRaw(fmt.Sprintf("PASS %s", ic.params.Password)); err != nil {
			ic.conn.Close()
			return err
		}
	}

	// Send NICK
	if err := ic.sendRaw(fmt.Sprintf("NICK %s", ic.params.Nick)); err != nil {
		ic.conn.Close()
		return err
	}

	// Send USER
	if err := ic.sendRaw(fmt.Sprintf("USER %s 0 * :%s", ic.params.Nick, ic.params.Nick)); err != nil {
		ic.conn.Close()
		return err
	}

	// Join channel if specified
	if ic.params.Channel != "" {
		if err := ic.sendRaw(fmt.Sprintf("JOIN %s", ic.params.Channel)); err != nil {
			ic.conn.Close()
			return err
		}
	}

	ic.connected = true
	return nil
}

// Disconnect closes the IRC connection.
func (ic *IrcConnection) Disconnect() error {
	if !ic.connected {
		return nil
	}

	_ = ic.sendRaw("QUIT")
	if ic.conn != nil {
		_ = ic.conn.Close()
		ic.conn = nil
	}
	ic.reader = nil
	ic.writer = nil
	ic.connected = false
	return nil
}

// sendRaw sends a raw IRC command. Sanitizes \r\n to prevent protocol injection.
func (ic *IrcConnection) sendRaw(line string) error {
	if !ic.connected || ic.writer == nil {
		return fmt.Errorf("not connected")
	}

	// Sanitize to prevent IRC protocol injection
	sanitized := strings.NewReplacer("\r", "", "\n", "").Replace(line)
	_, err := ic.writer.WriteString(sanitized + "\r\n")
	if err != nil {
		return err
	}
	return ic.writer.Flush()
}

// ReadLine reads a line from the IRC server with timeout.
func (ic *IrcConnection) ReadLine(timeout time.Duration) (string, error) {
	if !ic.connected || ic.reader == nil {
		return "", fmt.Errorf("not connected")
	}

	if timeout > 0 {
		_ = ic.conn.SetReadDeadline(time.Now().Add(timeout))
	}

	line, err := ic.reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(line), nil
}
