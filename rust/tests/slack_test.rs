use serde_json::{json, Value};
use wiremock::matchers::{method, path};
use wiremock::{Mock, MockServer, ResponseTemplate};
use wgrok::_set_slack_url;

#[tokio::test]
async fn test_slack_send_message() {
    let server = MockServer::start().await;
    _set_slack_url(Some(format!("{}/chat.postMessage", server.uri())));

    Mock::given(method("POST"))
        .and(path("/chat.postMessage"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({"ok": true, "ts": "1234.5678"})))
        .expect(1)
        .mount(&server)
        .await;

    let client = reqwest::Client::new();
    let result = wgrok::slack_send_message("xoxb-test-token", "#general", "test message", &client).await;

    assert!(result.is_ok());
    let response = result.unwrap();
    assert_eq!(response["ok"], true);
    assert_eq!(response["ts"], "1234.5678");

    let requests = server.received_requests().await.unwrap();
    assert_eq!(requests.len(), 1);

    let body: Value = serde_json::from_slice(&requests[0].body).unwrap();
    assert_eq!(body["channel"], "#general");
    assert_eq!(body["text"], "test message");
    assert!(body.get("blocks").is_none());

    // Check auth header
    let auth_header = requests[0]
        .headers
        .get("Authorization")
        .and_then(|h| h.to_str().ok());
    assert_eq!(auth_header, Some("Bearer xoxb-test-token"));

    _set_slack_url(None);
}

#[tokio::test]
async fn test_slack_send_card() {
    let server = MockServer::start().await;
    _set_slack_url(Some(format!("{}/chat.postMessage", server.uri())));

    Mock::given(method("POST"))
        .and(path("/chat.postMessage"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({"ok": true, "ts": "1234.5678"})))
        .expect(1)
        .mount(&server)
        .await;

    let client = reqwest::Client::new();
    let card = json!([
        {
            "type": "section",
            "text": { "type": "mrkdwn", "text": "Hello" }
        }
    ]);

    let result = wgrok::slack_send_card("xoxb-test-token", "#general", "test", &card, &client).await;

    assert!(result.is_ok());

    let requests = server.received_requests().await.unwrap();
    assert_eq!(requests.len(), 1);

    let body: Value = serde_json::from_slice(&requests[0].body).unwrap();
    assert_eq!(body["channel"], "#general");
    assert_eq!(body["text"], "test");
    assert!(body.get("blocks").is_some());

    _set_slack_url(None);
}

#[tokio::test]
async fn test_slack_retry_on_429() {
    let server = MockServer::start().await;
    _set_slack_url(Some(format!("{}/chat.postMessage", server.uri())));

    // First attempt: 429 with Retry-After
    Mock::given(method("POST"))
        .and(path("/chat.postMessage"))
        .respond_with(ResponseTemplate::new(429).append_header("Retry-After", "0"))
        .up_to_n_times(1)
        .mount(&server)
        .await;

    // Second attempt: 200 OK
    Mock::given(method("POST"))
        .and(path("/chat.postMessage"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({"ok": true, "ts": "1234.5678"})))
        .mount(&server)
        .await;

    let client = reqwest::Client::new();
    let result = wgrok::slack_send_message("xoxb-test-token", "#general", "test", &client).await;

    assert!(result.is_ok());

    _set_slack_url(None);
}

#[tokio::test]
async fn test_slack_error_response() {
    let server = MockServer::start().await;
    _set_slack_url(Some(format!("{}/chat.postMessage", server.uri())));

    Mock::given(method("POST"))
        .and(path("/chat.postMessage"))
        .respond_with(ResponseTemplate::new(400).set_body_string("invalid_channel"))
        .expect(1)
        .mount(&server)
        .await;

    let client = reqwest::Client::new();
    let result = wgrok::slack_send_message("xoxb-test-token", "invalid", "test", &client).await;

    assert!(result.is_err());
    assert!(result.unwrap_err().contains("400"));

    _set_slack_url(None);
}
