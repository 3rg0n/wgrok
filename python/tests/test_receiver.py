"""Tests for wgrok.receiver - driven by shared test cases."""

import json
from pathlib import Path
from unittest.mock import AsyncMock, patch

import pytest

from wgrok.config import ReceiverConfig
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
        msg = {
            "id": "msg-123",
            "personEmail": tc["sender"],
            "text": tc["text"],
            "roomId": "room-abc",
        }

        with patch.object(receiver, "_fetch_cards", return_value=tc["cards"]):
            await receiver._on_message(msg)

        if tc["expect_handler"]:
            handler.assert_called_once_with(
                tc["expected_slug"],
                tc["expected_payload"],
                tc["expected_cards"],
            )
        else:
            handler.assert_not_called()
