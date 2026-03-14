# wgrok

A message bus protocol over social messaging platforms. Uses platform APIs (Webex, Slack, Discord) as transport to allow agents, services, and orchestrators to communicate across network boundaries without inbound webhooks.

## Protocol

All wgrok messages are colon-delimited text prefixed with `./`:

```
./{context}:{...rest}
```

The `./` prefix signals "this is a wgrok command". What follows depends on the deployment mode.

The payload is opaque — wgrok never inspects or transforms it. JSON, CSV, NDJSON, plain text, base64, YAML — whatever the sender puts in, the receiver gets out verbatim.

### Mode A — App Routing

```
./{app}:{payload}
```

A routing bot proxies to internal REST APIs that will never be on the bus. The bot maintains a registry of apps and their internal endpoints. Services don't need to know about wgrok — the bot translates.

```
Developer ──./jira:create ticket──► Routing Bot ──► Jira REST API
Developer ◄──jira:PROJ-456 created──────────────── Routing Bot

Developer ──./github:create issue repo=foo──► Routing Bot ──► GitHub API
Developer ◄──github:issue #42 created────────────── Routing Bot
```

The developer sends one message. The bot handles the REST call, auth, pagination, error handling — whatever the target API requires.

### Mode B — Agent Bus

```
./{verb}:{slug}:{payload}
```

Agents share the same messaging token — all see all messages. The verb tells a relay bot what to do, and the slug identifies which agent should process the result.

```
Sender ──./echo:deploy-agent:start deploy──► Echo Bot
                                                │
Receiver ◄──deploy-agent:start deploy────────── Echo Bot
(only processes because slug matches "deploy-agent")
```

Simple, lightweight. Best for same trust zone, same network boundary. No registry needed — agents self-select by slug.

### Mode C — Registered Agents

```
./{verb}:{slug}:{payload}
```

Agents have their own bot identities and register with the routing bot. The routing bot maintains a registry mapping slugs to bot identities. Agents don't need shared tokens. Cross-platform routing is possible.

```
Routing bot registry:
  deploy = deploy-bot@spark.com
  status = status-bot@foo.com
  jira   = jira-agent@webex.bot

Sender ──./echo:deploy:start──► Routing Bot
                                    │ (looks up "deploy" → deploy-bot@spark.com)
deploy-bot@spark.com ◄──deploy:start── Routing Bot
```

This is how an agent joins the bus with its own identity. A Jira agent could wrap the Jira REST API and participate as a first-class bus citizen — accepting commands like create, delete, append, list, search — returning structured responses. Same for GitHub, ServiceNow, or any other service.

The difference from Mode A: Mode A proxies to dumb REST APIs. Mode C routes to smart agents that chose to participate on the bus.

### Layer 0 — Base format

```
{slug}:{payload}
```

After bot processing, the `./` prefix is stripped. Receivers always consume this format regardless of which mode produced it. Also used for direct agent-to-agent messaging without any bot.

### Protocol layers

```
Layer 0:  {slug}:{payload}                ← base: what receivers consume
Layer 1a: ./{app}:{payload}               ← Mode A: REST API proxy
Layer 1b: ./{verb}:{slug}:{payload}       ← Mode B/C: verb + slug filtering
```

Mode B and Mode C share the same wire format (Layer 1b). The difference is how the routing bot resolves the slug:
- **Mode B**: echo back to sender (agents self-select by slug)
- **Mode C**: look up slug in registry, route to registered bot
- **Fallback**: registered slugs get routed, unregistered slugs get echoed — Mode B and C coexist on the same routing bot

### Built-in verbs (Mode B / Mode C)

| Verb | Behavior | Status |
|------|----------|--------|
| `echo` | Reflect back to sender (B) or route to registered agent (C) | Implemented |
| `route` | Forward to internal system by slug | Reserved |
| `broadcast` | Fan out to all matching subscribers | Reserved |
| `store` | Persist payload for later retrieval | Reserved |

## Use cases

**App routing** — One bot, every internal API behind it. `./jira:...`, `./deploy:...`, `./grafana:...` — developers integrate with one SDK instead of building individual integrations for each service.

**Firewall traversal** — GitHub Actions orchestrator talks to on-prem orchestrator. Both share a token, echo bot relays through Webex. `./echo:{slug}:{payload}` traverses the firewall without inbound ports.

**Multi-agent pub/sub** — Multiple agents on one account, each with a unique slug. All agents see all messages but only process their own slug. NATS-style message bus over a chat platform.

**Cross-boundary agents** — Agents with their own bot identities register with the routing bot. A deploy agent on Webex, a status agent on Slack, a Jira agent wrapping the REST API — all reachable through the same routing bot. `WGROK_ROUTES` maps slugs to bot identities.

## Languages

Implemented in four languages with identical behavior and shared test cases:

| Language | Directory | Package |
|----------|-----------|---------|
| Python | `python/` | `wgrok` |
| Go | `go/` | `github.com/3rg0n/wgrok/go` |
| TypeScript | `ts/` | `wgrok` |
| Rust | `rust/` | `wgrok` |

