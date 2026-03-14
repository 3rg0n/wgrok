use serde_json::json;
use std::sync::Mutex;
use wiremock::matchers::{header, method, path};
use wiremock::{Mock, MockServer, ResponseTemplate};
use wgrok::{_set_attachment_actions_url, _set_messages_url};

static URL_LOCK: Mutex<()> = Mutex::new(());

#[tokio::test]
async fn test_webex_http_all() {
    let _lock = URL_LOCK.lock().unwrap();

    // -- send_message --
    {
        let server = MockServer::start().await;
        _set_messages_url(Some(format!("{}/messages", server.uri())));

        Mock::given(method("POST"))
            .and(path("/messages"))
            .and(header("Authorization", "Bearer tok123"))
            .respond_with(ResponseTemplate::new(200).set_body_json(json!({"id": "msg-1"})))
            .mount(&server)
            .await;

        let client = reqwest::Client::new();
        let result = wgrok::send_message("tok123", "user@example.com", "hello", &client)
            .await
            .unwrap();
        assert_eq!(result["id"], "msg-1");

        _set_messages_url(None);
    }

    // -- send_message error --
    {
        let server = MockServer::start().await;
        _set_messages_url(Some(format!("{}/messages", server.uri())));

        Mock::given(method("POST"))
            .and(path("/messages"))
            .respond_with(ResponseTemplate::new(401).set_body_string("unauthorized"))
            .mount(&server)
            .await;

        let client = reqwest::Client::new();
        let result = wgrok::send_message("badtoken", "user@example.com", "hello", &client).await;
        assert!(result.is_err());
        assert!(result.unwrap_err().contains("401"));

        _set_messages_url(None);
    }

    // -- send_card --
    {
        let server = MockServer::start().await;
        _set_messages_url(Some(format!("{}/messages", server.uri())));

        Mock::given(method("POST"))
            .and(path("/messages"))
            .respond_with(ResponseTemplate::new(200).set_body_json(json!({"id": "card-1"})))
            .mount(&server)
            .await;

        let card = json!({"type": "AdaptiveCard", "body": []});
        let client = reqwest::Client::new();
        let result = wgrok::send_card("tok", "user@x.com", "fallback", &card, &client)
            .await
            .unwrap();
        assert_eq!(result["id"], "card-1");

        _set_messages_url(None);
    }

    // -- get_message --
    {
        let server = MockServer::start().await;
        _set_messages_url(Some(format!("{}/messages", server.uri())));

        Mock::given(method("GET"))
            .and(path("/messages/msg-1"))
            .respond_with(
                ResponseTemplate::new(200).set_body_json(json!({"id": "msg-1", "text": "hello"})),
            )
            .mount(&server)
            .await;

        let client = reqwest::Client::new();
        let result = wgrok::get_message("tok", "msg-1", &client).await.unwrap();
        assert_eq!(result["id"], "msg-1");
        assert_eq!(result["text"], "hello");

        _set_messages_url(None);
    }

    // -- get_attachment_action --
    {
        let server = MockServer::start().await;
        _set_attachment_actions_url(Some(format!(
            "{}/attachment/actions",
            server.uri()
        )));

        Mock::given(method("GET"))
            .and(path("/attachment/actions/act-1"))
            .respond_with(
                ResponseTemplate::new(200).set_body_json(
                    json!({"id": "act-1", "type": "submit", "inputs": {"name": "test"}}),
                ),
            )
            .mount(&server)
            .await;

        let client = reqwest::Client::new();
        let result = wgrok::get_attachment_action("tok", "act-1", &client)
            .await
            .unwrap();
        assert_eq!(result["id"], "act-1");
        assert_eq!(result["type"], "submit");

        _set_attachment_actions_url(None);
    }
}

#[tokio::test]
async fn test_retry_after_retries_on_429_then_succeeds() {
    let _lock = URL_LOCK.lock().unwrap();

    let server = MockServer::start().await;
    _set_messages_url(Some(format!("{}/messages", server.uri())));

    // First attempt: 429 with Retry-After
    Mock::given(method("POST"))
        .and(path("/messages"))
        .respond_with(
            ResponseTemplate::new(429)
                .append_header("Retry-After", "1")
                .set_body_string("rate limited"),
        )
        .up_to_n_times(1)
        .mount(&server)
        .await;

    // Second attempt: 200 OK
    Mock::given(method("POST"))
        .and(path("/messages"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({"id": "msg-retry-1"})))
        .mount(&server)
        .await;

    let client = reqwest::Client::new();
    let result = wgrok::send_message("tok123", "user@example.com", "hello", &client)
        .await
        .unwrap();
    assert_eq!(result["id"], "msg-retry-1");

    _set_messages_url(None);
}

