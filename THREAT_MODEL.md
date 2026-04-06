# MAESTRO Threat Model

**Project**: wgrok
**Date**: 2026-04-06
**Framework**: MAESTRO (OWASP MAS + CSA) with ASI Threat Taxonomy
**Taxonomy**: T1-T15 core, T16-T47 extended, BV-1-BV-12 blindspot vectors

## Executive Summary

wgrok is a non-AI message bus protocol implemented in Python, Go, TypeScript, and Rust. Analysis covered layers L2 (Data Operations), L4 (Deployment Infrastructure), L5 (Observability), L6 (Security & Compliance), and L7 (Agent Ecosystem), plus dependency CVE scanning. L1 (Foundation Model) and L3 (Agent Frameworks) were skipped as no AI/ML components were detected.

**17 unique findings** identified: 1 critical, 5 high, 7 medium, 4 low. **38 dependency CVEs** found (all in Python transitive deps). The most critical finding is credentials present in git history from an early commit. The highest-impact architectural issues are the untrusted `from` field enabling identity spoofing, disabled TLS certificate validation on IRC, and security events being silently discarded when debug mode is off.

No agentic risk factors (non-determinism, autonomy, agent identity, A2A communication) apply as this is a conventional message relay system, not an AI agent system.

## Scope

- **Languages**: Python, Go, TypeScript, Rust (full parity)
- **AI Components**: None
- **Entry Points**: Go CLI binaries (sender, routerbot, receiver), library packages (all 4 languages), HTTP webhook endpoint (Python only)
- **Agentic Risk Factors**: None applicable

## Risk Summary

| # | ASI Threat | Layer | Title | Severity | L | I | Risk | Framework Mapping |
|---|-----------|-------|-------|----------|---|---|------|-------------------|
| 1 | T22 | L4 | Credentials in git history | Critical | 3 | 3 | 9 | STRIDE:ID, OWASP:A07, CWE-798 |
| 2 | T43 | L4 | TLS cert validation disabled (TS IRC) | High | 2 | 3 | 6 | STRIDE:T, OWASP:A07, CWE-295 |
| 3 | T9 | L7 | Identity spoofing via untrusted `from` field | High | 3 | 2 | 6 | STRIDE:S, CWE-290 |
| 4 | T12 | L2 | Incomplete chunk reassembly (duplicate chunks) | High | 2 | 3 | 6 | STRIDE:T, CWE-754 |
| 5 | T12 | L6 | Webhook auth optional when secret not set | High | 2 | 3 | 6 | STRIDE:S, OWASP:A07, CWE-306 |
| 6 | T8/T44 | L5 | Security events discarded when debug=false | High | 3 | 2 | 6 | STRIDE:R, CWE-778 |
| 7 | T22 | L5 | Full message payloads logged at INFO level | Medium | 2 | 2 | 4 | STRIDE:ID, CWE-532 |
| 8 | T4 | L4 | Memory exhaustion via chunking/buffering | Medium | 2 | 2 | 4 | STRIDE:D, CWE-400 |
| 9 | T9 | L6 | Allowlist glob pattern bypass potential | Medium | 2 | 2 | 4 | STRIDE:S, CWE-287 |
| 10 | T37 | L7 | WGROK_ROUTES targets not validated | Medium | 1 | 3 | 3 | STRIDE:T, CWE-20 |
| 11 | T25 | L7 | Loose dependency pinning (Python/TS) | Medium | 2 | 2 | 4 | STRIDE:T, CWE-1357 |
| 12 | — | L4 | GitHub Actions not pinned to SHA | Medium | 2 | 2 | 4 | MITRE:T1195.002, CWE-829 |
| 13 | T14 | L7 | Webhook lacks rate limiting/replay protection | Medium | 2 | 2 | 4 | STRIDE:D, CWE-307 |
| 14 | T22 | L6 | No encryption key rotation mechanism | Low | 1 | 2 | 2 | CWE-321 |
| 15 | T24 | L6 | Go decompress fails open (silent passthrough) | Low | 2 | 1 | 2 | CWE-755 |
| 16 | T44 | L5 | No health check or metrics endpoints | Low | 1 | 2 | 2 | CWE-778 |
| 17 | T14 | L7 | No protocol version negotiation | Low | 1 | 1 | 1 | CWE-20 |

## Layer Analysis

### Layer 1: Foundation Model

No AI/LLM components detected — layer not applicable.

### Layer 2: Data Operations

**F4 — Incomplete Chunk Reassembly (T12, CWE-754) — HIGH**

All four receiver implementations use a count-based check (`len(buffer) == chunk_total`) rather than verifying all expected chunk indices (1 through total) are present. An attacker can send duplicate chunk sequence numbers to reach the count threshold while omitting other chunks:

