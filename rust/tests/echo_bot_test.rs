use serde::Deserialize;
use serde_json::{json, Value};
use std::collections::HashMap;
use std::fs;
use webex_message_handler::{DecryptedMessage, MercuryActivity};
use wiremock::matchers::{method, path};
use wiremock::{Mock, MockServer, ResponseTemplate};
use wgrok::{BotConfig, WgrokEchoBot, _set_messages_url};

#[derive(Deserialize)]
struct EchoBotCases {
    config: EchoBotConfig,
    routes: HashMap<String, String>,
    cases: Vec<EchoBotCase>,
}

#[derive(Deserialize)]
struct EchoBotConfig {
    domains: Vec<String>,
}

#[derive(Deserialize)]
struct EchoBotCase {
    name: String,
    sender: String,
    text: String,
    cards: Vec<Value>,
    expect_send: bool,
    expected_reply_to: Option<String>,
    expected_reply_text: Option<String>,
    expected_reply_card: Option<Value>,
    #[serde(default)]
    use_routes: bool,
}

fn load_cases() -> EchoBotCases {
    let data = fs::read_to_string("../tests/echo_bot_cases.json").expect("load echo bot cases");
    serde_json::from_str(&data).expect("parse echo bot cases")
}

fn fake_msg(sender: &str, text: &str) -> DecryptedMessage {
    DecryptedMessage {
        id: "test-msg-id".to_string(),
        room_id: "room-abc".to_string(),
        person_id: "person-123".to_string(),
        person_email: sender.to_string(),
        text: text.to_string(),
        html: None,
        created: "2024-01-01T00:00:00Z".to_string(),
        room_type: None,
        raw: MercuryActivity {
            id: String::new(),
            verb: String::new(),
            actor: Default::default(),
            object: Default::default(),
            target: Default::default(),
            published: String::new(),
            encryption_key_url: None,
        },
    }
}

#[tokio::test]
async fn test_echo_bot_cases() {
    let cases = load_cases();

    for tc in &cases.cases {
        let server = MockServer::start().await;
        _set_messages_url(Some(format!("{}/messages", server.uri())));

        Mock::given(method("POST"))
            .and(path("/messages"))
            .respond_with(ResponseTemplate::new(200).set_body_json(json!({"id": "msg-1"})))
            .mount(&server)
            .await;

        let routes = if tc.use_routes {
            cases.routes.clone()
        } else {
            HashMap::new()
        };

        let bot = WgrokEchoBot::new(BotConfig {
            webex_token: "fake-token".to_string(),
            domains: cases.config.domains.clone(),
            debug: false,
            routes,
            platform_tokens: {
                let mut map = HashMap::new();
                map.insert("webex".to_string(), vec!["fake-token".to_string()]);
                map
            },
            webhook_port: None,
            webhook_secret: None,
        });

        let msg = fake_msg(&tc.sender, &tc.text);
        bot.on_message_with_cards(&msg, &tc.cards).await;

        let requests = server.received_requests().await.unwrap();

        if tc.expect_send {
            assert_eq!(
                requests.len(),
                1,
                "case {}: expected 1 request, got {}",
                tc.name,
                requests.len()
            );

            let body: Value = serde_json::from_slice(&requests[0].body).unwrap();
            if let Some(ref expected_to) = tc.expected_reply_to {
                assert_eq!(
                    body["toPersonEmail"], *expected_to,
                    "case {}: reply_to mismatch",
                    tc.name
                );
            }
            if let Some(ref expected_text) = tc.expected_reply_text {
                assert_eq!(
                    body["text"], *expected_text,
                    "case {}: reply_text mismatch",
                    tc.name
                );
            }
            if tc.expected_reply_card.is_some() {
                let has_attachments = body
                    .get("attachments")
                    .and_then(|a| a.as_array())
                    .map_or(false, |a| !a.is_empty());
                assert!(
                    has_attachments,
                    "case {}: expected card attachment",
                    tc.name
                );
            }
        } else {
            assert_eq!(
                requests.len(),
                0,
                "case {}: expected 0 requests, got {}",
                tc.name,
                requests.len()
            );
        }

        _set_messages_url(None);
    }
}
