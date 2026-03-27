use reqwest::Client;
use serde_json::Value;

use crate::discord;
use crate::irc;
use crate::slack;
use crate::webex;

pub async fn platform_send_message(
    platform: &str,
    token: &str,
    target: &str,
    text: &str,
    client: &Client,
) -> Result<Value, String> {
    match platform {
        "webex" => webex::send_message(token, target, text, client).await,
        "slack" => slack::send_message(token, target, text, client).await,
        "discord" => discord::send_message(token, target, text, client).await,
        "irc" => irc::send_message(token, target, text).await,
        _ => Err(format!("Unsupported platform: {}", platform)),
    }
}

pub async fn platform_send_card(
    platform: &str,
    token: &str,
    target: &str,
    text: &str,
    card: &Value,
    client: &Client,
) -> Result<Value, String> {
    match platform {
        "webex" => webex::send_card(token, target, text, card, client).await,
        "slack" => slack::send_card(token, target, text, card, client).await,
        "discord" => discord::send_card(token, target, text, card, client).await,
        "irc" => irc::send_card(token, target, text, card).await,
        _ => Err(format!("Unsupported platform: {}", platform)),
    }
}

pub async fn platform_send_message_to_room(
    platform: &str,
    token: &str,
    room_id: &str,
    text: &str,
    client: &Client,
) -> Result<Value, String> {
    match platform {
        "webex" => webex::send_message_to_room(token, room_id, text, client).await,
        _ => Err(format!("room-based send not supported for platform: {}", platform)),
    }
}

pub async fn platform_send_card_to_room(
    platform: &str,
    token: &str,
    room_id: &str,
    text: &str,
    card: &Value,
    client: &Client,
) -> Result<Value, String> {
    match platform {
        "webex" => webex::send_card_to_room(token, room_id, text, card, client).await,
        _ => Err(format!("room-based send not supported for platform: {}", platform)),
    }
}
