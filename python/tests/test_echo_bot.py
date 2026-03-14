"""Tests for wgrok.echo_bot - WgrokEchoBot message handling."""

from unittest.mock import AsyncMock, patch

from wgrok.echo_bot import WgrokEchoBot


class TestWgrokEchoBot:
    async def test_relays_valid_echo_no_cards(self, bot_config, fake_message):
        bot = WgrokEchoBot(bot_config)
        msg = fake_message("./echo:myslug:hello")

        with (
            patch("wgrok.echo_bot.send_message", new_callable=AsyncMock) as mock_send,
            patch.object(bot, "_fetch_cards", return_value=[]),
        ):
            mock_send.return_value = {"id": "msg-1"}
            await bot._on_message(msg)
            mock_send.assert_called_once()
            args = mock_send.call_args
            assert args[0][1] == "user@example.com"
            assert args[0][2] == "myslug:hello"

    async def test_relays_echo_with_card(self, bot_config, fake_message):
        bot = WgrokEchoBot(bot_config)
        msg = fake_message("./echo:myslug:hello")
        card = {"type": "AdaptiveCard", "body": [{"type": "TextBlock", "text": "Hi"}]}

        with (
            patch("wgrok.echo_bot.send_card", new_callable=AsyncMock) as mock_card,
            patch.object(bot, "_fetch_cards", return_value=[card]),
        ):
            mock_card.return_value = {"id": "card-1"}
            await bot._on_message(msg)
            mock_card.assert_called_once()
            args = mock_card.call_args
            assert args[0][1] == "user@example.com"
            assert args[0][2] == "myslug:hello"
            assert args[0][3] == card

    async def test_rejects_disallowed_sender(self, bot_config, fake_message):
        bot = WgrokEchoBot(bot_config)
        msg = fake_message("./echo:slug:payload", sender="hacker@evil.com")

        with patch("wgrok.echo_bot.send_message", new_callable=AsyncMock) as mock_send:
            await bot._on_message(msg)
            mock_send.assert_not_called()

    async def test_ignores_non_echo(self, bot_config, fake_message):
        bot = WgrokEchoBot(bot_config)
        msg = fake_message("just a regular message")

        with patch("wgrok.echo_bot.send_message", new_callable=AsyncMock) as mock_send:
            await bot._on_message(msg)
            mock_send.assert_not_called()

    async def test_handles_malformed_echo(self, bot_config, fake_message):
        bot = WgrokEchoBot(bot_config)
        msg = fake_message("./echo::payload")

        with patch("wgrok.echo_bot.send_message", new_callable=AsyncMock) as mock_send:
            await bot._on_message(msg)
            mock_send.assert_not_called()

    async def test_preserves_colons_in_payload(self, bot_config, fake_message):
        bot = WgrokEchoBot(bot_config)
        msg = fake_message("./echo:slug:a:b:c")

        with (
            patch("wgrok.echo_bot.send_message", new_callable=AsyncMock) as mock_send,
            patch.object(bot, "_fetch_cards", return_value=[]),
        ):
            mock_send.return_value = {"id": "msg-2"}
            await bot._on_message(msg)
            assert mock_send.call_args[0][2] == "slug:a:b:c"

    async def test_trusted_org_wildcard(self, bot_config, fake_message):
        """bot_config has *@trusted.org in domains."""
        bot = WgrokEchoBot(bot_config)
        msg = fake_message("./echo:s:p", sender="anyone@trusted.org")

        with (
            patch("wgrok.echo_bot.send_message", new_callable=AsyncMock) as mock_send,
            patch.object(bot, "_fetch_cards", return_value=[]),
        ):
            mock_send.return_value = {"id": "msg-3"}
            await bot._on_message(msg)
            mock_send.assert_called_once()
