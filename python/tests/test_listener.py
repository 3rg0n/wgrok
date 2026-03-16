"""Tests for wgrok.listener — platform listener factory and message normalization."""

import json
from pathlib import Path

import pytest

from wgrok.listener import (
    DiscordListener,
    IncomingMessage,
    IrcListener,
    SlackListener,
    WebexListener,
    create_listener,
)
from wgrok.logging import get_logger

CASES = json.loads(
    (Path(__file__).resolve().parents[2] / "tests" / "platform_dispatch_cases.json").read_text()
)

logger = get_logger(False, "test")


_TEST_TOKENS = {
    "webex": "token",
    "slack": "xapp-test-token",
    "discord": "bot-token",
    "irc": "bot:pass@irc.example.com:6697/#test",
}


class TestCreateListener:
    @pytest.mark.parametrize(
        "tc",
        [c for c in CASES["cases"] if not c.get("expected_error")],
        ids=lambda tc: tc["name"],
    )
    def test_creates_correct_listener_type(self, tc):
        token = _TEST_TOKENS.get(tc["platform"], "token")
        listener = create_listener(tc["platform"], token, logger)
        expected_types = {
            "webex": WebexListener,
            "slack": SlackListener,
            "discord": DiscordListener,
            "irc": IrcListener,
        }
        assert isinstance(listener, expected_types[tc["platform"]])

    @pytest.mark.parametrize(
        "tc",
        [c for c in CASES["cases"] if c.get("expected_error")],
        ids=lambda tc: tc["name"],
    )
    def test_invalid_platform_raises(self, tc):
        with pytest.raises(ValueError, match="Unsupported platform"):
            create_listener(tc["platform"], "token", logger)


class TestIncomingMessage:
    def test_fields(self):
        msg = IncomingMessage(
            sender="user@example.com",
            text="hello",
            msg_id="msg-1",
            platform="webex",
            cards=[{"type": "AdaptiveCard"}],
        )
        assert msg.sender == "user@example.com"
        assert msg.text == "hello"
        assert msg.msg_id == "msg-1"
        assert msg.platform == "webex"
        assert msg.cards == [{"type": "AdaptiveCard"}]


class TestIrcListener:
    def test_requires_valid_connection_string(self):
        # IrcListener wraps IrcConnection which needs a valid conn string
        listener = create_listener("irc", "bot:pass@irc.libera.chat:6697/#test", logger)
        assert isinstance(listener, IrcListener)

    def test_invalid_connection_string(self):
        with pytest.raises(ValueError):
            create_listener("irc", "invalid-no-at-sign", logger)
