"""Tests for wgrok.webex - Webex REST client."""

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


class TestExtractCards:
    def test_extracts_adaptive_cards(self):
        card = {"type": "AdaptiveCard", "body": []}
        msg = {
            "attachments": [
                {"contentType": ADAPTIVE_CARD_CONTENT_TYPE, "content": card},
            ]
        }
        assert extract_cards(msg) == [card]

    def test_ignores_non_card_attachments(self):
        msg = {
            "attachments": [
                {"contentType": "image/png", "content": {}},
            ]
        }
        assert extract_cards(msg) == []

    def test_handles_no_attachments(self):
        assert extract_cards({}) == []
        assert extract_cards({"attachments": None}) == []
        assert extract_cards({"attachments": []}) == []

    def test_multiple_cards(self):
        card1 = {"type": "AdaptiveCard", "body": [{"type": "TextBlock", "text": "1"}]}
        card2 = {"type": "AdaptiveCard", "body": [{"type": "TextBlock", "text": "2"}]}
        msg = {
            "attachments": [
                {"contentType": ADAPTIVE_CARD_CONTENT_TYPE, "content": card1},
                {"contentType": ADAPTIVE_CARD_CONTENT_TYPE, "content": card2},
            ]
        }
        assert extract_cards(msg) == [card1, card2]
