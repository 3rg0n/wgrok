// Package wgrok implements an ngrok clone using the Webex API as a message bus.
package wgrok

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const EchoPrefix = "./echo:"

// FormatEcho formats an outgoing echo message: ./echo:{to}:{from}:{flags}:{payload}
func FormatEcho(to, from, flags, payload string) string {
	return EchoPrefix + to + ":" + from + ":" + flags + ":" + payload
}

// ParseEcho parses an echo message, returning (to, from, flags, payload).
func ParseEcho(text string) (to, from, flags, payload string, err error) {
	if !IsEcho(text) {
		return "", "", "", "", fmt.Errorf("not an echo message: %q", text)
	}
	stripped := text[len(EchoPrefix):]
	parts := strings.SplitN(stripped, ":", 4)
	if len(parts) < 4 {
		// Pad with empty strings if necessary
		for len(parts) < 4 {
			parts = append(parts, "")
		}
	}
	to = parts[0]
	from = parts[1]
	flags = parts[2]
	payload = parts[3]

	if to == "" {
		return "", "", "", "", fmt.Errorf("empty to in echo message: %q", text)
	}
	return to, from, flags, payload, nil
}

// IsEcho checks if text is an echo-formatted message.
func IsEcho(text string) bool {
	return strings.HasPrefix(text, EchoPrefix)
}

// FormatResponse formats a response message: {to}:{from}:{flags}:{payload}
func FormatResponse(to, from, flags, payload string) string {
	return to + ":" + from + ":" + flags + ":" + payload
}

// ParseResponse parses a response message, returning (to, from, flags, payload).
func ParseResponse(text string) (to, from, flags, payload string, err error) {
	parts := strings.SplitN(text, ":", 4)
	if len(parts) < 4 {
		// Pad with empty strings if necessary
		for len(parts) < 4 {
			parts = append(parts, "")
		}
	}
	to = parts[0]
	from = parts[1]
	flags = parts[2]
	payload = parts[3]

	if to == "" {
		return "", "", "", "", errors.New("empty to in response message")
	}
	return to, from, flags, payload, nil
}

// ParseFlags parses a flags string and returns (compressed, chunkSeq, chunkTotal).
// Format: "-" for no flags, "z" for compressed, "N/T" for chunk N of T, "zN/T" for compressed chunk.
// If chunkSeq is 0, there is no chunking.
func ParseFlags(flags string) (compressed bool, chunkSeq, chunkTotal int, err error) {
	if flags == "" || flags == "-" {
		return false, 0, 0, nil
	}

	compressed = strings.HasPrefix(flags, "z")
	remainder := flags
	if compressed {
		remainder = flags[1:]
	}

	if remainder == "" {
		return compressed, 0, 0, nil
	}

	// Try to parse as N/T
	parts := strings.Split(remainder, "/")
	if len(parts) == 2 {
		seq, errSeq := strconv.Atoi(parts[0])
		total, errTotal := strconv.Atoi(parts[1])
		if errSeq == nil && errTotal == nil && seq > 0 && total > 0 {
			return compressed, seq, total, nil
		}
	}

	return false, 0, 0, fmt.Errorf("invalid flags format: %q", flags)
}

// FormatFlags formats a flags string from components.
// If chunkSeq is 0, no chunking. Otherwise format is "N/T" or "zN/T" if compressed.
func FormatFlags(compressed bool, chunkSeq, chunkTotal int) string {
	if chunkSeq == 0 {
		// No chunking
		if compressed {
			return "z"
		}
		return "-"
	}

	// Has chunking
	flags := fmt.Sprintf("%d/%d", chunkSeq, chunkTotal)
	if compressed {
		flags = "z" + flags
	}
	return flags
}
