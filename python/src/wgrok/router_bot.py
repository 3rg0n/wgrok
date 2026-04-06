"""WgrokRouterBot - listens for messages, validates allowlist, strips prefix, relays back.

Supports Mode B (echo back to sender) and Mode C (route to registered agent).
When WGROK_ROUTES is configured, registered slugs are routed to their target bot.
Unregistered slugs fall back to Mode B (echo back to sender).

Optionally exposes a webhook endpoint when WGROK_WEBHOOK_PORT is configured.
"""

from __future__ import annotations

import asyncio
import hmac
from typing import Any

import aiohttp

from .allowlist import Allowlist
from .config import BotConfig
from .listener import IncomingMessage, PlatformListener, create_listener
from .logging import get_logger
from .platform import platform_send_card, platform_send_message
from .protocol import (
    PAUSE_CMD,
    RESUME_CMD,
    format_response,
    is_echo,
    is_pause,
    is_resume,
    parse_echo,
    strip_bot_mention,
)
from .webex import extract_cards, get_message


class WgrokRouterBot:
    def __init__(self, config: BotConfig) -> None:
        self._config = config
        self._allowlist = Allowlist(config.domains)
        self._routes = config.routes
        self._logger = get_logger(config.debug, "wgrok.router_bot")
        self._listeners: list[PlatformListener] = []
        self._session: aiohttp.ClientSession | None = None
        self._stop_event: asyncio.Event = asyncio.Event()
        self._webhook_runner: Any = None
        self._paused_targets: set[str] = set()
        self._pause_buffer: dict[str, list[dict]] = {}

    async def run(self) -> None:
        """Connect to all configured platforms and listen for echo messages."""
        self._session = aiohttp.ClientSession()

        # Create a listener for each platform that has tokens configured
        pt = self._config.platform_tokens
        if not pt:
            # Backward compat: use webex_token directly
            pt = {"webex": [self._config.webex_token]}

        for platform, tokens in pt.items():
            if not tokens:
                continue
            listener = create_listener(platform, tokens[0], self._logger)
            listener.on_message(self._on_incoming)
            self._listeners.append(listener)

        self._logger.info("Router bot starting")
        if self._routes:
            for slug, target in self._routes.items():
                self._logger.info(f"Route configured: {slug!r} -> {target}")
        else:
            self._logger.info("No routes configured (Mode B: echo-back only)")

        if self._config.webhook_port is not None:
            if not self._config.webhook_secret:
                raise ValueError(
                    "WGROK_WEBHOOK_SECRET is required when WGROK_WEBHOOK_PORT is set. "
                    "Refusing to start unauthenticated webhook endpoint."
                )
            await self._start_webhook()

        # Connect all listeners
        for listener in self._listeners:
            await listener.connect()

        self._logger.info("Router bot connected")
        await self._stop_event.wait()

    async def stop(self) -> None:
        """Disconnect from all platforms and clean up."""
        self._stop_event.set()
        if self._webhook_runner:
            await self._webhook_runner.cleanup()
            self._webhook_runner = None
        for listener in self._listeners:
            await listener.disconnect()
        self._listeners.clear()
        if self._session:
            await self._session.close()
            self._session = None
        self._logger.info("Router bot stopped")

    async def _start_webhook(self) -> None:
        """Start the HTTP webhook endpoint."""
        from aiohttp import web

        app = web.Application(client_max_size=1024 * 1024)  # 1MB request size limit
        app.router.add_post("/wgrok", self._handle_webhook)
        runner = web.AppRunner(app)
        await runner.setup()
        site = web.TCPSite(runner, "0.0.0.0", self._config.webhook_port)
        await site.start()
        self._webhook_runner = runner
        self._logger.info(f"Webhook endpoint listening on port {self._config.webhook_port}")

    async def _handle_webhook(self, request) -> Any:
        """Handle an inbound webhook POST request."""
        from aiohttp import web

        # Authenticate
        if self._config.webhook_secret:
            auth = request.headers.get("Authorization", "")
            expected = f"Bearer {self._config.webhook_secret}"
            if not hmac.compare_digest(auth, expected):
                self._logger.warning("Webhook request rejected: invalid authorization")
                return web.json_response({"error": "unauthorized"}, status=401)

        try:
            body = await request.json()
        except Exception:
            return web.json_response({"error": "invalid json"}, status=400)

        text = body.get("text", "").strip()
        sender = body.get("from", "")

        if not text or not sender:
            return web.json_response({"error": "missing text or from"}, status=400)

        # Process through the same pipeline as WebSocket messages
        incoming = IncomingMessage(
            sender=sender, text=text, msg_id="", platform="webhook", cards=[], html="",
            room_id="", room_type="",
        )
        await self._on_incoming(incoming)
        return web.json_response({"status": "ok"})

    def _resolve_target(self, to: str, sender: str) -> str:
        """Resolve where to send the response. Mode C: registry lookup. Mode B: echo back to sender."""
        if to in self._routes:
            target = self._routes[to]
            self._logger.info(f"Route resolved: {to!r} -> {target}")
            return target
        return sender

    def _get_send_platform_token(self) -> tuple[str, str]:
        """Get the platform and token to use for sending.

        Uses the first available platform token pair, preferring webex for backward compatibility.
        """
        pt = self._config.platform_tokens
        for platform in ("webex", "slack", "discord", "irc"):
            if platform in pt and pt[platform]:
                return platform, pt[platform][0]
        return "webex", self._config.webex_token

    async def _on_incoming(self, incoming: IncomingMessage) -> None:
        """Process a normalized incoming message from any platform."""
        sender = incoming.sender
        text = strip_bot_mention(incoming.text, incoming.html)
        msg_id = incoming.msg_id

        if not self._allowlist.is_allowed(sender):
            self._logger.warning(f"Rejected message from {sender}: not in allowlist")
            return

        if is_pause(text):
            self._paused_targets.add(sender)
            self._pause_buffer.setdefault(sender, [])
            self._logger.info(f"Paused delivery to {sender}")
            return

        if is_resume(text):
            await self._flush_buffer(sender)
            return

        if not is_echo(text):
            self._logger.debug(f"Ignoring non-echo message from {sender}")
            return

        try:
            to, from_slug, flags_str, payload = parse_echo(text)
        except ValueError as e:
            self._logger.error(f"Failed to parse echo message: {e}")
            return

        response = format_response(to, from_slug, flags_str, payload)
        target = self._resolve_target(to, sender)
        platform, token = self._get_send_platform_token()

        # Use cards from incoming if present, otherwise fetch from Webex (only for webex platform)
        cards = incoming.cards if incoming.cards else (
            await self._fetch_cards(msg_id) if incoming.platform == "webex" else []
        )

        if target in self._paused_targets:
            buf = self._pause_buffer[target]
            if len(buf) >= 1000:
                self._logger.warning(f"Pause buffer full for {target}, dropping oldest message")
                buf.pop(0)
            buf.append({"response": response, "target": target, "cards": cards, "room_id": incoming.room_id})
            self._logger.info(f"Buffered message for paused target {target}")
            return

        # Always use roomId when available — works for both 1:1 and group rooms
        room_id = incoming.room_id

        if cards:
            self._logger.info(
                f"Relaying to {target} via {platform} [slug={to}, from={from_slug}] ({len(cards)} card(s))"
            )
            await platform_send_card(
                platform, token, target, response, cards[0], self._session, room_id=room_id,
            )
        else:
            self._logger.info(f"Relaying to {target} via {platform} [slug={to}, from={from_slug}]")
            await platform_send_message(
                platform, token, target, response, self._session, room_id=room_id,
            )

    async def _flush_buffer(self, target: str) -> None:
        """Flush buffered messages for a target and resume delivery."""
        self._paused_targets.discard(target)
        buffered = self._pause_buffer.pop(target, [])
        platform, token = self._get_send_platform_token()
        for msg in buffered:
            room_id = msg.get("room_id", "")
            if msg["cards"]:
                card = msg["cards"][0]
                await platform_send_card(
                    platform, token, msg["target"], msg["response"], card, self._session, room_id=room_id,
                )
            else:
                await platform_send_message(
                    platform, token, msg["target"], msg["response"], self._session, room_id=room_id,
                )
        self._logger.info(f"Resumed delivery to {target}, flushed {len(buffered)} message(s)")

    async def pause(self) -> None:
        """Send pause control to all registered agents (Mode C routes)."""
        platform, token = self._get_send_platform_token()
        for target in self._routes.values():
            await platform_send_message(platform, token, target, PAUSE_CMD, self._session)
            self._logger.info(f"Sent pause to {target}")

    async def resume(self) -> None:
        """Send resume control to all registered agents (Mode C routes)."""
        platform, token = self._get_send_platform_token()
        for target in self._routes.values():
            await platform_send_message(platform, token, target, RESUME_CMD, self._session)
            self._logger.info(f"Sent resume to {target}")

    async def _on_message(self, message) -> None:
        """Backward-compat: process a raw dict/object message (used by tests)."""
        sender = message.person_email if hasattr(message, "person_email") else message.get("personEmail", "")
        msg_id = message.id if hasattr(message, "id") else message.get("id", "")
        raw_text = message.text if hasattr(message, "text") else message.get("text", "")
        text = (raw_text or "").strip()

        html = getattr(message, "html", "") or ""
        room_id = getattr(message, "room_id", "") or ""
        room_type = getattr(message, "room_type", "") or ""
        incoming = IncomingMessage(
            sender=sender, text=text, msg_id=msg_id, platform="webex", cards=[], html=html,
            room_id=room_id, room_type=room_type,
        )
        await self._on_incoming(incoming)

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
