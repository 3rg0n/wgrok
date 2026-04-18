# wgrok

**v1.3.0** | [PyPI](https://pypi.org/project/wgrok/) | [npm](https://www.npmjs.com/package/wgrok-message-bus) | [crates.io](https://crates.io/crates/wgrok) | [Go](https://pkg.go.dev/github.com/3rg0n/wgrok/go)

A message bus protocol over social messaging platforms. Uses platform APIs (Webex, Slack, Discord) as transport to allow agents, services, and orchestrators to communicate across network boundaries without inbound webhooks.

## Install

```bash
pip install wgrok                          # Python
npm install wgrok-message-bus              # TypeScript
cargo add wgrok                            # Rust
go get github.com/3rg0n/wgrok/go@v1.3.0   # Go
```

## Protocol

All wgrok messages use a four-field colon-delimited format:

```text
./echo:{to}:{from}:{flags}:{payload}
```

- **`to`** — destination slug (which agent should process this)
- **`from`** — sender identifier (return address, or `-` for anonymous)
- **`flags`** — compression/encryption/chunking metadata (`-`, `z`, `e`, `ze`, `1/3`, `z2/5`, `ze1/3`)
- **`payload`** — message body (can contain colons)

The `./echo:` prefix signals "this is a wgrok message". The router bot strips it and relays the remaining four fields transparently.

The payload is opaque — wgrok never inspects or transforms it. JSON, CSV, NDJSON, plain text, base64, YAML — whatever the sender puts in, the receiver gets out verbatim.

### Wire format

```text
Sending (echo):    ./echo:{to}:{from}:{flags}:{payload}
Receiving (response): {to}:{from}:{flags}:{payload}
```

The router bot strips the `./echo:` prefix, routes on the `to` field, and passes through `from` and `flags` unchanged.

### Flags

| Flag | Meaning |
|------|---------|
| `-` | No compression, no encryption, no chunking |
| `z` | Payload is gzip+base64 compressed |
| `e` | Payload is AES-256-GCM encrypted |
| `ze` | Compressed then encrypted |
| `1/3` | Chunk 1 of 3 (uncompressed) |
| `z2/5` | Chunk 2 of 5 (compressed then chunked) |
| `e1/3` | Chunk 1 of 3 (encrypted) |
| `ze2/5` | Chunk 2 of 5 (compressed, encrypted, chunked) |

Flags are ordered: `[z][e][N/M]`. Compression, encryption, and chunking are handled automatically by the sender/receiver libraries. The router bot never inspects flags.

### Mode B — Agent Bus

Agents share the same messaging token — all see all messages. The `to` field identifies which agent should process the result.

```text
Sender ──./echo:deploy-agent:sender:-:start deploy──► Router Bot
                                                          │
Receiver ◄──deploy-agent:sender:-:start deploy──────── Router Bot
(only processes because to="deploy-agent" matches its slug)
```

Simple, lightweight. Best for same trust zone, same network boundary. No registry needed — agents self-select by slug.

### Mode C — Registered Agents

Agents have their own bot identities and register with the routing bot. The routing bot maintains a registry mapping slugs to bot identities. Agents don't need shared tokens. Cross-platform routing is possible.

```text
Routing bot registry:
  deploy = deploy-bot@spark.com
  status = status-bot@foo.com

Sender ──./echo:deploy:myagent:-:start──► Routing Bot
                                              │ (looks up "deploy" → deploy-bot@spark.com)
deploy-bot@spark.com ◄──deploy:myagent:-:start── Routing Bot
```

The `from` field tells the receiving agent who sent the message, enabling reply-to patterns.

### Protocol layers

```text
Echo (sent):      ./echo:{to}:{from}:{flags}:{payload}   ← what senders produce
Response (recv):  {to}:{from}:{flags}:{payload}           ← what receivers consume
```

Mode B and Mode C share the same wire format. The difference is how the routing bot resolves `to`:

- **Mode B**: echo back to sender (agents self-select by slug)
- **Mode C**: look up `to` in registry, route to registered bot
- **Fallback**: registered slugs get routed, unregistered slugs get echoed — Mode B and C coexist on the same routing bot

### Codec

The sender/receiver libraries include a built-in codec for large payloads:

- **Compression**: `compress=True` gzips the payload and base64-encodes it. The `z` flag tells the receiver to decompress.
- **Auto-chunking**: When a message exceeds the platform limit, the sender splits it into numbered chunks (`1/3`, `2/3`, `3/3`). The receiver buffers and reassembles automatically.

| Platform | Message limit |
|----------|--------------|
| Webex | 7,439 bytes |
| Slack | 4,000 chars |
| Discord | 2,000 chars |
| IRC | 400 bytes |

Codec is transparent — application code sends and receives full payloads without knowing about compression or chunking.

### Encryption

Optional AES-256-GCM encryption for payload confidentiality. On/off mode — set `WGROK_ENCRYPT_KEY` to enable, omit to disable.

```env
# Generate a 32-byte key (base64-encoded)
WGROK_ENCRYPT_KEY=$(openssl rand -base64 32)
```

When the key is configured:

- **Sender** auto-encrypts every outgoing payload and sets the `e` flag
- **Receiver** auto-decrypts when it sees the `e` flag
- **Router bot** relays encrypted payloads transparently (never inspects them)

Pipeline order: compress → encrypt → base64 → chunk (send), reassemble → base64 → decrypt → decompress (receive).

Wire format: `base64(12-byte IV || ciphertext || 16-byte GCM tag)`. Each message gets a random IV. The GCM tag provides both authenticity and integrity — tampered messages are rejected.

The same key must be shared between sender and receiver. The router bot does not need the key.

| Language | Crypto library |
|----------|---------------|
| Python | `cryptography` (AES-NI accelerated via OpenSSL) |
| Go | `crypto/aes` + `crypto/cipher` (stdlib, AES-NI accelerated) |
| TypeScript | `node:crypto` (AES-NI accelerated via OpenSSL) |
| Rust | `aes-gcm` crate (AES-NI accelerated) |

## Use cases

**App routing** — One bot, every internal API behind it. `./jira:...`, `./deploy:...`, `./grafana:...` — developers integrate with one SDK instead of building individual integrations for each service.

**Firewall traversal** — GitHub Actions orchestrator talks to on-prem orchestrator. Both share a token, router bot relays through Webex. `./echo:{to}:{from}:{flags}:{payload}` traverses the firewall without inbound ports.

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

# Encryption (optional — on/off, same key on sender + receiver)
WGROK_ENCRYPT_KEY=<base64-encoded 32-byte key>

# Optional
WGROK_DEBUG=true
WGROK_PROXY=http://proxy.corp.com:8080
```

### 3. Use in your project

**Python:**

```python
from wgrok import WgrokSender, WgrokReceiver, SenderConfig, ReceiverConfig

# Send — returns SendResult(message_id, message_ids, platform_response, buffered)
sender = WgrokSender(SenderConfig.from_env())
result = await sender.send("hello world")
print(result.message_id)
await sender.close()

# Receive — handler receives a MessageContext as its 5th argument
async def handler(slug, payload, cards, from_slug, ctx):
    print(f"[{ctx.platform}] {ctx.sender} in {ctx.room_id}: {payload}")

receiver = WgrokReceiver(ReceiverConfig.from_env(), handler)
await receiver.listen()
```

**Go:**

```go
import wgrok "github.com/3rg0n/wgrok/go"

// Send — returns *SendResult
cfg, _ := wgrok.SenderConfigFromEnv()
sender := wgrok.NewSender(cfg)
result, _ := sender.Send("hello world", nil)
fmt.Println(result.MessageID)

// Receive — handler takes a MessageContext as its 5th argument
rcfg, _ := wgrok.ReceiverConfigFromEnv()
receiver := wgrok.NewReceiver(rcfg, func(slug, payload string, cards []interface{}, fromSlug string, ctx wgrok.MessageContext) {
    fmt.Printf("[%s] %s in %s: %s\n", ctx.Platform, ctx.Sender, ctx.RoomID, payload)
})
receiver.Listen(ctx)
```

**TypeScript:**

```typescript
import { WgrokSender, WgrokReceiver, senderConfigFromEnv, receiverConfigFromEnv } from 'wgrok-message-bus';

// Send — returns SendResult { messageId, messageIds, platformResponse, buffered }
const sender = new WgrokSender(senderConfigFromEnv());
const result = await sender.send('hello world');
console.log(result.messageId);

// Receive — handler takes a MessageContext as its 5th argument
const receiver = new WgrokReceiver(receiverConfigFromEnv(), (slug, payload, cards, fromSlug, ctx) => {
  console.log(`[${ctx.platform}] ${ctx.sender} in ${ctx.roomId}: ${payload}`);
});
await receiver.listen();
```

**Rust:**

```rust
use wgrok::{WgrokSender, WgrokReceiver, SenderConfig, ReceiverConfig, MessageContext};

// Send — returns SendResult { message_id, message_ids, platform_response, buffered }
let cfg = SenderConfig::from_env()?;
let sender = WgrokSender::new(cfg);
let result = sender.send("hello world", None).await?;
println!("{:?}", result.message_id);

// Receive — handler takes a &MessageContext as its 5th argument
let cfg = ReceiverConfig::from_env()?;
let receiver = WgrokReceiver::new(cfg, Box::new(|slug, payload, cards, from_slug, ctx: &MessageContext| {
    println!("[{}] {} in {}: {}", ctx.platform, ctx.sender, ctx.room_id, payload);
}));
receiver.listen(shutdown_rx).await?;
```

### SendResult

`send()` returns a `SendResult` with normalized fields across platforms:

| Field | Type | Description |
|-------|------|-------------|
| `message_id` | string \| null | First platform message ID (Webex `id`, Slack `ts`, Discord `id`, IRC `None`) |
| `message_ids` | string[] | All IDs when chunking (one per chunk) |
| `platform_response` | object | Raw platform response for callers that need it |
| `buffered` | bool | `true` when the send was buffered due to pause state |

### MessageContext

The receiver handler's 5th argument carries platform metadata for replies, log correlation, and routing:

| Field | Description |
|-------|-------------|
| `msg_id` | Platform message ID |
| `sender` | Sender email / user ID |
| `platform` | `"webex"`, `"slack"`, `"discord"`, or `"irc"` |
| `room_id` | Room / channel ID |
| `room_type` | `"direct"`, `"group"`, or `""` when unknown |

## Transport bindings

The protocol is transport-agnostic. Each platform is a transport binding with a send API and a receive mechanism:

| Platform | Send | Receive (Persistent) | Receive (Webhook) | Status |
|----------|------|---------------------|--------------------|--------|
| Webex | REST `/v1/messages` | Mercury WebSocket | Webhook registration | Send + Receive |
| Slack | `chat.postMessage` | Socket Mode WebSocket | Events API | Send + Receive |
| Discord | REST `/channels/{id}/messages` | Gateway WebSocket | Interactions endpoint | Send + Receive |
| IRC | `PRIVMSG` | Persistent TCP/TLS | N/A | Send + Receive |

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
WGROK_WEBHOOK_SECRET=shared-secret   # required when WGROK_WEBHOOK_PORT is set
```

`WGROK_WEBHOOK_SECRET` is mandatory when the webhook port is configured — the router bot refuses to start without it. Request bodies are limited to 1 MB.

When enabled, the routing bot starts an HTTP server that accepts `POST` requests. This is useful for:

- **Platforms that prefer webhooks** over WebSocket (e.g., Teams Bot Framework)
- **Non-chat integrations** (CI/CD, monitoring, cron jobs) that want to post to the bus without a messaging platform token
- **High-throughput environments** where webhook is more efficient than WebSocket

```http
POST /wgrok HTTP/1.1
Authorization: Bearer <WGROK_WEBHOOK_SECRET>
Content-Type: application/json

{
  "text": "./echo:deploy:ci-pipeline:-:start deploy",
  "from": "ci-pipeline@example.com"
}
```

The webhook endpoint processes messages through the same pipeline as WebSocket messages — allowlist check, protocol parsing, routing.

## Allowlist / ACL

The `WGROK_DOMAINS` environment variable controls who can send messages through the system. Granular access control at the library level:

| Pattern | Matches |
|---------|---------|
| `example.com` | Any `*@example.com` (bare domain) |
| `*@example.com` | Any `*@example.com` (wildcard prefix) |
| `user@example.com` | Exact match only (case-insensitive) |

Patterns containing `[`, `]`, or `?` are rejected. All matching is case-insensitive.

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

**In scope:** message bus protocol (three modes), sender/relay/receiver libraries, agent registry, allowlist/ACL, `.env` configuration, multi-platform multi-token support, webhook endpoint, outbound proxy support, optional AES-256-GCM encryption.

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
go run ./cmd/routerbot
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

```text
tests/
├── protocol_cases.json
├── codec_cases.json
├── allowlist_cases.json
├── config_cases.json
├── webex_cases.json
├── slack_cases.json
├── discord_cases.json
├── irc_cases.json
├── platform_dispatch_cases.json
├── sender_cases.json
├── router_bot_cases.json
└── receiver_cases.json
```

Fix a bug in a test case — it applies to all languages. Add a new case — all languages must pass it.

## Project structure

```text
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

## Security

- **Allowlist enforcement** on all message paths (WebSocket and webhook)
- **AES-256-GCM encryption** (optional, end-to-end, router-transparent)
- **Webhook authentication** mandatory when webhook endpoint is enabled
- **Payload redaction** in logs — metadata only (slug, from, target, length), never payload content
- **Structured log correlation** — NDJSON log lines carry `slug`, `sender`, `msg_id`, and chunk fields for traceable audit without payload exposure
- **Security event logging** always emitted (WARN/ERROR) regardless of debug mode
- **Chunk validation** — sequence indices verified before reassembly, 5-minute timeout eviction
- **Fail-closed crypto** — decrypt/decompress errors reject the message, never pass through broken data
- **1 MB request size limit** on webhook endpoint
- **Dependencies pinned** with version ranges; GitHub Actions pinned to commit SHAs

See [THREAT_MODEL.md](THREAT_MODEL.md) for the full MAESTRO threat model.

## License

MIT

## Contributing

Contributions welcome. All four language implementations must maintain feature parity — if you add a feature to one language, add it to all four with shared test cases in `tests/`.
