use reqwest::Client;
use serde::Serialize;
use serde_json::Value;

pub const WEBEX_API_BASE: &str = "https://webexapis.com/v1";
pub const ADAPTIVE_CARD_CONTENT_TYPE: &str = "application/vnd.microsoft.card.adaptive";

fn messages_url() -> String {
    format!("{}/messages", WEBEX_API_BASE)
}

fn attachment_actions_url() -> String {
    format!("{}/attachment/actions", WEBEX_API_BASE)
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
    let resp = client
        .post(&messages_url())
        .header("Authorization", format!("Bearer {}", token))
        .json(payload)
        .send()
        .await
        .map_err(|e| format!("send message: {}", e))?;

    let status = resp.status();
    let body = resp.text().await.map_err(|e| format!("read response: {}", e))?;
    if !status.is_success() {
        return Err(format!("HTTP {}: {}", status.as_u16(), body));
    }
    serde_json::from_str(&body).map_err(|e| format!("parse response: {}", e))
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
    let resp = client
        .get(url)
        .header("Authorization", format!("Bearer {}", token))
        .header("Content-Type", "application/json")
        .send()
        .await
        .map_err(|e| format!("GET {}: {}", url, e))?;

    let status = resp.status();
    let body = resp.text().await.map_err(|e| format!("read response: {}", e))?;
    if !status.is_success() {
        return Err(format!("HTTP {}: {}", status.as_u16(), body));
    }
    serde_json::from_str(&body).map_err(|e| format!("parse response: {}", e))
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
