package wgrok

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const (
	DiscordAPIBase = "https://discord.com/api/v10"
)

// Package-level URL, overridable in tests.
var (
	DiscordChannelMessagesURL = func(channelID string) string {
		return DiscordAPIBase + "/channels/" + channelID + "/messages"
	}
)

type discordMessagePayload struct {
	Content string        `json:"content"`
	Embeds  []interface{} `json:"embeds,omitempty"`
}

// SendDiscordMessage sends a text-only Discord message to a channel.
func SendDiscordMessage(token, channelID, text string, client *http.Client) (map[string]interface{}, error) {
	payload := discordMessagePayload{Content: text}
	return postDiscordMessage(token, channelID, payload, client)
}

// SendDiscordCard sends a Discord message with an embed.
func SendDiscordCard(token, channelID, text string, card interface{}, client *http.Client) (map[string]interface{}, error) {
	var embeds []interface{}
	if embedList, ok := card.([]interface{}); ok {
		embeds = embedList
	} else {
		embeds = []interface{}{card}
	}
	payload := discordMessagePayload{Content: text, Embeds: embeds}
	return postDiscordMessage(token, channelID, payload, client)
}

func postDiscordMessage(token, channelID string, payload discordMessagePayload, client *http.Client) (map[string]interface{}, error) {
	if client == nil {
		client = http.DefaultClient
	}
	url := DiscordChannelMessagesURL(channelID)
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal message payload: %w", err)
	}

	for attempt := 0; attempt <= MaxRetries; attempt++ {
		req, err := http.NewRequest("POST", url, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bot "+token)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("send message: %w", err)
		}

		// Check for 429 Too Many Requests
		if resp.StatusCode == http.StatusTooManyRequests {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if attempt < MaxRetries {
				retryAfter := 1
				if retryAfterStr := resp.Header.Get("Retry-After"); retryAfterStr != "" {
					if parsed, err := strconv.Atoi(retryAfterStr); err == nil {
						retryAfter = parsed
						if retryAfter > 300 {
							retryAfter = 300
						}
					}
				}
				time.Sleep(time.Duration(retryAfter) * time.Second)
				continue
			}
			return nil, fmt.Errorf("HTTP %d: rate limited after %d retries", resp.StatusCode, MaxRetries)
		}

		defer resp.Body.Close()
		return readJSONResponse(resp)
	}
	return nil, fmt.Errorf("unreachable")
}
