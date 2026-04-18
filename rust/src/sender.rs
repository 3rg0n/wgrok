use std::collections::HashMap;

use reqwest::Client;
use serde_json::Value;

use crate::codec;
use crate::config::SenderConfig;
use crate::logging::{get_logger, WgrokLogger};
use crate::platform;
use crate::protocol::{format_echo, format_flags, ECHO_PREFIX, PAUSE_CMD, RESUME_CMD};

fn platform_limits() -> HashMap<&'static str, usize> {
    HashMap::from([("webex", 7439), ("slack", 4000), ("discord", 2000), ("irc", 400)])
}

#[derive(Debug, Clone)]
pub struct SendResult {
    pub message_id: Option<String>,
    pub message_ids: Vec<String>,
    pub platform_response: Value,
    pub buffered: bool,
}

fn extract_message_id(platform: &str, response: &Value) -> Option<String> {
    match platform {
        "webex" | "discord" => response.get("id").and_then(|v| v.as_str()).map(String::from),
        "slack" => response.get("ts").and_then(|v| v.as_str()).map(String::from),
        _ => None,
    }
}

pub struct WgrokSender {
    config: SenderConfig,
    client: Client,
    logger: WgrokLogger,
    paused: bool,
    buffer: Vec<(String, Option<Value>, bool)>,
}

impl WgrokSender {
    pub fn new(config: SenderConfig) -> Self {
        let logger = get_logger(config.debug, "wgrok.sender");
        Self {
            config,
            client: Client::new(),
            logger,
            paused: false,
            buffer: Vec::new(),
        }
    }

    pub async fn send(&mut self, payload: &str, card: Option<&Value>) -> Result<SendResult, String> {
        self.send_with_options(payload, card, false).await
    }

    pub async fn send_with_options(
        &mut self,
        payload: &str,
        card: Option<&Value>,
        compress: bool,
    ) -> Result<SendResult, String> {
        if self.paused {
            if self.buffer.len() >= 1000 {
                self.logger.warn("Pause buffer full (1000), dropping oldest message");
                self.buffer.remove(0);
            }
            self.buffer.push((payload.to_string(), card.cloned(), compress));
            self.logger.debug(&format!("Message buffered (paused). Buffer size: {}", self.buffer.len()));
            return Ok(SendResult {
                message_id: None,
                message_ids: Vec::new(),
                platform_response: Value::Null,
                buffered: true,
            });
        }

        let to = &self.config.slug;
        let from = &self.config.slug;
        let encrypted = self.config.encrypt_key.is_some();

        let flags = format_flags(compress, encrypted, None, None);

        let mut payload_to_send = payload.to_string();

        if compress {
            payload_to_send = codec::compress(&payload_to_send)?;
        }

        if let Some(key) = &self.config.encrypt_key {
            payload_to_send = codec::encrypt(&payload_to_send, key)?;
        }

        let text = format_echo(to, from, &flags, &payload_to_send);
        let limits = platform_limits();
        let limit = limits.get(self.config.platform.as_str()).copied().unwrap_or(7439);
        if text.len() > limit && card.is_none() {
            let overhead = ECHO_PREFIX.len() + to.len() + 1 + from.len() + 1 + flags.len() + 1;
            let max_payload = limit - overhead;
            let chunks = codec::chunk(&payload_to_send, max_payload)?;
            self.logger.info(&format!(
                "Payload exceeds {}B limit, sending {} chunks to {}",
                limit,
                chunks.len(),
                self.config.target
            ));
            let mut msg_ids: Vec<String> = Vec::new();
            let mut last_resp = Value::Null;
            for (i, ch) in chunks.iter().enumerate() {
                let chunk_flags =
                    format_flags(compress, encrypted, Some(i + 1), Some(chunks.len()));
                let chunk_text = format_echo(to, from, &chunk_flags, ch);
                let resp = platform::platform_send_message(
                    &self.config.platform,
                    &self.config.webex_token,
                    &self.config.target,
                    &chunk_text,
                    &self.client,
                )
                .await?;
                if let Some(mid) = extract_message_id(&self.config.platform, &resp) {
                    msg_ids.push(mid);
                }
                last_resp = resp;
            }
            return Ok(SendResult {
                message_id: msg_ids.first().cloned(),
                message_ids: msg_ids,
                platform_response: last_resp,
                buffered: false,
            });
        }
        self.logger
            .info(&format!("Sending to {} [slug={}, len={}]", self.config.target, self.config.slug, payload_to_send.len()));
        let resp = match card {
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
                .await?
            }
            None => {
                platform::platform_send_message(
                    &self.config.platform,
                    &self.config.webex_token,
                    &self.config.target,
                    &text,
                    &self.client,
                )
                .await?
            }
        };
        let mid = extract_message_id(&self.config.platform, &resp);
        Ok(SendResult {
            message_id: mid.clone(),
            message_ids: mid.into_iter().collect(),
            platform_response: resp,
            buffered: false,
        })
    }

    pub async fn pause(&mut self, notify: bool) -> Result<(), String> {
        if notify {
            self.logger.info("Sending pause command");
            platform::platform_send_message(
                &self.config.platform,
                &self.config.webex_token,
                &self.config.target,
                PAUSE_CMD,
                &self.client,
            )
            .await?;
        }
        self.paused = true;
        self.logger.info("Sender paused");
        Ok(())
    }

    pub async fn resume(&mut self, notify: bool) -> Result<(), String> {
        self.paused = false;

        if notify {
            self.logger.info("Sending resume command");
            platform::platform_send_message(
                &self.config.platform,
                &self.config.webex_token,
                &self.config.target,
                RESUME_CMD,
                &self.client,
            )
            .await?;
        }

        self.logger.info("Sender resumed, flushing buffer");

        // Flush buffered messages
        let buffer_size = self.buffer.len();
        let buffered = std::mem::take(&mut self.buffer);

        for (payload, card, compress) in buffered {
            self.send_with_options(&payload, card.as_ref(), compress).await?;
        }

        if buffer_size > 0 {
            self.logger.info(&format!("Flushed {} buffered messages", buffer_size));
        }

        Ok(())
    }
}
