use base64::Engine;
use base64::engine::general_purpose::STANDARD as BASE64;
use flate2::Compression;
use flate2::read::GzDecoder;
use flate2::write::GzEncoder;
use std::io::{Read, Write};

/// Compress data: gzip + base64 (NO prefix)
pub fn compress(data: &str) -> Result<String, String> {
    let mut encoder = GzEncoder::new(Vec::new(), Compression::default());
    encoder
        .write_all(data.as_bytes())
        .map_err(|e| format!("gzip write: {}", e))?;
    let compressed = encoder
        .finish()
        .map_err(|e| format!("gzip finish: {}", e))?;
    let b64 = BASE64.encode(&compressed);
    Ok(b64)
}

/// Decompress data: base64 + gunzip
pub fn decompress(data: &str) -> Result<String, String> {
    let compressed = BASE64
        .decode(data)
        .map_err(|e| format!("base64 decode: {}", e))?;
    let mut decoder = GzDecoder::new(&compressed[..]);
    let mut out = String::new();
    decoder
        .read_to_string(&mut out)
        .map_err(|e| format!("gzip read: {}", e))?;
    Ok(out)
}

/// Split data into chunks: returns raw strings (NO prefix)
pub fn chunk(data: &str, max_size: usize) -> Result<Vec<String>, String> {
    if max_size == 0 {
        return Err("max_size must be positive".to_string());
    }
    let len = data.len();
    let mut total = len.div_ceil(max_size);
    if total == 0 {
        total = 1;
    }
    let mut chunks = Vec::with_capacity(total);
    for i in 0..total {
        let start = i * max_size;
        let end = std::cmp::min(start + max_size, len);
        let chunk_data = &data[start..end];
        chunks.push(chunk_data.to_string());
    }
    Ok(chunks)
}
