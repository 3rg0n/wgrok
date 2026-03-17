"""Tests for wgrok.codec - driven by shared test cases."""

import base64
import json
from pathlib import Path

import pytest

from wgrok.codec import chunk, compress, decompress, decrypt, encrypt

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


class TestEncryptDecrypt:
    @pytest.fixture
    def test_key(self):
        """Test key from codec_cases.json."""
        return base64.b64decode(CASES["encrypt_test_key"])

    @pytest.mark.parametrize("tc", CASES["encrypt_roundtrips"], ids=lambda tc: repr(tc["input"][:30]))
    def test_roundtrip(self, tc, test_key):
        encrypted = encrypt(tc["input"], test_key)
        assert isinstance(encrypted, str)
        decrypted = decrypt(encrypted, test_key)
        assert decrypted == tc["input"]

    def test_wrong_key_fails(self, test_key):
        """Decryption with wrong key should fail."""
        from cryptography.exceptions import InvalidTag

        original = "secret message"
        encrypted = encrypt(original, test_key)
        wrong_key = base64.b64decode("d3JvbmdrZXkwMTIzNDU2Nzg5MDEyMzQ1Njc4OTAxMjM=")
        with pytest.raises(InvalidTag):
            decrypt(encrypted, wrong_key)

    def test_invalid_key_length(self):
        """Encryption with invalid key length should fail."""
        short_key = b"tooshort"
        with pytest.raises(ValueError, match="32 bytes"):
            encrypt("message", short_key)

    def test_no_cryptography_import_error(self, monkeypatch):
        """Test that missing cryptography raises helpful error."""

        # Block cryptography import
        def block_import(name, *args, **kwargs):
            if "cryptography" in name:
                raise ImportError("No module named 'cryptography'")
            return __import__(name, *args, **kwargs)

        monkeypatch.setattr("builtins.__import__", block_import)
        test_key = base64.b64decode(CASES["encrypt_test_key"])
        with pytest.raises(ImportError, match="cryptography.*pip install.*crypto"):
            encrypt("message", test_key)
