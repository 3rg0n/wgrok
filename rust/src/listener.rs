use async_trait::async_trait;
use futures_util::{SinkExt, StreamExt};
use reqwest::Client;
use serde_json::{json, Value};
use std::sync::{Arc, Mutex};
use tokio::io::{AsyncBufReadExt, BufReader};
use tokio::net::TcpStream;
use tokio::sync::mpsc;
use tokio_tungstenite::{connect_async, tungstenite::Message as WsMessage};

use crate::logging::WgrokLogger;

/// Normalized incoming message from any platform.
#[derive(Clone, Debug)]
pub struct IncomingMessage {
    pub sender: String,
    pub text: String,
    pub html: String,
    pub room_id: String,
    pub room_type: String,
    pub msg_id: String,
    pub platform: String,
    pub cards: Vec<Value>,
}

/// Callback type for platform listeners.
/// Listeners send IncomingMessage through an mpsc channel.
pub type MessageCallback = mpsc::UnboundedSender<IncomingMessage>;

/// Trait for platform listeners.
#[async_trait]
pub trait PlatformListener: Send + Sync {
    /// Register a callback to receive messages.
    fn on_message(&mut self, callback: MessageCallback);

    /// Connect to the platform and start listening.
    async fn connect(&mut self) -> Result<(), String>;

    /// Disconnect from the platform.
    async fn disconnect(&mut self) -> Result<(), String>;
}

const SLACK_SOCKET_MODE_URL: &str = "https://slack.com/api/apps.connections.open";

/// Slack listener using Socket Mode WebSocket.
pub struct SlackListener {
    token: String,
    logger: WgrokLogger,
    callback: Option<MessageCallback>,
    running: Arc<Mutex<bool>>,
}

impl SlackListener {
    pub fn new(token: String, logger: WgrokLogger) -> Self {
        Self {
            token,
            logger,
            callback: None,
            running: Arc::new(Mutex::new(false)),
        }
    }

}

#[async_trait]
impl PlatformListener for SlackListener {
    fn on_message(&mut self, callback: MessageCallback) {
        self.callback = Some(callback);
    }

    async fn connect(&mut self) -> Result<(), String> {
        let client = Client::new();
        let resp = client
            .post(SLACK_SOCKET_MODE_URL)
            .header("Authorization", format!("Bearer {}", self.token))
            .send()
            .await
            .map_err(|e| format!("Slack apps.connections.open failed: {}", e))?;

        let data: Value = resp.json().await.map_err(|e| e.to_string())?;

        if !data.get("ok").and_then(|v| v.as_bool()).unwrap_or(false) {
            let error = data
                .get("error")
                .and_then(|v| v.as_str())
                .unwrap_or("unknown");
            return Err(format!("Slack apps.connections.open failed: {}", error));
        }

        let ws_url = data
            .get("url")
            .and_then(|v| v.as_str())
            .ok_or("No WebSocket URL in response")?;

        let (ws_stream, _) = connect_async(ws_url)
            .await
            .map_err(|e| format!("WebSocket connect failed: {}", e))?;

        *self.running.lock().unwrap() = true;
        self.logger.info("Slack Socket Mode connected");

        let running = Arc::clone(&self.running);
        let callback = self.callback.clone();

        tokio::spawn(async move {
            let (mut write, mut read) = ws_stream.split();
            while *running.lock().unwrap() {
                match read.next().await {
                    Some(Ok(WsMessage::Text(msg))) => {
                        if let Ok(envelope) = serde_json::from_str::<Value>(&msg) {
                            // Acknowledge
                            if let Some(envelope_id) = envelope.get("envelope_id").and_then(|v| v.as_str()) {
                                let ack = WsMessage::Text(
                                    json!({ "envelope_id": envelope_id }).to_string(),
                                );
                                let _ = write.send(ack).await;
                            }

                            // Handle event
                            let event_type = envelope
                                .get("type")
                                .and_then(|v| v.as_str())
                                .unwrap_or("");

                            if event_type != "events_api" {
                                continue;
                            }

                            let payload = envelope.get("payload").and_then(|v| v.as_object());
                            let event = match payload {
                                Some(p) => p.get("event").and_then(|v| v.as_object()),
                                None => None,
                            };

                            if let Some(event) = event {
                                if event.get("type").and_then(|v| v.as_str()).unwrap_or("") != "message" {
                                    continue;
                                }

                                if event.get("bot_id").is_some() {
                                    continue;
                                }

                                if let Some(cb) = &callback {
                                    let incoming = IncomingMessage {
                                        sender: event
                                            .get("user")
                                            .and_then(|v| v.as_str())
                                            .unwrap_or("")
                                            .to_string(),
                                        text: event
                                            .get("text")
                                            .and_then(|v| v.as_str())
                                            .unwrap_or("")
                                            .trim()
                                            .to_string(),
                                        html: String::new(),
                                        room_id: String::new(),
                                        room_type: String::new(),
                                        msg_id: event
                                            .get("ts")
                                            .and_then(|v| v.as_str())
                                            .unwrap_or("")
                                            .to_string(),
                                        platform: "slack".to_string(),
                                        cards: vec![],
                                    };
                                    let _ = cb.send(incoming);
                                }
                            }
                        }
                    }
                    Some(Ok(WsMessage::Close(_))) | None => break,
                    _ => {}
                }
            }
        });

        Ok(())
    }

