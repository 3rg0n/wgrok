"""Configuration dataclasses for wgrok components, loaded from environment variables."""

from __future__ import annotations

import os
from dataclasses import dataclass, field

from dotenv import load_dotenv

VALID_PLATFORMS = ("webex", "slack", "discord", "irc")
PLATFORM_TOKEN_VARS = {
    "webex": "WGROK_WEBEX_TOKENS",
    "slack": "WGROK_SLACK_TOKENS",
    "discord": "WGROK_DISCORD_TOKENS",
    "irc": "WGROK_IRC_TOKENS",
}


def _load_env(env_file: str | None = None) -> None:
    load_dotenv(env_file, override=True)


def _require(name: str) -> str:
    val = os.environ.get(name)
    if not val:
        raise ValueError(f"Required environment variable {name} is not set")
    return val


def _parse_domains(raw: str) -> list[str]:
    return [d.strip() for d in raw.split(",") if d.strip()]


def _parse_debug(raw: str | None) -> bool:
    return raw is not None and raw.strip().lower() in ("true", "1", "yes")


def _parse_platform(raw: str | None) -> str:
    if raw is None or not raw.strip():
        return "webex"
    val = raw.strip().lower()
    if val not in VALID_PLATFORMS:
        raise ValueError(f"Invalid platform {val!r}, must be one of {VALID_PLATFORMS}")
    return val


def _parse_routes(raw: str | None) -> dict[str, str]:
    if not raw or not raw.strip():
        return {}
    routes = {}
    for entry in raw.split(","):
        entry = entry.strip()
        if not entry:
            continue
        slug, _, target = entry.partition(":")
        if slug and target:
            routes[slug.strip()] = target.strip()
    return routes


def _parse_int(raw: str | None) -> int | None:
    if raw is None or not raw.strip():
        return None
    return int(raw.strip())


def _parse_encrypt_key(raw: str | None) -> bytes | None:
    """Parse base64-encoded encryption key. Must be exactly 32 bytes when decoded."""
    if raw is None or not raw.strip():
        return None
    try:
        import base64

        key = base64.b64decode(raw.strip())
        if len(key) != 32:
            raise ValueError(f"encryption key must be 32 bytes when decoded, got {len(key)}")
        return key
    except Exception as e:
        raise ValueError(f"Invalid encryption key format: {e}") from e


def _parse_platform_tokens(env: dict[str, str] | None = None) -> dict[str, list[str]]:
    """Parse platform-specific token env vars into a dict of platform -> token list."""
    get = (env or os.environ).get
    tokens: dict[str, list[str]] = {}
    for platform, var_name in PLATFORM_TOKEN_VARS.items():
        raw = get(var_name)
        if raw and raw.strip():
            token_list = [t.strip() for t in raw.split(",") if t.strip()]
            if token_list:
                tokens[platform] = token_list
    # Fallback: if no platform-specific tokens, use WGROK_TOKEN as webex
    if not tokens:
        single = get("WGROK_TOKEN")
        if single and single.strip():
            tokens["webex"] = [single.strip()]
    return tokens


@dataclass
class SenderConfig:
    webex_token: str
    target: str
    slug: str
    domains: list[str]
    debug: bool = False
    platform: str = "webex"
    encrypt_key: bytes | None = None

    @classmethod
    def from_env(cls, env_file: str | None = None) -> SenderConfig:
        _load_env(env_file)
        return cls(
            webex_token=_require("WGROK_TOKEN"),
            target=_require("WGROK_TARGET"),
            slug=_require("WGROK_SLUG"),
            domains=_parse_domains(os.environ.get("WGROK_DOMAINS", "")),
            debug=_parse_debug(os.environ.get("WGROK_DEBUG")),
            platform=_parse_platform(os.environ.get("WGROK_PLATFORM")),
            encrypt_key=_parse_encrypt_key(os.environ.get("WGROK_ENCRYPT_KEY")),
        )


@dataclass
class BotConfig:
    webex_token: str
    domains: list[str]
    debug: bool = False
    routes: dict[str, str] = field(default_factory=dict)
    platform_tokens: dict[str, list[str]] = field(default_factory=dict)
    webhook_port: int | None = None
    webhook_secret: str | None = None

    @classmethod
    def from_env(cls, env_file: str | None = None) -> BotConfig:
        _load_env(env_file)
        platform_tokens = _parse_platform_tokens()
        # webex_token: use WGROK_TOKEN if set, else first webex token from platform tokens
        single_token = os.environ.get("WGROK_TOKEN", "")
        if not single_token and "webex" in platform_tokens:
            single_token = platform_tokens["webex"][0]
        elif not single_token and platform_tokens:
            # Use first token from any platform
            first_platform = next(iter(platform_tokens))
            single_token = platform_tokens[first_platform][0]
        return cls(
            webex_token=single_token,
            domains=_parse_domains(_require("WGROK_DOMAINS")),
            debug=_parse_debug(os.environ.get("WGROK_DEBUG")),
            routes=_parse_routes(os.environ.get("WGROK_ROUTES")),
            platform_tokens=platform_tokens,
            webhook_port=_parse_int(os.environ.get("WGROK_WEBHOOK_PORT")),
            webhook_secret=os.environ.get("WGROK_WEBHOOK_SECRET") or None,
        )


@dataclass
class ReceiverConfig:
    webex_token: str
    slug: str
    domains: list[str]
    debug: bool = False
    platform: str = "webex"
    encrypt_key: bytes | None = None

    @classmethod
    def from_env(cls, env_file: str | None = None) -> ReceiverConfig:
        _load_env(env_file)
        return cls(
            webex_token=_require("WGROK_TOKEN"),
            slug=_require("WGROK_SLUG"),
            domains=_parse_domains(_require("WGROK_DOMAINS")),
            debug=_parse_debug(os.environ.get("WGROK_DEBUG")),
            platform=_parse_platform(os.environ.get("WGROK_PLATFORM")),
            encrypt_key=_parse_encrypt_key(os.environ.get("WGROK_ENCRYPT_KEY")),
        )
