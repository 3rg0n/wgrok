use serde::Deserialize;
use std::fs;
use wgrok::protocol::*;

#[derive(Deserialize)]
struct ProtocolCases {
    echo_prefix: String,
    format_echo: Vec<FormatCase>,
    parse_echo: ParseCases,
    is_echo: Vec<IsEchoCase>,
    format_response: Vec<FormatCase>,
    parse_response: ParseCases,
    roundtrips: Roundtrips,
}

#[derive(Deserialize)]
struct FormatCase {
    slug: String,
    payload: String,
    expected: String,
}

#[derive(Deserialize)]
struct ParseCases {
    valid: Vec<ParseValid>,
    errors: Vec<ParseError>,
}

#[derive(Deserialize)]
struct ParseValid {
    input: String,
    slug: String,
    payload: String,
}

#[derive(Deserialize)]
struct ParseError {
    input: String,
    error_contains: String,
}

#[derive(Deserialize)]
struct IsEchoCase {
    input: String,
    expected: bool,
}

#[derive(Deserialize)]
struct Roundtrips {
    echo: Vec<RoundtripCase>,
    response: Vec<RoundtripCase>,
}

#[derive(Deserialize)]
struct RoundtripCase {
    slug: String,
    payload: String,
}

fn load_cases() -> ProtocolCases {
    let data = fs::read_to_string("../tests/protocol_cases.json").expect("load protocol cases");
    serde_json::from_str(&data).expect("parse protocol cases")
}

#[test]
fn test_echo_prefix() {
    let cases = load_cases();
    assert_eq!(ECHO_PREFIX, cases.echo_prefix);
}

#[test]
fn test_format_echo() {
    let cases = load_cases();
    for tc in &cases.format_echo {
        assert_eq!(format_echo(&tc.slug, &tc.payload), tc.expected);
    }
}

#[test]
fn test_parse_echo_valid() {
    let cases = load_cases();
    for tc in &cases.parse_echo.valid {
        let (slug, payload) = parse_echo(&tc.input).unwrap();
        assert_eq!(slug, tc.slug);
        assert_eq!(payload, tc.payload);
    }
}

#[test]
fn test_parse_echo_errors() {
    let cases = load_cases();
    for tc in &cases.parse_echo.errors {
        let result = parse_echo(&tc.input);
        assert!(result.is_err());
        let err = result.unwrap_err().to_lowercase();
        assert!(
            err.contains(&tc.error_contains.to_lowercase()),
            "error '{}' should contain '{}'",
            err,
            tc.error_contains
        );
    }
}

#[test]
fn test_is_echo() {
    let cases = load_cases();
    for tc in &cases.is_echo {
        assert_eq!(is_echo(&tc.input), tc.expected, "is_echo(\"{}\")", tc.input);
    }
}

#[test]
fn test_format_response() {
    let cases = load_cases();
    for tc in &cases.format_response {
        assert_eq!(format_response(&tc.slug, &tc.payload), tc.expected);
    }
}

#[test]
fn test_parse_response_valid() {
    let cases = load_cases();
    for tc in &cases.parse_response.valid {
        let (slug, payload) = parse_response(&tc.input).unwrap();
        assert_eq!(slug, tc.slug);
        assert_eq!(payload, tc.payload);
    }
}

#[test]
fn test_parse_response_errors() {
    let cases = load_cases();
    for tc in &cases.parse_response.errors {
        let result = parse_response(&tc.input);
        assert!(result.is_err());
    }
}

#[test]
fn test_echo_roundtrip() {
    let cases = load_cases();
    for tc in &cases.roundtrips.echo {
        let text = format_echo(&tc.slug, &tc.payload);
        let (slug, payload) = parse_echo(&text).unwrap();
        assert_eq!(slug, tc.slug);
        assert_eq!(payload, tc.payload);
    }
}

#[test]
fn test_response_roundtrip() {
    let cases = load_cases();
    for tc in &cases.roundtrips.response {
        let text = format_response(&tc.slug, &tc.payload);
        let (slug, payload) = parse_response(&text).unwrap();
        assert_eq!(slug, tc.slug);
        assert_eq!(payload, tc.payload);
    }
}
