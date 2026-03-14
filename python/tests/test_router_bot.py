"""Tests for wgrok.router_bot - driven by shared test cases."""

import json
from pathlib import Path
from unittest.mock import AsyncMock, patch

import pytest

from wgrok.config import BotConfig
from wgrok.router_bot import WgrokRouterBot

CASES = json.loads((Path(__file__).resolve().parents[2] / "tests" / "router_bot_cases.json").read_text())


def _make_config(use_routes=False):
    return BotConfig(
        webex_token="fake-bot-token",
        domains=CASES["config"]["domains"],
        debug=False,
        routes=CASES.get("routes", {}) if use_routes else {},
        platform_tokens={"webex": ["fake-bot-token"]},
    )


class TestWgrokRouterBot:
    @pytest.mark.parametrize("tc", CASES["cases"], ids=lambda tc: tc["name"])
    async def test_cases(self, tc):
        use_routes = tc.get("use_routes", False)
        bot = WgrokRouterBot(_make_config(use_routes=use_routes))
        msg = {
            "id": "msg-123",
            "personEmail": tc["sender"],
            "text": tc["text"],
            "roomId": "room-abc",
        }

        with (
            patch("wgrok.router_bot.platform_send_message", new_callable=AsyncMock) as mock_send,
            patch("wgrok.router_bot.platform_send_card", new_callable=AsyncMock) as mock_card,
            patch.object(bot, "_fetch_cards", return_value=tc["cards"]),
        ):
            mock_send.return_value = {"id": "reply-1"}
            mock_card.return_value = {"id": "reply-1"}
            await bot._on_message(msg)

            if tc["expect_send"]:
                if tc.get("expected_reply_card"):
                    mock_card.assert_called_once()
                    args = mock_card.call_args
                    # platform_send_card(platform, token, target, text, card, session)
                    assert args[0][2] == tc["expected_reply_to"]
                    assert args[0][3] == tc["expected_reply_text"]
                else:
                    mock_send.assert_called_once()
                    args = mock_send.call_args
                    # platform_send_message(platform, token, target, text, session)
                    assert args[0][2] == tc["expected_reply_to"]
                    assert args[0][3] == tc["expected_reply_text"]
            else:
                mock_send.assert_not_called()
                mock_card.assert_not_called()
