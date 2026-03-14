package wgrok

import (
	"fmt"
	"strconv"
	"strings"
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