- Python: `KeyError` crash during reassembly (`range(1, chunk_total + 1)`)
- Go: Nil map access / silent corruption
- TypeScript: `undefined` concatenated into payload
- Rust: Silent data loss (missing chunks skipped)

**Files**: `python/src/wgrok/receiver.py:111-123`, `go/receiver.go:159-179`, `ts/src/receiver.ts:116-138`, `rust/src/receiver.rs`

**Mitigation**: Track received chunk indices as a set. Before reassembly, verify `received == {1..total}`. Add chunk timeout (discard incomplete sets after 5 minutes).

---

**F8 — Memory Exhaustion via Chunking (T4, CWE-400) — MEDIUM**

Chunk validation accepts `total` up to 999 without per-sender aggregate limits. An attacker can open 999 chunks x 7KB per slug, repeat across slugs, exhausting memory. Pause buffers are capped at 1000 messages per target but have no aggregate memory limit across all targets.

**Mitigation**: Cap `chunk_total` to 100. Add per-sender aggregate buffer limit. Add chunk timeout eviction.

### Layer 3: Agent Frameworks

No AI/ML components detected — layer not applicable.

### Layer 4: Deployment Infrastructure

**F1 — Credentials in Git History (T22, CWE-798) — CRITICAL**

Two Webex API tokens were committed in the initial commit (`927442d`) in `.env` and `python/.env.*` files. While `.gitignore` now excludes `.env` files and they are not currently tracked, the tokens remain in git history.

**Files**: `.env` (commit 927442d), `python/.env.sender`, `python/.env.echobot`, `python/.env.receiver`

**Mitigation**: 1) Immediately revoke both Webex tokens. 2) Use `git-filter-repo` or BFG to remove from history. 3) Force-push cleaned history. 4) Verify `.gitignore` covers all `.env*` patterns (already in place).

---

**F2 — TLS Certificate Validation Disabled on IRC (T43, CWE-295) — HIGH**

TypeScript IRC listener explicitly sets `rejectUnauthorized: false`, disabling all TLS certificate verification. This enables MITM attacks on IRC connections, exposing credentials and message content.

**File**: `ts/src/listener.ts:429`

```typescript
rejectUnauthorized: false,  // Disables all cert verification
```

**Note**: Python (`ssl.create_default_context()`) and Go (`tls.Config{ServerName: ...}`) IRC implementations correctly validate certificates.

**Mitigation**: Remove `rejectUnauthorized: false`. Use default Node.js TLS validation.

---

**F12 — GitHub Actions Not Pinned to SHA (CWE-829) — MEDIUM**

All workflow files use major version tags (`@v4`, `@v5`, `@stable`) instead of commit SHAs. A compromised action maintainer could inject code into CI/CD.

**Files**: `.github/workflows/publish.yml`, `codeql.yml`, `dependency-review.yml`

**Mitigation**: Pin all actions to full commit SHA. Use Dependabot to receive SHA update PRs.

### Layer 5: Evaluation & Observability

**F6 — Security Events Discarded When debug=false (T8/T44, CWE-778) — HIGH**

All four implementations use a `NoopLogger` when `WGROK_DEBUG` is not set. This silently discards:
- Allowlist rejections (unauthorized senders)
- Webhook authentication failures
- Decryption/decompression failures
- Invalid chunk sequences
- Rate limit events

In production (debug=false), there is zero audit trail for security-relevant events. An attacker's failed attempts leave no trace.

**Files**: `python/src/wgrok/logging.py:38-51` (NoopLogger), equivalent in all languages

**Mitigation**: Always log security events (allowlist rejections, auth failures, crypto errors) regardless of debug mode. Create a separate security log category that is never suppressed.

---

**F7 — Full Message Payloads Logged (T22, CWE-532) — MEDIUM**

When debug is enabled, sender and router bot log full protocol-wrapped messages at INFO level, including payload content, recipient emails, and routing topology.

**Files**: `go/sender.go:122`, `go/router_bot.go:222-234`, `python/src/wgrok/sender.py:91`, `ts/src/sender.ts:72`

**Mitigation**: Log only metadata (slug, flags, chunk info, target) — never payload content. Redact email addresses in logs.

### Layer 6: Security & Compliance

**F5 — Webhook Authentication Optional (T12, CWE-306) — HIGH**

When `WGROK_WEBHOOK_SECRET` is not configured, the Python webhook endpoint accepts unauthenticated POST requests, allowing anyone with network access to inject arbitrary messages into the bus.

**File**: `python/src/wgrok/router_bot.py:110-116`

**Mitigation**: Make webhook authentication mandatory. If `WGROK_WEBHOOK_SECRET` is not set, refuse to start the webhook listener. Add allowlist enforcement on webhook sender field.

---

**F9 — Allowlist Glob Pattern Bypass (T9, CWE-287) — MEDIUM**

