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
                    with patch("wgrok.sender.send_card", new_callable=AsyncMock) as mock_card:
                        mock_card.return_value = {"id": "msg-1"}
                        await sender.send(tc["payload"], card=tc["card"])
                        args = mock_card.call_args
                        assert args[0][2] == tc["expected_text"], f'{tc["name"]}: text mismatch'
                        assert args[0][1] == tc["expected_target"], f'{tc["name"]}: target mismatch'
                else:
                    with patch("wgrok.sender.send_message", new_callable=AsyncMock) as mock_send:
                        mock_send.return_value = {"id": "msg-1"}
                        await sender.send(tc["payload"], card=tc.get("card"))
                        args = mock_send.call_args
                        assert args[0][2] == tc["expected_text"], f'{tc["name"]}: text mismatch'
                        assert args[0][1] == tc["expected_target"], f'{tc["name"]}: target mismatch'
            finally:
                await sender.close()

    async def test_close_idempotent(self):
        sender = WgrokSender(_make_config())
        await sender.close()
        await sender.close()
