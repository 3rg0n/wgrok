"""Tests for wgrok.webex - Webex REST client."""

import aiohttp
import pytest
from aioresponses import aioresponses
from yarl import URL

from wgrok.webex import WEBEX_API_URL, send_message


class TestSendMessage:
    async def test_sends_correct_payload(self):
        with aioresponses() as m:
            m.post(WEBEX_API_URL, payload={"id": "msg-1"})
            result = await send_message("tok123", "user@example.com", "hello")
            assert result == {"id": "msg-1"}

            key = ("POST", URL(WEBEX_API_URL))
            call = m.requests[key][0]
            assert call.kwargs["json"] == {"toPersonEmail": "user@example.com", "text": "hello"}
            assert call.kwargs["headers"]["Authorization"] == "Bearer tok123"

    async def test_raises_on_http_error(self):
        with aioresponses() as m:
            m.post(WEBEX_API_URL, status=401)
            with pytest.raises(aiohttp.ClientResponseError):
                await send_message("badtoken", "user@example.com", "hello")

    async def test_uses_provided_session(self):
        with aioresponses() as m:
            m.post(WEBEX_API_URL, payload={"id": "msg-2"})
            async with aiohttp.ClientSession() as session:
                result = await send_message("tok", "u@x.com", "hi", session=session)
                assert result == {"id": "msg-2"}
