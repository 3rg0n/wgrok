use serde_json::{json, Value};
use wiremock::matchers::{method, path};
use wiremock::{Mock, MockServer, ResponseTemplate};
use wgrok::_set_discord_api_base;

#[tokio::test]
async fn test_discord_send_message() {
    let server = MockServer::start().await;
    _set_discord_api_base(Some(format!("{}/api/v10", server.uri())));

    Mock::given(method("POST"))
        .and(path("/api/v10/channels/123456/messages"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({"id": "msg-123"})))
        .expect(1)
        .mount(&server)
        .await;

    let client = reqwest::Client::new();
    let result = wgrok::discord_send_message("Bot-token", "123456", "test message", &client).await;

    assert!(result.is_ok());
    let response = result.unwrap();
    assert_eq!(response["id"], "msg-123");

    let requests = server.received_requests().await.unwrap();
    assert_eq!(requests.len(), 1);

    let body: Value = serde_json::from_slice(&requests[0].body).unwrap();
    assert_eq!(body["content"], "test message");
    assert!(body.get("embeds").is_none());

    // Check auth header
    let auth_header = requests[0]
        .headers
        .get("Authorization")
        .and_then(|h| h.to_str().ok());
    assert_eq!(auth_header, Some("Bot Bot-token"));

    _set_discord_api_base(None);
}

#[tokio::test]
async fn test_discord_send_card() {
    let server = MockServer::start().await;
    _set_discord_api_base(Some(format!("{}/api/v10", server.uri())));

    Mock::given(method("POST"))
        .and(path("/api/v10/channels/123456/messages"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({"id": "msg-123"})))
        .expect(1)
        .mount(&server)
        .await;

    let client = reqwest::Client::new();
    let card = json!([
        {
            "title": "Test",
            "description": "Test embed"
        }
    ]);

    let result = wgrok::discord_send_card("Bot-token", "123456", "test", &card, &client).await;

    assert!(result.is_ok());

    let requests = server.received_requests().await.unwrap();
    assert_eq!(requests.len(), 1);

    let body: Value = serde_json::from_slice(&requests[0].body).unwrap();
    assert_eq!(body["content"], "test");
    assert!(body.get("embeds").is_some());
    assert_eq!(body["embeds"].as_array().unwrap().len(), 1);

    _set_discord_api_base(None);
}

#[tokio::test]
async fn test_discord_retry_on_429() {
    let server = MockServer::start().await;
    _set_discord_api_base(Some(format!("{}/api/v10", server.uri())));

    // First attempt: 429 with Retry-After
    Mock::given(method("POST"))
        .and(path("/api/v10/channels/123456/messages"))
        .respond_with(ResponseTemplate::new(429).append_header("Retry-After", "0"))
        .up_to_n_times(1)
        .mount(&server)
        .await;

    // Second attempt: 200 OK
    Mock::given(method("POST"))
        .and(path("/api/v10/channels/123456/messages"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({"id": "msg-123"})))
        .mount(&server)
        .await;

    let client = reqwest::Client::new();
    let result = wgrok::discord_send_message("Bot-token", "123456", "test", &client).await;

    assert!(result.is_ok());

    _set_discord_api_base(None);
}

#[tokio::test]
async fn test_discord_error_response() {
    let server = MockServer::start().await;
    _set_discord_api_base(Some(format!("{}/api/v10", server.uri())));

    Mock::given(method("POST"))
        .and(path("/api/v10/channels/123456/messages"))
        .respond_with(ResponseTemplate::new(404).set_body_string("channel not found"))
        .expect(1)
        .mount(&server)
        .await;

    let client = reqwest::Client::new();
    let result = wgrok::discord_send_message("Bot-token", "123456", "test", &client).await;

    assert!(result.is_err());
    assert!(result.unwrap_err().contains("404"));

    _set_discord_api_base(None);
}