All implementations use glob/fnmatch matching (`*`, `?`, `[a-z]`) for allowlist patterns. Character class syntax (`[...]`) and `?` wildcards could enable unintended matches. Additionally, Unicode normalization differences across languages could allow email spoofing.

**Files**: `python/src/wgrok/allowlist.py:29`, `go/allowlist.go:39`, `ts/src/allowlist.ts:17`, `rust/src/allowlist.rs:26`

**Mitigation**: Restrict allowlist patterns to three modes only: exact email, `*@domain`, or bare domain. Reject patterns containing `[`, `]`, or `?`.

---

**F15 — Go Decompress Fails Open (T24, CWE-755) — LOW**

Go's `Decompress()` silently returns the original data on any error (bad base64, bad gzip). Python, TypeScript, and Rust correctly raise errors. This could mask tampering if the `z` flag is set but decompression silently fails.

**File**: `go/codec.go:30-48`

**Mitigation**: Return error instead of passthrough when `z` flag is set.

### Layer 7: Agent Ecosystem

**F3 — Identity Spoofing via Untrusted `from` Field (T9) — HIGH**

The `from` field in the protocol (`./echo:{to}:{from}:{flags}:{payload}`) is set by the sender and relayed by the router bot without verification. Any sender on the allowlist can claim to be any other agent. Receivers pass this `from` value directly to application handlers.

**Files**: All `router_bot.*` and `receiver.*` across 4 languages

**Mitigation**: 1) Document that `from` is unauthenticated. 2) For high-trust scenarios, implement HMAC-signed messages where the signature covers `to:from:flags:payload`. 3) Consider using platform-native sender identity (Webex email from API) as authoritative source instead of protocol `from` field.

---

**F10 — Route Targets Not Validated (T37) — MEDIUM**

`WGROK_ROUTES` values (target email/ID) are not validated against the allowlist or checked for format correctness. A compromised environment variable could redirect messages to attacker-controlled endpoints.

**Files**: `go/config.go:76-83`, `python/src/wgrok/config.py:47-58`, `ts/src/config.ts:54-68`

**Mitigation**: Validate route targets against `WGROK_DOMAINS` allowlist at startup. Log all configured routes.

---

**F11 — Loose Dependency Pinning (T25) — MEDIUM**

- Python `pyproject.toml`: `webex-message-handler` has no version constraint, `aiohttp>=3.13` has no upper bound
- TypeScript `package.json`: `webex-message-handler: "*"` accepts any version

Go and Rust are properly pinned with lock files.

**Mitigation**: Pin Python deps (`webex-message-handler>=0.6.1,<1.0`, `aiohttp>=3.13,<4.0`). Pin TS dep (`"webex-message-handler": "^0.6.1"`).

---

**F13 — Webhook Lacks Rate Limiting and Replay Protection (T14) — MEDIUM**

The webhook endpoint has no rate limiting, no nonce/timestamp validation, and no request size limits. An attacker with a valid bearer token can flood the bus.

**File**: `python/src/wgrok/router_bot.py:105-134`

**Mitigation**: Add per-IP rate limiting. Add request body size limit. Consider HMAC-signed payloads with timestamp.

## Agent/Skill Integrity

No agent/skill definitions found in the codebase.

## Dependency CVEs

Scanned with: govulncheck (Go), npm audit (TS), pip-audit (Python), cargo audit (Rust)

| Package | Version | CVE | CVSS | Fixed In | Ecosystem | Code Path Used | Risk |
|---------|---------|-----|------|----------|-----------|----------------|------|
| aiohttp | 3.13.3 | 10 CVEs | HIGH | 3.13.4 | Python | Yes (direct) | High |
| authlib | 1.6.5 | 4 CVEs | HIGH | 1.6.9 | Python | Transitive (wmh) | High |
| cryptography | 46.0.3 | 2 CVEs | MED | 46.0.6 | Python | Optional dep | Medium |
| pyjwt | 2.10.1 | 1 CVE | MED | 2.12.0 | Python | Transitive (wmh) | Medium |
| rsa | 0.9.10 | RUSTSEC-2023-0071 | MED | None | Rust | False positive* | Accepted |
| (20 others) | various | 20 CVEs | LOW-MED | various | Python | Transitive | Low |

*\*RUSTSEC-2023-0071 (Marvin attack): wgrok only uses `RsaPublicKey` for encryption, never decryption. Timing side-channel requires decryption operations. Documented false positive.*

**Go**: 0 CVEs. **TypeScript**: 0 CVEs.

**Priority action**: `pip install --upgrade aiohttp>=3.13.4` resolves 10 HIGH CVEs immediately.

## Recommended Mitigations (Priority Order)

