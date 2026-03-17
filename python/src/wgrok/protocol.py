"""Message protocol for wgrok: format, parse, and validate echo/response messages.

v2.0 wire format: ./echo:{to}:{from}:{flags}:{payload}
- to: destination slug/agent ID
- from: sender identifier (or "-" for anonymous)
- flags: compression/chunking flags ("-", "z", "1/3", "z2/5", etc)
- payload: message body (can contain colons)
"""

ECHO_PREFIX = "./echo:"


def format_echo(to: str, from_slug: str, flags: str, payload: str) -> str:
    """Format an outgoing echo message: ./echo:{to}:{from}:{flags}:{payload}"""
    return f"{ECHO_PREFIX}{to}:{from_slug}:{flags}:{payload}"


def parse_echo(text: str) -> tuple[str, str, str, str]:
    """Parse an echo message, returning (to, from_slug, flags, payload).

    Raises ValueError if not an echo message or if 'to' is empty.
    """
    if not is_echo(text):
        raise ValueError(f"Not an echo message: {text!r}")
    stripped = text[len(ECHO_PREFIX) :]
    parts = stripped.split(":", 3)
    if len(parts) < 4:
        raise ValueError(f"Incomplete echo message format: {text!r}")
    to, from_slug, flags, payload = parts
    if not to:
        raise ValueError(f"Empty to field in echo message: {text!r}")
    return to, from_slug, flags, payload


def is_echo(text: str) -> bool:
    """Check if text is an echo-formatted message."""
    return text.startswith(ECHO_PREFIX)


def format_response(to: str, from_slug: str, flags: str, payload: str) -> str:
    """Format a response message: {to}:{from}:{flags}:{payload}"""
    return f"{to}:{from_slug}:{flags}:{payload}"


def parse_response(text: str) -> tuple[str, str, str, str]:
    """Parse a response message, returning (to, from_slug, flags, payload).

    Raises ValueError if 'to' is empty or not enough fields.
    """
    parts = text.split(":", 3)
    if len(parts) < 4:
        raise ValueError(f"Incomplete response format: {text!r}")
    to, from_slug, flags, payload = parts
    if not to:
        raise ValueError(f"Empty to field in response message: {text!r}")
    return to, from_slug, flags, payload


def parse_flags(flags: str) -> tuple[bool, int | None, int | None]:
    """Parse flags string, returning (compressed, chunk_seq, chunk_total).

    Flag formats:
    - "-": (False, None, None)
    - "z": (True, None, None)
    - "1/3": (False, 1, 3)
    - "z2/5": (True, 2, 5)
    """
    compressed = False
    chunk_seq = None
    chunk_total = None

    if flags.startswith("z"):
        compressed = True
        flags = flags[1:]

    if "/" in flags:
        parts = flags.split("/")
        if len(parts) == 2:
            try:
                chunk_seq = int(parts[0])
                chunk_total = int(parts[1])
            except ValueError:
                pass

    return compressed, chunk_seq, chunk_total


def format_flags(compressed: bool, chunk_seq: int | None = None, chunk_total: int | None = None) -> str:
    """Build flags string from components.

    Returns:
    - "-": if not compressed and no chunking
    - "z": if compressed and no chunking
    - "1/3": if chunking (seq 1 of 3)
    - "z2/5": if compressed and chunking
    """
    result = ""
    if compressed:
        result = "z"

    if chunk_seq is not None and chunk_total is not None:
        result += f"{chunk_seq}/{chunk_total}"

    return result if result else "-"
