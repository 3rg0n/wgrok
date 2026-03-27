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
	SlackAPIBase = "https://slack.com/api"
)

// Package-level URL, overridable in tests.
var (
	SlackPostMessageURL = SlackAPIBase + "/chat.postMessage"
)

type slackMessagePayload struct {
	Channel string        `json:"channel"`
	Text    string        `json:"text"`
	Blocks  []interface{} `json:"blocks,omitempty"`
}

// SendSlackMessage sends a text-only Slack message to a channel.
func SendSlackMessage(token, channel, text string, client *http.Client) (map[string]interface{}, error) {
	payload := slackMessagePayload{Channel: channel, Text: text}
	return postSlackMessage(token, payload, client)
}

// SendSlackCard sends a Slack message with Block Kit blocks.
func SendSlackCard(token, channel, text string, card interface{}, client *http.Client) (map[string]interface{}, error) {
	var blocks []interface{}
	if blockList, ok := card.([]interface{}); ok {
		blocks = blockList
	} else {
		blocks = []interface{}{card}
	}
	payload := slackMessagePayload{Channel: channel, Text: text, Blocks: blocks}
	return postSlackMessage(token, payload, client)
}

func postSlackMessage(token string, payload slackMessagePayload, client *http.Client) (map[string]interface{}, error) {
	if client == nil {
		client = http.DefaultClient
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal message payload: %w", err)
	}

	for attempt := 0; attempt <= MaxRetries; attempt++ {
		req, err := http.NewRequest("POST", SlackPostMessageURL, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("send message: %w", err)
		}

		// Check for 429 Too Many Requests
		if resp.StatusCode == http.StatusTooManyRequests {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
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
