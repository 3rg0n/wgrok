package wgrok

import (
	"fmt"
	"net/http"
)

// PlatformSendMessage sends a text message via the specified platform.
func PlatformSendMessage(platform, token, target, text string, client *http.Client) (map[string]interface{}, error) {
	switch platform {
	case "webex":
		return SendMessage(token, target, text, client)
	case "slack":
		return SendSlackMessage(token, target, text, client)
	case "discord":
		return SendDiscordMessage(token, target, text, client)
	case "irc":
		return SendIRCMessage(token, target, text)
	default:
		return nil, fmt.Errorf("unsupported platform: %s", platform)
	}
}

// PlatformSendCard sends a message with card/rich content via the specified platform.
func PlatformSendCard(platform, token, target, text string, card interface{}, client *http.Client) (map[string]interface{}, error) {
	switch platform {
	case "webex":
		return SendCard(token, target, text, card, client)
	case "slack":
		return SendSlackCard(token, target, text, card, client)
	case "discord":
		return SendDiscordCard(token, target, text, card, client)
	case "irc":
		return SendIRCCard(token, target, text, card)
	default:
		return nil, fmt.Errorf("unsupported platform: %s", platform)
	}
}

// PlatformSendMessageToRoom sends a text message to a room via the specified platform.
func PlatformSendMessageToRoom(platform, token, roomID, text string, client *http.Client) (map[string]interface{}, error) {
	if platform == "webex" {
		return SendMessageToRoom(token, roomID, text, client)
	}
	return nil, fmt.Errorf("room-based send not supported for platform: %s", platform)
}

// PlatformSendCardToRoom sends a message with card/rich content to a room via the specified platform.
func PlatformSendCardToRoom(platform, token, roomID, text string, card interface{}, client *http.Client) (map[string]interface{}, error) {
	if platform == "webex" {
		return SendCardToRoom(token, roomID, text, card, client)
	}
	return nil, fmt.Errorf("room-based send not supported for platform: %s", platform)
}
