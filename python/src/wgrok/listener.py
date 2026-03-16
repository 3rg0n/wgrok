"""Platform listener abstraction — normalized message receiving across transports.

Each platform listener connects via its native mechanism (WebSocket, TCP, etc.)
and emits messages through a common callback interface.
"""

from __future__ import annotations

import asyncio
import contextlib
import json
import re
from dataclasses import dataclass
from typing import Any

import aiohttp
from webex_message_handler import WebexMessageHandler, WebexMessageHandlerConfig

from .logging import NdjsonLogger, NoopLogger

Logger = NdjsonLogger | NoopLogger


@dataclass
class IncomingMessage:
    """Normalized incoming message from any platform."""

    sender: str
    text: str
    msg_id: str
    platform: str
    cards: list[dict]


MessageCallback = Any  # Callable[[IncomingMessage], Awaitable[None]]


class PlatformListener:
    """Base class for platform listeners."""

    async def connect(self) -> None:
        raise NotImplementedError

    async def disconnect(self) -> None:
        raise NotImplementedError

    def on_message(self, callback: MessageCallback) -> None:
        raise NotImplementedError


class WebexListener(PlatformListener):
    """Webex listener using webex-message-handler (Mercury WebSocket + KMS)."""

    def __init__(self, token: str, logger: Logger) -> None:
        self._token = token
        self._logger = logger
        self._handler: WebexMessageHandler | None = None
        self._callback: MessageCallback | None = None

    def on_message(self, callback: MessageCallback) -> None:
        self._callback = callback

    async def connect(self) -> None:
        config = WebexMessageHandlerConfig(token=self._token, logger=self._logger)
        self._handler = WebexMessageHandler(config)

        @self._handler.on("message:created")
        async def _on_msg(msg) -> None:
            if self._callback:
                incoming = IncomingMessage(
                    sender=msg.person_email,
                    text=(msg.text or "").strip(),
                    msg_id=msg.id,
                    platform="webex",
                    cards=[],
                )
                await self._callback(incoming)

        await self._handler.connect()

    async def disconnect(self) -> None:
        if self._handler:
            await self._handler.disconnect()
            self._handler = None


SLACK_SOCKET_MODE_URL = "https://slack.com/api/apps.connections.open"


class SlackListener(PlatformListener):
    """Slack listener using Socket Mode WebSocket.

    Requires an app-level token (xapp-*) for the WebSocket connection.
    """

    def __init__(self, token: str, logger: Logger) -> None:
        self._token = token
        self._logger = logger
        self._callback: MessageCallback | None = None
        self._ws: aiohttp.ClientWebSocketResponse | None = None
        self._session: aiohttp.ClientSession | None = None
        self._running = False

    def on_message(self, callback: MessageCallback) -> None:
        self._callback = callback

    async def connect(self) -> None:
        self._session = aiohttp.ClientSession()

        # Request a WebSocket URL via apps.connections.open
        async with self._session.post(
            SLACK_SOCKET_MODE_URL,
            headers={"Authorization": f"Bearer {self._token}"},
        ) as resp:
            resp.raise_for_status()
            data = await resp.json()
            if not data.get("ok"):
                raise ConnectionError(f"Slack apps.connections.open failed: {data.get('error', 'unknown')}")
            ws_url = data["url"]

        self._ws = await self._session.ws_connect(ws_url)
        self._running = True
        self._logger.info("Slack Socket Mode connected")

        # Start reading messages in background
        asyncio.create_task(self._read_loop())

    async def _read_loop(self) -> None:
        """Read Socket Mode events and dispatch message events."""
        while self._running and self._ws and not self._ws.closed:
            try:
                msg = await self._ws.receive()
            except Exception:
                break

            if msg.type == aiohttp.WSMsgType.TEXT:
                await self._handle_event(msg.data)
            elif msg.type in (aiohttp.WSMsgType.CLOSED, aiohttp.WSMsgType.ERROR):
                break

    async def _handle_event(self, raw: str) -> None:
        """Handle a Socket Mode envelope."""
        try:
            envelope = json.loads(raw)
        except json.JSONDecodeError:
            return

        # Acknowledge the envelope
        envelope_id = envelope.get("envelope_id")
        if envelope_id and self._ws and not self._ws.closed:
            await self._ws.send_json({"envelope_id": envelope_id})

        event_type = envelope.get("type")
        if event_type != "events_api":
            return

        payload = envelope.get("payload", {})
        event = payload.get("event", {})
        if event.get("type") != "message":
            return

        # Skip bot messages to avoid loops
        if event.get("bot_id"):
            return

        if self._callback:
            incoming = IncomingMessage(
                sender=event.get("user", ""),
                text=(event.get("text", "") or "").strip(),
                msg_id=event.get("ts", ""),
                platform="slack",
                cards=[],
            )
            await self._callback(incoming)

    async def disconnect(self) -> None:
        self._running = False
        if self._ws and not self._ws.closed:
            await self._ws.close()
            self._ws = None
        if self._session:
            await self._session.close()
            self._session = None
        self._logger.info("Slack listener disconnected")


