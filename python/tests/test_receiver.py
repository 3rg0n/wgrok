"""Tests for wgrok.receiver - driven by shared test cases."""

import json
from pathlib import Path
from unittest.mock import AsyncMock

import pytest

from wgrok.config import ReceiverConfig
from wgrok.listener import IncomingMessage
from wgrok.receiver import WgrokReceiver

CASES = json.loads((Path(__file__).resolve().parents[2] / "tests" / "receiver_cases.json").read_text())


def _make_config():
    return ReceiverConfig(
        webex_token="fake-token",
        slug=CASES["config"]["slug"],
        domains=CASES["config"]["domains"],
        debug=False,
    )


class TestWgrokReceiver:
    @pytest.mark.parametrize("tc", CASES["cases"], ids=lambda tc: tc["name"])
    async def test_cases(self, tc):
        handler = AsyncMock()
        receiver = WgrokReceiver(_make_config(), handler)
        incoming = IncomingMessage(
            sender=tc["sender"],
            text=tc["text"],
            msg_id="msg-123",
            platform="webex",
            cards=tc["cards"],
        )

        await receiver.on_message_with_cards(incoming)

        if tc["expect_handler"]:
            handler.assert_called_once_with(
                tc["expected_slug"],
                tc["expected_payload"],
                tc["expected_cards"],
            )
        else:
            handler.assert_not_called()
