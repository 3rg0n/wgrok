use reqwest::Client;
use serde_json::Value;

use crate::config::SenderConfig;
use crate::logging::{get_logger, WgrokLogger};
use crate::platform;
use crate::protocol::format_echo;

pub struct WgrokSender {
    config: SenderConfig,
    client: Client,
    logger: WgrokLogger,
}

impl WgrokSender {
    pub fn new(config: SenderConfig) -> Self {
        let logger = get_logger(config.debug, "wgrok.sender");
        Self {
            config,
            client: Client::new(),
            logger,
        }
    }

    pub async fn send(&self, payload: &str, card: Option<&Value>) -> Result<Value, String> {
        let text = format_echo(&self.config.slug, payload);
        self.logger.info(&format!("Sending to {}: {}", self.config.target, text));
        match card {
            Some(c) => {
                self.logger.info("Including adaptive card attachment");
                platform::platform_send_card(
                    &self.config.platform,
                    &self.config.webex_token,
                    &self.config.target,
                    &text,
                    c,
                    &self.client,
                )
                .await
            }
            None => {
                platform::platform_send_message(
                    &self.config.platform,
                    &self.config.webex_token,
                    &self.config.target,
                    &text,
                    &self.client,
                )
                .await
            }
        }
    }
}