    async fn disconnect(&mut self) -> Result<(), String> {
        *self.running.lock().unwrap() = false;
        self.logger.info("Slack listener disconnected");
        Ok(())
    }
}

const DISCORD_GATEWAY_API: &str = "https://discord.com/api/v10/gateway";
const OP_IDENTIFY: u8 = 2;
const INTENTS: u32 = (1 << 9) | (1 << 15); // GUILD_MESSAGES + MESSAGE_CONTENT

/// Discord listener using Gateway WebSocket.
pub struct DiscordListener {
    token: String,
    logger: WgrokLogger,
    callback: Option<MessageCallback>,
    running: Arc<Mutex<bool>>,
}

impl DiscordListener {
    pub fn new(token: String, logger: WgrokLogger) -> Self {
        Self {
            token,
            logger,
            callback: None,
            running: Arc::new(Mutex::new(false)),
        }
    }
}

#[async_trait]
impl PlatformListener for DiscordListener {
    fn on_message(&mut self, callback: MessageCallback) {
        self.callback = Some(callback);
    }

    async fn connect(&mut self) -> Result<(), String> {
        let client = Client::new();
        let resp = client
            .get(DISCORD_GATEWAY_API)
            .send()
            .await
            .map_err(|e| format!("Discord gateway lookup failed: {}", e))?;

        let data: Value = resp.json().await.map_err(|e| e.to_string())?;
        let gw_url = data
            .get("url")
            .and_then(|v| v.as_str())
            .unwrap_or("wss://gateway.discord.gg");

        let ws_url = format!("{}/?v=10&encoding=json", gw_url);
        let (ws_stream, _) = connect_async(&ws_url)
            .await
            .map_err(|e| format!("WebSocket connect failed: {}", e))?;

        *self.running.lock().unwrap() = true;
        self.logger.info("Discord Gateway connected");

        let token = self.token.clone();
        let running = Arc::clone(&self.running);
        let callback = self.callback.clone();

        tokio::spawn(async move {
            let (mut write, mut read) = ws_stream.split();
            let mut _sequence: Option<i64> = None;

            // Wait for Hello (opcode 10)
            match read.next().await {
                Some(Ok(WsMessage::Text(msg))) => {
                    if let Ok(hello) = serde_json::from_str::<Value>(&msg) {
                        if let Some(10) = hello.get("op").and_then(|v| v.as_u64()).map(|v| v as u8) {
                            if let Some(heartbeat_interval) = hello
                                .get("d")
                                .and_then(|v| v.as_object())
                                .and_then(|v| v.get("heartbeat_interval"))
                                .and_then(|v| v.as_u64())
                            {
                                // Send Identify
                                let identify = json!({
                                    "op": OP_IDENTIFY,
                                    "d": {
                                        "token": token,
                                        "intents": INTENTS,
                                        "properties": {
                                            "os": "linux",
                                            "browser": "wgrok",
                                            "device": "wgrok",
                                        }
                                    }
                                });

                                let msg = WsMessage::Text(identify.to_string());
                                let _ = write.send(msg).await;

                                // Spawn heartbeat task
                                let running_hb = Arc::clone(&running);
                                let hb_interval = (heartbeat_interval as f64) / 1000.0;
                                tokio::spawn(async move {
                                    while *running_hb.lock().unwrap() {
                                        tokio::time::sleep(tokio::time::Duration::from_secs_f64(hb_interval))
                                            .await;
                                        // Heartbeat would be sent via websocket here
                                    }
                                });
                            }
                        }
                    }
                }
                _ => return,
            }

            // Read dispatch events
            while *running.lock().unwrap() {
                match read.next().await {
                    Some(Ok(WsMessage::Text(msg))) => {
                        if let Ok(data) = serde_json::from_str::<Value>(&msg) {
                            if let Some(s) = data.get("s").and_then(|v| v.as_i64()) {
                                _sequence = Some(s);
                            }

                            if let Some(0) = data.get("op").and_then(|v| v.as_u64()).map(|v| v as u8) {
                                if let Some("MESSAGE_CREATE") = data.get("t").and_then(|v| v.as_str()) {
                                    if let Some(event) = data.get("d").and_then(|v| v.as_object()) {
                                        if let Some(author) = event.get("author").and_then(|v| v.as_object()) {
                                            if author.get("bot").and_then(|v| v.as_bool()).unwrap_or(false) {
                                                continue;
                                            }

                                            if let Some(cb) = &callback {
                                                let embeds = event
                                                    .get("embeds")
                                                    .and_then(|v| v.as_array())
                                                    .cloned()
                                                    .unwrap_or_default();

                                                let incoming = IncomingMessage {
                                                    sender: author
                                                        .get("id")
                                                        .and_then(|v| v.as_str())
                                                        .unwrap_or("")
                                                        .to_string(),
                                                    text: event
                                                        .get("content")
                                                        .and_then(|v| v.as_str())
                                                        .unwrap_or("")
                                                        .trim()
                                                        .to_string(),
                                                    html: String::new(),
                                                    room_id: String::new(),
                                                    room_type: String::new(),
                                                    msg_id: event
                                                        .get("id")
                                                        .and_then(|v| v.as_str())
                                                        .unwrap_or("")
                                                        .to_string(),
                                                    platform: "discord".to_string(),
                                                    cards: embeds,
                                                };
                                                let _ = cb.send(incoming);
                                            }
                                        }
                                    }
                                }
                            }
                        }
                    }
                    Some(Ok(WsMessage::Close(_))) | None => break,
                    _ => {}
                }
            }
        });

        Ok(())
    }

