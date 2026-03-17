use base64::Engine;
use base64::engine::general_purpose::STANDARD as BASE64;
use flate2::Compression;
use flate2::read::GzDecoder;
use flate2::write::GzEncoder;
use std::io::{Read, Write};
use aes_gcm::{Aes256Gcm, KeyInit, Nonce};
use aes_gcm::aead::Aead;
use rand::RngCore;

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

/// Encrypt data using AES-256-GCM: returns base64(nonce || ciphertext || tag)
/// Key must be 32 bytes (256 bits). Nonce is randomly generated (12 bytes).
pub fn encrypt(data: &str, key: &[u8]) -> Result<String, String> {
    if key.len() != 32 {
        return Err(format!("encryption key must be 32 bytes, got {}", key.len()));
    }

    let cipher = Aes256Gcm::new_from_slice(key)
        .map_err(|_| "failed to create cipher from key".to_string())?;

    // Generate random 12-byte nonce
    let mut nonce_bytes = [0u8; 12];
    rand::thread_rng().fill_bytes(&mut nonce_bytes);
    #[allow(deprecated)]
    let nonce = Nonce::from_slice(&nonce_bytes);

    // Encrypt data
    let ciphertext = cipher
        .encrypt(nonce, data.as_bytes())
        .map_err(|_| "encryption failed".to_string())?;

    // Combine nonce || ciphertext (includes 16-byte GCM tag)
    let mut result = nonce_bytes.to_vec();
    result.extend_from_slice(&ciphertext);

    // Base64 encode
    Ok(BASE64.encode(&result))
}

/// Decrypt data using AES-256-GCM: expects base64(nonce || ciphertext || tag)
/// Key must be 32 bytes (256 bits).
pub fn decrypt(data: &str, key: &[u8]) -> Result<String, String> {
    if key.len() != 32 {
        return Err(format!("decryption key must be 32 bytes, got {}", key.len()));
    }

    let cipher = Aes256Gcm::new_from_slice(key)
        .map_err(|_| "failed to create cipher from key".to_string())?;

    // Base64 decode
    let encrypted_bytes = BASE64
        .decode(data)
        .map_err(|e| format!("base64 decode: {}", e))?;

    // Must be at least 12 (nonce) + 16 (GCM tag) = 28 bytes
    if encrypted_bytes.len() < 28 {
        return Err(format!(
            "encrypted data too short: {} bytes (minimum 28)",
            encrypted_bytes.len()
        ));
    }

    // Split: first 12 bytes = nonce, rest = ciphertext + tag
    #[allow(deprecated)]
    let nonce = Nonce::from_slice(&encrypted_bytes[0..12]);
    let ciphertext = &encrypted_bytes[12..];

    // Decrypt and verify
    let plaintext = cipher
        .decrypt(nonce, ciphertext)
        .map_err(|_| "decryption failed (invalid tag or corrupted data)".to_string())?;

    // Convert to string
    String::from_utf8(plaintext)
        .map_err(|e| format!("invalid UTF-8 in decrypted data: {}", e))
}
