"""Run the wgrok echo bot. Can run on any host with network access to Webex."""

import asyncio
import sys
from pathlib import Path

from wgrok.config import BotConfig
from wgrok.echo_bot import WgrokEchoBot

ENV_FILE = Path(__file__).parent / ".env.echobot"


async def main() -> None:
    config = BotConfig.from_env(str(ENV_FILE))
    bot = WgrokEchoBot(config)
    print(f"Echo bot starting (domains: {config.domains})")
    try:
        await bot.run()
    except KeyboardInterrupt:
        pass
    finally:
        await bot.stop()
        print("Echo bot stopped")


if __name__ == "__main__":
    asyncio.run(main())
