"""Tests for wgrok.sender - WgrokSender."""

from unittest.mock import AsyncMock, patch

from wgrok.sender import WgrokSender


class TestWgrokSender:
    async def test_send_formats_and_sends(self, sender_config):
        sender = WgrokSender(sender_config)
        try:
            with patch("wgrok.sender.send_message", new_callable=AsyncMock) as mock_send:
                mock_send.return_value = {"id": "msg-1"}
                result = await sender.send("hello world")

                assert result == {"id": "msg-1"}
                mock_send.assert_called_once()
                args = mock_send.call_args
                assert args[0][0] == "fake-token"
                assert args[0][1] == "echobot@example.com"
                assert args[0][2] == "./echo:testagent:hello world"
        finally:
            await sender.close()

    async def test_send_payload_with_colons(self, sender_config):
        sender = WgrokSender(sender_config)
        try:
            with patch("wgrok.sender.send_message", new_callable=AsyncMock) as mock_send:
                mock_send.return_value = {"id": "msg-2"}
                await sender.send("a:b:c")
                text = mock_send.call_args[0][2]
                assert text == "./echo:testagent:a:b:c"
        finally:
            await sender.close()

    async def test_send_with_card(self, sender_config):
        sender = WgrokSender(sender_config)
        card = {"type": "AdaptiveCard", "body": [{"type": "TextBlock", "text": "Hi"}]}
        try:
            with patch("wgrok.sender.send_card", new_callable=AsyncMock) as mock_card:
                mock_card.return_value = {"id": "card-1"}
                result = await sender.send("hello", card=card)

                assert result == {"id": "card-1"}
                mock_card.assert_called_once()
                args = mock_card.call_args
                assert args[0][0] == "fake-token"
                assert args[0][1] == "echobot@example.com"
                assert args[0][2] == "./echo:testagent:hello"
                assert args[0][3] == card
        finally:
            await sender.close()

    async def test_send_without_card_uses_send_message(self, sender_config):
        sender = WgrokSender(sender_config)
        try:
            with (
                patch("wgrok.sender.send_message", new_callable=AsyncMock) as mock_msg,
                patch("wgrok.sender.send_card", new_callable=AsyncMock) as mock_card,
            ):
                mock_msg.return_value = {"id": "msg-3"}
                await sender.send("no card")
                mock_msg.assert_called_once()
                mock_card.assert_not_called()
        finally:
            await sender.close()

    async def test_close_idempotent(self, sender_config):
        sender = WgrokSender(sender_config)
        await sender.close()
        await sender.close()
