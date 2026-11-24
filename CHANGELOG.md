# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [v0.6.0-alpha] - 24-11-2026

### Added

- **Migration `006_drop_retired_tables.sql`**: drops the `policies`, `budgets`, `usage_records` and `credentials` tables left behind by removed features. Run `go run ./cmd/migrate` to apply it.

### Changed

- CLI messages, usage errors, code comments and the `004_chat_sessions.sql` header now refer to the new chat command names.

### Removed

- The `credentials` table and its index from `001_baseline.sql`, so new databases no longer create it. `credentials` is no longer accepted by `DeleteRowData`.

### Breaking

- **Destructive migration**: `006_drop_retired_tables.sql` permanently deletes all rows in `policies`, `budgets`, `usage_records` and `credentials`. Back them up before running `cmd/migrate` if you need that data.
- Chat sync commands renamed to git-style verbs: `/local-chat-sync` → `/push`, `/local-chat-import` → `/pull <id>`, `/local-chat-list` → `/log`, `/local-chat-sync-create` → `/clone`. The old names no longer work. README and dashboard quickstart updated to match.

## [v0.5.0-alpha] - 23-11-2026

### Changed

- **Renamed to Mako.** The product is the `mako` CLI and the `mako-proxy` server; Tokenly is the organisation that builds it. The server entrypoint moved from `cmd/gateway` to `cmd/mako-proxy`, and the Go module is now `github.com/thenameisvicky/tokenly-mako`, matching the repository.
- **Dashboard redesign**: the web dashboard uses the Tokenly landing page's light theme in a single plain stylesheet (Tailwind removed). Shared sessions are the home page (`/`).
- `/api/v1/me` moved into the auth package; `/health` no longer reports latency percentiles.

### Removed

- **Everything except chat sync.** Mako is now login plus Claude Code session sharing:
  - Request proxying to Anthropic (`/api/v1/proxy`, `/api/v1/messages`), the `/claude` and `/set-provider-key` commands, and `UPSTREAM_PROVIDER_URL`.
  - Team policy, token budgets and usage recording (`/api/v1/policy`, `/api/v1/budget`, `/api/v1/usage`), and the dashboard's Usage, Policy and Budget pages.
  - Latency telemetry, the unused provider-credential storage and cache, and empty placeholder packages.
  - Migrations `003_policy_and_budget.sql` and `005_governance_uniqueness.sql`. Existing databases keep the `policies`, `budgets` and `usage_records` tables; drop them manually if you no longer need them.

### Breaking

- Auth header `X-Tokenly-Token` → `X-Mako-Token`. Update the CLI and the proxy together.
- Environment variables: `TOKENLY_GATEWAY_URL` → `MAKO_PROXY_URL`, `TOKENLY_CREDENTIALS_BACKEND` → `MAKO_CREDENTIALS_BACKEND`, `TOKENLY_CREDENTIALS_FILE` → `MAKO_CREDENTIALS_FILE`, `NEXT_PUBLIC_GATEWAY_URL` → `NEXT_PUBLIC_MAKO_PROXY_URL`.
- CLI flag `--gateway` → `--proxy`.
- `UPSTREAM_PROVIDER_URL` is no longer read; remove it from your environment.
- Credentials move from keychain service `tokenly` (and `~/.tokenly/credentials.json`) to `mako` (and `~/.mako/credentials.json`). Run `/login` again after upgrading.

## [v0.4.0-alpha] - 23-11-2026

### Added

- **Web dashboard** (`web/`): A Next.js 15 App Router app for configuring the gateway from a browser. Sign in, edit policy, set token budgets, watch live p50/p95/p99 latency and usage, and read synced transcripts. Themed with the mako CLI's xterm-256 palette converted to hex, so terminal and browser render identical colours.
- **Governance API** (`internal/governance`): `GET /api/v1/me`, `GET|PUT /api/v1/policy`, `GET|PUT /api/v1/budget` and `GET /api/v1/usage`. Policy writes invalidate the engine's cache so a change applies to the next proxied request. All are control-plane endpoints and never run on the hot path.
- **Usage reporting** (`GetTeamUsage`): totals, per-model and per-day rollups for the dashboard.
- **Policy and budget upserts**: `UpsertPolicy` and `UpsertBudget`, with unique indexes so each team has one policy and one team-wide budget.
- **CORS middleware** with an explicit origin allowlist and preflight short-circuit, plus tests covering the origin-reflection case.
- **Query builder**: `OrderBy` and `Limit` for SELECT.
- **Create a team at signup**: The dashboard's create-account form has a "create a new team" checkbox. With it ticked the team ID field is replaced by a team name and is no longer required, so the first user of a deployment can sign up before any team exists; the creator becomes that team's admin. `CreateTeamWithUser` inserts the team and its first user in one transaction, so a failed signup cannot leave an orphan team.
- **`cmd/migrate`**: Applies the schema with `go run ./cmd/migrate` (`-dry-run` lists what would run). The SQL is embedded in the binary, so deploying needs neither the repository nor `psql`. Migrations moved to `cmd/migrate/sql/`.

