pub const ECHO_PREFIX: &str = "./echo:";
pub const PAUSE_CMD: &str = "./pause";
pub const RESUME_CMD: &str = "./resume";

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

/// Parse flags string into components: (compressed, encrypted, chunk_seq, chunk_total)
/// Format: "-" = no flags, "z" = compressed, "e" = encrypted, "ze" = both
/// "1/3" = chunk 1 of 3, "z2/5" = compressed chunk 2 of 5, "e1/3" = encrypted chunk 1 of 3, "ze2/5" = both chunk 2 of 5
pub fn parse_flags(flags: &str) -> (bool, bool, Option<usize>, Option<usize>) {
    let compressed = flags.contains('z');
    let encrypted = flags.contains('e');

    // Strip leading z and e to get chunk portion
    let mut chunk_part = flags;
    if chunk_part.starts_with('z') {
        chunk_part = &chunk_part[1..];
    }
    if chunk_part.starts_with('e') {
        chunk_part = &chunk_part[1..];
    }

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

    (compressed, encrypted, chunk_seq, chunk_total)
}

/// Format flags: (compressed, encrypted, chunk_seq, chunk_total) -> string
/// Returns "-", "z", "e", "ze", "1/3", "z2/5", "e1/3", "ze2/5", etc.
pub fn format_flags(
    compressed: bool,
    encrypted: bool,
    chunk_seq: Option<usize>,
    chunk_total: Option<usize>,
) -> String {
    let mut result = String::new();

    if compressed {
        result.push('z');
    }
    if encrypted {
        result.push('e');
    }

    if let (Some(seq), Some(total)) = (chunk_seq, chunk_total) {
        result.push_str(&format!("{}/{}", seq, total));
    }

    if result.is_empty() {
        "-".to_string()
    } else {
        result
    }
}

/// Check if text is a pause control command
pub fn is_pause(text: &str) -> bool {
    text.trim() == PAUSE_CMD
}

/// Check if text is a resume control command
pub fn is_resume(text: &str) -> bool {
    text.trim() == RESUME_CMD
}

/// Strip bot display name prefix from text using spark-mention tags in HTML.
/// Extracts the display name from <spark-mention>DisplayName</spark-mention>
/// and removes it from the start of the text if present. Loops over ALL spark-mention tags.
pub fn strip_bot_mention(text: &str, html: &str) -> String {
    if html.is_empty() {
        return text.to_string();
    }

    let start_tag = "<spark-mention";
    let end_tag = "</spark-mention>";
    let mut result = text.to_string();
    let mut search_from = 0;

    while let Some(tag_start) = html[search_from..].find(start_tag) {
        let abs_tag_start = search_from + tag_start;
        if let Some(content_start_offset) = html[abs_tag_start..].find('>') {
            let content_start = abs_tag_start + content_start_offset + 1;
            if let Some(end_pos) = html[content_start..].find(end_tag) {
                let display_name = &html[content_start..content_start + end_pos];
                if let Some(stripped) = result.strip_prefix(display_name) {
                    result = stripped.to_string();
                }
                result = result.trim().to_string();
                search_from = content_start + end_pos + end_tag.len();
            } else {
                break;
            }
        } else {
            break;
        }
    }

    result
}
