"""Payload codec: gzip+base64 compression and chunking for large payloads.

Pure data functions with no markers or prefixes.
"""

from __future__ import annotations

import base64
import gzip
import os


def compress(data: str) -> str:
    """Gzip compress and base64 encode. Returns raw base64 string (no prefix)."""
    compressed = gzip.compress(data.encode("utf-8"))
    return base64.b64encode(compressed).decode("ascii")


def decompress(data: str) -> str:
    """Base64 decode and gunzip. Input is raw base64 string."""
    compressed = base64.b64decode(data)
    return gzip.decompress(compressed).decode("utf-8")


def chunk(payload: str, max_size: int) -> list[str]:
    """Split payload into chunks of at most max_size characters each.

    Returns raw chunk strings (no N/T: prefix).
    """
    if max_size <= 0:
        raise ValueError("max_size must be positive")
    total = (len(payload) + max_size - 1) // max_size
    if total == 0:
        total = 1
    chunks = []
    for i in range(total):
        start = i * max_size
        end = start + max_size
        chunk_data = payload[start:end]
        chunks.append(chunk_data)
    return chunks


def encrypt(data: str, key: bytes) -> str:
    """AES-256-GCM encrypt. Returns base64(iv + ciphertext + tag).

    Args:
        data: Plain text string to encrypt.
        key: 32-byte encryption key.

    Returns:
        Base64-encoded encrypted payload (IV || ciphertext || 16-byte tag).
    """
    try:
        from cryptography.hazmat.primitives.ciphers.aead import AESGCM
    except ImportError as e:
        raise ImportError(
            "encryption requires the 'cryptography' package. Install it with: pip install 'wgrok[crypto]'"
        ) from e

    if len(key) != 32:
        raise ValueError(f"encryption key must be 32 bytes, got {len(key)}")

    aesgcm = AESGCM(key)
    iv = os.urandom(12)
    ct = aesgcm.encrypt(iv, data.encode("utf-8"), None)
    return base64.b64encode(iv + ct).decode("ascii")


def decrypt(data: str, key: bytes) -> str:
    """AES-256-GCM decrypt. Input is base64(iv + ciphertext + tag).

    Args:
        data: Base64-encoded encrypted payload.
        key: 32-byte decryption key.

    Returns:
        Decrypted plain text string.
    """
    try:
        from cryptography.hazmat.primitives.ciphers.aead import AESGCM
    except ImportError as e:
        raise ImportError(
            "decryption requires the 'cryptography' package. Install it with: pip install 'wgrok[crypto]'"
        ) from e

    if len(key) != 32:
        raise ValueError(f"decryption key must be 32 bytes, got {len(key)}")

    raw = base64.b64decode(data)
    if len(raw) < 28:
        raise ValueError(f"encrypted payload too short: expected at least 28 bytes (12 IV + 16 tag), got {len(raw)}")
    iv, ct = raw[:12], raw[12:]
    aesgcm = AESGCM(key)
    return aesgcm.decrypt(iv, ct, None).decode("utf-8")
