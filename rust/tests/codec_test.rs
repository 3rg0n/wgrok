use serde::Deserialize;
use std::fs;
use wgrok::codec;

#[derive(Deserialize)]
#[allow(dead_code)]
struct CodecCases {
    roundtrips: Vec<RoundtripCase>,
    encrypt_roundtrips: Vec<RoundtripCase>,
    encrypt_test_key: String,
    chunking: Vec<ChunkingCase>,
}

#[derive(Deserialize)]
struct RoundtripCase {
    input: String,
}

#[derive(Deserialize)]
struct ChunkingCase {
    input: String,
    max_size: usize,
    expected_count: usize,
    expected_chunks: Vec<String>,
    description: String,
}

fn load_cases() -> CodecCases {
    let data = fs::read_to_string("../tests/codec_cases.json").expect("load codec cases");
    serde_json::from_str(&data).expect("parse codec cases")
}

#[test]
fn test_compress_decompress_roundtrip() {
    let cases = load_cases();
    for tc in &cases.roundtrips {
        let compressed = codec::compress(&tc.input).expect("compress");
        let decompressed = codec::decompress(&compressed).expect("decompress");
        assert_eq!(
            decompressed, tc.input,
            "roundtrip failed for input {:?}",
            tc.input
        );
    }
}

#[test]
fn test_codec_chunking() {
    let cases = load_cases();
    for tc in &cases.chunking {
        let chunks = codec::chunk(&tc.input, tc.max_size).expect("chunk");
        assert_eq!(
            chunks.len(),
            tc.expected_count,
            "{}: chunk count mismatch",
            tc.description
        );
        assert_eq!(chunks, tc.expected_chunks, "{}: chunks mismatch", tc.description);
    }
}

#[test]
fn test_encrypt_decrypt_roundtrip() {
    let cases = load_cases();

    // Decode the test key from base64
    use base64::Engine;
    use base64::engine::general_purpose::STANDARD as BASE64;

    let key_bytes = BASE64.decode(&cases.encrypt_test_key).expect("decode test key");
    assert_eq!(
        key_bytes.len(),
        32,
        "test key must be 32 bytes after decoding"
    );

    for tc in &cases.encrypt_roundtrips {
        let encrypted = codec::encrypt(&tc.input, &key_bytes).expect("encrypt");
        let decrypted = codec::decrypt(&encrypted, &key_bytes).expect("decrypt");
        assert_eq!(
            decrypted, tc.input,
            "roundtrip failed for input {:?}",
            tc.input
        );
    }
}

#[test]
fn test_encrypt_wrong_key_fails() {
    let cases = load_cases();

    use base64::Engine;
    use base64::engine::general_purpose::STANDARD as BASE64;

    let key_bytes = BASE64.decode(&cases.encrypt_test_key).expect("decode test key");
    let wrong_key = {
        let mut wrong = key_bytes.clone();
        wrong[0] = wrong[0].wrapping_add(1);
        wrong
    };

    let input = "test data";
    let encrypted = codec::encrypt(input, &key_bytes).expect("encrypt");
    let result = codec::decrypt(&encrypted, &wrong_key);
    assert!(result.is_err(), "decryption with wrong key should fail");
}
