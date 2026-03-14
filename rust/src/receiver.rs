use reqwest::Client;
use serde_json::Value;
use tokio::sync::watch;
use webex_message_handler::{Config, DecryptedMessage, HandlerEvent, WebexMessageHandler};

use crate::allowlist::Allowlist;
use crate::config::ReceiverConfig;
use crate::logging::{get_logger, WgrokLogger};
use crate::protocol::parse_response;
use crate::webex;

pub type MessageHandler = Box<dyn Fn(&str, &str, &[Value]) + Send + Sync>;

pub struct WgrokReceiver {
    config: ReceiverConfig,
    allowlist: Allowlist,
    handler: MessageHandler,
    logger: WgrokLogger,
    client: Client,
}

impl WgrokReceiver {
    pub fn new(config: ReceiverConfig, handler: MessageHandler) -> Self {
        let logger = get_logger(config.debug, "wgrok.receiver");
        let allowlist = Allowlist::new(&config.domains);
        Self {
            config,
            allowlist,
            handler,
            logger,
            client: Client::new(),
        }
    }

    pub async fn listen(&self, mut shutdown_rx: watch::Receiver<bool>) -> Result<(), String> {
        let ws_handler = WebexMessageHandler::new(Config {
            token: self.config.webex_token.clone(),
            ..Default::default()
        })
        .map_err(|e| format!("create handler: {}", e))?;

        let mut rx = ws_handler
            .take_event_rx()
            .await
            .ok_or("failed to take event rx")?;

        ws_handler
            .connect()
            .await
            .map_err(|e| format!("connect: {}", e))?;

        self.logger
            .info(&format!("Receiver listening for slug: {}", self.config.slug));

        loop {
            tokio::select! {
                event = rx.recv() => {
                    match event {
                        Some(HandlerEvent::MessageCreated(msg)) => {
                            let cards = self.fetch_cards(&msg.id).await;
                            self.on_message_with_cards(&msg, &cards);
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

        ws_handler.disconnect().await;
        self.logger.info("Receiver stopped");
        Ok(())
    }

    pub async fn fetch_action(&self, action_id: &str) -> Result<Value, String> {
        webex::get_attachment_action(&self.config.webex_token, action_id, &self.client).await
    }

    pub fn on_message_with_cards(&self, msg: &DecryptedMessage, cards: &[Value]) {
        let sender = &msg.person_email;
        let text = msg.text.trim();

        if !self.allowlist.is_allowed(sender) {
            self.logger
                .warn(&format!("Rejected message from {}: not in allowlist", sender));
            return;
        }

        let (slug, payload) = match parse_response(text) {
            Ok(v) => v,
            Err(_) => {
                self.logger
                    .debug(&format!("Ignoring unparseable message from {}", sender));
                return;
            }
        };

        if slug != self.config.slug {
            self.logger.debug(&format!(
                "Ignoring message with slug \"{}\" (expected \"{}\")",
                slug, self.config.slug
            ));
            return;
        }

        if !cards.is_empty() {
            self.logger.info(&format!(
                "Received payload for slug \"{}\" from {} (with {} card(s))",
                slug,
                sender,
                cards.len()
            ));
        } else {
            self.logger.info(&format!(
                "Received payload for slug \"{}\" from {}",
                slug, sender
            ));
        }
        (self.handler)(&slug, &payload, cards);
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
