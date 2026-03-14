use serde::Deserialize;
use serde_json::Value;
use std::fs;
use wgrok::extract_cards;

#[derive(Deserialize)]
struct WebexCases {
    extract_cards: Vec<ExtractCardsCase>,
}

#[derive(Deserialize)]
struct ExtractCardsCase {
    name: String,
    message: Value,
    expected: Vec<Value>,
}

fn load_cases() -> WebexCases {
    let data = fs::read_to_string("../tests/webex_cases.json").expect("load webex cases");
    serde_json::from_str(&data).expect("parse webex cases")
}

#[test]
fn test_extract_cards() {
    let cases = load_cases();
    for tc in &cases.extract_cards {
        let result = extract_cards(&tc.message);
        assert_eq!(result, tc.expected, "case: {}", tc.name);
    }
}
