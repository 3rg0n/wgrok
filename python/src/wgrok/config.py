"""Configuration dataclasses for wgrok components, loaded from environment variables."""

from __future__ import annotations

import os
from dataclasses import dataclass

from dotenv import load_dotenv


def _load_env() -> None:
    load_dotenv()


def _require(name: str) -> str:
    val = os.environ.get(name)
    if not val:
        raise ValueError(f"Required environment variable {name} is not set")
    return val


def _parse_domains(raw: str) -> list[str]:
    return [d.strip() for d in raw.split(",") if d.strip()]


def _parse_debug(raw: str | None) -> bool:
    return raw is not None and raw.strip().lower() in ("true", "1", "yes")


@dataclass
class SenderConfig:
    webex_token: str
    target: str
    slug: str
    domains: list[str]
    debug: bool = False

    @classmethod
    def from_env(cls) -> SenderConfig:
        _load_env()
        return cls(
            webex_token=_require("WGROK_TOKEN"),
            target=_require("WGROK_TARGET"),
            slug=_require("WGROK_SLUG"),
            domains=_parse_domains(os.environ.get("WGROK_DOMAINS", "")),
            debug=_parse_debug(os.environ.get("WGROK_DEBUG")),
        )


@dataclass
class BotConfig:
    webex_token: str
    domains: list[str]
    debug: bool = False

    @classmethod
    def from_env(cls) -> BotConfig:
        _load_env()
        return cls(
            webex_token=_require("WGROK_TOKEN"),
            domains=_parse_domains(_require("WGROK_DOMAINS")),
            debug=_parse_debug(os.environ.get("WGROK_DEBUG")),
        )


@dataclass
class ReceiverConfig:
    webex_token: str
    slug: str
    domains: list[str]
    debug: bool = False

    @classmethod
    def from_env(cls) -> ReceiverConfig:
        _load_env()
        return cls(
            webex_token=_require("WGROK_TOKEN"),
            slug=_require("WGROK_SLUG"),
            domains=_parse_domains(_require("WGROK_DOMAINS")),
            debug=_parse_debug(os.environ.get("WGROK_DEBUG")),
        )
