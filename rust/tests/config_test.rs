use serde::Deserialize;
use std::collections::HashMap;
use std::fs;
use std::sync::Mutex;
use wgrok::config::{parse_debug, BotConfig, ReceiverConfig, SenderConfig};

static ENV_LOCK: Mutex<()> = Mutex::new(());

#[derive(Deserialize)]
struct ConfigCases {
    sender: SenderCases,
    bot: BotCases,
    receiver: ReceiverCases,
    debug_truthy_values: Vec<String>,
    debug_falsy_values: Vec<String>,
}

#[derive(Deserialize)]
struct SenderPlatformDefaults {
    env: HashMap<String, String>,
    expected_platform: String,
}

#[derive(Deserialize)]
struct SenderPlatformExplicit {
    env: HashMap<String, String>,
    expected_platform: String,
}

#[derive(Deserialize)]
struct SenderCases {
    valid: ValidSender,
    missing_token: ErrorCase,
    missing_target: ErrorCase,
    debug_defaults_false: DebugDefaultCase,
    domains_optional: DomainsOptionalCase,
    #[serde(default)]
    platform_defaults_webex: Option<SenderPlatformDefaults>,
    #[serde(default)]
    platform_explicit: Option<SenderPlatformExplicit>,
}

#[derive(Deserialize)]
struct ValidSender {
    env: HashMap<String, String>,
    expected: ExpectedSender,
}

#[derive(Deserialize)]
struct ExpectedSender {
    webex_token: String,
    target: String,
    slug: String,
    domains: Vec<String>,
    debug: bool,
    platform: String,
}

#[derive(Deserialize)]
struct ErrorCase {
    env: HashMap<String, String>,
    error_contains: String,
}

#[derive(Deserialize)]
struct DebugDefaultCase {
    env: HashMap<String, String>,
    expected_debug: bool,
}

#[derive(Deserialize)]
struct DomainsOptionalCase {
    env: HashMap<String, String>,
    expected_domains: Vec<String>,
}

#[derive(Deserialize)]
struct BotCases {
    valid: ValidBot,
    missing_domains: ErrorCase,
    #[serde(default)]
    with_routes: Option<BotWithRoutes>,
    #[serde(default)]
    routes_empty_when_not_set: Option<BotRoutesEmpty>,
    #[serde(default)]
    with_webhook: Option<BotWithWebhook>,
    #[serde(default)]
    webhook_disabled_by_default: Option<BotWebhookDisabled>,
    #[serde(default)]
    with_platform_tokens: Option<BotWithPlatformTokens>,
    #[serde(default)]
    fallback_single_token: Option<BotFallbackToken>,
}

#[derive(Deserialize)]
struct ValidBot {
    env: HashMap<String, String>,
    expected: ExpectedBot,
}

#[derive(Deserialize)]
struct ExpectedBot {
    webex_token: String,
    domains: Vec<String>,
}

#[derive(Deserialize)]
struct BotWithRoutes {
    env: HashMap<String, String>,
    expected_routes: HashMap<String, String>,
}

#[derive(Deserialize)]
struct BotRoutesEmpty {
    env: HashMap<String, String>,
    expected_routes: HashMap<String, String>,
}

#[derive(Deserialize)]
struct BotWithWebhook {
    env: HashMap<String, String>,
    expected_webhook_port: Option<u16>,
    expected_webhook_secret: Option<String>,
}

#[derive(Deserialize)]
struct BotWebhookDisabled {
    env: HashMap<String, String>,
    expected_webhook_port: Option<u16>,
    expected_webhook_secret: Option<String>,
}

#[derive(Deserialize)]
struct BotWithPlatformTokens {
    env: HashMap<String, String>,
    expected_platform_tokens: HashMap<String, Vec<String>>,
}

#[derive(Deserialize)]
struct BotFallbackToken {
    env: HashMap<String, String>,
    expected_platform_tokens: HashMap<String, Vec<String>>,
}

#[derive(Deserialize)]
struct ReceiverCases {
    valid: ValidReceiver,
    #[serde(default)]
    platform_explicit: Option<SenderPlatformExplicit>,
}

#[derive(Deserialize)]
struct ValidReceiver {
    env: HashMap<String, String>,
    expected: ExpectedReceiver,
}

#[derive(Deserialize)]
struct ExpectedReceiver {
    webex_token: String,
    slug: String,
    domains: Vec<String>,
    platform: String,
}

fn load_cases() -> ConfigCases {
    let data = fs::read_to_string("../tests/config_cases.json").expect("load config cases");
    serde_json::from_str(&data).expect("parse config cases")
}

fn set_env(env: &HashMap<String, String>) {
    for (key, _) in std::env::vars() {
        if key.starts_with("WGROK_") {
            unsafe { std::env::remove_var(&key); }
        }
    }
    for (k, v) in env {
        unsafe { std::env::set_var(k, v); }
    }
}

fn clear_env() {
    for (key, _) in std::env::vars() {
        if key.starts_with("WGROK_") {
            unsafe { std::env::remove_var(&key); }
        }
    }
}

