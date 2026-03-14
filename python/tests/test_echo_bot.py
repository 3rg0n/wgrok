"""Tests for wgrok.echo_bot - driven by shared test cases."""

import json
from pathlib import Path
from unittest.mock import AsyncMock, patch

import pytest

from wgrok.config import BotConfig
from wgrok.echo_bot import WgrokEchoBot

CASES = json.loads((Path(__file__).resolve().parents[2] / "tests" / "echo_bot_cases.json").read_text())


def _make_config(use_routes=False):
    return BotConfig(
        webex_token="fake-bot-token",
        domains=CASES["config"]["domains"],
        debug=False,
        routes=CASES.get("routes", {}) if use_routes else {},
    )


class TestWgrokEchoBot:
    @pytest.mark.parametrize("tc", CASES["cases"], ids=lambda tc: tc["name"])
    async def test_cases(self, tc):
        use_routes = tc.get("use_routes", False)
        bot = WgrokEchoBot(_make_config(use_routes=use_routes))
        msg = {
            "id": "msg-123",
            "personEmail": tc["sender"],
            "text": tc["text"],
            "roomId": "room-abc",
        }

        with (
            patch("wgrok.echo_bot.send_message", new_callable=AsyncMock) as mock_send,
            patch("wgrok.echo_bot.send_card", new_callable=AsyncMock) as mock_card,
            patch.object(bot, "_fetch_cards", return_value=tc["cards"]),
        ):
            mock_send.return_value = {"id": "reply-1"}
            mock_card.return_value = {"id": "reply-1"}
            await bot._on_message(msg)

            if tc["expect_send"]:
                if tc.get("expected_reply_card"):
                    mock_card.assert_called_once()
                    args = mock_card.call_args
                    assert args[0][1] == tc["expected_reply_to"]
                    assert args[0][2] == tc["expected_reply_text"]
                else:
                    mock_send.assert_called_once()
                    args = mock_send.call_args
                    assert args[0][1] == tc["expected_reply_to"]
                    assert args[0][2] == tc["expected_reply_text"]
            else:
                mock_send.assert_not_called()
                mock_card.assert_not_called()
