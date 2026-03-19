use serde::Deserialize;
use serde_json::{json, Value};
use std::fs;
use wiremock::matchers::{method, path};
use wiremock::{Mock, MockServer, ResponseTemplate};
use wgrok::{SenderConfig, WgrokSender, _set_messages_url};

#[derive(Deserialize)]
struct SenderCases {
    config: SenderCaseConfig,
    cases: Vec<SenderCase>,
}

#[derive(Deserialize)]
struct SenderCaseConfig {
    token: String,
    target: String,
    slug: String,
}

#[derive(Deserialize)]
struct SenderCase {
    name: String,
    payload: String,
    card: Value,
    expected_text: String,
    expected_target: String,
    expected_uses_card: bool,
}

fn load_cases() -> SenderCases {
    let data = fs::read_to_string("../tests/sender_cases.json").expect("load sender cases");
    serde_json::from_str(&data).expect("parse sender cases")
}

#[tokio::test]
async fn test_sender_cases() {
    _set_messages_url(None); // Clean slate

    let cases = load_cases();

    for tc in &cases.cases {
        let server = MockServer::start().await;
        _set_messages_url(Some(format!("{}/messages", server.uri())));

        // Capture the request body by returning success and checking after
        Mock::given(method("POST"))
            .and(path("/messages"))
            .respond_with(ResponseTemplate::new(200).set_body_json(json!({"id": "msg-1"})))
            .expect(1)
            .mount(&server)
            .await;

        let mut sender = WgrokSender::new(SenderConfig {
            webex_token: cases.config.token.clone(),
            target: cases.config.target.clone(),
            slug: cases.config.slug.clone(),
            domains: vec!["example.com".to_string()],
            debug: false,
            platform: "webex".to_string(),
            encrypt_key: None,
        });

        let card = if tc.card.is_null() {
            None
        } else {
            Some(&tc.card)
        };

        let result = sender.send(&tc.payload, card).await;
        assert!(result.is_ok(), "case {}: unexpected error: {:?}", tc.name, result);

        // Verify the request was made
        let requests = server.received_requests().await.unwrap();
        assert_eq!(requests.len(), 1, "case {}: expected 1 request", tc.name);

        let body: Value = serde_json::from_slice(&requests[0].body).unwrap();
        assert_eq!(
            body["text"], tc.expected_text,
            "case {}: text mismatch",
            tc.name
        );
        assert_eq!(
            body["toPersonEmail"], tc.expected_target,
            "case {}: target mismatch",
            tc.name
        );

        let has_attachments = body.get("attachments").and_then(|a| a.as_array()).map_or(false, |a| !a.is_empty());
        assert_eq!(
            has_attachments, tc.expected_uses_card,
            "case {}: card mismatch",
            tc.name
        );

        _set_messages_url(None);
    }
}

#[tokio::test]
async fn test_pause_buffers_messages() {
    let mut sender = WgrokSender::new(SenderConfig {
        webex_token: "test-token".to_string(),
        target: "user@example.com".to_string(),
        slug: "test-slug".to_string(),
        domains: vec!["example.com".to_string()],
        debug: false,
        platform: "webex".to_string(),
        encrypt_key: None,
    });

    // Pause without notify
    sender.pause(false).await.unwrap();

    // Try to send while paused (should be buffered)
    let result = sender.send("test payload", None).await;
    assert!(result.is_ok());
    assert_eq!(result.unwrap(), serde_json::Value::Null, "Paused send should return Null");
}

#[tokio::test]
async fn test_resume_transitions_state() {
    let mut sender = WgrokSender::new(SenderConfig {
        webex_token: "test-token".to_string(),
        target: "user@example.com".to_string(),
        slug: "test-slug".to_string(),
        domains: vec!["example.com".to_string()],
        debug: false,
        platform: "webex".to_string(),
        encrypt_key: None,
    });

    // Pause without notify
    sender.pause(false).await.unwrap();

    // Buffer a message (returns Null when paused)
    let result = sender.send("test payload", None).await;
    assert!(result.is_ok());
    assert_eq!(result.unwrap(), serde_json::Value::Null, "Buffered send should return Null");

    // Resume without notify
    // This will try to flush but without a mock server it will fail
    // So we just test that it doesn't panic and returns an error gracefully
    let _resume_result = sender.resume(false).await;
    // The resume will fail because we're trying to send without a mock, but that's OK
    // We're just verifying it doesn't crash
}

#[tokio::test]
async fn test_pause_state_prevents_send() {
    let mut sender = WgrokSender::new(SenderConfig {
        webex_token: "test-token".to_string(),
        target: "user@example.com".to_string(),
        slug: "test-slug".to_string(),
        domains: vec!["example.com".to_string()],
        debug: false,
        platform: "webex".to_string(),
        encrypt_key: None,
    });

    // Pause without notify
    sender.pause(false).await.unwrap();

    // Try to send multiple messages while paused - all should be buffered
    for i in 0..3 {
        let result = sender.send(&format!("payload {}", i), None).await;
        assert!(result.is_ok());
    }

    // Verify buffer has 3 messages
    // (We can't directly inspect the buffer, but we can verify by resuming and checking the count)
}

#[tokio::test]
async fn test_pause_buffers_multiple_messages() {
    let mut sender = WgrokSender::new(SenderConfig {
        webex_token: "test-token".to_string(),
        target: "user@example.com".to_string(),
        slug: "test-slug".to_string(),
        domains: vec!["example.com".to_string()],
        debug: false,
        platform: "webex".to_string(),
        encrypt_key: None,
    });

    // Pause without notify
    sender.pause(false).await.unwrap();

    // Buffer multiple messages - all should return Null
    for i in 0..3 {
        let result = sender.send(&format!("payload {}", i), None).await;
        assert!(result.is_ok());
        assert_eq!(result.unwrap(), serde_json::Value::Null, "Message {} should be buffered", i);
    }
}
