package wgrok

import (
	"encoding/json"
	"os"
	"testing"
)

type configCases struct {
	Sender struct {
		Valid struct {
			Env      map[string]string `json:"env"`
			Expected struct {
				WebexToken string   `json:"webex_token"`
				Target     string   `json:"target"`
				Slug       string   `json:"slug"`
				Domains    []string `json:"domains"`
				Debug      bool     `json:"debug"`
				Platform   string   `json:"platform"`
			} `json:"expected"`
		} `json:"valid"`
		MissingToken struct {
			Env           map[string]string `json:"env"`
			ErrorContains string            `json:"error_contains"`
		} `json:"missing_token"`
		MissingTarget struct {
			Env           map[string]string `json:"env"`
			ErrorContains string            `json:"error_contains"`
		} `json:"missing_target"`
		DebugDefaultsFalse struct {
			Env           map[string]string `json:"env"`
			ExpectedDebug bool              `json:"expected_debug"`
		} `json:"debug_defaults_false"`
		DomainsOptional struct {
			Env             map[string]string `json:"env"`
			ExpectedDomains []string          `json:"expected_domains"`
		} `json:"domains_optional"`
		PlatformDefaultsWebex struct {
			Env              map[string]string `json:"env"`
			ExpectedPlatform string            `json:"expected_platform"`
		} `json:"platform_defaults_webex"`
		PlatformExplicit struct {
			Env              map[string]string `json:"env"`
			ExpectedPlatform string            `json:"expected_platform"`
		} `json:"platform_explicit"`
	} `json:"sender"`
	Bot struct {
		Valid struct {
			Env      map[string]string `json:"env"`
			Expected struct {
				WebexToken string   `json:"webex_token"`
				Domains    []string `json:"domains"`
			} `json:"expected"`
		} `json:"valid"`
		MissingDomains struct {
			Env           map[string]string `json:"env"`
			ErrorContains string            `json:"error_contains"`
		} `json:"missing_domains"`
		WithRoutes struct {
			Env            map[string]string `json:"env"`
			ExpectedRoutes map[string]string `json:"expected_routes"`
		} `json:"with_routes"`
		RoutesEmptyWhenNotSet struct {
			Env            map[string]string `json:"env"`
			ExpectedRoutes map[string]string `json:"expected_routes"`
		} `json:"routes_empty_when_not_set"`
		WithWebhook struct {
			Env                   map[string]string `json:"env"`
			ExpectedWebhookPort   int               `json:"expected_webhook_port"`
			ExpectedWebhookSecret string            `json:"expected_webhook_secret"`
		} `json:"with_webhook"`
		WebhookDisabledByDefault struct {
			Env                   map[string]string `json:"env"`
			ExpectedWebhookPort   interface{}       `json:"expected_webhook_port"`
			ExpectedWebhookSecret interface{}       `json:"expected_webhook_secret"`
		} `json:"webhook_disabled_by_default"`
		WithPlatformTokens struct {
			Env                    map[string]string   `json:"env"`
			ExpectedPlatformTokens map[string][]string `json:"expected_platform_tokens"`
		} `json:"with_platform_tokens"`
		FallbackSingleToken struct {
			Env                    map[string]string   `json:"env"`
			ExpectedPlatformTokens map[string][]string `json:"expected_platform_tokens"`
		} `json:"fallback_single_token"`
	} `json:"bot"`
	Receiver struct {
		Valid struct {
			Env      map[string]string `json:"env"`
			Expected struct {
				WebexToken string   `json:"webex_token"`
				Slug       string   `json:"slug"`
				Domains    []string `json:"domains"`
				Platform   string   `json:"platform"`
			} `json:"expected"`
		} `json:"valid"`
		PlatformExplicit struct {
			Env              map[string]string `json:"env"`
			ExpectedPlatform string            `json:"expected_platform"`
		} `json:"platform_explicit"`
	} `json:"receiver"`
	DebugTruthyValues []string `json:"debug_truthy_values"`
	DebugFalsyValues  []string `json:"debug_falsy_values"`
}

func loadConfigCases(t *testing.T) configCases {
	t.Helper()
	data, err := os.ReadFile("../tests/config_cases.json")
	if err != nil {
		t.Fatalf("load config cases: %v", err)
	}
	var cases configCases
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatalf("parse config cases: %v", err)
	}
	return cases
}

func setEnv(t *testing.T, env map[string]string) {
	t.Helper()
	// Clear all WGROK_ vars first
	for _, kv := range os.Environ() {
		if len(kv) > 6 && kv[:6] == "WGROK_" {
			key := kv[:indexOf(kv, "=")]
			t.Setenv(key, "")
			os.Unsetenv(key)
		}
	}
	for k, v := range env {
		t.Setenv(k, v)
	}
}

func indexOf(s string, c string) int {
	for i := range s {
		if string(s[i]) == c {
			return i
		}
	}
	return len(s)
}