#[tokio::test]
async fn test_retry_after_retries_multiple_429s() {
    let _lock = URL_LOCK.lock().unwrap();

    let server = MockServer::start().await;
    _set_messages_url(Some(format!("{}/messages", server.uri())));

    // First attempt: 429 with Retry-After: 1
    Mock::given(method("POST"))
        .and(path("/messages"))
        .respond_with(
            ResponseTemplate::new(429)
                .append_header("Retry-After", "1")
                .set_body_string("rate limited 1"),
        )
        .up_to_n_times(1)
        .mount(&server)
        .await;

    // Second attempt: 429 with Retry-After: 2
    Mock::given(method("POST"))
        .and(path("/messages"))
        .respond_with(
            ResponseTemplate::new(429)
                .append_header("Retry-After", "2")
                .set_body_string("rate limited 2"),
        )
        .up_to_n_times(1)
        .mount(&server)
        .await;

    // Third attempt: 200 OK
    Mock::given(method("POST"))
        .and(path("/messages"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({"id": "msg-retry-2"})))
        .mount(&server)
        .await;

    let client = reqwest::Client::new();
    let result = wgrok::send_message("tok123", "user@example.com", "hello", &client)
        .await
        .unwrap();
    assert_eq!(result["id"], "msg-retry-2");

    _set_messages_url(None);
}

#[tokio::test]
async fn test_retry_after_raises_after_max_retries() {
    let _lock = URL_LOCK.lock().unwrap();

    let server = MockServer::start().await;
    _set_messages_url(Some(format!("{}/messages", server.uri())));

    // All attempts: 429 (4 total attempts before giving up)
    Mock::given(method("POST"))
        .and(path("/messages"))
        .respond_with(
            ResponseTemplate::new(429)
                .append_header("Retry-After", "1")
                .set_body_string("rate limited"),
        )
        .mount(&server)
        .await;

    let client = reqwest::Client::new();
    let result = wgrok::send_message("tok123", "user@example.com", "hello", &client).await;
    assert!(result.is_err());
    assert!(result.unwrap_err().contains("429"));

    _set_messages_url(None);
}

#[tokio::test]
async fn test_retry_after_uses_retry_after_header_value() {
    let _lock = URL_LOCK.lock().unwrap();

    let server = MockServer::start().await;
    _set_messages_url(Some(format!("{}/messages", server.uri())));

    // First attempt: 429 with Retry-After: 1 (we'll sleep for this)
    Mock::given(method("POST"))
        .and(path("/messages"))
        .respond_with(
            ResponseTemplate::new(429)
                .append_header("Retry-After", "1")
                .set_body_string("rate limited"),
        )
        .up_to_n_times(1)
        .mount(&server)
        .await;

    // Second attempt: 200 OK
    Mock::given(method("POST"))
        .and(path("/messages"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({"id": "msg-retry-3"})))
        .mount(&server)
        .await;

    let client = reqwest::Client::new();
    let result = wgrok::send_message("tok123", "user@example.com", "hello", &client)
        .await
        .unwrap();
    assert_eq!(result["id"], "msg-retry-3");

    _set_messages_url(None);
}

#[tokio::test]
async fn test_retry_after_defaults_retry_after_to_1_when_missing() {
    let _lock = URL_LOCK.lock().unwrap();

    let server = MockServer::start().await;
    _set_messages_url(Some(format!("{}/messages", server.uri())));

    // First attempt: 429 without Retry-After header (should default to 1)
    Mock::given(method("POST"))
        .and(path("/messages"))
        .respond_with(ResponseTemplate::new(429).set_body_string("rate limited"))
        .up_to_n_times(1)
        .mount(&server)
        .await;

    // Second attempt: 200 OK
    Mock::given(method("POST"))
        .and(path("/messages"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({"id": "msg-retry-4"})))
        .mount(&server)
        .await;

    let client = reqwest::Client::new();
    let result = wgrok::send_message("tok123", "user@example.com", "hello", &client)
        .await
        .unwrap();
    assert_eq!(result["id"], "msg-retry-4");

    _set_messages_url(None);
}

#[tokio::test]
async fn test_retry_after_get_json() {
    let _lock = URL_LOCK.lock().unwrap();

    let server = MockServer::start().await;
    _set_messages_url(Some(format!("{}/messages", server.uri())));

    // First attempt: 429 with Retry-After
    Mock::given(method("GET"))
        .and(path("/messages/msg-retry-5"))
        .respond_with(
            ResponseTemplate::new(429)
                .append_header("Retry-After", "1")
                .set_body_string("rate limited"),
        )
        .up_to_n_times(1)
        .mount(&server)
        .await;

    // Second attempt: 200 OK
    Mock::given(method("GET"))
        .and(path("/messages/msg-retry-5"))
        .respond_with(
            ResponseTemplate::new(200)
                .set_body_json(json!({"id": "msg-retry-5", "text": "retrieved"})),
        )
        .mount(&server)
        .await;

    let client = reqwest::Client::new();
    let result = wgrok::get_message("tok", "msg-retry-5", &client)
        .await
        .unwrap();
    assert_eq!(result["id"], "msg-retry-5");
    assert_eq!(result["text"], "retrieved");

    _set_messages_url(None);
}
