use std::collections::HashMap;
use std::sync::Mutex;
use std::time::Instant;

use reqwest::Client;
use serde_json::Value;
use tokio::sync::watch;
use webex_message_handler::{Config, DecryptedMessage, HandlerEvent, WebexMessageHandler};

use crate::allowlist::Allowlist;
use crate::codec;
use crate::config::ReceiverConfig;
use crate::logging::{get_logger, WgrokLogger};
use crate::protocol::{parse_flags, parse_response, is_pause, is_resume, strip_bot_mention};
use crate::webex;

#[derive(Debug, Clone)]
pub struct MessageContext {
    pub msg_id: String,
    pub sender: String,
    pub platform: String,
    pub room_id: String,
    pub room_type: String,
}

pub type MessageHandler = Box<dyn Fn(&str, &str, &[Value], &str, &MessageContext) + Send + Sync>;
pub type ControlHandler = Box<dyn Fn(&str) + Send + Sync>;

type ChunkKey = (String, String); // (sender, slug)

pub struct WgrokReceiver {
    config: ReceiverConfig,
    allowlist: Allowlist,
    handler: MessageHandler,
    on_control: Option<ControlHandler>,
    logger: WgrokLogger,
    client: Client,
    chunk_buffer: Mutex<HashMap<ChunkKey, HashMap<usize, String>>>,
    chunk_timestamps: Mutex<HashMap<ChunkKey, Instant>>,
}

impl WgrokReceiver {
    pub fn new(config: ReceiverConfig, handler: MessageHandler) -> Self {
        let logger = get_logger(config.debug, "wgrok.receiver");
        let allowlist = Allowlist::new(&config.domains);
        Self {
            config,
            allowlist,
            handler,
            on_control: None,
            logger,
            client: Client::new(),
            chunk_buffer: Mutex::new(HashMap::new()),
            chunk_timestamps: Mutex::new(HashMap::new()),
        }
    }

    pub fn set_control_handler(&mut self, on_control: ControlHandler) {
        self.on_control = Some(on_control);
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
        let html = msg.html.as_deref().unwrap_or("");
        let text = strip_bot_mention(msg.text.trim(), html);

        if !self.allowlist.is_allowed(sender) {
            self.logger
                .warn(&format!("Rejected message from {}: not in allowlist", sender));
            return;
        }

        // Check for control commands before parsing as response
        if is_pause(&text) {
            self.logger.info(&format!("Received pause command from {}", sender));
            if let Some(ref handler) = self.on_control {
                handler("pause");
            }
            return;
        }

        if is_resume(&text) {
            self.logger.info(&format!("Received resume command from {}", sender));
            if let Some(ref handler) = self.on_control {
                handler("resume");
            }
            return;
        }

        let (to, from_slug, flags, mut payload) = match parse_response(&text) {
            Ok(v) => v,
            Err(_) => {
                self.logger
                    .debug(&format!("Ignoring unparseable message from {}", sender));
                return;
            }
        };

        if to != self.config.slug {
            self.logger.debug(&format!(
                "Ignoring message with slug \"{}\" (expected \"{}\")",
                to, self.config.slug
            ));
            return;
        }

        let (compressed, encrypted, chunk_seq, chunk_total) = parse_flags(&flags);

        // Handle chunking
        if let (Some(seq), Some(total)) = (chunk_seq, chunk_total) {
            if total > 100 || seq > total || seq < 1 {
                self.logger.warn(&format!("Invalid chunk {}/{} from {}", seq, total, sender));
                return;
            }
            let key = (sender.clone(), to.clone());

            // Check for chunk timeout (5 minutes)
            {
                let mut timestamps = self.chunk_timestamps.lock().unwrap();
                if let Some(ts) = timestamps.get(&key) {
                    if ts.elapsed() > std::time::Duration::from_secs(300) {
                        self.logger.warn(&format!(
                            "Discarding incomplete chunk set for {:?} (timeout after 5 minutes)", key
                        ));
                        let mut buffer = self.chunk_buffer.lock().unwrap();
                        buffer.remove(&key);
                        timestamps.remove(&key);
                        return;
                    }
                } else {
                    timestamps.insert(key.clone(), Instant::now());
                }
            }

            let mut buffer = self.chunk_buffer.lock().unwrap();
            buffer.entry(key.clone()).or_default().insert(seq, payload);
            if buffer[&key].len() < total {
                self.logger.debug(&format!(
                    "Buffered chunk {}/{} for slug \"{}\" from {}",
                    seq, total, to, sender
                ));
                return;
            }

            // Verify all indices 1..total are present before reassembly
            let all_present = (1..=total).all(|i| buffer[&key].contains_key(&i));
            if !all_present {
                self.logger.warn(&format!("Incomplete chunk set for {:?}: missing indices, discarding", key));
                buffer.remove(&key);
                drop(buffer);
                self.chunk_timestamps.lock().unwrap().remove(&key);
                return;
            }

            // All chunks received and verified — reassemble
            let chunks = buffer.remove(&key).unwrap();
            drop(buffer);
            self.chunk_timestamps.lock().unwrap().remove(&key);
            let mut assembled = String::new();
            for i in 1..=total {
                if let Some(part) = chunks.get(&i) {
                    assembled.push_str(part);
                }
            }
            self.logger.debug(&format!(
                "Reassembled {} chunks for slug \"{}\" from {}",
                total, to, sender
            ));
            payload = assembled;
        }

        // Decrypt if needed
        if encrypted {
            if let Some(key) = &self.config.encrypt_key {
                match codec::decrypt(&payload, key) {
                    Ok(plaintext) => payload = plaintext,
                    Err(e) => {
                        self.logger.warn(&format!("Decryption failed: {}", e));
                        return;
                    }
                }
            } else {
                self.logger.warn("Message is encrypted but WGROK_ENCRYPT_KEY not set, skipping");
                return;
            }
        }

        // Decompress if needed
        if compressed {
            match codec::decompress(&payload) {
                Ok(decompressed) => payload = decompressed,
                Err(e) => {
                    self.logger.warn(&format!("Decompression failed: {}", e));
                    return;
                }
            }
        }

        if !cards.is_empty() {
            self.logger.info(&format!(
                "Received payload for slug \"{}\" from {} (with {} card(s))",
                to,
                sender,
                cards.len()
            ));
        } else {
            self.logger.info(&format!(
                "Received payload for slug \"{}\" from {}",
                to, sender
            ));
        }
        let ctx = MessageContext {
            msg_id: msg.id.clone(),
            sender: sender.to_string(),
            platform: "webex".to_string(),
            room_id: msg.room_id.clone(),
            room_type: msg.room_type.clone().unwrap_or_default(),
        };
        (self.handler)(&to, &payload, cards, &from_slug, &ctx);
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
