package wgrok

import (
	"fmt"
	"os"
	"strings"
)

// SenderConfig holds configuration for WgrokSender.
type SenderConfig struct {
	WebexToken string
	Target     string
	Slug       string
	Domains    []string
	Platform   string
	Debug      bool
}

// BotConfig holds configuration for WgrokRouterBot.
type BotConfig struct {
	WebexToken     string
	Domains        []string
	Routes         map[string]string
	PlatformTokens map[string][]string
	WebhookPort    *int
	WebhookSecret  *string
	Debug          bool
}

// ReceiverConfig holds configuration for WgrokReceiver.
type ReceiverConfig struct {
	WebexToken string
	Slug       string
	Domains    []string
	Platform   string
	Debug      bool
}

func envRequire(name string) (string, error) {
	val := os.Getenv(name)
	if val == "" {
		return "", fmt.Errorf("required environment variable %s is not set", name)
	}
	return val, nil
}

func envParseDomains(raw string) []string {
	var domains []string
	for _, d := range strings.Split(raw, ",") {
		d = strings.TrimSpace(d)
		if d != "" {
			domains = append(domains, d)
		}
	}
	return domains
}

func envParseDebug(raw string) bool {
	v := strings.TrimSpace(strings.ToLower(raw))
	return v == "true" || v == "1" || v == "yes"
}

func parseRoutes(raw string) map[string]string {
	routes := make(map[string]string)
	if raw == "" {
		return routes
	}
	for _, pair := range strings.Split(raw, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) == 2 {
			slug := strings.TrimSpace(parts[0])
			target := strings.TrimSpace(parts[1])
			if slug != "" && target != "" {
				routes[slug] = target
			}
		}
	}
	return routes
}

func parsePlatformTokens() map[string][]string {
	tokens := make(map[string][]string)

	// Check each platform-specific token env var
	platforms := []string{"webex", "slack", "discord", "irc"}
	for _, platform := range platforms {
		envVar := fmt.Sprintf("WGROK_%s_TOKENS", strings.ToUpper(platform))
		if raw := os.Getenv(envVar); raw != "" {
			var platformTokens []string
			for _, token := range strings.Split(raw, ",") {
				token = strings.TrimSpace(token)
				if token != "" {
					platformTokens = append(platformTokens, token)
				}
			}
			if len(platformTokens) > 0 {
				tokens[platform] = platformTokens
			}
		}
	}

	// If no platform-specific tokens, fall back to WGROK_TOKEN as webex
	if len(tokens) == 0 {
		if token := os.Getenv("WGROK_TOKEN"); token != "" {
			tokens["webex"] = []string{token}
		}
	}

	return tokens
}

func parseInt(raw string) *int {
	if raw == "" {
		return nil
	}
	val := 0
	_, err := fmt.Sscanf(strings.TrimSpace(raw), "%d", &val)
	if err != nil {
		return nil
	}
	return &val
}

func parseStringPtr(raw string) *string {
	if raw == "" {
		return nil
	}
	return &raw
}

// SenderConfigFromEnv loads a SenderConfig from environment variables.
func SenderConfigFromEnv() (*SenderConfig, error) {
	token, err := envRequire("WGROK_TOKEN")
	if err != nil {
		return nil, err
	}
	target, err := envRequire("WGROK_TARGET")
	if err != nil {
		return nil, err
	}
	slug, err := envRequire("WGROK_SLUG")
	if err != nil {
		return nil, err
	}
	platform := os.Getenv("WGROK_PLATFORM")
	if platform == "" {
		platform = "webex"
	}
	return &SenderConfig{
		WebexToken: token,
		Target:     target,
		Slug:       slug,
		Domains:    envParseDomains(os.Getenv("WGROK_DOMAINS")),
		Platform:   platform,
		Debug:      envParseDebug(os.Getenv("WGROK_DEBUG")),
	}, nil
}

// BotConfigFromEnv loads a BotConfig from environment variables.
func BotConfigFromEnv() (*BotConfig, error) {
	// If platform-specific tokens are provided, use those; otherwise fall back to WGROK_TOKEN
	platformTokens := parsePlatformTokens()

	domainsRaw, err := envRequire("WGROK_DOMAINS")
	if err != nil {
		return nil, err
	}

	// If no platform-specific tokens and no WGROK_TOKEN, error out
	if len(platformTokens) == 0 {
		return nil, fmt.Errorf("required environment variable WGROK_TOKEN is not set")
	}

	// For backward compatibility, extract webex token if it exists
	var webexToken string
	if tokens, ok := platformTokens["webex"]; ok && len(tokens) > 0 {
		webexToken = tokens[0]
	}

	return &BotConfig{
		WebexToken:     webexToken,
		Domains:        envParseDomains(domainsRaw),
		Routes:         parseRoutes(os.Getenv("WGROK_ROUTES")),
		PlatformTokens: platformTokens,
		WebhookPort:    parseInt(os.Getenv("WGROK_WEBHOOK_PORT")),
		WebhookSecret:  parseStringPtr(os.Getenv("WGROK_WEBHOOK_SECRET")),
		Debug:          envParseDebug(os.Getenv("WGROK_DEBUG")),
	}, nil
}

// ReceiverConfigFromEnv loads a ReceiverConfig from environment variables.
func ReceiverConfigFromEnv() (*ReceiverConfig, error) {
	token, err := envRequire("WGROK_TOKEN")
	if err != nil {
		return nil, err
	}
	slug, err := envRequire("WGROK_SLUG")
	if err != nil {
		return nil, err
	}
	domainsRaw, err := envRequire("WGROK_DOMAINS")
	if err != nil {
		return nil, err
	}
	platform := os.Getenv("WGROK_PLATFORM")
	if platform == "" {
		platform = "webex"
	}
	return &ReceiverConfig{
		WebexToken: token,
		Slug:       slug,
		Domains:    envParseDomains(domainsRaw),
		Platform:   platform,
		Debug:      envParseDebug(os.Getenv("WGROK_DEBUG")),
	}, nil
}
