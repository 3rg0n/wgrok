"""E2E test fixtures - mock Webex transport for full flow testing."""

import pytest

from wgrok.config import BotConfig, ReceiverConfig, SenderConfig


@pytest.fixture
def e2e_sender_config():
    return SenderConfig(
        webex_token="fake-sender-token",
        target="echobot@example.com",
        slug="e2e-slug",
        domains=["example.com"],
        debug=True,
    )


@pytest.fixture
def e2e_bot_config():
    return BotConfig(
        webex_token="fake-bot-token",
        domains=["example.com"],
        debug=True,
    )


@pytest.fixture
def e2e_receiver_config():
    return ReceiverConfig(
        webex_token="fake-receiver-token",
        slug="e2e-slug",
        domains=["example.com"],
        debug=True,
    )
