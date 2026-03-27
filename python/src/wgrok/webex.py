"""Thin Webex REST client for sending messages, cards, and fetching resources."""

from __future__ import annotations

import asyncio

import aiohttp

WEBEX_API_BASE = "https://webexapis.com/v1"
MAX_RETRIES = 3
WEBEX_MESSAGES_URL = f"{WEBEX_API_BASE}/messages"
WEBEX_ATTACHMENT_ACTIONS_URL = f"{WEBEX_API_BASE}/attachment/actions"

ADAPTIVE_CARD_CONTENT_TYPE = "application/vnd.microsoft.card.adaptive"


def _headers(token: str) -> dict:
    return {"Authorization": f"Bearer {token}", "Content-Type": "application/json"}


async def _manage_session(session, func):
    """Run func with session, closing it afterward if we created it."""
    owns_session = session is None
    if owns_session:
        session = aiohttp.ClientSession()
    try:
        return await func(session)
    finally:
        if owns_session:
            await session.close()


async def _request_with_retry(session, method, url, headers, **kwargs):
    """Execute an HTTP request with Retry-After handling for 429 responses."""
    for attempt in range(MAX_RETRIES + 1):
        async with session.request(method, url, headers=headers, **kwargs) as resp:
            if resp.status == 429:
                if attempt >= MAX_RETRIES:
                    resp.raise_for_status()
                try:
                    retry_after = min(int(resp.headers.get("Retry-After", "1")), 300)
                except (ValueError, TypeError):
                    retry_after = 1
                await asyncio.sleep(retry_after)
                continue
            resp.raise_for_status()
            return await resp.json()
    # Should not reach here, but just in case
    msg = f"HTTP request to {url} failed after {MAX_RETRIES + 1} attempts"
    raise aiohttp.ClientResponseError(None, None, message=msg, status=429)


async def send_message(
    token: str,
    to_email: str,
    text: str,
    session: aiohttp.ClientSession | None = None,
    room_id: str = "",
) -> dict:
    """Send a text-only Webex message to a person by email or room by ID."""
    payload = {"roomId": room_id, "text": text} if room_id else {"toPersonEmail": to_email, "text": text}

    async def _do(s):
        return await _request_with_retry(s, "POST", WEBEX_MESSAGES_URL, _headers(token), json=payload)

    return await _manage_session(session, _do)


async def send_card(
    token: str,
    to_email: str,
    text: str,
    card: dict,
    session: aiohttp.ClientSession | None = None,
    room_id: str = "",
) -> dict:
    """Send a Webex message with an adaptive card attachment.

    Args:
        text: Fallback text for clients that can't render cards.
        card: Adaptive Card JSON body (the content inside the attachment).
        room_id: Optional room ID to send to room instead of person email.
    """
    if room_id:
        payload = {
            "roomId": room_id,
            "text": text,
            "attachments": [{"contentType": ADAPTIVE_CARD_CONTENT_TYPE, "content": card}],
        }
    else:
        payload = {
            "toPersonEmail": to_email,
            "text": text,
            "attachments": [{"contentType": ADAPTIVE_CARD_CONTENT_TYPE, "content": card}],
        }

    async def _do(s):
        return await _request_with_retry(s, "POST", WEBEX_MESSAGES_URL, _headers(token), json=payload)

    return await _manage_session(session, _do)


async def get_message(
    token: str,
    message_id: str,
    session: aiohttp.ClientSession | None = None,
) -> dict:
    """Fetch full message details by ID (includes attachments)."""
    url = f"{WEBEX_MESSAGES_URL}/{message_id}"

    async def _do(s):
        return await _request_with_retry(s, "GET", url, _headers(token))

    return await _manage_session(session, _do)


async def get_attachment_action(
    token: str,
    action_id: str,
    session: aiohttp.ClientSession | None = None,
) -> dict:
    """Fetch an attachment action (card submission) by ID."""
    url = f"{WEBEX_ATTACHMENT_ACTIONS_URL}/{action_id}"

    async def _do(s):
        return await _request_with_retry(s, "GET", url, _headers(token))

    return await _manage_session(session, _do)


def extract_cards(message: dict) -> list[dict]:
    """Extract adaptive card content dicts from a message's attachments."""
    attachments = message.get("attachments") or []
    return [
        att["content"]
        for att in attachments
        if att.get("contentType") == ADAPTIVE_CARD_CONTENT_TYPE and "content" in att
    ]
