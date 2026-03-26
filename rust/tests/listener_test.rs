use wgrok::create_listener;
use wgrok::get_logger;

#[test]
fn test_create_listener_factory_webex() {
    let logger = get_logger(false, "test");
    let result = create_listener("webex", "test_token_webex", logger);
    assert!(result.is_ok(), "Should create Webex listener");
}

#[test]
fn test_create_listener_factory_slack() {
    let logger = get_logger(false, "test");
    let result = create_listener("slack", "xapp-test-token", logger);
    assert!(result.is_ok(), "Should create Slack listener");
}

#[test]
fn test_create_listener_factory_discord() {
    let logger = get_logger(false, "test");
    let result = create_listener("discord", "test_token_discord", logger);
    assert!(result.is_ok(), "Should create Discord listener");
}

#[test]
fn test_create_listener_factory_irc() {
    let logger = get_logger(false, "test");
    let result = create_listener("irc", "nick@localhost:6667/channel", logger);
    assert!(result.is_ok(), "Should create IRC listener");
}

#[test]
fn test_create_listener_factory_invalid_platform() {
    let logger = get_logger(false, "test");
    let result = create_listener("invalid_platform", "token", logger);
    assert!(result.is_err(), "Should fail for invalid platform");
    match result {
        Err(e) => assert!(e.contains("Unsupported"), "Error should mention unsupported platform"),
        Ok(_) => panic!("Should have failed for invalid platform"),
    }
}

#[test]
fn test_incoming_message_structure() {
    use wgrok::IncomingMessage;
    use serde_json::json;

    let cards = vec![json!({"title": "Test Card"})];
    let msg = IncomingMessage {
        sender: "user@example.com".to_string(),
        text: "Hello, world!".to_string(),
        html: String::new(),
        msg_id: "msg_123".to_string(),
        platform: "webex".to_string(),
        cards,
    };

    assert_eq!(msg.sender, "user@example.com");
    assert_eq!(msg.text, "Hello, world!");
    assert_eq!(msg.msg_id, "msg_123");
    assert_eq!(msg.platform, "webex");
    assert_eq!(msg.cards.len(), 1);
}

#[test]
fn test_incoming_message_clone() {
    use wgrok::IncomingMessage;

    let msg1 = IncomingMessage {
        sender: "alice@example.com".to_string(),
        text: "Test message".to_string(),
        html: String::new(),
        msg_id: "id_1".to_string(),
        platform: "slack".to_string(),
        cards: vec![],
    };

    let msg2 = msg1.clone();
    assert_eq!(msg1.sender, msg2.sender);
    assert_eq!(msg1.platform, msg2.platform);
}

#[tokio::test]
async fn test_message_callback_mpsc() {
    use wgrok::IncomingMessage;
    use tokio::sync::mpsc;

    let (tx, mut rx) = mpsc::unbounded_channel();

    let msg = IncomingMessage {
        sender: "test@example.com".to_string(),
        text: "Test".to_string(),
        html: String::new(),
        msg_id: "123".to_string(),
        platform: "test".to_string(),
        cards: vec![],
    };

    tx.send(msg.clone()).expect("Should send message");

    let received = rx.recv().await;
    assert!(received.is_some(), "Should receive message");
    let received_msg = received.unwrap();
    assert_eq!(received_msg.sender, msg.sender);
    assert_eq!(received_msg.text, msg.text);
}
