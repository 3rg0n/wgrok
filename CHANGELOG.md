# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Fixed
- **F6**: Security events (WARN/ERROR) now always logged regardless of debug mode (TS, Rust)
  - Replaced NoopLogger with MinLevelLogger that suppresses DEBUG/INFO but emits WARN/ERROR
- **F7**: Payload content redacted from sender/router_bot log messages (all 4 languages)
  - Logs now show metadata only: slug, from, target, length
- **F5**: Webhook authentication now mandatory when WGROK_WEBHOOK_PORT is set (Python)
  - Raises ValueError if WGROK_WEBHOOK_SECRET is missing
- **F13**: Webhook endpoint request size limited to 1MB (Python)
- **F10**: Configured routes logged at startup in all router_bot implementations
- **F11**: Dependencies pinned: pyproject.toml, package.json, all GitHub Actions to commit SHAs
- Rust chunk timeout (5-minute eviction) implemented for parity with Python/Go/TS
- Rust allowlist clippy warning resolved (manual_strip → strip_prefix)

## [1.2.2] - 2026-04-05

### Fixed
- Add README to npm, PyPI, and crates.io packages

## [1.2.1] - 2026-04-05

### Fixed
- Bump `webex-message-handler` to 0.6.2 across all four languages
- Bump `go-jose/v4` to v4.1.4 — fixes JWE decryption panic (HIGH)
- Bump `lodash` — fixes code injection and prototype pollution (HIGH, MODERATE)

### Added
- GitHub Advanced Security: CodeQL, dependency review, Dependabot
- Secret scanning and push protection enabled

## [1.2.0] - 2026-03-28

### Added
- Optional AES-256-GCM encryption across all four languages
  - On/off mode: set `WGROK_ENCRYPT_KEY` to enable, omit to disable
  - `e` flag in protocol wire format indicates encrypted payload
  - Pipeline: compress → encrypt → chunk (send), reassemble → decrypt → decompress (receive)
  - 32-byte base64-encoded symmetric key via environment variable
- Pause/resume flow control with per-target message buffering (1000 msg cap)
- Bot mention stripping for Webex group rooms (`strip_bot_mention`)
- Room-based routing — always use `roomId` for replies (works for 1:1 and group)
- Published to npm, PyPI, crates.io, and Go modules

### Changed
- Switch Python and Rust `webex-message-handler` from git dep to published registry version
- MIT license

### Fixed
- Go listener context propagation (gosec G118) — lifecycle context for background goroutines
- TS config error cause chain preservation
- Go unhandled Close/io.Copy errors (gosec G104)
- Retry-After header capped to 300s across all platforms
- Chunk validation: seq >= 1, seq <= total, total <= 999
- Fail-closed decrypt/decompress in all receivers

## [0.3.0] - 2026-03-17

### Changed
- **Protocol v2**: Four-field wire format `./echo:{to}:{from}:{flags}:{payload}`
  - `from` field enables return addressing and reply-to patterns
  - `flags` field carries compression (`z`), chunking (`N/M`), and metadata
  - Breaking change from v1 two-field format

### Added
- Payload codec with gzip+base64 compression and auto-chunking
- Platform-aware message limits (Webex 7439B, Slack 4000, Discord 2000, IRC 400)
- `compress` parameter on sender for automatic gzip+base64 encoding
- Receiver auto-decompression and chunk reassembly

## [0.2.0] - 2026-03-16

### Added
- Platform listener abstraction for WebSocket/TCP receive across all languages
- Slack, Discord, and IRC transport bindings (send + receive)
- Multi-platform support with `WGROK_{PLATFORM}_TOKENS` environment variables
- 429 retry handling with exponential backoff for all platforms

### Changed
- Renamed EchoBot to RouterBot across all languages
- Updated all dependencies to latest versions

### Fixed
- Go response body leak in retry loop
- webex-message-handler v0.5.1 for Go ping loop panic fix

## [0.1.0] - 2026-03-14

### Added
- Initial implementation in Python, Go, TypeScript, and Rust
- Mode B (agent bus) — shared token, agents self-select by slug
- Mode C (registered agents) — `WGROK_ROUTES` maps slugs to bot identities
- Webhook endpoint for non-WebSocket integrations (`WGROK_WEBHOOK_PORT`)
- Domain-based allowlist/ACL via `WGROK_DOMAINS`
- Adaptive card support (send, relay, receive)
- Outbound proxy support via `WGROK_PROXY`
- AsyncAPI 3.0.0 protocol specification
- Shared JSON test cases consumed by all four languages
- Cross-language test runner (`tests/run_all.sh`)