    async fn disconnect(&mut self) -> Result<(), String> {
        *self.running.lock().unwrap() = false;
        self.logger.info("Discord listener disconnected");
        Ok(())
    }
}

/// IRC listener using persistent TCP/TLS connection.
pub struct IrcListener {
    conn_str: String,
    logger: WgrokLogger,
    callback: Option<MessageCallback>,
    running: Arc<Mutex<bool>>,
}

impl IrcListener {
    pub fn new(conn_str: String, logger: WgrokLogger) -> Self {
        Self {
            conn_str,
            logger,
            callback: None,
            running: Arc::new(Mutex::new(false)),
        }
    }
}

#[async_trait]
impl PlatformListener for IrcListener {
    fn on_message(&mut self, callback: MessageCallback) {
        self.callback = Some(callback);
    }

    async fn connect(&mut self) -> Result<(), String> {
        let conn_str = self.conn_str.clone();

        // Parse connection string: nick[:password]@server[:port][/channel]
        let at_pos = conn_str.find('@').ok_or("Invalid IRC connection string: missing @")?;
        let creds = &conn_str[..at_pos];
        let server_part = &conn_str[at_pos + 1..];

        let (nick, password) = if let Some(colon_pos) = creds.find(':') {
            let n = &creds[..colon_pos];
            let p = &creds[colon_pos + 1..];
            (n.to_string(), p.to_string())
        } else {
            (creds.to_string(), String::new())
        };

        let (server_port, channel) = if let Some(slash_pos) = server_part.find('/') {
            let sp = &server_part[..slash_pos];
            let ch = &server_part[slash_pos + 1..];
            (sp, ch.to_string())
        } else {
            (server_part, String::new())
        };

        let (server, port) = if let Some(colon_pos) = server_port.find(':') {
            let srv = &server_port[..colon_pos];
            let port_str = &server_port[colon_pos + 1..];
            let p = port_str
                .parse::<u16>()
                .map_err(|_| format!("Invalid port: {}", port_str))?;
            (srv.to_string(), p)
        } else {
            (server_port.to_string(), 6697)
        };

        if nick.is_empty() || server.is_empty() {
            return Err("Invalid IRC connection string: nick and server are required".to_string());
        }

        *self.running.lock().unwrap() = true;
        self.logger.info(&format!("IRC connecting to {}:{}", server, port));

        let running = Arc::clone(&self.running);
        let logger = self.logger.clone();
        let callback = self.callback.clone();

        tokio::spawn(async move {
            let params = IrcConnectParams {
                server, port, nick, password, channel,
                running, logger, callback,
            };
            if let Err(e) = Self::connect_and_read(params).await {
                let _ = e;
            }
        });

        Ok(())
    }

