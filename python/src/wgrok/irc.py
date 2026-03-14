"""IRC client for sending messages over TCP/TLS.

Connection string format: nick:password@server:port/channel
Example: wgrok-bot:pass@irc.libera.chat:6697/#wgrok
"""

from __future__ import annotations

import asyncio
import contextlib
import ssl


def parse_connection_string(conn_str: str) -> dict:
    """Parse an IRC connection string into components.

    Format: nick:password@server:port/channel
    """
    # Split on @ to get credentials and server parts
    if "@" not in conn_str:
        raise ValueError(f"Invalid IRC connection string (missing @): {conn_str}")

    creds, server_part = conn_str.split("@", 1)

    # Parse credentials
    if ":" in creds:
        nick, password = creds.split(":", 1)
    else:
        nick = creds
        password = ""

    # Parse server:port/channel
    if "/" in server_part:
        host_port, channel = server_part.split("/", 1)
    else:
        host_port = server_part
        channel = ""

    if ":" in host_port:
        server, port_str = host_port.rsplit(":", 1)
        port = int(port_str)
    else:
        server = host_port
        port = 6697  # Default TLS port

    return {
        "nick": nick,
        "password": password,
        "server": server,
        "port": port,
        "channel": channel,
    }


class IrcConnection:
    """Manages a persistent IRC TCP/TLS connection."""

    def __init__(self, conn_str: str) -> None:
        self._params = parse_connection_string(conn_str)
        self._reader: asyncio.StreamReader | None = None
        self._writer: asyncio.StreamWriter | None = None
        self._connected = False

    async def connect(self) -> None:
        """Establish TLS connection to IRC server and authenticate."""
        ctx = ssl.create_default_context()
        self._reader, self._writer = await asyncio.open_connection(
            self._params["server"],
            self._params["port"],
            ssl=ctx,
        )
        self._connected = True

        # Authenticate
        if self._params["password"]:
            await self._send_raw(f"PASS {self._params['password']}")
        await self._send_raw(f"NICK {self._params['nick']}")
        await self._send_raw(f"USER {self._params['nick']} 0 * :{self._params['nick']}")

        # Wait for welcome (001) or error
        await self._wait_for_welcome()

        # Join channel if specified
        if self._params["channel"]:
            await self._send_raw(f"JOIN {self._params['channel']}")

    async def _send_raw(self, line: str) -> None:
        """Send a raw IRC line."""
        if self._writer is None:
            raise ConnectionError("Not connected to IRC server")
        self._writer.write(f"{line}\r\n".encode())
        await self._writer.drain()

    async def _wait_for_welcome(self) -> None:
        """Wait for IRC RPL_WELCOME (001) response."""
        if self._reader is None:
            raise ConnectionError("Not connected to IRC server")
        while True:
            line = await asyncio.wait_for(self._reader.readline(), timeout=30)
            decoded = line.decode("utf-8", errors="replace").strip()
            if not decoded:
                continue
            # Respond to PING during connect
            if decoded.startswith("PING"):
                pong_arg = decoded[5:] if len(decoded) > 5 else ""
                await self._send_raw(f"PONG {pong_arg}")
                continue
            # Check for 001 (welcome) or error codes
            parts = decoded.split()
            if len(parts) >= 2:
                if parts[1] == "001":
                    return
                if parts[1] in ("432", "433", "436", "461", "462"):
                    raise ConnectionError(f"IRC authentication failed: {decoded}")

    async def send_message(self, target: str, text: str) -> None:
        """Send a PRIVMSG to a channel or nick."""
        # IRC messages have a max length of 512 bytes including CRLF
        # Split long messages if needed
        max_payload = 400  # Leave room for PRIVMSG prefix + CRLF
        for line in text.split("\n"):
            while len(line.encode()) > max_payload:
                # Find a safe split point
                chunk = line[:max_payload]
                await self._send_raw(f"PRIVMSG {target} :{chunk}")
                line = line[max_payload:]
            if line:
                await self._send_raw(f"PRIVMSG {target} :{line}")

    async def disconnect(self) -> None:
        """Cleanly disconnect from IRC server."""
        if self._writer and self._connected:
            with contextlib.suppress(Exception):
                await self._send_raw("QUIT :wgrok shutting down")
            self._writer.close()
            self._connected = False
            self._reader = None
            self._writer = None

    @property
    def connected(self) -> bool:
        return self._connected

    @property
    def channel(self) -> str:
        return self._params.get("channel", "")

    @property
    def nick(self) -> str:
        return self._params.get("nick", "")


async def send_message(
    conn_str: str,
    target: str,
    text: str,
    connection: IrcConnection | None = None,
) -> dict:
    """Send a text message via IRC.

    If no connection is provided, creates a temporary one.
    """
    owns_conn = connection is None
    if owns_conn:
        connection = IrcConnection(conn_str)
        await connection.connect()
    try:
        await connection.send_message(target, text)
        return {"status": "sent", "target": target}
    finally:
        if owns_conn:
            await connection.disconnect()


async def send_card(
    conn_str: str,
    target: str,
    text: str,
    card: dict,
    connection: IrcConnection | None = None,
) -> dict:
    """Send a message via IRC. Cards are not supported — only text is sent."""
    return await send_message(conn_str, target, text, connection)
