"""Tests for wgrok.webex - driven by shared test cases for extract_cards."""

import json
from pathlib import Path
from unittest.mock import patch

import aiohttp
import pytest
from aioresponses import aioresponses
from yarl import URL

from wgrok.webex import (
    ADAPTIVE_CARD_CONTENT_TYPE,
    WEBEX_ATTACHMENT_ACTIONS_URL,
    WEBEX_MESSAGES_URL,
    extract_cards,
    get_attachment_action,
    get_message,
    send_card,
    send_message,
)

CASES = json.loads((Path(__file__).resolve().parents[2] / "tests" / "webex_cases.json").read_text())


class TestExtractCards:
    @pytest.mark.parametrize("tc", CASES["extract_cards"], ids=lambda tc: tc["name"])
    def test_cases(self, tc):
        result = extract_cards(tc["message"])
        assert result == tc["expected"]


class TestSendMessage:
    async def test_sends_correct_payload(self):
        with aioresponses() as m:
            m.post(WEBEX_MESSAGES_URL, payload={"id": "msg-1"})
            result = await send_message("tok123", "user@example.com", "hello")
            assert result == {"id": "msg-1"}

            key = ("POST", URL(WEBEX_MESSAGES_URL))
            call = m.requests[key][0]
            assert call.kwargs["json"] == {"toPersonEmail": "user@example.com", "text": "hello"}
            assert call.kwargs["headers"]["Authorization"] == "Bearer tok123"

    async def test_raises_on_http_error(self):
        with aioresponses() as m:
            m.post(WEBEX_MESSAGES_URL, status=401)
            with pytest.raises(aiohttp.ClientResponseError):
                await send_message("badtoken", "user@example.com", "hello")

    async def test_uses_provided_session(self):
        with aioresponses() as m:
            m.post(WEBEX_MESSAGES_URL, payload={"id": "msg-2"})
            async with aiohttp.ClientSession() as session:
                result = await send_message("tok", "u@x.com", "hi", session=session)
                assert result == {"id": "msg-2"}


class TestSendCard:
    async def test_sends_card_attachment(self):
        card = {"type": "AdaptiveCard", "body": [{"type": "TextBlock", "text": "Hello"}]}
        with aioresponses() as m:
            m.post(WEBEX_MESSAGES_URL, payload={"id": "card-1"})
            result = await send_card("tok", "user@x.com", "fallback", card)
            assert result == {"id": "card-1"}

            key = ("POST", URL(WEBEX_MESSAGES_URL))
            call = m.requests[key][0]
            body = call.kwargs["json"]
            assert body["text"] == "fallback"
            assert body["toPersonEmail"] == "user@x.com"
            assert len(body["attachments"]) == 1
            assert body["attachments"][0]["contentType"] == ADAPTIVE_CARD_CONTENT_TYPE
            assert body["attachments"][0]["content"] == card


class TestGetMessage:
    async def test_fetches_message(self):
        msg_data = {"id": "msg-1", "text": "hello", "attachments": []}
        with aioresponses() as m:
            m.get(f"{WEBEX_MESSAGES_URL}/msg-1", payload=msg_data)
            result = await get_message("tok", "msg-1")
            assert result == msg_data

    async def test_raises_on_not_found(self):
        with aioresponses() as m:
            m.get(f"{WEBEX_MESSAGES_URL}/bad-id", status=404)
            with pytest.raises(aiohttp.ClientResponseError):
                await get_message("tok", "bad-id")


class TestGetAttachmentAction:
    async def test_fetches_action(self):
        action_data = {"id": "act-1", "type": "submit", "inputs": {"name": "test"}}
        with aioresponses() as m:
            m.get(f"{WEBEX_ATTACHMENT_ACTIONS_URL}/act-1", payload=action_data)
            result = await get_attachment_action("tok", "act-1")
            assert result == action_data


