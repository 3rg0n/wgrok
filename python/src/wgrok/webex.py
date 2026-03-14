"""Thin Webex REST client for sending messages, cards, and fetching resources."""

from __future__ import annotations

import aiohttp

WEBEX_API_BASE = "https://webexapis.com/v1"
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


async def send_message(
    token: str,
    to_email: str,
    text: str,
    session: aiohttp.ClientSession | None = None,
) -> dict:
    """Send a text-only Webex message to a person by email."""
    payload = {"toPersonEmail": to_email, "text": text}

    async def _do(s):
        async with s.post(WEBEX_MESSAGES_URL, json=payload, headers=_headers(token)) as resp:
            resp.raise_for_status()
            return await resp.json()

    return await _manage_session(session, _do)


async def send_card(
    token: str,
    to_email: str,
    text: str,
    card: dict,
    session: aiohttp.ClientSession | None = None,
) -> dict:
    """Send a Webex message with an adaptive card attachment.

    Args:
        text: Fallback text for clients that can't render cards.
        card: Adaptive Card JSON body (the content inside the attachment).
    """
    payload = {
        "toPersonEmail": to_email,
        "text": text,
        "attachments": [{"contentType": ADAPTIVE_CARD_CONTENT_TYPE, "content": card}],
    }

    async def _do(s):
        async with s.post(WEBEX_MESSAGES_URL, json=payload, headers=_headers(token)) as resp:
            resp.raise_for_status()
            return await resp.json()

    return await _manage_session(session, _do)


async def get_message(
    token: str,
    message_id: str,
    session: aiohttp.ClientSession | None = None,
) -> dict:
    """Fetch full message details by ID (includes attachments)."""
    url = f"{WEBEX_MESSAGES_URL}/{message_id}"

    async def _do(s):
        async with s.get(url, headers=_headers(token)) as resp:
            resp.raise_for_status()
            return await resp.json()

    return await _manage_session(session, _do)


async def get_attachment_action(
    token: str,
    action_id: str,
    session: aiohttp.ClientSession | None = None,
) -> dict:
    """Fetch an attachment action (card submission) by ID."""
    url = f"{WEBEX_ATTACHMENT_ACTIONS_URL}/{action_id}"

    async def _do(s):
        async with s.get(url, headers=_headers(token)) as resp:
            resp.raise_for_status()
            return await resp.json()

    return await _manage_session(session, _do)


def extract_cards(message: dict) -> list[dict]:
    """Extract adaptive card content dicts from a message's attachments."""
    attachments = message.get("attachments") or []
    return [
        att["content"]
        for att in attachments
        if att.get("contentType") == ADAPTIVE_CARD_CONTENT_TYPE and "content" in att
    ]
