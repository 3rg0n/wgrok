pub const ECHO_PREFIX: &str = "./echo:";

/// Format a v2 echo message: ./echo:{to}:{from}:{flags}:{payload}
pub fn format_echo(to: &str, from: &str, flags: &str, payload: &str) -> String {
    format!(
        "{}{}:{}:{}:{}",
        ECHO_PREFIX, to, from, flags, payload
    )
}

/// Parse a v2 echo message: ./echo:{to}:{from}:{flags}:{payload}
/// Returns (to, from, flags, payload)
pub fn parse_echo(text: &str) -> Result<(String, String, String, String), String> {
    if !is_echo(text) {
        return Err(format!("not an echo message: \"{}\"", text));
    }
    let stripped = &text[ECHO_PREFIX.len()..];
    let parts: Vec<&str> = stripped.splitn(4, ':').collect();
    if parts.len() < 4 {
        return Err(format!("malformed echo message: \"{}\"", text));
    }
    let to = parts[0];
    if to.is_empty() {
        return Err(format!("empty to in echo message: \"{}\"", text));
    }
    Ok((
        to.to_string(),
        parts[1].to_string(),
        parts[2].to_string(),
        parts[3].to_string(),
    ))
}

pub fn is_echo(text: &str) -> bool {
    text.starts_with(ECHO_PREFIX)
}

/// Format a v2 response message: {to}:{from}:{flags}:{payload}
pub fn format_response(to: &str, from: &str, flags: &str, payload: &str) -> String {
    format!("{}:{}:{}:{}", to, from, flags, payload)
}

/// Parse a v2 response message: {to}:{from}:{flags}:{payload}
/// Returns (to, from, flags, payload)
pub fn parse_response(text: &str) -> Result<(String, String, String, String), String> {
    let parts: Vec<&str> = text.splitn(4, ':').collect();
    if parts.len() < 4 {
        return Err(format!("malformed response message: \"{}\"", text));
    }
    let to = parts[0];
    if to.is_empty() {
        return Err(format!("empty to in response message: \"{}\"", text));
    }
    Ok((
        to.to_string(),
        parts[1].to_string(),
        parts[2].to_string(),
        parts[3].to_string(),
    ))
}

/// Parse flags string into components: (compressed, chunk_seq, chunk_total)
/// Format: "-" = no flags, "z" = compressed, "1/3" = chunk 1 of 3, "z2/5" = compressed chunk 2 of 5
pub fn parse_flags(flags: &str) -> (bool, Option<usize>, Option<usize>) {
    let compressed = flags.contains('z');

    // Find the chunk portion (e.g., "2/5" or "1/3")
    let chunk_part = if compressed {
        flags.strip_prefix('z').unwrap_or("")
    } else {
        flags
    };

    // Parse chunk_seq/chunk_total
    let (chunk_seq, chunk_total) = if let Some(slash_pos) = chunk_part.find('/') {
        let seq_str = &chunk_part[..slash_pos];
        let total_str = &chunk_part[slash_pos + 1..];

        let seq = seq_str.parse::<usize>().ok();
        let total = total_str.parse::<usize>().ok();
        (seq, total)
    } else {
        (None, None)
    };

    (compressed, chunk_seq, chunk_total)
}

/// Format flags: (compressed, chunk_seq, chunk_total) -> string
/// Returns "-", "z", "1/3", "z2/5", etc.
pub fn format_flags(
    compressed: bool,
    chunk_seq: Option<usize>,
    chunk_total: Option<usize>,
) -> String {
    match (compressed, chunk_seq, chunk_total) {
        (false, None, None) => "-".to_string(),
        (true, None, None) => "z".to_string(),
        (false, Some(seq), Some(total)) => format!("{}/{}", seq, total),
        (true, Some(seq), Some(total)) => format!("z{}/{}", seq, total),
        _ => "-".to_string(), // Default for invalid combinations
    }
}
