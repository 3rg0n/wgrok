use std::collections::HashMap;
use reqwest::Client;
use serde_json::Value;
use tokio::sync::watch;
use webex_message_handler::{Config, DecryptedMessage, HandlerEvent, WebexMessageHandler};

use crate::allowlist::Allowlist;
use crate::config::BotConfig;
use crate::logging::{get_logger, WgrokLogger};
use crate::platform;
use crate::protocol::{format_response, is_echo, parse_echo};
use crate::webex;

pub struct WgrokRouterBot {
    config: BotConfig,
    allowlist: Allowlist,
    logger: WgrokLogger,
    client: Client,
    routes: HashMap<String, String>,
}

impl WgrokRouterBot {
    pub fn new(config: BotConfig) -> Self {
        let logger = get_logger(config.debug, "wgrok.router_bot");
        let allowlist = Allowlist::new(&config.domains);
        let routes = config.routes.clone();
        Self {
            config,
            allowlist,
            logger,
            client: Client::new(),
            routes,
        }
    }

    fn resolve_target(&self, slug: &str, sender: &str) -> String {
        if let Some(target) = self.routes.get(slug) {
            target.clone()
        } else {
            sender.to_string()
        }
    }

    fn get_send_platform_token(&self) -> (String, String) {
        // Prefer webex platform
        if let Some(tokens) = self.config.platform_tokens.get("webex") {
            if let Some(token) = tokens.first() {
                return ("webex".to_string(), token.clone());
            }
        }
        // Fall back to first available platform
        for (platform, tokens) in &self.config.platform_tokens {
            if let Some(token) = tokens.first() {
                return (platform.clone(), token.clone());
            }
        }
        // Last resort: use webex_token with webex platform
        ("webex".to_string(), self.config.webex_token.clone())
    }

    pub async fn run(&self, mut shutdown_rx: watch::Receiver<bool>) -> Result<(), String> {
        let handler = WebexMessageHandler::new(Config {
            token: self.config.webex_token.clone(),
            ..Default::default()
        })
        .map_err(|e| format!("create handler: {}", e))?;

        let mut rx = handler
            .take_event_rx()
            .await
            .ok_or("failed to take event rx")?;

        handler
            .connect()
            .await
            .map_err(|e| format!("connect: {}", e))?;

        self.logger.info("Router bot connected");

        loop {
            tokio::select! {
                event = rx.recv() => {
                    match event {
                        Some(HandlerEvent::MessageCreated(msg)) => {
                            // Only fetch cards from Webex REST API (this handler is Webex-only)
                            let cards = self.fetch_cards(&msg.id).await;
                            self.on_message_with_cards(&msg, &cards).await;
                        }
                        None => break,
                        _ => {}
                    }
                }
                _ = shutdown_rx.changed() => {
                    break;
                }
            }
        }

        handler.disconnect().await;
        self.logger.info("Router bot stopped");
        Ok(())
    }

    pub async fn on_message_with_cards(&self, msg: &DecryptedMessage, cards: &[Value]) {
        let sender = &msg.person_email;
        let text = msg.text.trim();

        if !self.allowlist.is_allowed(sender) {
            self.logger
                .warn(&format!("Rejected message from {}: not in allowlist", sender));
            return;
        }

        if !is_echo(text) {
            self.logger
                .debug(&format!("Ignoring non-echo message from {}", sender));
            return;
        }

        let (to, from_slug, flags, payload) = match parse_echo(text) {
            Ok(v) => v,
            Err(e) => {
                self.logger
                    .error(&format!("Failed to parse echo message: {}", e));
                return;
            }
        };

        let target = self.resolve_target(&to, sender);
        let response = format_response(&to, &from_slug, &flags, &payload);
        let (platform, token) = self.get_send_platform_token();

        let result = if !cards.is_empty() {
            self.logger.info(&format!(
                "Relaying to {}: {} (with {} card(s))",
                target,
                response,
                cards.len()
            ));
            platform::platform_send_card(
                &platform,
                &token,
                &target,
                &response,
                &cards[0],
                &self.client,
            )
            .await
        } else {
            self.logger
                .info(&format!("Relaying to {}: {}", target, response));
            platform::platform_send_message(&platform, &token, &target, &response, &self.client).await
        };

        if let Err(e) = result {
            self.logger
                .error(&format!("Failed to relay message: {}", e));
        }
    }

    async fn fetch_cards(&self, message_id: &str) -> Vec<Value> {
        if message_id.is_empty() {
            return vec![];
        }
        match webex::get_message(&self.config.webex_token, message_id, &self.client).await {
            Ok(msg) => webex::extract_cards(&msg),
            Err(e) => {
                self.logger
                    .debug(&format!("Could not fetch message attachments: {}", e));
                vec![]
            }
        }
    }
}
