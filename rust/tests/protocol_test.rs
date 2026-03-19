use serde::Deserialize;
use std::fs;
use wgrok::protocol::*;

#[derive(Deserialize)]
struct ProtocolCases {
    echo_prefix: String,
    format_echo: Vec<FormatEchoCase>,
    parse_echo: ParseCases,
    is_echo: Vec<IsEchoCase>,
    format_response: Vec<FormatResponseCase>,
    parse_response: ParseCases,
    parse_flags: Vec<ParseFlagsCase>,
    format_flags: Vec<FormatFlagsCase>,
    is_pause: Vec<IsPauseCase>,
    is_resume: Vec<IsResumeCase>,
    roundtrips: Roundtrips,
}

#[derive(Deserialize)]
struct FormatEchoCase {
    to: String,
    from: String,
    flags: String,
    payload: String,
    expected: String,
}

#[derive(Deserialize)]
struct FormatResponseCase {
    to: String,
    from: String,
    flags: String,
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
    to: String,
    from: String,
    flags: String,
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
struct IsPauseCase {
    input: String,
    expected: bool,
}

#[derive(Deserialize)]
struct IsResumeCase {
    input: String,
    expected: bool,
}

#[derive(Deserialize)]
struct ParseFlagsCase {
    input: String,
    compressed: bool,
    #[serde(default)]
    encrypted: bool,
    chunk_seq: Option<usize>,
    chunk_total: Option<usize>,
}

#[derive(Deserialize)]
struct FormatFlagsCase {
    compressed: bool,
    #[serde(default)]
    encrypted: bool,
    chunk_seq: Option<usize>,
    chunk_total: Option<usize>,
    expected: String,
}

#[derive(Deserialize)]
struct Roundtrips {
    echo: Vec<RoundtripCase>,
    response: Vec<RoundtripCase>,
}

#[derive(Deserialize)]
struct RoundtripCase {
    to: String,
    from: String,
    flags: String,
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
        assert_eq!(
            format_echo(&tc.to, &tc.from, &tc.flags, &tc.payload),
            tc.expected
        );
    }
}

#[test]
fn test_parse_echo_valid() {
    let cases = load_cases();
    for tc in &cases.parse_echo.valid {
        let (to, from, flags, payload) = parse_echo(&tc.input).unwrap();
        assert_eq!(to, tc.to);
        assert_eq!(from, tc.from);
        assert_eq!(flags, tc.flags);
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
        assert_eq!(
            format_response(&tc.to, &tc.from, &tc.flags, &tc.payload),
            tc.expected
        );
    }
}

#[test]
fn test_parse_response_valid() {
    let cases = load_cases();
    for tc in &cases.parse_response.valid {
        let (to, from, flags, payload) = parse_response(&tc.input).unwrap();
        assert_eq!(to, tc.to);
        assert_eq!(from, tc.from);
        assert_eq!(flags, tc.flags);
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
fn test_parse_flags() {
    let cases = load_cases();
    for tc in &cases.parse_flags {
        let (compressed, encrypted, chunk_seq, chunk_total) = parse_flags(&tc.input);
        assert_eq!(compressed, tc.compressed, "compressed mismatch for {:?}", tc.input);
        assert_eq!(encrypted, tc.encrypted, "encrypted mismatch for {:?}", tc.input);
        assert_eq!(chunk_seq, tc.chunk_seq, "chunk_seq mismatch for {:?}", tc.input);
        assert_eq!(chunk_total, tc.chunk_total, "chunk_total mismatch for {:?}", tc.input);
    }
}

#[test]
fn test_format_flags() {
    let cases = load_cases();
    for tc in &cases.format_flags {
        assert_eq!(
            format_flags(tc.compressed, tc.encrypted, tc.chunk_seq, tc.chunk_total),
            tc.expected
        );
    }
}

#[test]
fn test_echo_roundtrip() {
    let cases = load_cases();
    for tc in &cases.roundtrips.echo {
        let text = format_echo(&tc.to, &tc.from, &tc.flags, &tc.payload);
        let (to, from, flags, payload) = parse_echo(&text).unwrap();
        assert_eq!(to, tc.to);
        assert_eq!(from, tc.from);
        assert_eq!(flags, tc.flags);
        assert_eq!(payload, tc.payload);
    }
}

#[test]
fn test_response_roundtrip() {
    let cases = load_cases();
    for tc in &cases.roundtrips.response {
        let text = format_response(&tc.to, &tc.from, &tc.flags, &tc.payload);
        let (to, from, flags, payload) = parse_response(&text).unwrap();
        assert_eq!(to, tc.to);
        assert_eq!(from, tc.from);
        assert_eq!(flags, tc.flags);
        assert_eq!(payload, tc.payload);
    }
}

#[test]
fn test_is_pause() {
    let cases = load_cases();
    for tc in &cases.is_pause {
        assert_eq!(is_pause(&tc.input), tc.expected, "is_pause(\"{}\")", tc.input);
    }
}

#[test]
fn test_is_resume() {
    let cases = load_cases();
    for tc in &cases.is_resume {
        assert_eq!(is_resume(&tc.input), tc.expected, "is_resume(\"{}\")", tc.input);
    }
}
