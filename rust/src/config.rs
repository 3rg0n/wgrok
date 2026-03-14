use std::collections::HashMap;
use std::env;

#[derive(Debug, Clone)]
pub struct SenderConfig {
    pub webex_token: String,
    pub target: String,
    pub slug: String,
    pub domains: Vec<String>,
    pub debug: bool,
    pub platform: String,
}

#[derive(Debug, Clone)]
pub struct BotConfig {
    pub webex_token: String,
    pub domains: Vec<String>,
    pub debug: bool,
    pub routes: HashMap<String, String>,
    pub platform_tokens: HashMap<String, Vec<String>>,
    pub webhook_port: Option<u16>,
    pub webhook_secret: Option<String>,
}

#[derive(Debug, Clone)]
pub struct ReceiverConfig {
    pub webex_token: String,
    pub slug: String,
    pub domains: Vec<String>,
    pub debug: bool,
    pub platform: String,
}

fn env_require(name: &str) -> Result<String, String> {
    env::var(name).map_err(|_| format!("Required environment variable {} is not set", name))
}

fn parse_domains(raw: &str) -> Vec<String> {
    raw.split(',')
        .map(|d| d.trim().to_string())
        .filter(|d| !d.is_empty())
        .collect()
}

pub fn parse_debug(raw: &str) -> bool {
    let v = raw.trim().to_lowercase();
    v == "true" || v == "1" || v == "yes"
}

fn parse_routes(raw: &str) -> HashMap<String, String> {
    let mut routes = HashMap::new();
    if raw.is_empty() {
        return routes;
    }
    for pair in raw.split(',') {
        let parts: Vec<&str> = pair.trim().split(':').collect();
        if parts.len() == 2 {
            let slug = parts[0].trim().to_string();
            let target = parts[1].trim().to_string();
            if !slug.is_empty() && !target.is_empty() {
                routes.insert(slug, target);
            }
        }
    }
    routes
}

fn parse_platform_tokens() -> HashMap<String, Vec<String>> {
    let mut platform_tokens = HashMap::new();

    if let Ok(webex_tokens) = env::var("WGROK_WEBEX_TOKENS") {
        if !webex_tokens.is_empty() {
            let tokens: Vec<String> = webex_tokens
                .split(',')
                .map(|t| t.trim().to_string())
                .filter(|t| !t.is_empty())
                .collect();
            if !tokens.is_empty() {
                platform_tokens.insert("webex".to_string(), tokens);
            }
        }
    }

    if let Ok(slack_tokens) = env::var("WGROK_SLACK_TOKENS") {
        if !slack_tokens.is_empty() {
            let tokens: Vec<String> = slack_tokens
                .split(',')
                .map(|t| t.trim().to_string())
                .filter(|t| !t.is_empty())
                .collect();
            if !tokens.is_empty() {
                platform_tokens.insert("slack".to_string(), tokens);
            }
        }
    }

    if let Ok(discord_tokens) = env::var("WGROK_DISCORD_TOKENS") {
        if !discord_tokens.is_empty() {
            let tokens: Vec<String> = discord_tokens
                .split(',')
                .map(|t| t.trim().to_string())
                .filter(|t| !t.is_empty())
                .collect();
            if !tokens.is_empty() {
                platform_tokens.insert("discord".to_string(), tokens);
            }
        }
    }

    if let Ok(irc_tokens) = env::var("WGROK_IRC_TOKENS") {
        if !irc_tokens.is_empty() {
            let tokens: Vec<String> = irc_tokens
                .split(',')
                .map(|t| t.trim().to_string())
                .filter(|t| !t.is_empty())
                .collect();
            if !tokens.is_empty() {
                platform_tokens.insert("irc".to_string(), tokens);
            }
        }
    }

    // Fallback: if no platform tokens set, use WGROK_TOKEN as webex token
    if platform_tokens.is_empty() {
        if let Ok(token) = env::var("WGROK_TOKEN") {
            if !token.is_empty() {
                platform_tokens.insert("webex".to_string(), vec![token]);
            }
        }
    }

    platform_tokens
}

impl SenderConfig {
    pub fn from_env() -> Result<Self, String> {
        Ok(Self {
            webex_token: env_require("WGROK_TOKEN")?,
            target: env_require("WGROK_TARGET")?,
            slug: env_require("WGROK_SLUG")?,
            domains: parse_domains(&env::var("WGROK_DOMAINS").unwrap_or_default()),
            debug: parse_debug(&env::var("WGROK_DEBUG").unwrap_or_default()),
            platform: env::var("WGROK_PLATFORM").unwrap_or_else(|_| "webex".to_string()),
        })
    }
}

impl BotConfig {
    pub fn from_env() -> Result<Self, String> {
        let platform_tokens = parse_platform_tokens();

        // Get webex_token for backwards compatibility
        let webex_token = if let Some(tokens) = platform_tokens.get("webex") {
            tokens.first().cloned().ok_or("No webex tokens available")?
        } else {
            env_require("WGROK_TOKEN")?
        };

        Ok(Self {
            webex_token,
            domains: parse_domains(&env_require("WGROK_DOMAINS")?),
            debug: parse_debug(&env::var("WGROK_DEBUG").unwrap_or_default()),
            routes: parse_routes(&env::var("WGROK_ROUTES").unwrap_or_default()),
            platform_tokens,
            webhook_port: env::var("WGROK_WEBHOOK_PORT")
                .ok()
                .and_then(|p| p.parse::<u16>().ok()),
            webhook_secret: env::var("WGROK_WEBHOOK_SECRET").ok(),
        })
    }
}

impl ReceiverConfig {
    pub fn from_env() -> Result<Self, String> {
        Ok(Self {
            webex_token: env_require("WGROK_TOKEN")?,
            slug: env_require("WGROK_SLUG")?,
            domains: parse_domains(&env_require("WGROK_DOMAINS")?),
            debug: parse_debug(&env::var("WGROK_DEBUG").unwrap_or_default()),
            platform: env::var("WGROK_PLATFORM").unwrap_or_else(|_| "webex".to_string()),
        })
    }
}
