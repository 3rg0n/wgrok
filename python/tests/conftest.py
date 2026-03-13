"""Shared fixtures for wgrok unit tests."""

import pytest

from wgrok.config import BotConfig, ReceiverConfig, SenderConfig


@pytest.fixture
def sender_config():
    return SenderConfig(
        webex_token="fake-token",
        target="echobot@example.com",
        slug="testagent",
        domains=["example.com"],
        debug=False,
    )


@pytest.fixture
def bot_config():
    return BotConfig(
        webex_token="fake-bot-token",
        domains=["example.com", "*@trusted.org"],
        debug=False,
    )


@pytest.fixture
def receiver_config():
    return ReceiverConfig(
        webex_token="fake-token",
        slug="testagent",
        domains=["example.com"],
        debug=False,
    )


@pytest.fixture
def fake_message():
    """Factory for creating fake Webex message dicts."""
    def _make(text: str, sender: str = "user@example.com"):
        return {
            "id": "msg-123",
            "personEmail": sender,
            "text": text,
            "roomId": "room-abc",
        }
    return _make