/// All config tests run sequentially under a single mutex to avoid env var races.
#[test]
fn test_all_config() {
    let _lock = ENV_LOCK.lock().unwrap();
    let cases = load_cases();

    // Sender valid
    set_env(&cases.sender.valid.env);
    let cfg = SenderConfig::from_env().unwrap();
    let exp = &cases.sender.valid.expected;
    assert_eq!(cfg.webex_token, exp.webex_token);
    assert_eq!(cfg.target, exp.target);
    assert_eq!(cfg.slug, exp.slug);
    assert_eq!(cfg.domains, exp.domains);
    assert_eq!(cfg.debug, exp.debug);
    assert_eq!(cfg.platform, exp.platform);

    // Sender missing token
    clear_env();
    set_env(&cases.sender.missing_token.env);
    let result = SenderConfig::from_env();
    assert!(result.is_err());
    assert!(result
        .unwrap_err()
        .to_lowercase()
        .contains(&cases.sender.missing_token.error_contains.to_lowercase()));

    // Sender missing target
    clear_env();
    set_env(&cases.sender.missing_target.env);
    let result = SenderConfig::from_env();
    assert!(result.is_err());
    assert!(result
        .unwrap_err()
        .to_lowercase()
        .contains(&cases.sender.missing_target.error_contains.to_lowercase()));

    // Sender debug defaults false
    clear_env();
    set_env(&cases.sender.debug_defaults_false.env);
    let cfg = SenderConfig::from_env().unwrap();
    assert_eq!(cfg.debug, cases.sender.debug_defaults_false.expected_debug);

    // Sender domains optional
    clear_env();
    set_env(&cases.sender.domains_optional.env);
    let cfg = SenderConfig::from_env().unwrap();
    assert_eq!(cfg.domains, cases.sender.domains_optional.expected_domains);

    // Sender platform defaults webex
    if let Some(tc) = &cases.sender.platform_defaults_webex {
        clear_env();
        set_env(&tc.env);
        let cfg = SenderConfig::from_env().unwrap();
        assert_eq!(cfg.platform, tc.expected_platform);
    }

    // Sender platform explicit
    if let Some(tc) = &cases.sender.platform_explicit {
        clear_env();
        set_env(&tc.env);
        let cfg = SenderConfig::from_env().unwrap();
        assert_eq!(cfg.platform, tc.expected_platform);
    }

    // Bot valid
    clear_env();
    set_env(&cases.bot.valid.env);
    let cfg = BotConfig::from_env().unwrap();
    assert_eq!(cfg.webex_token, cases.bot.valid.expected.webex_token);
    assert_eq!(cfg.domains, cases.bot.valid.expected.domains);

    // Bot missing domains
    clear_env();
    set_env(&cases.bot.missing_domains.env);
    let result = BotConfig::from_env();
    assert!(result.is_err());

    // Bot with routes
    if let Some(tc) = &cases.bot.with_routes {
        clear_env();
        set_env(&tc.env);
        let cfg = BotConfig::from_env().unwrap();
        assert_eq!(cfg.routes, tc.expected_routes);
    }

    // Bot routes empty when not set
    if let Some(tc) = &cases.bot.routes_empty_when_not_set {
        clear_env();
        set_env(&tc.env);
        let cfg = BotConfig::from_env().unwrap();
        assert_eq!(cfg.routes, tc.expected_routes);
    }

    // Bot with webhook
    if let Some(tc) = &cases.bot.with_webhook {
        clear_env();
        set_env(&tc.env);
        let cfg = BotConfig::from_env().unwrap();
        assert_eq!(cfg.webhook_port, tc.expected_webhook_port);
        assert_eq!(cfg.webhook_secret, tc.expected_webhook_secret);
    }

    // Bot webhook disabled by default
    if let Some(tc) = &cases.bot.webhook_disabled_by_default {
        clear_env();
        set_env(&tc.env);
        let cfg = BotConfig::from_env().unwrap();
        assert_eq!(cfg.webhook_port, tc.expected_webhook_port);
        assert_eq!(cfg.webhook_secret, tc.expected_webhook_secret);
    }

    // Bot with platform tokens
    if let Some(tc) = &cases.bot.with_platform_tokens {
        clear_env();
        set_env(&tc.env);
        let cfg = BotConfig::from_env().unwrap();
        assert_eq!(cfg.platform_tokens, tc.expected_platform_tokens);
    }

    // Bot fallback single token
    if let Some(tc) = &cases.bot.fallback_single_token {
        clear_env();
        set_env(&tc.env);
        let cfg = BotConfig::from_env().unwrap();
        assert_eq!(cfg.platform_tokens, tc.expected_platform_tokens);
    }

    // Receiver valid
    clear_env();
    set_env(&cases.receiver.valid.env);
    let cfg = ReceiverConfig::from_env().unwrap();
    assert_eq!(cfg.webex_token, cases.receiver.valid.expected.webex_token);
    assert_eq!(cfg.slug, cases.receiver.valid.expected.slug);
    assert_eq!(cfg.domains, cases.receiver.valid.expected.domains);
    assert_eq!(cfg.platform, cases.receiver.valid.expected.platform);

    // Receiver platform explicit
    if let Some(tc) = &cases.receiver.platform_explicit {
        clear_env();
        set_env(&tc.env);
        let cfg = ReceiverConfig::from_env().unwrap();
        assert_eq!(cfg.platform, tc.expected_platform);
    }

    clear_env();
}

#[test]
fn test_debug_parsing() {
    let cases = load_cases();
    for val in &cases.debug_truthy_values {
        assert!(parse_debug(val), "parse_debug({}) should be true", val);
    }
    for val in &cases.debug_falsy_values {
        assert!(!parse_debug(val), "parse_debug({}) should be false", val);
    }
}
