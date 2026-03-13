"""Tests for wgrok.receiver - WgrokReceiver message handling."""

from unittest.mock import AsyncMock

from wgrok.receiver import WgrokReceiver


class TestWgrokReceiver:
    async def test_calls_handler_on_slug_match(self, receiver_config, fake_message):
        handler = AsyncMock()
        receiver = WgrokReceiver(receiver_config, handler)
        msg = fake_message("testagent:hello world")

        await receiver._on_message(msg)
        handler.assert_called_once_with("testagent", "hello world")

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

        await receiver._on_message(msg)
        handler.assert_called_once_with("testagent", "a:b:c")

    async def test_ignores_unparseable_message(self, receiver_config, fake_message):
        handler = AsyncMock()
        receiver = WgrokReceiver(receiver_config, handler)
        msg = fake_message(":no slug")

        await receiver._on_message(msg)
        handler.assert_not_called()
