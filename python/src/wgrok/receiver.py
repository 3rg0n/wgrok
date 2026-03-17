"""WgrokReceiver - listens for response messages, matches slug, invokes handler callback."""

from __future__ import annotations

import asyncio
from collections.abc import Awaitable, Callable

import aiohttp

from . import codec
from .allowlist import Allowlist
from .config import ReceiverConfig
from .listener import IncomingMessage, PlatformListener, create_listener
from .logging import get_logger
from .protocol import parse_flags, parse_response
from .webex import extract_cards, get_attachment_action, get_message

MessageHandler = Callable[[str, str, list[dict], str], Awaitable[None]]


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
        self._listener: PlatformListener | None = None
        self._session: aiohttp.ClientSession | None = None
        self._stop_event: asyncio.Event = asyncio.Event()
        self._chunk_buffer: dict[tuple[str, str], dict[int, str]] = {}

    async def listen(self) -> None:
        """Connect to the configured platform and listen for response messages matching our slug."""
        self._session = aiohttp.ClientSession()
        platform = self._config.platform
        token = self._config.webex_token

        self._listener = create_listener(platform, token, self._logger)
        self._listener.on_message(self._on_incoming)

        self._logger.info(f"Receiver listening for slug: {self._config.slug} via {platform}")
        await self._listener.connect()
        self._logger.info("Receiver connected")
        await self._stop_event.wait()

    async def stop(self) -> None:
        """Disconnect from the platform."""
        self._stop_event.set()
        if self._listener:
            await self._listener.disconnect()
            self._listener = None
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

    async def _on_incoming(self, incoming: IncomingMessage) -> None:
        """Process a normalized incoming message from any platform."""
        sender = incoming.sender
        text = incoming.text
        msg_id = incoming.msg_id

        if not self._allowlist.is_allowed(sender):
            self._logger.warning(f"Rejected message from {sender}: not in allowlist")
            return

        try:
            to, from_slug, flags_str, payload = parse_response(text)
        except ValueError:
            self._logger.debug(f"Ignoring unparseable message from {sender}")
            return

        if to != self._config.slug:
            self._logger.debug(f"Ignoring message with slug {to!r} (expected {self._config.slug!r})")
            return

        # Parse flags to get compression, encryption, and chunking info
        compressed, encrypted, chunk_seq, chunk_total = parse_flags(flags_str)

        # Check if this is a chunked payload
        if chunk_seq is not None and chunk_total is not None:
            key = (sender, to)
            self._chunk_buffer.setdefault(key, {})[chunk_seq] = payload
            if len(self._chunk_buffer[key]) < chunk_total:
                self._logger.debug(f"Buffered chunk {chunk_seq}/{chunk_total} for slug {to!r} from {sender}")
                return
            # All chunks received — reassemble
            payload = "".join(self._chunk_buffer[key][i] for i in range(1, chunk_total + 1))
            del self._chunk_buffer[key]
            self._logger.debug(f"Reassembled {chunk_total} chunks for slug {to!r} from {sender}")

        # Decrypt if marked as encrypted
        if encrypted:
            if not self._config.encrypt_key:
                self._logger.warning(
                    f"Message from {sender} marked as encrypted but no WGROK_ENCRYPT_KEY configured — skipping"
                )
                return
            try:
                payload = codec.decrypt(payload, self._config.encrypt_key)
            except Exception as e:
                self._logger.warning(f"Failed to decrypt message from {sender}: {e}")
                return

        # Decompress if marked as compressed
        if compressed:
            payload = codec.decompress(payload)

        # Use cards from the incoming message if present, otherwise fetch from Webex
        cards = incoming.cards if incoming.cards else await self._fetch_cards(msg_id)

        if cards:
            self._logger.info(f"Received payload for slug {to!r} from {sender} (with {len(cards)} card(s))")
        else:
            self._logger.info(f"Received payload for slug {to!r} from {sender}")
        await self._handler_callback(to, payload, cards, from_slug)

    async def on_message_with_cards(self, message: IncomingMessage) -> None:
        """Public test hook — process a message with pre-injected cards."""
        await self._on_incoming(message)

    async def _fetch_cards(self, message_id: str) -> list[dict]:
        """Fetch card attachments from the message via REST API (Webex only)."""
        if not message_id or self._config.platform != "webex":
            return []
        try:
            full_msg = await get_message(self._config.webex_token, message_id, self._session)
            return extract_cards(full_msg)
        except Exception as e:
            self._logger.debug(f"Could not fetch message attachments: {e}")
            return []