## Quick start

### 1. Register a bot

Create a bot on your messaging platform. You need at minimum two tokens:
- One for the **bot** (the relay/routing service)
- One **shared token** for senders and receivers (Mode B), or individual tokens per agent (Mode C)

### 2. Configure environment

```bash
cp python/.env.example python/.env   # or go/, ts/, rust/
```

```env
# Sender / Receiver
WGROK_TOKEN=<shared token>
WGROK_PLATFORM=webex
WGROK_TARGET=bot@webex.bot
WGROK_SLUG=myagent
WGROK_DOMAINS=example.com

# Routing Bot (separate .env)
WGROK_WEBEX_TOKENS=token1,token2,token3
WGROK_SLACK_TOKENS=xoxb-token1,xoxb-token2
WGROK_DOMAINS=example.com

# Agent Registry (Mode C — optional)
WGROK_ROUTES=deploy:deploy-bot@spark.com,status:status-bot@foo.com

# Webhook endpoint (routing bot — optional)
WGROK_WEBHOOK_PORT=8080
WGROK_WEBHOOK_SECRET=shared-secret

# Optional
WGROK_DEBUG=true
WGROK_PROXY=http://proxy.corp.com:8080
```

### 3. Use in your project

**Python**
```python
from wgrok import WgrokSender, WgrokReceiver, SenderConfig, ReceiverConfig

# Send
sender = WgrokSender(SenderConfig.from_env())
await sender.send("hello world")
await sender.close()

# Receive
async def handler(slug, payload, cards):
    print(f"Got: {payload}")

receiver = WgrokReceiver(ReceiverConfig.from_env(), handler)
await receiver.listen()
```

**Go**
```go
import wgrok "github.com/3rg0n/wgrok/go"

// Send
cfg, _ := wgrok.SenderConfigFromEnv()
sender := wgrok.NewSender(cfg)
sender.Send("hello world", nil)

// Receive
rcfg, _ := wgrok.ReceiverConfigFromEnv()
receiver := wgrok.NewReceiver(rcfg, func(slug, payload string, cards []interface{}) {
    fmt.Printf("Got: %s\n", payload)
})
receiver.Listen(ctx)
```

**TypeScript**
```typescript
import { WgrokSender, WgrokReceiver, senderConfigFromEnv, receiverConfigFromEnv } from 'wgrok';

// Send
const sender = new WgrokSender(senderConfigFromEnv());
await sender.send('hello world');

// Receive
const receiver = new WgrokReceiver(receiverConfigFromEnv(), (slug, payload, cards) => {
  console.log(`Got: ${payload}`);
});
await receiver.listen();
```

**Rust**
```rust
use wgrok::{WgrokSender, WgrokReceiver, SenderConfig, ReceiverConfig};

// Send
let cfg = SenderConfig::from_env()?;
let sender = WgrokSender::new(cfg);
sender.send("hello world", None).await?;

// Receive
let cfg = ReceiverConfig::from_env()?;
let receiver = WgrokReceiver::new(cfg, Box::new(|slug, payload, cards| {
    println!("Got: {payload}");
}));
receiver.listen(shutdown_rx).await?;
```

## Transport bindings

The protocol is transport-agnostic. Each platform is a transport binding with a send API and a receive mechanism:

| Platform | Send | Receive (Persistent) | Receive (Webhook) | Status |
|----------|------|---------------------|--------------------|--------|
| Webex | REST `/v1/messages` | Mercury WebSocket | Webhook registration | Implemented |
| Slack | `chat.postMessage` | Socket Mode WebSocket | Events API | Planned |
| Discord | REST `/channels/{id}/messages` | Gateway WebSocket | Interactions endpoint | Planned |
| IRC | `PRIVMSG` | Persistent TCP/TLS | N/A | Planned |

### Platform tokens

The routing bot supports **multiple platforms simultaneously** and **multiple tokens per platform** for load balancing:

```env
# Multiple Webex tokens (load balanced across outbound sends)
WGROK_WEBEX_TOKENS=token1,token2,token3

# Multiple Slack tokens
WGROK_SLACK_TOKENS=xoxb-token1,xoxb-token2

# Single Discord token
WGROK_DISCORD_TOKENS=bot-token1

# IRC (connection string format: nick:password@server:port/channel)
WGROK_IRC_TOKENS=wgrok-bot:pass@irc.libera.chat:6697/#wgrok
```

Each `WGROK_{PLATFORM}_TOKENS` env var accepts CSV. The routing bot:
- Opens a WebSocket listener per token (receives messages from all connected platforms)
- Load balances outbound sends across tokens for the same platform
- Routes cross-platform: a message arriving on Webex can be delivered to a Slack agent

For senders and receivers (simple case), a single token with explicit platform:

```env
WGROK_TOKEN=<token>
WGROK_PLATFORM=webex
```

`WGROK_PLATFORM` defaults to `webex` for backward compatibility.

### Webhook endpoint

