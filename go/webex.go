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
	WebexAPIBase            = "https://webexapis.com/v1"
	AdaptiveCardContentType = "application/vnd.microsoft.card.adaptive"
	MaxRetries              = 3
)

// Package-level URLs, overridable in tests.
var (
	WebexMessagesURL          = WebexAPIBase + "/messages"
	WebexAttachmentActionsURL = WebexAPIBase + "/attachment/actions"
)

// CardAttachment represents a Webex message card attachment.
type CardAttachment struct {
	ContentType string      `json:"contentType"`
	Content     interface{} `json:"content"`
}

type sendMessagePayload struct {
	ToPersonEmail string           `json:"toPersonEmail"`
	Text          string           `json:"text"`
	Attachments   []CardAttachment `json:"attachments,omitempty"`
}

// SendMessage sends a text-only Webex message to a person by email.
func SendMessage(token, toEmail, text string, client *http.Client) (map[string]interface{}, error) {
	payload := sendMessagePayload{ToPersonEmail: toEmail, Text: text}
	return postMessage(token, payload, client)
}

// SendCard sends a Webex message with an adaptive card attachment.
func SendCard(token, toEmail, text string, card interface{}, client *http.Client) (map[string]interface{}, error) {
	payload := sendMessagePayload{
		ToPersonEmail: toEmail,
		Text:          text,
		Attachments: []CardAttachment{
			{ContentType: AdaptiveCardContentType, Content: card},
		},
	}
	return postMessage(token, payload, client)
}

func postMessage(token string, payload sendMessagePayload, client *http.Client) (map[string]interface{}, error) {
	if client == nil {
		client = http.DefaultClient
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal message payload: %w", err)
	}

	for attempt := 0; attempt <= MaxRetries; attempt++ {
		req, err := http.NewRequest("POST", WebexMessagesURL, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("send message: %w", err)
		}
		defer resp.Body.Close()

		// Check for 429 Too Many Requests
		if resp.StatusCode == http.StatusTooManyRequests {
			if attempt < MaxRetries {
				// Read Retry-After header, default to 1 second
				retryAfter := 1
				if retryAfterStr := resp.Header.Get("Retry-After"); retryAfterStr != "" {
					if parsed, err := strconv.Atoi(retryAfterStr); err == nil {
						retryAfter = parsed
					}
				}
				time.Sleep(time.Duration(retryAfter) * time.Second)
				continue
			}
			// Max retries exhausted, fall through to readJSONResponse which will handle the error
		}

		return readJSONResponse(resp)
	}
	return nil, fmt.Errorf("unreachable")
}

// GetMessage fetches full message details by ID (includes attachments).
func GetMessage(token, messageID string, client *http.Client) (map[string]interface{}, error) {
	url := WebexMessagesURL + "/" + messageID
	return getJSON(token, url, client)
}

// GetAttachmentAction fetches an attachment action (card submission) by ID.
func GetAttachmentAction(token, actionID string, client *http.Client) (map[string]interface{}, error) {
	url := WebexAttachmentActionsURL + "/" + actionID
	return getJSON(token, url, client)
}

func getJSON(token, url string, client *http.Client) (map[string]interface{}, error) {
	if client == nil {
		client = http.DefaultClient
	}

	for attempt := 0; attempt <= MaxRetries; attempt++ {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("GET %s: %w", url, err)
		}
		defer resp.Body.Close()

		// Check for 429 Too Many Requests
		if resp.StatusCode == http.StatusTooManyRequests {
			if attempt < MaxRetries {
				// Read Retry-After header, default to 1 second
				retryAfter := 1
				if retryAfterStr := resp.Header.Get("Retry-After"); retryAfterStr != "" {
					if parsed, err := strconv.Atoi(retryAfterStr); err == nil {
						retryAfter = parsed
					}
				}
				time.Sleep(time.Duration(retryAfter) * time.Second)
				continue
			}
			// Max retries exhausted, fall through to readJSONResponse which will handle the error
		}

		return readJSONResponse(resp)
	}
	return nil, fmt.Errorf("unreachable")
}

func readJSONResponse(resp *http.Response) (map[string]interface{}, error) {
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(data))
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return result, nil
}

// ExtractCards extracts adaptive card content from a message's attachments.
func ExtractCards(message map[string]interface{}) []interface{} {
	attachments, ok := message["attachments"].([]interface{})
	if !ok {
		return nil
	}
	var cards []interface{}
	for _, att := range attachments {
		attMap, ok := att.(map[string]interface{})
		if !ok {
			continue
		}
		if attMap["contentType"] == AdaptiveCardContentType {
			if content, ok := attMap["content"]; ok {
				cards = append(cards, content)
			}
		}
	}
	return cards
}
