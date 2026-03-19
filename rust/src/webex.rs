use reqwest::Client;
use serde::Serialize;
use serde_json::Value;
use std::sync::RwLock;
use std::time::Duration;
use tokio::time::sleep;

pub const WEBEX_API_BASE: &str = "https://webexapis.com/v1";
pub const ADAPTIVE_CARD_CONTENT_TYPE: &str = "application/vnd.microsoft.card.adaptive";
const MAX_RETRIES: u32 = 3;

static MESSAGES_URL_OVERRIDE: RwLock<Option<String>> = RwLock::new(None);
static ATTACHMENT_ACTIONS_URL_OVERRIDE: RwLock<Option<String>> = RwLock::new(None);

fn messages_url() -> String {
    if let Ok(guard) = MESSAGES_URL_OVERRIDE.read() {
        if let Some(url) = guard.as_ref() {
            return url.clone();
        }
    }
    format!("{}/messages", WEBEX_API_BASE)
}

fn attachment_actions_url() -> String {
    if let Ok(guard) = ATTACHMENT_ACTIONS_URL_OVERRIDE.read() {
        if let Some(url) = guard.as_ref() {
            return url.clone();
        }
    }
    format!("{}/attachment/actions", WEBEX_API_BASE)
}

/// Override messages URL for testing. Pass None to reset.
pub fn _set_messages_url(url: Option<String>) {
    *MESSAGES_URL_OVERRIDE.write().unwrap() = url;
}

/// Override attachment actions URL for testing. Pass None to reset.
pub fn _set_attachment_actions_url(url: Option<String>) {
    *ATTACHMENT_ACTIONS_URL_OVERRIDE.write().unwrap() = url;
}

#[derive(Serialize)]
struct SendMessagePayload {
    #[serde(rename = "toPersonEmail")]
    to_person_email: String,
    text: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    attachments: Option<Vec<CardAttachment>>,
}

#[derive(Serialize)]
struct CardAttachment {
    #[serde(rename = "contentType")]
    content_type: String,
    content: Value,
}

pub async fn send_message(
    token: &str,
    to_email: &str,
    text: &str,
    client: &Client,
) -> Result<Value, String> {
    let payload = SendMessagePayload {
        to_person_email: to_email.to_string(),
        text: text.to_string(),
        attachments: None,
    };
    post_message(token, &payload, client).await
}

pub async fn send_card(
    token: &str,
    to_email: &str,
    text: &str,
    card: &Value,
    client: &Client,
) -> Result<Value, String> {
    let payload = SendMessagePayload {
        to_person_email: to_email.to_string(),
        text: text.to_string(),
        attachments: Some(vec![CardAttachment {
            content_type: ADAPTIVE_CARD_CONTENT_TYPE.to_string(),
            content: card.clone(),
        }]),
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

pub async fn get_message(
    token: &str,
    message_id: &str,
    client: &Client,
) -> Result<Value, String> {
    get_json(token, &format!("{}/{}", messages_url(), message_id), client).await
}

pub async fn get_attachment_action(
    token: &str,
    action_id: &str,
    client: &Client,
) -> Result<Value, String> {
    get_json(
        token,
        &format!("{}/{}", attachment_actions_url(), action_id),
        client,
    )
    .await
}

async fn get_json(token: &str, url: &str, client: &Client) -> Result<Value, String> {
    let mut attempt = 0;
    loop {
        let resp = client
            .get(url)
            .header("Authorization", format!("Bearer {}", token))
            .header("Content-Type", "application/json")
            .send()
            .await
            .map_err(|e| format!("GET {}: {}", url, e))?;

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

pub fn extract_cards(message: &Value) -> Vec<Value> {
    let attachments = match message.get("attachments").and_then(|a| a.as_array()) {
        Some(arr) => arr,
        None => return vec![],
    };
    attachments
        .iter()
        .filter_map(|att| {
            let ct = att.get("contentType")?.as_str()?;
            if ct == ADAPTIVE_CARD_CONTENT_TYPE {
                att.get("content").cloned()
            } else {
                None
            }
        })
        .collect()
}