DISCORD_GATEWAY_URL = "wss://gateway.discord.gg/?v=10&encoding=json"
DISCORD_GATEWAY_API = "https://discord.com/api/v10/gateway"

# Discord Gateway opcodes
_OP_DISPATCH = 0
_OP_HEARTBEAT = 1
_OP_IDENTIFY = 2
_OP_HELLO = 10
_OP_HEARTBEAT_ACK = 11

# Intents: GUILD_MESSAGES (1 << 9) + MESSAGE_CONTENT (1 << 15)
_INTENTS = (1 << 9) | (1 << 15)


class DiscordListener(PlatformListener):
    """Discord listener using Gateway WebSocket."""

    def __init__(self, token: str, logger: Logger) -> None:
        self._token = token
        self._logger = logger
        self._callback: MessageCallback | None = None
        self._ws: aiohttp.ClientWebSocketResponse | None = None
        self._session: aiohttp.ClientSession | None = None
        self._running = False
        self._heartbeat_task: asyncio.Task | None = None
        self._sequence: int | None = None

    def on_message(self, callback: MessageCallback) -> None:
        self._callback = callback

    async def connect(self) -> None:
        self._session = aiohttp.ClientSession()

        # Get gateway URL
        async with self._session.get(DISCORD_GATEWAY_API) as resp:
            resp.raise_for_status()
            data = await resp.json()
            gw_url = data.get("url", "wss://gateway.discord.gg")

        ws_url = f"{gw_url}/?v=10&encoding=json"
        self._ws = await self._session.ws_connect(ws_url)
        self._running = True

        # Wait for Hello (opcode 10)
        hello_msg = await self._ws.receive_json()
        if hello_msg.get("op") != _OP_HELLO:
            raise ConnectionError(f"Expected Hello (op 10), got op {hello_msg.get('op')}")

        heartbeat_interval = hello_msg["d"]["heartbeat_interval"] / 1000.0

        # Start heartbeat
        self._heartbeat_task = asyncio.create_task(self._heartbeat_loop(heartbeat_interval))

        # Send Identify
        await self._ws.send_json({
            "op": _OP_IDENTIFY,
            "d": {
                "token": self._token,
                "intents": _INTENTS,
                "properties": {
                    "os": "linux",
                    "browser": "wgrok",
                    "device": "wgrok",
                },
            },
        })

        self._logger.info("Discord Gateway connected")

        # Start reading events in background
        asyncio.create_task(self._read_loop())

    async def _heartbeat_loop(self, interval: float) -> None:
        """Send periodic heartbeats to keep the connection alive."""
        while self._running:
            await asyncio.sleep(interval)
            if self._ws and not self._ws.closed:
                await self._ws.send_json({"op": _OP_HEARTBEAT, "d": self._sequence})

    async def _read_loop(self) -> None:
        """Read Gateway events and dispatch MESSAGE_CREATE."""
        while self._running and self._ws and not self._ws.closed:
            try:
                msg = await self._ws.receive()
            except Exception:
                break

            if msg.type == aiohttp.WSMsgType.TEXT:
                try:
                    data = json.loads(msg.data)
                except json.JSONDecodeError:
                    continue

                # Track sequence number for heartbeat
                if data.get("s") is not None:
                    self._sequence = data["s"]

                op = data.get("op")
                if op == _OP_DISPATCH and data.get("t") == "MESSAGE_CREATE":
                    await self._handle_message_create(data.get("d", {}))

            elif msg.type in (aiohttp.WSMsgType.CLOSED, aiohttp.WSMsgType.ERROR):
                break

    async def _handle_message_create(self, event: dict) -> None:
        """Handle a MESSAGE_CREATE dispatch event."""
        # Skip bot messages
        author = event.get("author", {})
        if author.get("bot"):
            return

        if self._callback:
            # Build embeds list as "cards"
            embeds = event.get("embeds", [])
            incoming = IncomingMessage(
                sender=author.get("id", ""),
                text=(event.get("content", "") or "").strip(),
                msg_id=event.get("id", ""),
                platform="discord",
                cards=embeds,
            )
            await self._callback(incoming)

    async def disconnect(self) -> None:
        self._running = False
        if self._heartbeat_task:
            self._heartbeat_task.cancel()
            with contextlib.suppress(asyncio.CancelledError):
                await self._heartbeat_task
            self._heartbeat_task = None
        if self._ws and not self._ws.closed:
            await self._ws.close()
            self._ws = None
        if self._session:
            await self._session.close()
            self._session = None
        self._logger.info("Discord listener disconnected")


