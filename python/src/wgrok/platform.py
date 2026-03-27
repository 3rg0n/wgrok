"""Platform dispatcher — routes send/receive calls to the correct transport module."""

from __future__ import annotations

from . import discord, irc, slack, webex


async def platform_send_message(
    platform: str, token: str, target: str, text: str, session=None, room_id: str = "",
) -> dict:
    """Send a text message via the specified platform."""
    if platform == "webex":
        return await webex.send_message(token, target, text, session, room_id=room_id)
    if platform == "slack":
        return await slack.send_message(token, target, text, session)
    if platform == "discord":
        return await discord.send_message(token, target, text, session)
    if platform == "irc":
        return await irc.send_message(token, target, text, session)
    raise ValueError(f"Unsupported platform: {platform}")


async def platform_send_card(
    platform: str, token: str, target: str, text: str, card: dict, session=None, room_id: str = "",
) -> dict:
    """Send a message with card/rich content via the specified platform."""
    if platform == "webex":
        return await webex.send_card(token, target, text, card, session, room_id=room_id)
    if platform == "slack":
        return await slack.send_card(token, target, text, card, session)
    if platform == "discord":
        return await discord.send_card(token, target, text, card, session)
    if platform == "irc":
        return await irc.send_card(token, target, text, card, session)
    raise ValueError(f"Unsupported platform: {platform}")
