use reqwest::Client;
use serde_json::Value;
use tokio::sync::watch;
use webex_message_handler::{Config, DecryptedMessage, HandlerEvent, WebexMessageHandler};

use crate::allowlist::Allowlist;
use crate::config::BotConfig;
use crate::logging::{get_logger, WgrokLogger};
use crate::protocol::{format_response, is_echo, parse_echo};
use crate::webex;

pub struct WgrokEchoBot {
    config: BotConfig,
    allowlist: Allowlist,
    logger: WgrokLogger,
    client: Client,
}

impl WgrokEchoBot {
    pub fn new(config: BotConfig) -> Self {
        let logger = get_logger(config.debug, "wgrok.echo_bot");
        let allowlist = Allowlist::new(&config.domains);
        Self {
            config,
            allowlist,
            logger,
            client: Client::new(),
        }
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

        self.logger.info("Echo bot connected");

        loop {
            tokio::select! {
                event = rx.recv() => {
                    match event {
                        Some(HandlerEvent::MessageCreated(msg)) => {
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
        self.logger.info("Echo bot stopped");
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

        let (slug, payload) = match parse_echo(text) {
            Ok(v) => v,
            Err(e) => {
                self.logger
                    .error(&format!("Failed to parse echo message: {}", e));
                return;
            }
        };

        let response = format_response(&slug, &payload);

        let result = if !cards.is_empty() {
            self.logger.info(&format!(
                "Relaying to {}: {} (with {} card(s))",
                sender,
                response,
                cards.len()
            ));
            webex::send_card(
                &self.config.webex_token,
                sender,
                &response,
                &cards[0],
                &self.client,
            )
            .await
        } else {
            self.logger
                .info(&format!("Relaying to {}: {}", sender, response));
            webex::send_message(&self.config.webex_token, sender, &response, &self.client).await
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
