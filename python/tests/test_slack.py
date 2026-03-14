"""Tests for wgrok.slack - Slack REST client."""

import json
from pathlib import Path
from unittest.mock import patch

import aiohttp
import pytest
from aioresponses import aioresponses
from yarl import URL

from wgrok.slack import SLACK_POST_MESSAGE_URL, send_card, send_message

CASES = json.loads((Path(__file__).resolve().parents[2] / "tests" / "slack_cases.json").read_text())


class TestSendMessage:
    async def test_sends_correct_payload(self):
        with aioresponses() as m:
            m.post(SLACK_POST_MESSAGE_URL, payload={"ok": True, "ts": "1234.5678"})
            result = await send_message("xoxb-tok", "C1234567890", "hello")
            assert result == {"ok": True, "ts": "1234.5678"}

            key = ("POST", URL(SLACK_POST_MESSAGE_URL))
            call = m.requests[key][0]
            assert call.kwargs["json"]["channel"] == "C1234567890"
            assert call.kwargs["json"]["text"] == "hello"
            assert call.kwargs["headers"]["Authorization"] == "Bearer xoxb-tok"

    async def test_raises_on_http_error(self):
        with aioresponses() as m:
            m.post(SLACK_POST_MESSAGE_URL, status=401)
            with pytest.raises(aiohttp.ClientResponseError):
                await send_message("badtoken", "C123", "hello")


class TestSendCard:
    async def test_sends_blocks(self):
        blocks = [{"type": "section", "text": {"type": "mrkdwn", "text": "Hello"}}]
        with aioresponses() as m:
            m.post(SLACK_POST_MESSAGE_URL, payload={"ok": True})
            result = await send_card("xoxb-tok", "C123", "fallback", blocks)
            assert result == {"ok": True}

            key = ("POST", URL(SLACK_POST_MESSAGE_URL))
            call = m.requests[key][0]
            body = call.kwargs["json"]
            assert body["text"] == "fallback"
            assert body["channel"] == "C123"
            assert body["blocks"] == blocks


class TestRetryAfter:
    retry_cases = CASES.get("retry_after", {}).get("cases", [])

    @pytest.mark.parametrize(
        "tc",
        [c for c in retry_cases if c["name"] == "retries_on_429_then_succeeds"],
        ids=lambda tc: tc["name"],
    )
    async def test_retries_on_429_then_succeeds(self, tc):
        with aioresponses() as m, patch("wgrok.slack.asyncio.sleep", return_value=None) as mock_sleep:
            for resp in tc["responses"]:
                if resp["status"] == 429:
                    headers = {"Retry-After": resp["retry_after"]} if resp.get("retry_after") else {}
                    m.post(SLACK_POST_MESSAGE_URL, status=429, headers=headers)
                else:
                    m.post(SLACK_POST_MESSAGE_URL, payload=resp["body"])
            result = await send_message("tok", "C123", "hi")
            assert result == tc["expected_result"]
            assert mock_sleep.call_count == 1

    @pytest.mark.parametrize(
        "tc",
        [c for c in retry_cases if c["name"] == "raises_after_max_retries"],
        ids=lambda tc: tc["name"],
    )
    async def test_raises_after_max_retries(self, tc):
        with aioresponses() as m, patch("wgrok.slack.asyncio.sleep", return_value=None):
            for resp in tc["responses"]:
                headers = {"Retry-After": resp["retry_after"]} if resp.get("retry_after") else {}
                m.post(SLACK_POST_MESSAGE_URL, status=429, headers=headers)
            with pytest.raises(aiohttp.ClientResponseError) as exc_info:
                await send_message("tok", "C123", "hi")
            assert "429" in str(exc_info.value.status)
