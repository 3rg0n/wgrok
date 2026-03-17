use serde::Deserialize;
use std::fs;
use wgrok::codec;

#[derive(Deserialize)]
#[allow(dead_code)]
struct CodecCases {
    roundtrips: Vec<RoundtripCase>,
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
