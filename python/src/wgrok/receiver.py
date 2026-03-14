"""WgrokReceiver - listens for response messages, matches slug, invokes handler callback."""

from __future__ import annotations

import asyncio
from collections.abc import Awaitable, Callable

import aiohttp
from webex_message_handler import WebexMessageHandler, WebexMessageHandlerConfig

from .allowlist import Allowlist
from .config import ReceiverConfig
from .logging import get_logger
from .protocol import parse_response
from .webex import extract_cards, get_attachment_action, get_message

MessageHandler = Callable[[str, str, list[dict]], Awaitable[None]]


class WgrokReceiver:
    """Listens for wgrok response messages and optional card attachments.

    The handler callback receives (slug, payload, cards) where cards is a
    list of adaptive card content dicts (empty if no cards attached).
    """

    def __init__(self, config: ReceiverConfig, handler: MessageHandler) -> None:
        self._config = config
        self._allowlist = Allowlist(config.domains)
        self._handler_callback = handler
        self._logger = get_logger(config.debug, "wgrok.receiver")
        self._ws_handler: WebexMessageHandler | None = None
        self._session: aiohttp.ClientSession | None = None
        self._stop_event: asyncio.Event = asyncio.Event()

    async def listen(self) -> None:
        """Connect to Webex and listen for response messages matching our slug."""
        self._session = aiohttp.ClientSession()
        wmh_config = WebexMessageHandlerConfig(token=self._config.webex_token, logger=self._logger)
        self._ws_handler = WebexMessageHandler(wmh_config)

        @self._ws_handler.on("message:created")
        async def on_message(message) -> None:
            await self._on_message(message)

        self._logger.info(f"Receiver listening for slug: {self._config.slug}")
        await self._ws_handler.connect()
        self._logger.info("Receiver connected")
        await self._stop_event.wait()

    async def stop(self) -> None:
        """Disconnect from Webex."""
        self._stop_event.set()
        if self._ws_handler:
            await self._ws_handler.disconnect()
            self._ws_handler = None
        if self._session:
            await self._session.close()
            self._session = None
        self._logger.info("Receiver stopped")

    async def fetch_action(self, action_id: str) -> dict:
        """Fetch an attachment action (card form submission) by ID.

        Note: webex-message-handler does not currently surface attachmentActions
        events via Mercury. This method is for use when action IDs are obtained
        through other means (e.g. polling or webhooks).
        """
        session = self._session or aiohttp.ClientSession()
        owns = self._session is None
        try:
            return await get_attachment_action(self._config.webex_token, action_id, session)
        finally:
            if owns:
                await session.close()

    async def _on_message(self, message) -> None:
        """Process an incoming message: check allowlist, parse response, match slug, call handler."""
        sender = message.person_email if hasattr(message, "person_email") else message.get("personEmail", "")
        msg_id = message.id if hasattr(message, "id") else message.get("id", "")
        raw_text = message.text if hasattr(message, "text") else message.get("text", "")
        text = (raw_text or "").strip()

        if not self._allowlist.is_allowed(sender):
            self._logger.warning(f"Rejected message from {sender}: not in allowlist")
            return

        try:
            slug, payload = parse_response(text)
        except ValueError:
            self._logger.debug(f"Ignoring unparseable message from {sender}")
            return

        if slug != self._config.slug:
            self._logger.debug(f"Ignoring message with slug {slug!r} (expected {self._config.slug!r})")
            return

        # Fetch card attachments from the full message
        cards = await self._fetch_cards(msg_id)

        if cards:
            self._logger.info(f"Received payload for slug {slug!r} from {sender} (with {len(cards)} card(s))")
        else:
            self._logger.info(f"Received payload for slug {slug!r} from {sender}")
        await self._handler_callback(slug, payload, cards)

    async def _fetch_cards(self, message_id: str) -> list[dict]:
        """Fetch card attachments from the message via REST API."""
        if not message_id:
            return []
        try:
            full_msg = await get_message(self._config.webex_token, message_id, self._session)
            return extract_cards(full_msg)
        except Exception as e:
            self._logger.debug(f"Could not fetch message attachments: {e}")
            return []
