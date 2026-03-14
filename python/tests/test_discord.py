"""Tests for wgrok.discord - Discord REST client."""

import json
from pathlib import Path
from unittest.mock import patch

import aiohttp
import pytest
from aioresponses import aioresponses
from yarl import URL

from wgrok.discord import DISCORD_API_BASE, send_card, send_message

CASES = json.loads((Path(__file__).resolve().parents[2] / "tests" / "discord_cases.json").read_text())

CHANNEL_ID = "1234567890"
MESSAGES_URL = f"{DISCORD_API_BASE}/channels/{CHANNEL_ID}/messages"


class TestSendMessage:
    async def test_sends_correct_payload(self):
        with aioresponses() as m:
            m.post(MESSAGES_URL, payload={"id": "msg-discord-1"})
            result = await send_message("bot-token", CHANNEL_ID, "hello")
            assert result == {"id": "msg-discord-1"}

            key = ("POST", URL(MESSAGES_URL))
            call = m.requests[key][0]
            assert call.kwargs["json"]["content"] == "hello"
            assert call.kwargs["headers"]["Authorization"] == "Bot bot-token"

    async def test_raises_on_http_error(self):
        with aioresponses() as m:
            m.post(MESSAGES_URL, status=401)
            with pytest.raises(aiohttp.ClientResponseError):
                await send_message("badtoken", CHANNEL_ID, "hello")


class TestSendCard:
    async def test_sends_embeds(self):
        embed = {"title": "Test", "description": "Hello"}
        with aioresponses() as m:
            m.post(MESSAGES_URL, payload={"id": "msg-embed-1"})
            result = await send_card("bot-token", CHANNEL_ID, "fallback", embed)
            assert result == {"id": "msg-embed-1"}

            key = ("POST", URL(MESSAGES_URL))
            call = m.requests[key][0]
            body = call.kwargs["json"]
            assert body["content"] == "fallback"
            assert body["embeds"] == [embed]


class TestRetryAfter:
    retry_cases = CASES.get("retry_after", {}).get("cases", [])

    @pytest.mark.parametrize(
        "tc",
        [c for c in retry_cases if c["name"] == "retries_on_429_then_succeeds"],
        ids=lambda tc: tc["name"],
    )
    async def test_retries_on_429_then_succeeds(self, tc):
        with aioresponses() as m, patch("wgrok.discord.asyncio.sleep", return_value=None) as mock_sleep:
            for resp in tc["responses"]:
                if resp["status"] == 429:
                    headers = {"Retry-After": resp["retry_after"]} if resp.get("retry_after") else {}
                    m.post(MESSAGES_URL, status=429, headers=headers)
                else:
                    m.post(MESSAGES_URL, payload=resp["body"])
            result = await send_message("tok", CHANNEL_ID, "hi")
            assert result == tc["expected_result"]
            assert mock_sleep.call_count == 1

    @pytest.mark.parametrize(
        "tc",
        [c for c in retry_cases if c["name"] == "raises_after_max_retries"],
        ids=lambda tc: tc["name"],
    )
    async def test_raises_after_max_retries(self, tc):
        with aioresponses() as m, patch("wgrok.discord.asyncio.sleep", return_value=None):
            for resp in tc["responses"]:
                headers = {"Retry-After": resp["retry_after"]} if resp.get("retry_after") else {}
                m.post(MESSAGES_URL, status=429, headers=headers)
            with pytest.raises(aiohttp.ClientResponseError) as exc_info:
                await send_message("tok", CHANNEL_ID, "hi")
            assert "429" in str(exc_info.value.status)
