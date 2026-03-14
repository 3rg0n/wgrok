"""Tests for wgrok.config - driven by shared test cases."""

import json
import os
from pathlib import Path

import pytest

from wgrok.config import BotConfig, ReceiverConfig, SenderConfig

CASES = json.loads((Path(__file__).resolve().parents[2] / "tests" / "config_cases.json").read_text())


@pytest.fixture
def _clean_env(monkeypatch):
    """Remove all WGROK_ env vars before each test."""
    for key in list(os.environ):
        if key.startswith("WGROK_"):
            monkeypatch.delenv(key, raising=False)


@pytest.mark.usefixtures("_clean_env")
class TestSenderConfig:
    def test_from_env(self, monkeypatch):
        for k, v in CASES["sender"]["valid"]["env"].items():
            monkeypatch.setenv(k, v)
        cfg = SenderConfig.from_env()
        exp = CASES["sender"]["valid"]["expected"]
        assert cfg.webex_token == exp["webex_token"]
        assert cfg.target == exp["target"]
        assert cfg.slug == exp["slug"]
        assert cfg.domains == exp["domains"]
        assert cfg.debug is exp["debug"]
        assert cfg.platform == exp["platform"]

    def test_missing_token_raises(self, monkeypatch):
        for k, v in CASES["sender"]["missing_token"]["env"].items():
            monkeypatch.setenv(k, v)
        with pytest.raises(ValueError, match=CASES["sender"]["missing_token"]["error_contains"]):
            SenderConfig.from_env()

    def test_missing_target_raises(self, monkeypatch):
        for k, v in CASES["sender"]["missing_target"]["env"].items():
            monkeypatch.setenv(k, v)
        with pytest.raises(ValueError, match=CASES["sender"]["missing_target"]["error_contains"]):
            SenderConfig.from_env()

    def test_debug_defaults_false(self, monkeypatch):
        for k, v in CASES["sender"]["debug_defaults_false"]["env"].items():
            monkeypatch.setenv(k, v)
        cfg = SenderConfig.from_env()
        assert cfg.debug is CASES["sender"]["debug_defaults_false"]["expected_debug"]

    def test_domains_optional(self, monkeypatch):
        for k, v in CASES["sender"]["domains_optional"]["env"].items():
            monkeypatch.setenv(k, v)
        cfg = SenderConfig.from_env()
        assert cfg.domains == CASES["sender"]["domains_optional"]["expected_domains"]

    def test_platform_defaults_webex(self, monkeypatch):
        for k, v in CASES["sender"]["platform_defaults_webex"]["env"].items():
            monkeypatch.setenv(k, v)
        cfg = SenderConfig.from_env()
        assert cfg.platform == CASES["sender"]["platform_defaults_webex"]["expected_platform"]

    def test_platform_explicit(self, monkeypatch):
        for k, v in CASES["sender"]["platform_explicit"]["env"].items():
            monkeypatch.setenv(k, v)
        cfg = SenderConfig.from_env()
        assert cfg.platform == CASES["sender"]["platform_explicit"]["expected_platform"]


@pytest.mark.usefixtures("_clean_env")
class TestBotConfig:
    def test_from_env(self, monkeypatch):
        for k, v in CASES["bot"]["valid"]["env"].items():
            monkeypatch.setenv(k, v)
        cfg = BotConfig.from_env()
        exp = CASES["bot"]["valid"]["expected"]
        assert cfg.webex_token == exp["webex_token"]
        assert cfg.domains == exp["domains"]

    def test_missing_domains_raises(self, monkeypatch):
        for k, v in CASES["bot"]["missing_domains"]["env"].items():
            monkeypatch.setenv(k, v)
        with pytest.raises(ValueError, match=CASES["bot"]["missing_domains"]["error_contains"]):
            BotConfig.from_env()

    def test_with_routes(self, monkeypatch):
        for k, v in CASES["bot"]["with_routes"]["env"].items():
            monkeypatch.setenv(k, v)
        cfg = BotConfig.from_env()
        assert cfg.routes == CASES["bot"]["with_routes"]["expected_routes"]

    def test_routes_empty_when_not_set(self, monkeypatch):
        for k, v in CASES["bot"]["routes_empty_when_not_set"]["env"].items():
            monkeypatch.setenv(k, v)
        cfg = BotConfig.from_env()
        assert cfg.routes == CASES["bot"]["routes_empty_when_not_set"]["expected_routes"]

    def test_with_webhook(self, monkeypatch):
        tc = CASES["bot"]["with_webhook"]
        for k, v in tc["env"].items():
            monkeypatch.setenv(k, v)
        cfg = BotConfig.from_env()
        assert cfg.webhook_port == tc["expected_webhook_port"]
        assert cfg.webhook_secret == tc["expected_webhook_secret"]

    def test_webhook_disabled_by_default(self, monkeypatch):
        tc = CASES["bot"]["webhook_disabled_by_default"]
        for k, v in tc["env"].items():
            monkeypatch.setenv(k, v)
        cfg = BotConfig.from_env()
        assert cfg.webhook_port is None
        assert cfg.webhook_secret is None

    def test_with_platform_tokens(self, monkeypatch):
        tc = CASES["bot"]["with_platform_tokens"]
        for k, v in tc["env"].items():
            monkeypatch.setenv(k, v)
        cfg = BotConfig.from_env()
        assert cfg.platform_tokens == tc["expected_platform_tokens"]

    def test_fallback_single_token(self, monkeypatch):
        tc = CASES["bot"]["fallback_single_token"]
        for k, v in tc["env"].items():
            monkeypatch.setenv(k, v)
        cfg = BotConfig.from_env()
        assert cfg.platform_tokens == tc["expected_platform_tokens"]


@pytest.mark.usefixtures("_clean_env")
class TestReceiverConfig:
    def test_from_env(self, monkeypatch):
        for k, v in CASES["receiver"]["valid"]["env"].items():
            monkeypatch.setenv(k, v)
        cfg = ReceiverConfig.from_env()
        exp = CASES["receiver"]["valid"]["expected"]
        assert cfg.webex_token == exp["webex_token"]
        assert cfg.slug == exp["slug"]
        assert cfg.domains == exp["domains"]
        assert cfg.platform == exp["platform"]

    def test_platform_explicit(self, monkeypatch):
        tc = CASES["receiver"]["platform_explicit"]
        for k, v in tc["env"].items():
            monkeypatch.setenv(k, v)
        cfg = ReceiverConfig.from_env()
        assert cfg.platform == tc["expected_platform"]

    def test_debug_values(self, monkeypatch):
        base_env = CASES["receiver"]["valid"]["env"]
        for k, v in base_env.items():
            monkeypatch.setenv(k, v)
        for val in CASES["debug_truthy_values"]:
            monkeypatch.setenv("WGROK_DEBUG", val)
            assert ReceiverConfig.from_env().debug is True
        for val in CASES["debug_falsy_values"]:
            monkeypatch.setenv("WGROK_DEBUG", val)
            assert ReceiverConfig.from_env().debug is False