The routing bot can optionally expose an HTTP webhook endpoint for environments that allow inbound traffic:

```env
WGROK_WEBHOOK_PORT=8080
WGROK_WEBHOOK_SECRET=shared-secret
```

When enabled, the routing bot starts an HTTP server that accepts `POST` requests. This is useful for:
- **Platforms that prefer webhooks** over WebSocket (e.g., Teams Bot Framework)
- **Non-chat integrations** (CI/CD, monitoring, cron jobs) that want to post to the bus without a messaging platform token
- **High-throughput environments** where webhook is more efficient than WebSocket

```
POST /wgrok HTTP/1.1
Authorization: Bearer <WGROK_WEBHOOK_SECRET>
Content-Type: application/json

{
  "text": "./echo:deploy:start deploy",
  "from": "ci-pipeline@example.com"
}
```

The webhook endpoint processes messages through the same pipeline as WebSocket messages — allowlist check, protocol parsing, routing.

## Allowlist / ACL

The `WGROK_DOMAINS` environment variable controls who can send messages through the system. Granular access control at the library level:

| Pattern | Matches |
|---------|---------|
| `example.com` | `*@example.com` |
| `*@example.com` | Any user at example.com |
| `user@example.com` | Exact match only |
| `*.com` | Any `*@*.com` (TLD match) |
| `*@*.domain.com` | Any user at any subdomain of domain.com |

All modes enforce the allowlist. The minimum configuration is a `.env` file. Developers can wrap the library with their own ACL solution (OpenBao, Postgres, LDAP, etc.) if needed.

## Agent registry

The `WGROK_ROUTES` environment variable maps slugs to bot identities for Mode C:

```env
WGROK_ROUTES=deploy:deploy-bot@spark.com,status:status-bot@foo.com,jira:jira-agent@webex.bot
```

CSV format — same whether loaded from `.env`, a database column, or an API. Parse: split on `,`, split each on first `:`.

The routing bot uses this registry to resolve slugs:
- Slug found in registry → route to registered bot (Mode C)
- Slug not found → echo back to sender (Mode B fallback)

## Proxy support

All outbound HTTP and WebSocket connections can be routed through a proxy via the `WGROK_PROXY` environment variable:

```env
WGROK_PROXY=http://proxy.corp.com:8080
```

Each language implementation wires the proxy through its native HTTP client:

| Language | Mechanism |
|----------|-----------|
| Python | `aiohttp` connector (`aiohttp_socks` / `ProxyConnector`) |
| Go | `http.Client` with proxy transport |
| TypeScript | `undici` `ProxyAgent` (passed as `dispatcher`) |
| Rust | `reqwest::Client` with proxy config |

## Scope

wgrok provides the core message bus protocol. It is deliberately minimal — wrap it with whatever you need.

**In scope:** message bus protocol (three modes), sender/relay/receiver libraries, agent registry, allowlist/ACL, `.env` configuration, multi-platform multi-token support, webhook endpoint, outbound proxy support.

**Out of scope (wrap these around the library):** secret management (OpenBao, Vault), database-backed ACLs (Postgres), observability backends (OpenTelemetry, Loki), authentication beyond the allowlist, UI/dashboards, agent command vocabularies.

## Specification

The formal protocol specification is in [`asyncapi.yaml`](asyncapi.yaml) (AsyncAPI 3.0.0).

## Build and test

### Python
```bash
cd python
pip install -e ".[dev]"
ruff check src/ tests/
pytest tests/ -v
```

### Go
```bash
cd go
go test ./... -v
go run ./cmd/sender <payload>
go run ./cmd/echobot
go run ./cmd/receiver
```

### TypeScript
```bash
cd ts
npm install
npx tsc --noEmit
npm test
```

### Rust
```bash
cd rust
cargo build
cargo test
cargo clippy
```

### Cross-language test report
```bash
bash tests/run_all.sh
```

## Testing architecture

Test cases are defined once as JSON in `tests/` and consumed by all four languages via thin shims:

```
tests/
├── protocol_cases.json
├── allowlist_cases.json
├── config_cases.json
├── webex_cases.json
├── sender_cases.json
├── echo_bot_cases.json
└── receiver_cases.json
```

Fix a bug in a test case — it applies to all languages. Add a new case — all languages must pass it.

## Project structure

```
wgrok/
├── python/           # Python SDK
│   ├── src/wgrok/    # Source modules
│   └── tests/        # Test shims
├── go/               # Go SDK
│   ├── cmd/          # Runner commands
│   └── *_test.go     # Test shims
├── ts/               # TypeScript SDK
│   ├── src/          # Source modules
│   └── tests/        # Test shims
├── rust/             # Rust SDK
│   ├── src/          # Source modules
│   └── tests/        # Test shims
├── tests/            # Shared JSON test cases
├── asyncapi.yaml     # Protocol specification
├── .plan/            # Design docs
└── README.md
```

## License

MIT

## Contributing

Contributions welcome. All four language implementations must maintain feature parity — if you add a feature to one language, add it to all four with shared test cases in `tests/`.
