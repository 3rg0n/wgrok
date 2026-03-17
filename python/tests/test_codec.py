"""Tests for wgrok.codec - driven by shared test cases."""

import json
from pathlib import Path

import pytest

from wgrok.codec import chunk, compress, decompress

CASES = json.loads((Path(__file__).resolve().parents[2] / "tests" / "codec_cases.json").read_text())


class TestCompressDecompress:
    @pytest.mark.parametrize("tc", CASES["roundtrips"], ids=lambda tc: repr(tc["input"][:30]))
    def test_roundtrip(self, tc):
        compressed_data = compress(tc["input"])
        assert isinstance(compressed_data, str)
        assert decompress(compressed_data) == tc["input"]


class TestChunking:
    @pytest.mark.parametrize("tc", CASES["chunking"], ids=lambda tc: tc["description"])
    def test_chunk(self, tc):
        chunks = chunk(tc["input"], tc["max_size"])
        assert len(chunks) == tc["expected_count"]
        assert chunks == tc["expected_chunks"]
