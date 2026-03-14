use std::env;

#[derive(Debug, Clone)]
pub struct SenderConfig {
    pub webex_token: String,
    pub target: String,
    pub slug: String,
    pub domains: Vec<String>,
    pub debug: bool,
}

#[derive(Debug, Clone)]
pub struct BotConfig {
    pub webex_token: String,
    pub domains: Vec<String>,
    pub debug: bool,
}

#[derive(Debug, Clone)]
pub struct ReceiverConfig {
    pub webex_token: String,
    pub slug: String,
    pub domains: Vec<String>,
    pub debug: bool,
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

impl SenderConfig {
    pub fn from_env() -> Result<Self, String> {
        Ok(Self {
            webex_token: env_require("WGROK_TOKEN")?,
            target: env_require("WGROK_TARGET")?,
            slug: env_require("WGROK_SLUG")?,
            domains: parse_domains(&env::var("WGROK_DOMAINS").unwrap_or_default()),
            debug: parse_debug(&env::var("WGROK_DEBUG").unwrap_or_default()),
        })
    }
}

impl BotConfig {
    pub fn from_env() -> Result<Self, String> {
        Ok(Self {
            webex_token: env_require("WGROK_TOKEN")?,
            domains: parse_domains(&env_require("WGROK_DOMAINS")?),
            debug: parse_debug(&env::var("WGROK_DEBUG").unwrap_or_default()),
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
        })
    }
}