### Changed

- `GetBudgetForTeam` now matches the team-wide row explicitly (`user_id IS NULL`) instead of whichever budget row the team happened to have first.
- `policy.Engine` and `budget.Tracker` are nil-safe, so the gateway can run with policy or budgeting disabled.

### Configuration

- **`CORS_ALLOWED_ORIGINS` is a new required environment variable.** The gateway will not start without it, matching the project's no-fallback config rule. Set it to the dashboard's origin (e.g. `http://localhost:3000`), or `*` for local development only.

## [v0.3.0-alpha] - 23-11-2026

### Removed

- **Credential swapping**: The gateway no longer substitutes the caller's token for a shared provider key held in the database. Each developer now supplies their own provider credential, which is forwarded untouched. The `/api/v1/credentials` endpoint has been removed along with the credential decrypt step on the request path.

### Added

- **Policy engine** (`internal/policy`): Per-team model allow/block lists and a `max_tokens` ceiling, evaluated from a process-local TTL cache. Request bodies are inspected with a bounded 8KB peek and replayed upstream unchanged, so evaluation cost does not scale with payload size.
- **Token budgeting** (`internal/budget`): Per-team and per-user token limits held as atomic in-memory counters, warmed once per period from the database. Usage is queued and flushed by a background worker, never written on the request path.
- **Chat sync**: `/local-chat-sync`, `/chat-list` and `/local-chat-import` move Claude Code transcripts between machines. Transcripts are encrypted with AES-256-GCM at rest and scoped to the user's team.
- **`/set-provider-key`**: Stores the developer's own provider API key in the system keychain.
- **Encrypted identity at rest**: Username, email and password columns are AES-256-GCM encrypted, with an HMAC-SHA256 blind index (`username_bidx`) so login can still locate a row. Legacy bcrypt rows are verified and re-encrypted transparently on next login.
- **Migrations** (`migrations/`): Idempotent, additive SQL for the baseline schema, encrypted identity, policy/budget/usage, and chat sessions.
- **Unit test coverage** for body peeking, usage sniffing, field encryption, blind indexing, transcript scan/import, and proxy credential pass-through — all runnable without a database.

### Changed

- **Header separation**: The Tokenly JWT now travels in `X-Tokenly-Token`. `Authorization` and `x-api-key` carry the developer's own provider credential on proxied paths and are no longer read as Tokenly tokens there.
- **Streaming**: The proxy flushes each chunk and no longer applies a client timeout, so server-sent events stay interactive and long responses are not cut off.
- **Request logging** now records model and token counts.

### Security

- Removed a debug log that dumped every request header, including `Authorization`.
- Removed live database, JWT and encryption credentials from `.vscode/launch.json` and the test suite; both now read from the environment. `.vscode/launch.json` is gitignored and `.vscode/launch.json.example` added.

## [v0.2.0-alpha] - 15-08-2026

### Added

- **Monorepo Migration**: Unified CLI client (`mako`) and Gateway server (`tokenly-gateway`) into a single Go workspace monorepo under the `github.com/thenameisvicky/tokenly-gateway` module.
- **Shared Data Structures**: Introduced `pkg/models` to define shared API payloads (e.g., `LoginRequest`, `LoginResponse`) used for communication between the CLI and the Gateway.
- **Unified Build Pipeline**: Configured compiling for both CLI (`bin/mako`) and Gateway (`bin/gateway`) from a single root package layout.

## [v0.1.0-alpha] - 26-07-2026

### Added

- **Interactive Go Shell**: Low-latency terminal REPL using raw console mode with history, command routing, and basic autocompletion.
- **Secure Keychain Storage**: Platform-agnostic integration utilizing native credential vaults (macOS Keychain, Linux Secret Service/DBus, and Windows Credential Manager) via `go-keyring`.
- **Downstream Agent Execution**: Process runner to fork and execute `claude` (Claude Code) as a child process.
- **Native Terminal Pass-Through**: Direct piping of `stdin`, `stdout`, and `stderr` streams to preserve colors, input captures, and interactive prompts.
- **Slash Commands**: Initial set of terminal utilities:
  - `/init` to configure Tokenly backend credentials.
  - `/status` to display the active keychain credential state.
  - `/claude` to locate and launch the Claude Code sub-shell.
  - `/logout` to clear credentials.
  - `/clear` and `/clean` to clear terminal output.
  - `/help` and `/exit` to manage the REPL session.
