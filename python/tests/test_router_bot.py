"""Tests for wgrok.router_bot - driven by shared test cases."""

import json
from pathlib import Path
from unittest.mock import AsyncMock, patch

import pytest

from wgrok.config import BotConfig
from wgrok.listener import IncomingMessage
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


class TestPauseResume:
    async def test_pause_buffers_messages(self):
        """After ./pause, messages echoed back to that sender are buffered."""
        bot = WgrokRouterBot(_make_config())
        with patch("wgrok.router_bot.platform_send_message", new_callable=AsyncMock) as mock_send:
            mock_send.return_value = {"id": "x"}
            # Agent sends ./pause
            await bot._on_incoming(IncomingMessage(
                sender="user@example.com", text="./pause", msg_id="", platform="webex", cards=[],
            ))
            # Same sender sends a message — Mode B echoes back to them (paused)
            await bot._on_incoming(IncomingMessage(
                sender="user@example.com", text="./echo:slug:user:-:hello", msg_id="", platform="webex", cards=[],
            ))
            mock_send.assert_not_called()

    async def test_resume_flushes_buffer(self):
        """./resume flushes buffered messages."""
        bot = WgrokRouterBot(_make_config())
        with patch("wgrok.router_bot.platform_send_message", new_callable=AsyncMock) as mock_send:
            mock_send.return_value = {"id": "x"}
            # Pause
            await bot._on_incoming(IncomingMessage(
                sender="user@example.com", text="./pause", msg_id="", platform="webex", cards=[],
            ))
            # Buffer a message (echoes back to sender in Mode B)
            await bot._on_incoming(IncomingMessage(
                sender="user@example.com", text="./echo:slug:user:-:hello", msg_id="", platform="webex", cards=[],
            ))
            assert mock_send.call_count == 0
            # Resume
            await bot._on_incoming(IncomingMessage(
                sender="user@example.com", text="./resume", msg_id="", platform="webex", cards=[],
            ))
            assert mock_send.call_count == 1
            assert mock_send.call_args[0][3] == "slug:user:-:hello"

    async def test_pause_does_not_affect_other_senders(self):
        """Pausing one target doesn't block messages to others."""
        bot = WgrokRouterBot(_make_config())
        with patch("wgrok.router_bot.platform_send_message", new_callable=AsyncMock) as mock_send:
            mock_send.return_value = {"id": "x"}
            # Pause user A
            await bot._on_incoming(IncomingMessage(
                sender="a@example.com", text="./pause", msg_id="", platform="webex", cards=[],
            ))
            # Message from user B echoes back to B — should go through
            await bot._on_incoming(IncomingMessage(
                sender="b@example.com", text="./echo:slug:b:-:hello", msg_id="", platform="webex", cards=[],
            ))
            assert mock_send.call_count == 1

    async def test_pause_mode_c_buffers_route_target(self):
        """In Mode C, pause applies to the resolved route target."""
        config = BotConfig(
            webex_token="fake-bot-token",
            domains=["example.com", "*@spark.com"],
            debug=False,
            routes=CASES.get("routes", {}),
            platform_tokens={"webex": ["fake-bot-token"]},
        )
        bot = WgrokRouterBot(config)
        with patch("wgrok.router_bot.platform_send_message", new_callable=AsyncMock) as mock_send:
            mock_send.return_value = {"id": "x"}
            # deploy-bot pauses
            await bot._on_incoming(IncomingMessage(
                sender="deploy-bot@spark.com", text="./pause", msg_id="", platform="webex", cards=[],
            ))
            # Message routed to deploy slug → deploy-bot@spark.com (paused)
            await bot._on_incoming(IncomingMessage(
                sender="user@example.com", text="./echo:deploy:user:-:go", msg_id="", platform="webex", cards=[],
            ))
            mock_send.assert_not_called()
            # Resume
            await bot._on_incoming(IncomingMessage(
                sender="deploy-bot@spark.com", text="./resume", msg_id="", platform="webex", cards=[],
            ))
            assert mock_send.call_count == 1
            assert mock_send.call_args[0][2] == "deploy-bot@spark.com"

    async def test_router_pause_sends_to_routes(self):
        """router.pause() sends ./pause to all Mode C agents."""
        bot = WgrokRouterBot(_make_config(use_routes=True))
        bot._session = AsyncMock()
        with patch("wgrok.router_bot.platform_send_message", new_callable=AsyncMock) as mock_send:
            mock_send.return_value = {"id": "x"}
            await bot.pause()
            targets = [call[0][2] for call in mock_send.call_args_list]
            assert "deploy-bot@spark.com" in targets
            assert "status-bot@foo.com" in targets

    async def test_router_resume_sends_to_routes(self):
        """router.resume() sends ./resume to all Mode C agents."""
        bot = WgrokRouterBot(_make_config(use_routes=True))
        bot._session = AsyncMock()
        with patch("wgrok.router_bot.platform_send_message", new_callable=AsyncMock) as mock_send:
            mock_send.return_value = {"id": "x"}
            await bot.resume()
            texts = [call[0][3] for call in mock_send.call_args_list]
            assert all(t == "./resume" for t in texts)
