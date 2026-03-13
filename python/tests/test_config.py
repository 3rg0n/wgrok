"""Tests for wgrok.config - environment variable loading."""

import os

import pytest

from wgrok.config import BotConfig, ReceiverConfig, SenderConfig


@pytest.fixture
def _clean_env(monkeypatch):
    """Remove all WGROK_ env vars before each test."""
    for key in list(os.environ):
        if key.startswith("WGROK_"):
            monkeypatch.delenv(key, raising=False)


@pytest.mark.usefixtures("_clean_env")
class TestSenderConfig:
    def test_from_env(self, monkeypatch):
        monkeypatch.setenv("WGROK_TOKEN", "tok123")
        monkeypatch.setenv("WGROK_TARGET", "bot@example.com")
        monkeypatch.setenv("WGROK_SLUG", "myslug")
        monkeypatch.setenv("WGROK_DOMAINS", "example.com,trusted.org")
        monkeypatch.setenv("WGROK_DEBUG", "true")

        cfg = SenderConfig.from_env()
        assert cfg.webex_token == "tok123"
        assert cfg.target == "bot@example.com"
        assert cfg.slug == "myslug"
        assert cfg.domains == ["example.com", "trusted.org"]
        assert cfg.debug is True

    def test_missing_token_raises(self, monkeypatch):
        monkeypatch.setenv("WGROK_TARGET", "bot@example.com")
        monkeypatch.setenv("WGROK_SLUG", "myslug")
        with pytest.raises(ValueError, match="WGROK_TOKEN"):
            SenderConfig.from_env()

    def test_missing_target_raises(self, monkeypatch):
        monkeypatch.setenv("WGROK_TOKEN", "tok123")
        monkeypatch.setenv("WGROK_SLUG", "myslug")
        with pytest.raises(ValueError, match="WGROK_TARGET"):
            SenderConfig.from_env()

    def test_debug_defaults_false(self, monkeypatch):
        monkeypatch.setenv("WGROK_TOKEN", "tok")
        monkeypatch.setenv("WGROK_TARGET", "bot@x.com")
        monkeypatch.setenv("WGROK_SLUG", "s")
        cfg = SenderConfig.from_env()
        assert cfg.debug is False

    def test_domains_optional(self, monkeypatch):
        monkeypatch.setenv("WGROK_TOKEN", "tok")
        monkeypatch.setenv("WGROK_TARGET", "bot@x.com")
        monkeypatch.setenv("WGROK_SLUG", "s")
        cfg = SenderConfig.from_env()
        assert cfg.domains == []


@pytest.mark.usefixtures("_clean_env")
class TestBotConfig:
    def test_from_env(self, monkeypatch):
        monkeypatch.setenv("WGROK_TOKEN", "bottok")
        monkeypatch.setenv("WGROK_DOMAINS", "example.com")
        cfg = BotConfig.from_env()
        assert cfg.webex_token == "bottok"
        assert cfg.domains == ["example.com"]

    def test_missing_domains_raises(self, monkeypatch):
        monkeypatch.setenv("WGROK_TOKEN", "bottok")
        with pytest.raises(ValueError, match="WGROK_DOMAINS"):
            BotConfig.from_env()


@pytest.mark.usefixtures("_clean_env")
class TestReceiverConfig:
    def test_from_env(self, monkeypatch):
        monkeypatch.setenv("WGROK_TOKEN", "tok")
        monkeypatch.setenv("WGROK_SLUG", "myslug")
        monkeypatch.setenv("WGROK_DOMAINS", "example.com")
        cfg = ReceiverConfig.from_env()
        assert cfg.webex_token == "tok"
        assert cfg.slug == "myslug"
        assert cfg.domains == ["example.com"]

    def test_debug_values(self, monkeypatch):
        monkeypatch.setenv("WGROK_TOKEN", "tok")
        monkeypatch.setenv("WGROK_SLUG", "s")
        monkeypatch.setenv("WGROK_DOMAINS", "x.com")
        for val in ("true", "1", "yes", "True", "YES"):
            monkeypatch.setenv("WGROK_DEBUG", val)
            assert ReceiverConfig.from_env().debug is True
        for val in ("false", "0", "no", ""):
            monkeypatch.setenv("WGROK_DEBUG", val)
            assert ReceiverConfig.from_env().debug is False
