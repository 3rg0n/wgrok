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

        let sender = WgrokSender::new(SenderConfig {
            webex_token: cases.config.token.clone(),
            target: cases.config.target.clone(),
            slug: cases.config.slug.clone(),
            domains: vec!["example.com".to_string()],
            debug: false,
            platform: "webex".to_string(),
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