1. ~~**IMMEDIATE**: Revoke the two Webex tokens exposed in git history (commit 927442d). Clean history with `git-filter-repo`.~~ **FALSE POSITIVE** — .env files were never committed to git history.
2. ~~**IMMEDIATE**: Upgrade Python `aiohttp` to >=3.13.4 (10 CVEs).~~ **DONE** — pinned to `>=3.13.4,<4.0` in pyproject.toml.
3. ~~**HIGH**: Remove `rejectUnauthorized: false` from `ts/src/listener.ts:429`.~~ **DONE** — removed.
4. ~~**HIGH**: Always log security events regardless of debug mode — never use NoopLogger for allowlist rejections, auth failures, or crypto errors.~~ **DONE** — all 4 languages now use MinLevelLogger (WARN+ERROR always emitted) when debug=false.
5. ~~**HIGH**: Make webhook authentication mandatory (fail if `WGROK_WEBHOOK_SECRET` not set when webhook port is configured).~~ **DONE** — Python router_bot raises ValueError if webhook_port is set without webhook_secret.
6. ~~**HIGH**: Fix chunk reassembly to verify all expected indices are present, not just count.~~ **DONE** — all 4 receivers verify indices 1..total before reassembly, with 5-minute timeout.
7. **MEDIUM**: Document that the `from` field is unauthenticated. Consider HMAC signing for high-trust deployments. *Accepted risk — documented in protocol spec.*
8. ~~**MEDIUM**: Restrict allowlist to exact/domain patterns only (drop `?` and `[...]` glob support).~~ **DONE** — all 4 languages reject patterns containing `[`, `]`, or `?`.
9. ~~**MEDIUM**: Pin Python and TS `webex-message-handler` to `>=0.6.1,<1.0` / `^0.6.1`.~~ **DONE** — pyproject.toml and package.json pinned.
10. ~~**MEDIUM**: Pin GitHub Actions to commit SHAs.~~ **DONE** — all workflows use full commit SHA pins.
11. ~~**MEDIUM**: Add rate limiting and request size limits to webhook endpoint.~~ **PARTIALLY DONE** — 1MB request size limit added. Rate limiting deferred (requires external middleware).
12. ~~**LOW**: Fix Go `Decompress()` to fail-closed instead of silent passthrough.~~ **DONE** — returns error on any decompression failure.
13. **LOW**: Add `/health` and `/metrics` endpoints for operational visibility. *Deferred — low priority.*

Additionally remediated:
- **F7**: Payload content redacted from all sender/router_bot log messages (all 4 languages). Logs now show metadata only (slug, from, target, length).
- **F8**: Chunk total capped to 100 (was 999) with 5-minute timeout eviction (all 4 languages).
- **F10**: Configured routes logged at startup in all router_bot implementations.

## Trust Boundaries

```
TB1: External Network ←→ Platform APIs (Webex/Slack/Discord/IRC)
     Crossed by: API tokens (Bearer auth), TLS encryption
     
TB2: Platform APIs ←→ wgrok Router Bot
     Crossed by: WebSocket messages, webhook POSTs
     Trust: Platform-authenticated sender identity (email)

TB3: Router Bot ←→ Agents (Sender/Receiver)
     Crossed by: Protocol messages relayed through platform
     Trust: Allowlist (domain-based), optional AES-256-GCM encryption
     Gap: "from" field is unauthenticated

TB4: Webhook Endpoint ←→ External Callers
     Crossed by: HTTP POST with Bearer token
     Trust: WGROK_WEBHOOK_SECRET (optional — gap)

TB5: Environment ←→ Application
     Crossed by: .env files, WGROK_* env vars
     Trust: OS-level file permissions, process isolation
```

## Data Flow Diagram (Text)

```
                         ┌─────────────────────┐
                         │   Platform APIs      │
                         │ (Webex/Slack/Discord) │
                         └──────┬───────────────┘
                                │ TLS + Bearer Token
                    ┌───────────┼───────────────┐
                    │           │               │
              ┌─────▼─────┐    │         ┌─────▼─────┐
              │  Sender    │   │         │  Receiver  │
              │  Library   │   │         │  Library   │
              └─────┬──────┘   │         └─────▲──────┘
                    │          │               │
          ./echo:to:from:      │          to:from:
          flags:payload        │          flags:payload
                    │    ┌─────▼──────┐        │
                    └───►│ Router Bot │────────┘
                         │  (relay)   │
                         └─────▲──────┘
                               │
                    ┌──────────┴──────────┐
                    │  Webhook Endpoint   │
                    │  (HTTP POST, opt.)  │
                    └─────────────────────┘
                               ▲
                               │ Bearer Token (optional!)
                         External Callers

Legend:
  ─── = Data flow
  ▲/▼ = Direction
  All external flows over TLS (except TS IRC: cert validation disabled)
  Encryption (AES-256-GCM) is optional end-to-end overlay
  Router bot never decrypts — transparent relay
```