_PRIVMSG_RE = re.compile(r"^:([^!]+)![^ ]+ PRIVMSG ([^ ]+) :(.+)$")


class IrcListener(PlatformListener):
    """IRC listener using persistent TCP/TLS connection.

    Parses incoming PRIVMSG lines and emits them as normalized messages.
    """

    def __init__(self, conn_str: str, logger: Logger) -> None:
        from .irc import IrcConnection

        self._conn = IrcConnection(conn_str)
        self._logger = logger
        self._callback: MessageCallback | None = None
        self._running = False

    def on_message(self, callback: MessageCallback) -> None:
        self._callback = callback

    async def connect(self) -> None:
        await self._conn.connect()
        self._running = True
        self._logger.info(f"IRC connected to {self._conn.nick}@{self._conn.channel}")
        asyncio.create_task(self._read_loop())

    async def _read_loop(self) -> None:
        """Read IRC lines and dispatch PRIVMSG events."""
        reader = self._conn._reader
        while self._running and reader:
            try:
                line = await asyncio.wait_for(reader.readline(), timeout=300)
            except asyncio.TimeoutError:
                # Send a PING to keep connection alive
                if self._conn.connected:
                    await self._conn._send_raw("PING :keepalive")
                continue
            except Exception:
                break

            if not line:
                break

            decoded = line.decode("utf-8", errors="replace").strip()
            if not decoded:
                continue

            # Handle server PING
            if decoded.startswith("PING"):
                pong_arg = decoded[5:] if len(decoded) > 5 else ""
                await self._conn._send_raw(f"PONG {pong_arg}")
                continue

            # Parse PRIVMSG
            match = _PRIVMSG_RE.match(decoded)
            if match and self._callback:
                nick, target, text = match.groups()
                incoming = IncomingMessage(
                    sender=nick,
                    text=text.strip(),
                    msg_id="",
                    platform="irc",
                    cards=[],
                )
                await self._callback(incoming)

    async def disconnect(self) -> None:
        self._running = False
        await self._conn.disconnect()
        self._logger.info("IRC listener disconnected")


def create_listener(platform: str, token: str, logger: Logger) -> PlatformListener:
    """Factory: create the right listener for a platform."""
    if platform == "webex":
        return WebexListener(token, logger)
    if platform == "slack":
        return SlackListener(token, logger)
    if platform == "discord":
        return DiscordListener(token, logger)
    if platform == "irc":
        return IrcListener(token, logger)
    raise ValueError(f"Unsupported platform: {platform}")
