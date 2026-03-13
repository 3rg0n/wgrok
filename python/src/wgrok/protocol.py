"""Message protocol for wgrok: format, parse, and validate echo/response messages."""

ECHO_PREFIX = "./echo:"


def format_echo(slug: str, payload: str) -> str:
    """Format an outgoing echo message: ./echo:{slug}:{payload}"""
    return f"{ECHO_PREFIX}{slug}:{payload}"


def parse_echo(text: str) -> tuple[str, str]:
    """Parse an echo message, returning (slug, payload). Raises ValueError if not an echo message."""
    if not is_echo(text):
        raise ValueError(f"Not an echo message: {text!r}")
    stripped = text[len(ECHO_PREFIX) :]
    slug, _, payload = stripped.partition(":")
    if not slug:
        raise ValueError(f"Empty slug in echo message: {text!r}")
    return slug, payload


def is_echo(text: str) -> bool:
    """Check if text is an echo-formatted message."""
    return text.startswith(ECHO_PREFIX)


def format_response(slug: str, payload: str) -> str:
    """Format a response message: {slug}:{payload}"""
    return f"{slug}:{payload}"


def parse_response(text: str) -> tuple[str, str]:
    """Parse a response message, returning (slug, payload). Raises ValueError if no slug found."""
    slug, _, payload = text.partition(":")
    if not slug:
        raise ValueError(f"Empty slug in response message: {text!r}")
    return slug, payload
