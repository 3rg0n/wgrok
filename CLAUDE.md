# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

wgrok is an ngrok clone that uses the Webex API as a message bus. It enables agents, services, and registered endpoints to send and receive messages between systems even when firewalls or network rules prevent direct webhooks.

## Architecture

Three components, each implemented in Python, Go, Rust, and TypeScript:

1. **Sending library** - Reads `webex_token` from `.env`, wraps input with `./echo:<slug>:<payload>`, sends to the configured `target` via Webex webhook API.
2. **Echo bot** - A relay service using [webex-message-handler](https://github.com/3rg0n/webex-message-handler) for WebSocket connections. Listens for messages, checks allowlist, strips `./echo:` prefix, and sends the payload back to the originator.
3. **Receiving library** - Checks message sender against allowlist, matches slug to `.env` config, and processes or ignores the message.

## Message Protocol (v1.0)

```
Sending:   <./echo:<{slug}:payload>>
Receiving: <{slug}:<payload>>
Routing:   sender@domain > ./echo:{slug}:<payload> > echo bot strips ./echo: > sender@domain <message>
```

The `{slug}` acts as a message bus tag — agents/services only act on messages matching their configured slug.

## Environment Variables

```env
# Sender/Receiver
webex_token=<shared token>
target=bot@domain.tld

# Echo bot
webex_token=<unique token>

# Allowlist
domains=domain.tld,*@domain.tld

# Optional slug index
{slug}=agentid
```

## Build & Test Commands

### Python

```bash
cd python
pip install -e ".[dev]"        # install with dev dependencies
ruff check src/ tests/ tests_e2e/  # lint
pytest tests/ -v               # unit tests
pytest tests_e2e/ -v           # e2e tests
pytest tests/ tests_e2e/ -v    # all tests
```
