"""WgrokEchoBot - listens for echo messages, validates allowlist, strips prefix, relays back."""

from __future__ import annotations

import asyncio

import aiohttp
from webex_message_handler import WebexMessageHandler, WebexMessageHandlerConfig

from .allowlist import Allowlist
from .config import BotConfig
from .logging import get_logger
from .protocol import format_response, is_echo, parse_echo
from .webex import extract_cards, get_message, send_card, send_message


class WgrokEchoBot:
    def __init__(self, config: BotConfig) -> None:
        self._config = config
        self._allowlist = Allowlist(config.domains)
        self._logger = get_logger(config.debug, "wgrok.echo_bot")
        self._handler: WebexMessageHandler | None = None
        self._session: aiohttp.ClientSession | None = None
        self._stop_event: asyncio.Event = asyncio.Event()

    async def run(self) -> None:
        """Connect to Webex and listen for echo messages."""
        self._session = aiohttp.ClientSession()
        wmh_config = WebexMessageHandlerConfig(token=self._config.webex_token, logger=self._logger)
        self._handler = WebexMessageHandler(wmh_config)

        @self._handler.on("message:created")
        async def on_message(message) -> None:
            await self._on_message(message)

        self._logger.info("Echo bot starting")
        await self._handler.connect()
        self._logger.info("Echo bot connected")
        await self._stop_event.wait()

    async def stop(self) -> None:
        """Disconnect from Webex and clean up."""
        self._stop_event.set()
        if self._handler:
            await self._handler.disconnect()
            self._handler = None
        if self._session:
            await self._session.close()
            self._session = None
        self._logger.info("Echo bot stopped")

    async def _on_message(self, message) -> None:
        """Process an incoming message: check allowlist, parse echo, relay response.

        If the original message includes adaptive card attachments, they are
        fetched via REST API and relayed alongside the response text.
        """
        sender = message.person_email if hasattr(message, "person_email") else message.get("personEmail", "")
        msg_id = message.id if hasattr(message, "id") else message.get("id", "")
        raw_text = message.text if hasattr(message, "text") else message.get("text", "")
        text = (raw_text or "").strip()

        if not self._allowlist.is_allowed(sender):
            self._logger.warning(f"Rejected message from {sender}: not in allowlist")
            return

        if not is_echo(text):
            self._logger.debug(f"Ignoring non-echo message from {sender}")
            return

        try:
            slug, payload = parse_echo(text)
        except ValueError as e:
            self._logger.error(f"Failed to parse echo message: {e}")
            return

        response = format_response(slug, payload)

        # Check for card attachments on the original message
        cards = await self._fetch_cards(msg_id)

        if cards:
            self._logger.info(f"Relaying to {sender}: {response} (with {len(cards)} card(s))")
            await send_card(self._config.webex_token, sender, response, cards[0], self._session)
        else:
            self._logger.info(f"Relaying to {sender}: {response}")
            await send_message(self._config.webex_token, sender, response, self._session)

    async def _fetch_cards(self, message_id: str) -> list[dict]:
        """Fetch card attachments from the original message via REST API."""
        if not message_id:
            return []
        try:
            full_msg = await get_message(self._config.webex_token, message_id, self._session)
            return extract_cards(full_msg)
        except Exception as e:
            self._logger.debug(f"Could not fetch message attachments: {e}")
            return []
