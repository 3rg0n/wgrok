"""Run the wgrok router bot. Can run on any host with network access to Webex."""

import asyncio
import sys
from pathlib import Path

from wgrok.config import BotConfig
from wgrok.router_bot import WgrokRouterBot

ENV_FILE = Path(__file__).parent / ".env.routerbot"


async def main() -> None:
    config = BotConfig.from_env(str(ENV_FILE))
    bot = WgrokRouterBot(config)
    print(f"Router bot starting (domains: {config.domains})")
    try:
        await bot.run()
    except KeyboardInterrupt:
        pass
    finally:
        await bot.stop()
        print("Router bot stopped")


if __name__ == "__main__":
    asyncio.run(main())
