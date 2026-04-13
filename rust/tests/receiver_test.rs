use serde::Deserialize;
use serde_json::Value;
use std::fs;
use std::sync::{Arc, Mutex};
use webex_message_handler::{DecryptedMessage, MercuryActivity};
use wgrok::{ReceiverConfig, WgrokReceiver};

#[derive(Deserialize)]
struct ReceiverCases {
    config: ReceiverCaseConfig,
    cases: Vec<ReceiverCase>,
}

#[derive(Deserialize)]
struct ReceiverCaseConfig {
    slug: String,
    domains: Vec<String>,
}

#[derive(Deserialize)]
struct ReceiverCase {
    name: String,
    sender: String,
    text: String,
    cards: Vec<Value>,
    expect_handler: bool,
    expected_slug: Option<String>,
    expected_payload: Option<String>,
    expected_from: Option<String>,
    expected_cards: Option<Vec<Value>>,
}

fn load_cases() -> ReceiverCases {
    let data = fs::read_to_string("../tests/receiver_cases.json").expect("load receiver cases");
    serde_json::from_str(&data).expect("parse receiver cases")
}

fn fake_msg(sender: &str, text: &str) -> DecryptedMessage {
    DecryptedMessage {
        id: "test-msg-id".to_string(),
        parent_id: None,
        mentioned_people: Vec::new(),
        mentioned_groups: Vec::new(),
        room_id: "room-abc".to_string(),
        person_id: "person-123".to_string(),
        person_email: sender.to_string(),
        text: text.to_string(),
        html: None,
        created: "2024-01-01T00:00:00Z".to_string(),
        room_type: None,
        files: Vec::new(),
        raw: MercuryActivity {
            id: String::new(),
            verb: String::new(),
            actor: Default::default(),
            object: Default::default(),
            target: Default::default(),
            published: String::new(),
            encryption_key_url: None,
            parent: None,
        },
    }
}

#[test]
fn test_receiver_cases() {
    let cases = load_cases();

    for tc in &cases.cases {
        let called = Arc::new(Mutex::new(false));
        let captured_slug = Arc::new(Mutex::new(String::new()));
        let captured_payload = Arc::new(Mutex::new(String::new()));
        let captured_cards = Arc::new(Mutex::new(Vec::<Value>::new()));
        let captured_from = Arc::new(Mutex::new(String::new()));

        let called_c = called.clone();
        let slug_c = captured_slug.clone();
        let payload_c = captured_payload.clone();
        let cards_c = captured_cards.clone();
        let from_c = captured_from.clone();

        let receiver = WgrokReceiver::new(
            ReceiverConfig {
                webex_token: "fake-token".to_string(),
                slug: cases.config.slug.clone(),
                domains: cases.config.domains.clone(),
                debug: false,
                platform: "webex".to_string(),
                encrypt_key: None,
            },
            Box::new(move |slug: &str, payload: &str, cards: &[Value], from_slug: &str| {
                *called_c.lock().unwrap() = true;
                *slug_c.lock().unwrap() = slug.to_string();
                *payload_c.lock().unwrap() = payload.to_string();
                *cards_c.lock().unwrap() = cards.to_vec();
                *from_c.lock().unwrap() = from_slug.to_string();
            }),
        );

        let msg = fake_msg(&tc.sender, &tc.text);
        receiver.on_message_with_cards(&msg, &tc.cards);

        let was_called = *called.lock().unwrap();
        assert_eq!(
            was_called, tc.expect_handler,
            "case {}: handler called = {}, expected {}",
            tc.name, was_called, tc.expect_handler
        );

        if tc.expect_handler {
            if let Some(ref expected_slug) = tc.expected_slug {
                assert_eq!(
                    *captured_slug.lock().unwrap(),
                    *expected_slug,
                    "case {}: slug mismatch",
                    tc.name
                );
            }
            if let Some(ref expected_payload) = tc.expected_payload {
                assert_eq!(
                    *captured_payload.lock().unwrap(),
                    *expected_payload,
                    "case {}: payload mismatch",
                    tc.name
                );
            }
            if let Some(ref expected_from) = tc.expected_from {
                assert_eq!(
                    *captured_from.lock().unwrap(),
                    *expected_from,
                    "case {}: from_slug mismatch",
                    tc.name
                );
            }
            if let Some(ref expected_cards) = tc.expected_cards {
                assert_eq!(
                    *captured_cards.lock().unwrap(),
                    *expected_cards,
                    "case {}: cards mismatch",
                    tc.name
                );
            }
        }
    }
}