func sliceEqual(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func mapEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func platformTokensEqual(a, b map[string][]string) bool {
	if len(a) != len(b) {
		return false
	}
	for platform, tokens := range a {
		bTokens, ok := b[platform]
		if !ok {
			return false
		}
		if !sliceEqual(tokens, bTokens) {
			return false
		}
	}
	return true
}

func TestSenderConfigFromEnv(t *testing.T) {
	cases := loadConfigCases(t)

	t.Run("valid", func(t *testing.T) {
		setEnv(t, cases.Sender.Valid.Env)
		cfg, err := SenderConfigFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		exp := cases.Sender.Valid.Expected
		if cfg.WebexToken != exp.WebexToken {
			t.Errorf("WebexToken = %q, want %q", cfg.WebexToken, exp.WebexToken)
		}
		if cfg.Target != exp.Target {
			t.Errorf("Target = %q, want %q", cfg.Target, exp.Target)
		}
		if cfg.Slug != exp.Slug {
			t.Errorf("Slug = %q, want %q", cfg.Slug, exp.Slug)
		}
		if !sliceEqual(cfg.Domains, exp.Domains) {
			t.Errorf("Domains = %v, want %v", cfg.Domains, exp.Domains)
		}
		if cfg.Debug != exp.Debug {
			t.Errorf("Debug = %v, want %v", cfg.Debug, exp.Debug)
		}
		if cfg.Platform != exp.Platform {
			t.Errorf("Platform = %q, want %q", cfg.Platform, exp.Platform)
		}
	})

	t.Run("missing_token", func(t *testing.T) {
		setEnv(t, cases.Sender.MissingToken.Env)
		_, err := SenderConfigFromEnv()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !containsInsensitive(err.Error(), cases.Sender.MissingToken.ErrorContains) {
			t.Errorf("error %q should contain %q", err.Error(), cases.Sender.MissingToken.ErrorContains)
		}
	})

	t.Run("missing_target", func(t *testing.T) {
		setEnv(t, cases.Sender.MissingTarget.Env)
		_, err := SenderConfigFromEnv()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !containsInsensitive(err.Error(), cases.Sender.MissingTarget.ErrorContains) {
			t.Errorf("error %q should contain %q", err.Error(), cases.Sender.MissingTarget.ErrorContains)
		}
	})

	t.Run("debug_defaults_false", func(t *testing.T) {
		setEnv(t, cases.Sender.DebugDefaultsFalse.Env)
		cfg, err := SenderConfigFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Debug != cases.Sender.DebugDefaultsFalse.ExpectedDebug {
			t.Errorf("Debug = %v, want %v", cfg.Debug, cases.Sender.DebugDefaultsFalse.ExpectedDebug)
		}
	})

	t.Run("domains_optional", func(t *testing.T) {
		setEnv(t, cases.Sender.DomainsOptional.Env)
		cfg, err := SenderConfigFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !sliceEqual(cfg.Domains, cases.Sender.DomainsOptional.ExpectedDomains) {
			t.Errorf("Domains = %v, want %v", cfg.Domains, cases.Sender.DomainsOptional.ExpectedDomains)
		}
	})

	t.Run("platform_defaults_webex", func(t *testing.T) {
		setEnv(t, cases.Sender.PlatformDefaultsWebex.Env)
		cfg, err := SenderConfigFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Platform != cases.Sender.PlatformDefaultsWebex.ExpectedPlatform {
			t.Errorf("Platform = %q, want %q", cfg.Platform, cases.Sender.PlatformDefaultsWebex.ExpectedPlatform)
		}
	})

	t.Run("platform_explicit", func(t *testing.T) {
		setEnv(t, cases.Sender.PlatformExplicit.Env)
		cfg, err := SenderConfigFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Platform != cases.Sender.PlatformExplicit.ExpectedPlatform {
			t.Errorf("Platform = %q, want %q", cfg.Platform, cases.Sender.PlatformExplicit.ExpectedPlatform)
		}
	})
}

func TestBotConfigFromEnv(t *testing.T) {
	cases := loadConfigCases(t)

	t.Run("valid", func(t *testing.T) {
		setEnv(t, cases.Bot.Valid.Env)
		cfg, err := BotConfigFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		exp := cases.Bot.Valid.Expected
		if cfg.WebexToken != exp.WebexToken {
			t.Errorf("WebexToken = %q, want %q", cfg.WebexToken, exp.WebexToken)
		}
		if !sliceEqual(cfg.Domains, exp.Domains) {
			t.Errorf("Domains = %v, want %v", cfg.Domains, exp.Domains)
		}
	})

	t.Run("missing_domains", func(t *testing.T) {
		setEnv(t, cases.Bot.MissingDomains.Env)
		_, err := BotConfigFromEnv()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !containsInsensitive(err.Error(), cases.Bot.MissingDomains.ErrorContains) {
			t.Errorf("error %q should contain %q", err.Error(), cases.Bot.MissingDomains.ErrorContains)
		}
	})

	t.Run("with_routes", func(t *testing.T) {
		setEnv(t, cases.Bot.WithRoutes.Env)
		cfg, err := BotConfigFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !mapEqual(cfg.Routes, cases.Bot.WithRoutes.ExpectedRoutes) {
			t.Errorf("Routes = %v, want %v", cfg.Routes, cases.Bot.WithRoutes.ExpectedRoutes)
		}
	})

	t.Run("routes_empty_when_not_set", func(t *testing.T) {
		setEnv(t, cases.Bot.RoutesEmptyWhenNotSet.Env)
		cfg, err := BotConfigFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !mapEqual(cfg.Routes, cases.Bot.RoutesEmptyWhenNotSet.ExpectedRoutes) {
			t.Errorf("Routes = %v, want %v", cfg.Routes, cases.Bot.RoutesEmptyWhenNotSet.ExpectedRoutes)
		}
	})

	t.Run("with_webhook", func(t *testing.T) {
		setEnv(t, cases.Bot.WithWebhook.Env)
		cfg, err := BotConfigFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.WebhookPort == nil || *cfg.WebhookPort != cases.Bot.WithWebhook.ExpectedWebhookPort {
			var got int
			if cfg.WebhookPort != nil {
				got = *cfg.WebhookPort
			}
			t.Errorf("WebhookPort = %v, want %v", got, cases.Bot.WithWebhook.ExpectedWebhookPort)
		}
		if cfg.WebhookSecret == nil || *cfg.WebhookSecret != cases.Bot.WithWebhook.ExpectedWebhookSecret {
			var got string
			if cfg.WebhookSecret != nil {
				got = *cfg.WebhookSecret
			}
			t.Errorf("WebhookSecret = %q, want %q", got, cases.Bot.WithWebhook.ExpectedWebhookSecret)
		}
	})

	t.Run("webhook_disabled_by_default", func(t *testing.T) {
		setEnv(t, cases.Bot.WebhookDisabledByDefault.Env)
		cfg, err := BotConfigFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.WebhookPort != nil {
			t.Errorf("WebhookPort = %v, want nil", *cfg.WebhookPort)
		}
		if cfg.WebhookSecret != nil {
			t.Errorf("WebhookSecret = %q, want nil", *cfg.WebhookSecret)
		}
	})

	t.Run("with_platform_tokens", func(t *testing.T) {
		setEnv(t, cases.Bot.WithPlatformTokens.Env)
		cfg, err := BotConfigFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !platformTokensEqual(cfg.PlatformTokens, cases.Bot.WithPlatformTokens.ExpectedPlatformTokens) {
			t.Errorf("PlatformTokens = %v, want %v", cfg.PlatformTokens, cases.Bot.WithPlatformTokens.ExpectedPlatformTokens)
		}
	})

	t.Run("fallback_single_token", func(t *testing.T) {
		setEnv(t, cases.Bot.FallbackSingleToken.Env)
		cfg, err := BotConfigFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !platformTokensEqual(cfg.PlatformTokens, cases.Bot.FallbackSingleToken.ExpectedPlatformTokens) {
			t.Errorf("PlatformTokens = %v, want %v", cfg.PlatformTokens, cases.Bot.FallbackSingleToken.ExpectedPlatformTokens)
		}
	})
}

func TestReceiverConfigFromEnv(t *testing.T) {
	cases := loadConfigCases(t)

	t.Run("valid", func(t *testing.T) {
		setEnv(t, cases.Receiver.Valid.Env)
		cfg, err := ReceiverConfigFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		exp := cases.Receiver.Valid.Expected
		if cfg.WebexToken != exp.WebexToken {
			t.Errorf("WebexToken = %q, want %q", cfg.WebexToken, exp.WebexToken)
		}
		if cfg.Slug != exp.Slug {
			t.Errorf("Slug = %q, want %q", cfg.Slug, exp.Slug)
		}
		if !sliceEqual(cfg.Domains, exp.Domains) {
			t.Errorf("Domains = %v, want %v", cfg.Domains, exp.Domains)
		}
		if cfg.Platform != exp.Platform {
			t.Errorf("Platform = %q, want %q", cfg.Platform, exp.Platform)
		}
	})

	t.Run("platform_explicit", func(t *testing.T) {
		setEnv(t, cases.Receiver.PlatformExplicit.Env)
		cfg, err := ReceiverConfigFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Platform != cases.Receiver.PlatformExplicit.ExpectedPlatform {
			t.Errorf("Platform = %q, want %q", cfg.Platform, cases.Receiver.PlatformExplicit.ExpectedPlatform)
		}
	})
}

func TestDebugParsing(t *testing.T) {
	cases := loadConfigCases(t)
	for _, val := range cases.DebugTruthyValues {
		t.Run("truthy_"+val, func(t *testing.T) {
			if !envParseDebug(val) {
				t.Errorf("envParseDebug(%q) = false, want true", val)
			}
		})
	}
	for _, val := range cases.DebugFalsyValues {
		t.Run("falsy_"+val, func(t *testing.T) {
			if envParseDebug(val) {
				t.Errorf("envParseDebug(%q) = true, want false", val)
			}
		})
	}
}