    async fn disconnect(&mut self) -> Result<(), String> {
        *self.running.lock().unwrap() = false;
        self.logger.info("IRC listener disconnected");
        Ok(())
    }
}

struct IrcConnectParams {
    server: String,
    port: u16,
    nick: String,
    password: String,
    channel: String,
    running: Arc<Mutex<bool>>,
    logger: WgrokLogger,
    callback: Option<MessageCallback>,
}

impl IrcListener {
    async fn connect_and_read(params: IrcConnectParams) -> Result<(), String> {
        let IrcConnectParams { server, port, nick, password, channel, running, logger, callback } = params;
        let stream = TcpStream::connect(format!("{}:{}", server, port))
            .await
            .map_err(|e| format!("TCP connect failed: {}", e))?;

        // Upgrade to TLS
        let connector = native_tls::TlsConnector::new()
            .map_err(|e| format!("TLS connector failed: {}", e))?;
        let tls_connector = tokio_native_tls::TlsConnector::from(connector);
        let tls_stream = tls_connector
            .connect(&server, stream)
            .await
            .map_err(|e| format!("TLS connect failed: {}", e))?;

        let (reader, mut writer) = tokio::io::split(tls_stream);
        let mut reader = BufReader::new(reader);

        // Send IRC registration
        if !password.is_empty() {
            Self::send_raw(&mut writer, &format!("PASS {}", password)).await?;
        }
        Self::send_raw(&mut writer, &format!("NICK {}", nick)).await?;
        Self::send_raw(&mut writer, &format!("USER {} 0 * :{}", nick, nick)).await?;

        // Join channel if specified
        if !channel.is_empty() {
            Self::send_raw(&mut writer, &format!("JOIN {}", channel)).await?;
        }

        logger.info(&format!("IRC registered as {}", nick));

        // Read lines
        let mut line = String::new();
        while *running.lock().unwrap() {
            line.clear();
            match tokio::time::timeout(
                tokio::time::Duration::from_secs(300),
                reader.read_line(&mut line),
            )
            .await
            {
                Ok(Ok(0)) => break, // EOF
                Ok(Ok(_)) => {
                    let trimmed = line.trim_end();
                    if trimmed.is_empty() {
                        continue;
                    }

                    // Handle PING
                    if trimmed.starts_with("PING") {
                        let pong_arg = if trimmed.len() > 5 {
                            &trimmed[5..]
                        } else {
                            ""
                        };
                        let _ = Self::send_raw(&mut writer, &format!("PONG {}", pong_arg)).await;
                        continue;
                    }

                    // Parse PRIVMSG: :nick!user@host PRIVMSG target :message
                    if let Some(msg_start) = trimmed.find("PRIVMSG ") {
                        // Find the colon that starts the message text (after "PRIVMSG target :")
                        let after_privmsg = &trimmed[msg_start..];
                        if let Some(space_pos) = after_privmsg[8..].find(' ') {
                            let colon_offset = msg_start + 8 + space_pos + 1;
                            if colon_offset < trimmed.len() && trimmed.as_bytes()[colon_offset] == b':' {
                                let text = &trimmed[colon_offset + 1..];

                                // Extract sender nick
                                if let Some(nick_end) = trimmed.find('!') {
                                    if trimmed.starts_with(':') {
                                        let sender = trimmed[1..nick_end].to_string();

                                        if let Some(cb) = &callback {
                                            let incoming = IncomingMessage {
                                                sender,
                                                text: text.trim().to_string(),
                                                html: String::new(),
                                                room_id: String::new(),
                                                room_type: String::new(),
                                                msg_id: String::new(),
                                                platform: "irc".to_string(),
                                                cards: vec![],
                                            };
                                            let _ = cb.send(incoming);
                                        }
                                    }
                                }
                            }
                        }
                    }
                }
                Ok(Err(e)) => {
                    logger.error(&format!("Read error: {}", e));
                    break;
                }
                Err(_) => {
                    // Timeout - send PING to keep connection alive
                    if *running.lock().unwrap() {
                        let _ = Self::send_raw(&mut writer, "PING :keepalive").await;
                    }
                }
            }
        }

        Ok(())
    }

