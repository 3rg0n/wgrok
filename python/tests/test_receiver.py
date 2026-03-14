"""Tests for wgrok.receiver - WgrokReceiver message handling."""

from unittest.mock import AsyncMock, patch

from wgrok.receiver import WgrokReceiver


class TestWgrokReceiver:
    async def test_calls_handler_on_slug_match(self, receiver_config, fake_message):
        handler = AsyncMock()
        receiver = WgrokReceiver(receiver_config, handler)
        msg = fake_message("testagent:hello world")

        with patch.object(receiver, "_fetch_cards", return_value=[]):
            await receiver._on_message(msg)
        handler.assert_called_once_with("testagent", "hello world", [])

    async def test_ignores_wrong_slug(self, receiver_config, fake_message):
        handler = AsyncMock()
        receiver = WgrokReceiver(receiver_config, handler)
        msg = fake_message("otheragent:payload")

        await receiver._on_message(msg)
        handler.assert_not_called()

    async def test_rejects_disallowed_sender(self, receiver_config, fake_message):
        handler = AsyncMock()
        receiver = WgrokReceiver(receiver_config, handler)
        msg = fake_message("testagent:payload", sender="hacker@evil.com")

        await receiver._on_message(msg)
        handler.assert_not_called()

    async def test_handles_payload_with_colons(self, receiver_config, fake_message):
        handler = AsyncMock()
        receiver = WgrokReceiver(receiver_config, handler)
        msg = fake_message("testagent:a:b:c")

        with patch.object(receiver, "_fetch_cards", return_value=[]):
            await receiver._on_message(msg)
        handler.assert_called_once_with("testagent", "a:b:c", [])

    async def test_ignores_unparseable_message(self, receiver_config, fake_message):
        handler = AsyncMock()
        receiver = WgrokReceiver(receiver_config, handler)
        msg = fake_message(":no slug")

        await receiver._on_message(msg)
        handler.assert_not_called()

    async def test_passes_cards_to_handler(self, receiver_config, fake_message):
        handler = AsyncMock()
        receiver = WgrokReceiver(receiver_config, handler)
        msg = fake_message("testagent:data")
        sample_card = {"type": "AdaptiveCard", "body": [{"type": "TextBlock", "text": "Hi"}]}

        with patch.object(receiver, "_fetch_cards", return_value=[sample_card]):
            await receiver._on_message(msg)
        handler.assert_called_once_with("testagent", "data", [sample_card])
