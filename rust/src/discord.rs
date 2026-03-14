use reqwest::Client;
use serde::Serialize;
use serde_json::Value;
use std::sync::RwLock;
use std::time::Duration;
use tokio::time::sleep;

pub const DISCORD_API_BASE: &str = "https://discord.com/api/v10";
const MAX_RETRIES: u32 = 3;

static DISCORD_API_BASE_OVERRIDE: RwLock<Option<String>> = RwLock::new(None);

fn api_base() -> String {
    if let Ok(guard) = DISCORD_API_BASE_OVERRIDE.read() {
        if let Some(base) = guard.as_ref() {
            return base.clone();
        }
    }
    DISCORD_API_BASE.to_string()
}

/// Override Discord API base URL for testing. Pass None to reset.
pub fn _set_discord_api_base(base: Option<String>) {
    *DISCORD_API_BASE_OVERRIDE.write().unwrap() = base;
}

#[derive(Serialize)]
struct SendMessagePayload {
    content: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    embeds: Option<Vec<Value>>,
}

pub async fn send_message(
    token: &str,
    channel_id: &str,
    text: &str,
    client: &Client,
) -> Result<Value, String> {
    let payload = SendMessagePayload {
        content: text.to_string(),
        embeds: None,
    };
    post_message(token, channel_id, &payload, client).await
}

pub async fn send_card(
    token: &str,
    channel_id: &str,
    text: &str,
    card: &Value,
    client: &Client,
) -> Result<Value, String> {
    let embeds = card
        .as_array()
        .cloned()
        .unwrap_or_else(|| vec![card.clone()]);

    let payload = SendMessagePayload {
        content: text.to_string(),
        embeds: Some(embeds),
    };
    post_message(token, channel_id, &payload, client).await
}

async fn post_message(
    token: &str,
    channel_id: &str,
    payload: &SendMessagePayload,
    client: &Client,
) -> Result<Value, String> {
    let url = format!("{}/channels/{}/messages", api_base(), channel_id);
    let payload_json = serde_json::to_string(payload)
        .map_err(|e| format!("serialize payload: {}", e))?;

    let mut attempt = 0;
    loop {
        let resp = client
            .post(&url)
            .header("Authorization", format!("Bot {}", token))
            .header("Content-Type", "application/json")
            .body(payload_json.clone())
            .send()
            .await
            .map_err(|e| format!("send message: {}", e))?;

        let status = resp.status();

        if status.as_u16() == 429 && attempt < MAX_RETRIES {
            let retry_after = resp
                .headers()
                .get("Retry-After")
                .and_then(|h| h.to_str().ok())
                .and_then(|s| s.parse::<u64>().ok())
                .unwrap_or(1);

            sleep(Duration::from_secs(retry_after)).await;
            attempt += 1;
            continue;
        }

        let body = resp.text().await.map_err(|e| format!("read response: {}", e))?;

        if !status.is_success() {
            return Err(format!("HTTP {}: {}", status.as_u16(), body));
        }
        return serde_json::from_str(&body).map_err(|e| format!("parse response: {}", e));
    }
}
