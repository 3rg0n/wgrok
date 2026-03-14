use serde_json::json;
use wiremock::matchers::{method, path};
use wiremock::{Mock, MockServer, ResponseTemplate};
use wgrok::{_set_messages_url, _set_slack_url, _set_discord_api_base};

#[tokio::test]
async fn test_platform_dispatch_webex() {
    let server = MockServer::start().await;
    _set_messages_url(Some(format!("{}/messages", server.uri())));

    Mock::given(method("POST"))
        .and(path("/messages"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({"id": "msg-1"})))
        .expect(1)
        .mount(&server)
        .await;

    let client = reqwest::Client::new();
    let result = wgrok::platform_send_message("webex", "token", "user@example.com", "test", &client).await;

    assert!(result.is_ok());
    _set_messages_url(None);
}

#[tokio::test]
async fn test_platform_dispatch_slack() {
    let server = MockServer::start().await;
    _set_slack_url(Some(format!("{}/chat.postMessage", server.uri())));

    Mock::given(method("POST"))
        .and(path("/chat.postMessage"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({"ok": true})))
        .expect(1)
        .mount(&server)
        .await;

    let client = reqwest::Client::new();
    let result = wgrok::platform_send_message("slack", "token", "#general", "test", &client).await;

    assert!(result.is_ok());
    _set_slack_url(None);
}

#[tokio::test]
async fn test_platform_dispatch_discord() {
    let server = MockServer::start().await;
    _set_discord_api_base(Some(format!("{}/api/v10", server.uri())));

    Mock::given(method("POST"))
        .and(path("/api/v10/channels/123/messages"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({"id": "msg-1"})))
        .expect(1)
        .mount(&server)
        .await;

    let client = reqwest::Client::new();
    let result = wgrok::platform_send_message("discord", "token", "123", "test", &client).await;

    assert!(result.is_ok());
    _set_discord_api_base(None);
}

#[tokio::test]
async fn test_platform_dispatch_irc() {
    let client = reqwest::Client::new();
    let result = wgrok::platform_send_message(
        "irc",
        "wgrok:pass@irc.libera.chat:6697/#wgrok",
        "#target",
        "test",
        &client,
    )
    .await;

    assert!(result.is_ok());
}

#[tokio::test]
async fn test_platform_dispatch_unknown_platform() {
    let client = reqwest::Client::new();
    let result = wgrok::platform_send_message("teams", "token", "target", "test", &client).await;

    assert!(result.is_err());
    assert!(result.unwrap_err().contains("Unsupported platform"));
}

#[tokio::test]
async fn test_platform_dispatch_send_card_webex() {
    let server = MockServer::start().await;
    _set_messages_url(Some(format!("{}/messages", server.uri())));

    Mock::given(method("POST"))
        .and(path("/messages"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({"id": "msg-1"})))
        .expect(1)
        .mount(&server)
        .await;

    let client = reqwest::Client::new();
    let card = json!({"type": "AdaptiveCard"});
    let result = wgrok::platform_send_card("webex", "token", "user@example.com", "test", &card, &client).await;

    assert!(result.is_ok());
    _set_messages_url(None);
}

#[tokio::test]
async fn test_platform_dispatch_send_card_slack() {
    let server = MockServer::start().await;
    _set_slack_url(Some(format!("{}/chat.postMessage", server.uri())));

    Mock::given(method("POST"))
        .and(path("/chat.postMessage"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({"ok": true})))
        .expect(1)
        .mount(&server)
        .await;

    let client = reqwest::Client::new();
    let card = json!([{"type": "section"}]);
    let result = wgrok::platform_send_card("slack", "token", "#general", "test", &card, &client).await;

    assert!(result.is_ok());
    _set_slack_url(None);
}

#[tokio::test]
async fn test_platform_dispatch_send_card_discord() {
    let server = MockServer::start().await;
    _set_discord_api_base(Some(format!("{}/api/v10", server.uri())));

    Mock::given(method("POST"))
        .and(path("/api/v10/channels/123/messages"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({"id": "msg-1"})))
        .expect(1)
        .mount(&server)
        .await;

    let client = reqwest::Client::new();
    let card = json!([{"title": "Test"}]);
    let result = wgrok::platform_send_card("discord", "token", "123", "test", &card, &client).await;

    assert!(result.is_ok());
    _set_discord_api_base(None);
}
