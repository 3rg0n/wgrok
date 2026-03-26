// Package wgrok implements an ngrok clone using the Webex API as a message bus.
package wgrok

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const EchoPrefix = "./echo:"
const PauseCmd = "./pause"
const ResumeCmd = "./resume"

var sparkMentionRe = regexp.MustCompile(`<spark-mention[^>]*>([^<]+)</spark-mention>`)

// StripBotMention strips a bot display name prefix from text using spark-mention tags in HTML.
func StripBotMention(text, html string) string {
	if html == "" {
		return text
	}
	matches := sparkMentionRe.FindStringSubmatch(html)
	if len(matches) < 2 {
		return text
	}
	displayName := matches[1]
	if strings.HasPrefix(text, displayName) {
		return strings.TrimLeft(text[len(displayName):], " ")
	}
	return text
}

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

// ParseFlags parses a flags string and returns (compressed, encrypted, chunkSeq, chunkTotal).
// Format: "-" for no flags, "z" for compressed, "e" for encrypted, "N/T" for chunk N of T.
// Combinations: "z", "e", "ze", "1/3", "z1/3", "e1/3", "ze1/3" etc.
// If chunkSeq is 0, there is no chunking.
func ParseFlags(flags string) (compressed, encrypted bool, chunkSeq, chunkTotal int, err error) {
	if flags == "" || flags == "-" {
		return false, false, 0, 0, nil
	}

	remainder := flags

	// Strip leading 'z' for compression
	compressed = strings.HasPrefix(remainder, "z")
	if compressed {
		remainder = remainder[1:]
	}

	// Strip leading 'e' for encryption
	encrypted = strings.HasPrefix(remainder, "e")
	if encrypted {
		remainder = remainder[1:]
	}

	if remainder == "" {
		return compressed, encrypted, 0, 0, nil
	}

	// Try to parse as N/T
	parts := strings.Split(remainder, "/")
	if len(parts) == 2 {
		seq, errSeq := strconv.Atoi(parts[0])
		total, errTotal := strconv.Atoi(parts[1])
		if errSeq == nil && errTotal == nil && seq > 0 && total > 0 {
			return compressed, encrypted, seq, total, nil
		}
	}

	return false, false, 0, 0, fmt.Errorf("invalid flags format: %q", flags)
}

// FormatFlags formats a flags string from components.
// If chunkSeq is 0, no chunking. Otherwise format is "N/T" or "zN/T" if compressed, "eN/T" if encrypted, "zeN/T" if both.
func FormatFlags(compressed, encrypted bool, chunkSeq, chunkTotal int) string {
	var flags string

	if chunkSeq == 0 {
		// No chunking
		flags = ""
	} else {
		// Has chunking
		flags = fmt.Sprintf("%d/%d", chunkSeq, chunkTotal)
	}

	// Prepend markers: z first (compression), then e (encryption)
	result := flags
	if encrypted {
		result = "e" + result
	}
	if compressed {
		result = "z" + result
	}

	if result == "" {
		return "-"
	}
	return result
}

// IsPause checks if text is a pause control message.
func IsPause(text string) bool {
	return strings.TrimSpace(text) == PauseCmd
}

// IsResume checks if text is a resume control message.
func IsResume(text string) bool {
	return strings.TrimSpace(text) == ResumeCmd
}
