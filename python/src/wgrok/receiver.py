"""WgrokReceiver - listens for response messages, matches slug, invokes handler callback."""

from __future__ import annotations

import asyncio
import time
from collections.abc import Awaitable, Callable
from dataclasses import dataclass

import aiohttp

from . import codec
from .allowlist import Allowlist
from .config import ReceiverConfig
from .listener import IncomingMessage, PlatformListener, create_listener
from .logging import get_logger
from .protocol import is_pause, is_resume, parse_flags, parse_response, strip_bot_mention
from .webex import extract_cards, get_attachment_action, get_message


@dataclass
class MessageContext:
    """Platform metadata passed to the receiver handler."""

    msg_id: str
    sender: str
    platform: str
    room_id: str
    room_type: str


MessageHandler = Callable[[str, str, list[dict], str, MessageContext], Awaitable[None]]
ControlHandler = Callable[[str], None] | None


class WgrokReceiver:
    """Listens for wgrok response messages and optional card attachments.

    The handler callback receives (slug, payload, cards) where cards is a
    list of adaptive card content dicts (empty if no cards attached).
    """

    def __init__(self, config: ReceiverConfig, handler: MessageHandler, on_control: ControlHandler = None) -> None:
        self._config = config
        self._allowlist = Allowlist(config.domains)
        self._handler_callback = handler
        self._on_control = on_control
        self._logger = get_logger(config.debug, "wgrok.receiver")
        self._listener: PlatformListener | None = None
        self._session: aiohttp.ClientSession | None = None
        self._stop_event: asyncio.Event = asyncio.Event()
        self._chunk_buffer: dict[tuple[str, str], dict[int, str]] = {}
        self._chunk_timestamps: dict[tuple[str, str], float] = {}

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
        text = strip_bot_mention(incoming.text, incoming.html)
        msg_id = incoming.msg_id

        if not self._allowlist.is_allowed(sender):
            self._logger.warning(f"Rejected message from {sender}: not in allowlist", sender=sender)
            return

        if is_pause(text) or is_resume(text):
            cmd = "pause" if is_pause(text) else "resume"
            self._logger.info(f"Received {cmd} control from {sender}", sender=sender)
            if self._on_control:
                self._on_control(cmd)
            return

        try:
            to, from_slug, flags_str, payload = parse_response(text)
        except ValueError:
            self._logger.debug(f"Ignoring unparseable message from {sender}", sender=sender)
            return

        if to != self._config.slug:
            self._logger.debug(f"Ignoring message with slug {to!r} (expected {self._config.slug!r})", sender=sender)
            return

        compressed, encrypted, chunk_seq, chunk_total = parse_flags(flags_str)

        if chunk_seq is not None and chunk_total is not None:
            if chunk_total > 100 or chunk_seq > chunk_total or chunk_seq < 1:
                self._logger.warning(
                    f"Invalid chunk {chunk_seq}/{chunk_total} from {sender}",
                    slug=to, sender=sender, chunk_seq=str(chunk_seq), chunk_total=str(chunk_total),
                )
                return
            key = (sender, to)

            now = time.time()
            if key in self._chunk_timestamps:
                if now - self._chunk_timestamps[key] > 300:
                    self._logger.warning(
                        f"Discarding incomplete chunk set for {key} (timeout after 5 minutes)",
                        slug=to, sender=sender,
                    )
                    del self._chunk_buffer[key]
                    del self._chunk_timestamps[key]
                    return
            else:
                self._chunk_timestamps[key] = now

            self._chunk_buffer.setdefault(key, {})[chunk_seq] = payload
            if len(self._chunk_buffer[key]) < chunk_total:
                self._logger.debug(
                    f"Buffered chunk {chunk_seq}/{chunk_total} for slug {to!r} from {sender}",
                    slug=to, sender=sender, chunk_seq=str(chunk_seq), chunk_total=str(chunk_total),
                )
                return

            if not all(i in self._chunk_buffer[key] for i in range(1, chunk_total + 1)):
                self._logger.warning(
                    f"Incomplete chunk set for {key}: missing indices, discarding",
                    slug=to, sender=sender,
                )
                del self._chunk_buffer[key]
                del self._chunk_timestamps[key]
                return

            payload = "".join(self._chunk_buffer[key][i] for i in range(1, chunk_total + 1))
            del self._chunk_buffer[key]
            del self._chunk_timestamps[key]
            self._logger.debug(
                f"Reassembled {chunk_total} chunks for slug {to!r} from {sender}",
                slug=to, sender=sender, chunk_total=str(chunk_total),
            )

        if encrypted:
            if not self._config.encrypt_key:
                self._logger.warning(
                    f"Message from {sender} marked as encrypted but no WGROK_ENCRYPT_KEY configured — skipping",
                    slug=to, sender=sender,
                )
                return
            try:
                payload = codec.decrypt(payload, self._config.encrypt_key)
            except Exception as e:
                self._logger.warning(f"Failed to decrypt message from {sender}: {e}", slug=to, sender=sender)
                return

        if compressed:
            payload = codec.decompress(payload)

        cards = incoming.cards if incoming.cards else await self._fetch_cards(msg_id)

        if cards:
            self._logger.info(
                f"Received payload for slug {to!r} from {sender} (with {len(cards)} card(s))",
                slug=to, sender=sender, msg_id=msg_id,
            )
        else:
            self._logger.info(
                f"Received payload for slug {to!r} from {sender}",
                slug=to, sender=sender, msg_id=msg_id,
            )
        ctx = MessageContext(
            msg_id=msg_id,
            sender=sender,
            platform=incoming.platform,
            room_id=incoming.room_id,
            room_type=incoming.room_type,
        )
        await self._handler_callback(to, payload, cards, from_slug, ctx)

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
