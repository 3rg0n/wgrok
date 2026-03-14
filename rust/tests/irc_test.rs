use serde::Deserialize;
use serde_json::Value;
use std::fs;
use wgrok::parse_connection_string;

#[derive(Deserialize)]
struct IrcCases {
    parse_connection_string: Vec<ParseConnectionStringCase>,
}

#[derive(Deserialize)]
struct ParseConnectionStringCase {
    name: String,
    input: String,
    expected: Option<Value>,
    expected_error: Option<bool>,
}

fn load_cases() -> IrcCases {
    let data = fs::read_to_string("../tests/irc_cases.json").expect("load irc cases");
    serde_json::from_str(&data).expect("parse irc cases")
}

#[test]
fn test_irc_parse_connection_string() {
    let cases = load_cases();
    for tc in &cases.parse_connection_string {
        if tc.expected_error.unwrap_or(false) {
            let result = parse_connection_string(&tc.input);
            assert!(result.is_err(), "case {}: expected error but got success", tc.name);
        } else {
            let result = parse_connection_string(&tc.input);
            assert!(result.is_ok(), "case {}: unexpected error: {:?}", tc.name, result);

            let params = result.unwrap();
            let expected = tc.expected.as_ref().unwrap();

            if let Some(nick) = expected.get("nick").and_then(|v| v.as_str()) {
                assert_eq!(params.nick, nick, "case {}: nick mismatch", tc.name);
            }
            if let Some(password) = expected.get("password").and_then(|v| v.as_str()) {
                assert_eq!(params.password, password, "case {}: password mismatch", tc.name);
            }
            if let Some(server) = expected.get("server").and_then(|v| v.as_str()) {
                assert_eq!(params.server, server, "case {}: server mismatch", tc.name);
            }
            if let Some(port) = expected.get("port").and_then(|v| v.as_u64()) {
                assert_eq!(params.port, port as u16, "case {}: port mismatch", tc.name);
            }
            if let Some(channel) = expected.get("channel").and_then(|v| v.as_str()) {
                assert_eq!(params.channel, channel, "case {}: channel mismatch", tc.name);
            }
        }
    }
}

#[tokio::test]
async fn test_irc_send_message() {
    let conn_str = "wgrok-bot:pass@irc.libera.chat:6697/#wgrok";
    let result = wgrok::irc_send_message(conn_str, "#target", "test message").await;

    assert!(result.is_ok());
    let response = result.unwrap();
    assert_eq!(response["ok"], true);
    assert_eq!(response["message"], "test message");
}

#[tokio::test]
async fn test_irc_send_card() {
    let conn_str = "wgrok-bot:pass@irc.libera.chat:6697/#wgrok";
    let card = serde_json::json!({"type": "section"});
    let result = wgrok::irc_send_card(conn_str, "#target", "test message", &card).await;

    assert!(result.is_ok());
    let response = result.unwrap();
    // IRC should just send the message, cards are not supported
    assert_eq!(response["ok"], true);
    assert_eq!(response["message"], "test message");
}

#[tokio::test]
async fn test_irc_invalid_connection_string() {
    let conn_str = "invalid-no-at-sign";
    let result = wgrok::irc_send_message(conn_str, "#target", "test").await;

    assert!(result.is_err());
    assert!(result.unwrap_err().contains("@"));
}
