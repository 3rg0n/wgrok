"""Tests for wgrok.sender - driven by shared test cases."""

import json
from pathlib import Path
from unittest.mock import AsyncMock, patch

from wgrok.config import SenderConfig
from wgrok.sender import WgrokSender

CASES = json.loads((Path(__file__).resolve().parents[2] / "tests" / "sender_cases.json").read_text())


def _make_config():
    c = CASES["config"]
    return SenderConfig(
        webex_token=c["token"],
        target=c["target"],
        slug=c["slug"],
        domains=["example.com"],
        debug=False,
    )


class TestWgrokSender:
    async def test_cases(self):
        for tc in CASES["cases"]:
            sender = WgrokSender(_make_config())
            try:
                if tc["expected_uses_card"]:
                    with patch("wgrok.sender.platform_send_card", new_callable=AsyncMock) as mock_card:
                        mock_card.return_value = {"id": "msg-1"}
                        await sender.send(tc["payload"], card=tc["card"])
                        args = mock_card.call_args
                        # platform_send_card(platform, token, target, text, card, session)
                        assert args[0][3] == tc["expected_text"], f'{tc["name"]}: text mismatch'
                        assert args[0][2] == tc["expected_target"], f'{tc["name"]}: target mismatch'
                else:
                    with patch("wgrok.sender.platform_send_message", new_callable=AsyncMock) as mock_send:
                        mock_send.return_value = {"id": "msg-1"}
                        await sender.send(tc["payload"], card=tc.get("card"))
                        args = mock_send.call_args
                        # platform_send_message(platform, token, target, text, session)
                        assert args[0][3] == tc["expected_text"], f'{tc["name"]}: text mismatch'
                        assert args[0][2] == tc["expected_target"], f'{tc["name"]}: target mismatch'
            finally:
                await sender.close()

    async def test_close_idempotent(self):
        sender = WgrokSender(_make_config())
        await sender.close()
        await sender.close()


class TestSenderPauseResume:
    async def test_pause_buffers_send(self):
        """While paused, send() buffers instead of sending."""
        sender = WgrokSender(_make_config())
        with patch("wgrok.sender.platform_send_message", new_callable=AsyncMock) as mock_send:
            mock_send.return_value = {"id": "x"}
            await sender.pause()
            assert mock_send.call_count == 1  # ./pause sent to router
            result = await sender.send("hello")
            assert result == {"buffered": True}
            assert mock_send.call_count == 1  # no additional send
        await sender.close()

    async def test_resume_flushes_buffer(self):
        """Resume sends ./resume then flushes buffered messages."""
        sender = WgrokSender(_make_config())
        with patch("wgrok.sender.platform_send_message", new_callable=AsyncMock) as mock_send:
            mock_send.return_value = {"id": "x"}
            await sender.pause()
            await sender.send("msg1")
            await sender.send("msg2")
            assert mock_send.call_count == 1  # only ./pause
            await sender.resume()
            # ./pause + ./resume + msg1 + msg2 = 4 calls
            assert mock_send.call_count == 4
            texts = [call[0][3] for call in mock_send.call_args_list]
            assert texts[0] == "./pause"
            assert texts[1] == "./resume"
            assert "msg1" in texts[2]
            assert "msg2" in texts[3]
        await sender.close()

    async def test_pause_notify_false_no_send(self):
        """pause(notify=False) buffers without sending ./pause to router."""
        sender = WgrokSender(_make_config())
        with patch("wgrok.sender.platform_send_message", new_callable=AsyncMock) as mock_send:
            mock_send.return_value = {"id": "x"}
            await sender.pause(notify=False)
            result = await sender.send("hello")
            assert result == {"buffered": True}
            mock_send.assert_not_called()
        await sender.close()

    async def test_resume_notify_false_no_send(self):
        """resume(notify=False) flushes without sending ./resume to router."""
        sender = WgrokSender(_make_config())
        with patch("wgrok.sender.platform_send_message", new_callable=AsyncMock) as mock_send:
            mock_send.return_value = {"id": "x"}
            await sender.pause(notify=False)
            await sender.send("hello")
            await sender.resume(notify=False)
            # Only the flushed message, no ./pause or ./resume
            assert mock_send.call_count == 1
            assert "hello" in mock_send.call_args[0][3]
        await sender.close()