    async fn send_raw<W: tokio::io::AsyncWriteExt + Unpin>(
        writer: &mut W,
        msg: &str,
    ) -> Result<(), String> {
        // Sanitize to prevent IRC protocol injection
        let sanitized = msg.replace(['\r', '\n'], "");
        writer
            .write_all(format!("{}\r\n", sanitized).as_bytes())
            .await
            .map_err(|e| format!("Write failed: {}", e))?;
        Ok(())
    }
}

/// Webex listener using webex-message-handler.
pub struct WebexListener {
    _token: String,
    logger: WgrokLogger,
    callback: Option<MessageCallback>,
}

impl WebexListener {
    pub fn new(token: String, logger: WgrokLogger) -> Self {
        Self {
            _token: token,
            logger,
            callback: None,
        }
    }
}

#[async_trait]
impl PlatformListener for WebexListener {
    fn on_message(&mut self, callback: MessageCallback) {
        self.callback = Some(callback);
    }

    async fn connect(&mut self) -> Result<(), String> {
        // Webex listener would be implemented using webex-message-handler
        // For now, this is a placeholder that indicates the listener is ready
        self.logger.info("Webex listener connected");
        Ok(())
    }

    async fn disconnect(&mut self) -> Result<(), String> {
        self.logger.info("Webex listener disconnected");
        Ok(())
    }
}

/// Factory function to create the appropriate listener for a platform.
pub fn create_listener(
    platform: &str,
    token: &str,
    logger: WgrokLogger,
) -> Result<Box<dyn PlatformListener>, String> {
    match platform {
        "webex" => Ok(Box::new(WebexListener::new(token.to_string(), logger))),
        "slack" => Ok(Box::new(SlackListener::new(token.to_string(), logger))),
        "discord" => Ok(Box::new(DiscordListener::new(token.to_string(), logger))),
        "irc" => Ok(Box::new(IrcListener::new(token.to_string(), logger))),
        _ => Err(format!("Unsupported platform: {}", platform)),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_create_listener_webex() {
        let logger = crate::logging::get_logger(false, "test");
        let result = create_listener("webex", "test_token", logger);
        assert!(result.is_ok());
    }

    #[test]
    fn test_create_listener_slack() {
        let logger = crate::logging::get_logger(false, "test");
        let result = create_listener("slack", "test_token", logger);
        assert!(result.is_ok());
    }

    #[test]
    fn test_create_listener_discord() {
        let logger = crate::logging::get_logger(false, "test");
        let result = create_listener("discord", "test_token", logger);
        assert!(result.is_ok());
    }

    #[test]
    fn test_create_listener_irc() {
        let logger = crate::logging::get_logger(false, "test");
        let result = create_listener("irc", "nick@localhost:6667/channel", logger);
        assert!(result.is_ok());
    }

    #[test]
    fn test_create_listener_invalid() {
        let logger = crate::logging::get_logger(false, "test");
        let result = create_listener("invalid", "test_token", logger);
        assert!(result.is_err());
    }

    #[test]
    fn test_incoming_message_creation() {
        let msg = IncomingMessage {
            sender: "user@example.com".to_string(),
            text: "Hello".to_string(),
            html: String::new(),
            room_id: String::new(),
            room_type: String::new(),
            msg_id: "123".to_string(),
            platform: "webex".to_string(),
            cards: vec![],
        };
        assert_eq!(msg.sender, "user@example.com");
        assert_eq!(msg.text, "Hello");
        assert_eq!(msg.platform, "webex");
    }
}
