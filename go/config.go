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
	Debug      bool
}

// BotConfig holds configuration for WgrokEchoBot.
type BotConfig struct {
	WebexToken string
	Domains    []string
	Debug      bool
}

// ReceiverConfig holds configuration for WgrokReceiver.
type ReceiverConfig struct {
	WebexToken string
	Slug       string
	Domains    []string
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
	return &SenderConfig{
		WebexToken: token,
		Target:     target,
		Slug:       slug,
		Domains:    envParseDomains(os.Getenv("WGROK_DOMAINS")),
		Debug:      envParseDebug(os.Getenv("WGROK_DEBUG")),
	}, nil
}

// BotConfigFromEnv loads a BotConfig from environment variables.
func BotConfigFromEnv() (*BotConfig, error) {
	token, err := envRequire("WGROK_TOKEN")
	if err != nil {
		return nil, err
	}
	domainsRaw, err := envRequire("WGROK_DOMAINS")
	if err != nil {
		return nil, err
	}
	return &BotConfig{
		WebexToken: token,
		Domains:    envParseDomains(domainsRaw),
		Debug:      envParseDebug(os.Getenv("WGROK_DEBUG")),
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
	return &ReceiverConfig{
		WebexToken: token,
		Slug:       slug,
		Domains:    envParseDomains(domainsRaw),
		Debug:      envParseDebug(os.Getenv("WGROK_DEBUG")),
	}, nil
}