class TestRetryAfter:
    """Test 429 Retry-After handling driven by shared test cases."""

    retry_cases = CASES.get("retry_after", {}).get("cases", [])

    @pytest.mark.parametrize(
        "tc",
        [c for c in retry_cases if c["name"] == "retries_on_429_then_succeeds"],
        ids=lambda tc: tc["name"],
    )
    async def test_retries_on_429_then_succeeds(self, tc):
        with aioresponses() as m, patch("wgrok.webex.asyncio.sleep", return_value=None) as mock_sleep:
            for resp in tc["responses"]:
                if resp["status"] == 429:
                    headers = {"Retry-After": resp["retry_after"]} if resp.get("retry_after") else {}
                    m.post(WEBEX_MESSAGES_URL, status=429, headers=headers)
                else:
                    m.post(WEBEX_MESSAGES_URL, payload=resp["body"])
            result = await send_message("tok", "u@x.com", "hi")
            assert result == tc["expected_result"]
            assert mock_sleep.call_count == 1

    @pytest.mark.parametrize(
        "tc",
        [c for c in retry_cases if c["name"] == "retries_multiple_429s"],
        ids=lambda tc: tc["name"],
    )
    async def test_retries_multiple_429s(self, tc):
        with aioresponses() as m, patch("wgrok.webex.asyncio.sleep", return_value=None) as mock_sleep:
            for resp in tc["responses"]:
                if resp["status"] == 429:
                    headers = {"Retry-After": resp["retry_after"]} if resp.get("retry_after") else {}
                    m.post(WEBEX_MESSAGES_URL, status=429, headers=headers)
                else:
                    m.post(WEBEX_MESSAGES_URL, payload=resp["body"])
            result = await send_message("tok", "u@x.com", "hi")
            assert result == tc["expected_result"]
            assert mock_sleep.call_count == 2

    @pytest.mark.parametrize(
        "tc",
        [c for c in retry_cases if c["name"] == "raises_after_max_retries"],
        ids=lambda tc: tc["name"],
    )
    async def test_raises_after_max_retries(self, tc):
        with aioresponses() as m, patch("wgrok.webex.asyncio.sleep", return_value=None):
            for resp in tc["responses"]:
                headers = {"Retry-After": resp["retry_after"]} if resp.get("retry_after") else {}
                m.post(WEBEX_MESSAGES_URL, status=429, headers=headers)
            with pytest.raises(aiohttp.ClientResponseError) as exc_info:
                await send_message("tok", "u@x.com", "hi")
            assert "429" in str(exc_info.value.status)

    @pytest.mark.parametrize(
        "tc",
        [c for c in retry_cases if c["name"] == "uses_retry_after_header_value"],
        ids=lambda tc: tc["name"],
    )
    async def test_uses_retry_after_header_value(self, tc):
        with aioresponses() as m, patch("wgrok.webex.asyncio.sleep", return_value=None) as mock_sleep:
            for resp in tc["responses"]:
                if resp["status"] == 429:
                    m.post(WEBEX_MESSAGES_URL, status=429, headers={"Retry-After": resp["retry_after"]})
                else:
                    m.post(WEBEX_MESSAGES_URL, payload=resp["body"])
            await send_message("tok", "u@x.com", "hi")
            mock_sleep.assert_called_with(tc["expected_sleep_seconds"][0])

    @pytest.mark.parametrize(
        "tc",
        [c for c in retry_cases if c["name"] == "defaults_retry_after_to_1_when_missing"],
        ids=lambda tc: tc["name"],
    )
    async def test_defaults_retry_after_to_1_when_missing(self, tc):
        with aioresponses() as m, patch("wgrok.webex.asyncio.sleep", return_value=None) as mock_sleep:
            for resp in tc["responses"]:
                if resp["status"] == 429:
                    m.post(WEBEX_MESSAGES_URL, status=429)
                else:
                    m.post(WEBEX_MESSAGES_URL, payload=resp["body"])
            await send_message("tok", "u@x.com", "hi")
            mock_sleep.assert_called_with(1)
