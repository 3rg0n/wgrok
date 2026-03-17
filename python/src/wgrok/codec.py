"""Payload codec: gzip+base64 compression and chunking for large payloads.

Pure data functions with no markers or prefixes.
"""

from __future__ import annotations

import base64
import gzip


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
