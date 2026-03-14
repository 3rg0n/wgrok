pub const ECHO_PREFIX: &str = "./echo:";

pub fn format_echo(slug: &str, payload: &str) -> String {
    format!("{}{}{}{}", ECHO_PREFIX, slug, ":", payload)
}

pub fn parse_echo(text: &str) -> Result<(String, String), String> {
    if !is_echo(text) {
        return Err(format!("Not an echo message: \"{}\"", text));
    }
    let stripped = &text[ECHO_PREFIX.len()..];
    let (slug, payload) = match stripped.find(':') {
        Some(idx) => (&stripped[..idx], &stripped[idx + 1..]),
        None => (stripped, ""),
    };
    if slug.is_empty() {
        return Err(format!("Empty slug in echo message: \"{}\"", text));
    }
    Ok((slug.to_string(), payload.to_string()))
}

pub fn is_echo(text: &str) -> bool {
    text.starts_with(ECHO_PREFIX)
}

pub fn format_response(slug: &str, payload: &str) -> String {
    format!("{}:{}", slug, payload)
}

pub fn parse_response(text: &str) -> Result<(String, String), String> {
    let (slug, payload) = match text.find(':') {
        Some(idx) => (&text[..idx], &text[idx + 1..]),
        None => (text, ""),
    };
    if slug.is_empty() {
        return Err("Empty slug in response message".to_string());
    }
    Ok((slug.to_string(), payload.to_string()))
}
