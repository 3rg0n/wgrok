// Package wgrok implements an ngrok clone using the Webex API as a message bus.
package wgrok

import (
	"errors"
	"fmt"
	"strings"
)

const EchoPrefix = "./echo:"

// FormatEcho formats an outgoing echo message: ./echo:{slug}:{payload}
func FormatEcho(slug, payload string) string {
	return EchoPrefix + slug + ":" + payload
}

// ParseEcho parses an echo message, returning (slug, payload).
func ParseEcho(text string) (slug, payload string, err error) {
	if !IsEcho(text) {
		return "", "", fmt.Errorf("not an echo message: %q", text)
	}
	stripped := text[len(EchoPrefix):]
	idx := strings.Index(stripped, ":")
	if idx < 0 {
		slug = stripped
		payload = ""
	} else {
		slug = stripped[:idx]
		payload = stripped[idx+1:]
	}
	if slug == "" {
		return "", "", fmt.Errorf("empty slug in echo message: %q", text)
	}
	return slug, payload, nil
}

// IsEcho checks if text is an echo-formatted message.
func IsEcho(text string) bool {
	return strings.HasPrefix(text, EchoPrefix)
}

// FormatResponse formats a response message: {slug}:{payload}
func FormatResponse(slug, payload string) string {
	return slug + ":" + payload
}

// ParseResponse parses a response message, returning (slug, payload).
func ParseResponse(text string) (slug, payload string, err error) {
	idx := strings.Index(text, ":")
	if idx < 0 {
		slug = text
		payload = ""
	} else {
		slug = text[:idx]
		payload = text[idx+1:]
	}
	if slug == "" {
		return "", "", errors.New("empty slug in response message")
	}
	return slug, payload, nil
}
