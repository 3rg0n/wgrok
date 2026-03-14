use serde::Deserialize;
use std::fs;
use wgrok::Allowlist;

#[derive(Deserialize)]
struct AllowlistCases {
    cases: Vec<Case>,
}

#[derive(Deserialize)]
struct Case {
    name: String,
    patterns: Vec<String>,
    email: String,
    expected: bool,
}

fn load_cases() -> AllowlistCases {
    let data = fs::read_to_string("../tests/allowlist_cases.json").expect("load allowlist cases");
    serde_json::from_str(&data).expect("parse allowlist cases")
}

#[test]
fn test_allowlist() {
    let cases = load_cases();
    for tc in &cases.cases {
        let al = Allowlist::new(&tc.patterns);
        assert_eq!(
            al.is_allowed(&tc.email),
            tc.expected,
            "case '{}': is_allowed(\"{}\") with patterns {:?}",
            tc.name,
            tc.email,
            tc.patterns
        );
    }
}
