"""WgrokRouterBot - listens for messages, validates allowlist, strips prefix, relays back.

Supports Mode B (echo back to sender) and Mode C (route to registered agent).
When WGROK_ROUTES is configured, registered slugs are routed to their target bot.
Unregistered slugs fall back to Mode B (echo back to sender).

Optionally exposes a webhook endpoint when WGROK_WEBHOOK_PORT is configured.
"""

from __future__ import annotations

import asyncio
from typing import Any

import aiohttp

from .allowlist import Allowlist
from .config import BotConfig
from .listener import IncomingMessage, PlatformListener, create_listener
from .logging import get_logger
from .platform import platform_send_card, platform_send_message
from .protocol import format_response, is_echo, parse_echo
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

        if self._config.webhook_port is not None:
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

        app = web.Application()
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
            if auth != expected:
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
            sender=sender, text=text, msg_id="", platform="webhook", cards=[],
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
        text = incoming.text
        msg_id = incoming.msg_id

        if not self._allowlist.is_allowed(sender):
            self._logger.warning(f"Rejected message from {sender}: not in allowlist")
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

        if cards:
            self._logger.info(f"Relaying to {target} via {platform}: {response} (with {len(cards)} card(s))")
            await platform_send_card(platform, token, target, response, cards[0], self._session)
        else:
            self._logger.info(f"Relaying to {target} via {platform}: {response}")
            await platform_send_message(platform, token, target, response, self._session)

    async def _on_message(self, message) -> None:
        """Backward-compat: process a raw dict/object message (used by tests)."""
        sender = message.person_email if hasattr(message, "person_email") else message.get("personEmail", "")
        msg_id = message.id if hasattr(message, "id") else message.get("id", "")
        raw_text = message.text if hasattr(message, "text") else message.get("text", "")
        text = (raw_text or "").strip()

        incoming = IncomingMessage(
            sender=sender, text=text, msg_id=msg_id, platform="webex", cards=[],
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
