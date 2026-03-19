use reqwest::Client;
use serde::Serialize;
use serde_json::Value;
use std::sync::RwLock;
use std::time::Duration;
use tokio::time::sleep;

pub const SLACK_API_BASE: &str = "https://slack.com/api";
const MAX_RETRIES: u32 = 3;

static SLACK_API_URL_OVERRIDE: RwLock<Option<String>> = RwLock::new(None);

fn messages_url() -> String {
    if let Ok(guard) = SLACK_API_URL_OVERRIDE.read() {
        if let Some(url) = guard.as_ref() {
            return url.clone();
        }
    }
    format!("{}/chat.postMessage", SLACK_API_BASE)
}

/// Override Slack API URL for testing. Pass None to reset.
pub fn _set_slack_url(url: Option<String>) {
    *SLACK_API_URL_OVERRIDE.write().unwrap() = url;
}

#[derive(Serialize)]
struct SendMessagePayload {
    channel: String,
    text: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    blocks: Option<Vec<Value>>,
}

pub async fn send_message(
    token: &str,
    channel: &str,
    text: &str,
    client: &Client,
) -> Result<Value, String> {
    let payload = SendMessagePayload {
        channel: channel.to_string(),
        text: text.to_string(),
        blocks: None,
    };
    post_message(token, &payload, client).await
}

pub async fn send_card(
    token: &str,
    channel: &str,
    text: &str,
    card: &Value,
    client: &Client,
) -> Result<Value, String> {
    let blocks = card
        .as_array()
        .cloned()
        .unwrap_or_else(|| vec![card.clone()]);

    let payload = SendMessagePayload {
        channel: channel.to_string(),
        text: text.to_string(),
        blocks: Some(blocks),
    };
    post_message(token, &payload, client).await
}

async fn post_message(
    token: &str,
    payload: &SendMessagePayload,
    client: &Client,
) -> Result<Value, String> {
    let payload_json = serde_json::to_string(payload)
        .map_err(|e| format!("serialize payload: {}", e))?;

    let mut attempt = 0;
    loop {
        let resp = client
            .post(messages_url())
            .header("Authorization", format!("Bearer {}", token))
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
                .unwrap_or(1)
                .min(300);

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
