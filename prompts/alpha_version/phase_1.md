# Tokenly gateway - controlled implementation vibe coding plan

## Context & Hard constraints

- You are working on Tokenly gateway, a high speed gateway binary that swaps API key of the incoming request from tokenly-cli with the master API key stored in DB after checking quota, limit, team, usage in high speed. Minimize gateway overhead and avoid unnecessary database work on the hot path. Measure p50/p95/p99 latency
- The goal is to implement today's MVP incrementally and conservatively.
- I do NOT want uncontrolled vibe coding.
- Before making architectural changes, inspect the existing codebase and existing implementations. 
- Reuse existing patterns where appropriate. Do not introduce unnecessary frameworks, ORMs, dependency injection frameworks, Redis, KMS, background workers, or abstractions that are not required for the current MVP.
- The priority order is:
    1. Correctness
    2. Security
    3. Low latency on the hot path
    4. Simple architecture
    5. Understandability
    6. Measurable performance
    7. Production readiness without premature complexity

## Middleware system mechanisim

- Refer this - Documents/Personal/wAI/wAI-server-new project's middleware and implement same in Tokenly gateway
- All routes with prefix /api/v1/* except /api/v1/login, api/v1/health needs to be protected with middleware
- Implement the middleware in auth/middleware.go
- Translate the excat beheavior from Documents/Personal/wAI/wAI-server-new middleware from python to go and implement it here

## Authentication & Authorization mechanisim

- Implement same system design followed in Documents/Personal/wAI/wAI-server-new for authenticaiton & authorization
- Translate the excat beheavior from Documents/Personal/wAI/wAI-server-new for authenticaiton & authorization from python I want a small custom data-access layer.to go and implement it here
- Responsible for JWT parsing, JWT signature verification, claims, expiration, authentication, authorization, user identity.

## Database access layer

- Implement an abstract layer for postgres sql database in internal/storage/
- Do not use any ORM or third party database access layer
- Implement using plain SQL through Go's database SQL layer / existing PostgreSQL driver.
- I want small low latecny thin custom data-access layer.
- By default all new document writes will have flagUseStatus: 1 & all reads only read items with flagUseStatus: 1
- flagUseStatus is a logical deletion / soft deletion mechanisim simulation
- This storage layer need to expose operations such as
  1. GetUserByID(...)
  2. GetCredential(...)
  3. CreateCredential(...)
  4. GetTeamForUser(...)
  5. UpdateCredential(...)
  6. DeleteRowData (It just set the flagUseStatus to 0)
- All operations must be optimized for latency and speed.
- Implement query builder pattern for building sql queries
- Implement proper error handling for database operations
- Every SQL query must have a comment (tiny and no noise) explaining what it does in comparison with mongo db query
- Query needs to be internal not from external client.

## Configuration loading & sharing mechanisim

- All configuration needs to be read in one place and injected to other module instance declared in main.go and other modules
- No circular dependency between modules must exists
- Add APP_BASE_URL to launch.json http://localhost:8080 for local development
- All configuration must be loaded from environment variables & no code level fallbacks - strictly! if any missing it will error and stop the process that is expected beheavior
- This makes the system testable and makes startup configuration obvious.

## Encryption mechanisim

- Implement internal/encryption/symmentric.go
- use AES-256-GCM, module receives the encryption key throught consructor/parameter
- It must not read envroinment variables by itself
- Conceptually
  1. main.go
  2. cfg.ENCRYPTION_KEY
  3. encryption.New()
- Encryption module should validate that the AES-256 key is exactly 32 bytes
- create an AES cipher
- create GCM
- generate a cryptographically secure random nonce for every encryption
- encrypt the supplied master/provider API key
- return ciphertext and nonce
- decrypt ciphertext using the supplied nonce
- fail if authentication/tag verification fails
- Do not implement custom
- The nonce is not secret and may be stored alongside ciphertext.
- The encryption key must never be stored in the database.
- The encryption key must never be logged.
- The plaintext provider API key must never be logged.
- Prefer explicit types/structures such as:
  1. EncryptedValue
  2. Ciphertext
  3. Nonce
- rather than ambiguous byte slices passed around everywhere.
- Responsibility AES-256-GCM, nonce, ciphertext, plaintext.

## Credential storage

- The database should store:
  1. ciphertext
  2. nonce
- and metadata such as:
  1. user_id
  2. team_id
  3. provider
  4. created_at
  5. updated_at
- The plaintext provider API key must never be persisted.
- The database must never contain the encryption master key.
- Credential flow:
  1. Provider API key
        ↓
  2. Credential service
        ↓
  3. AES-256-GCM
        ↓
  4. ciphertext + nonce
        ↓
  5. storage repository
        ↓
  6. database
- Retrieval:
  1. database
  2. ciphertext + nonce
        ↓
  3. credential service
        ↓
  4. AES-256-GCM decrypt
        ↓
  5. plaintext API key
- The plaintext should exist in memory only for as long as required.

## Credential cache

- Implement internal/cache/cipher.go
- For the MVP, use an in-memory cache.
- The initial conceptual key is:
  1. userID
- The cached value should represent the encrypted credential:
  2. userID → planText
- Do NOT cache plaintext provider API keys for the initial implementation unless there is a demonstrated performance requirement.
- The cache should be:
  1. process-local
  2. concurrency safe
  3. TTL based
  4. simple
  5. bounded/controlled
  6. easy to invalidate
- Every credential read should follow:
  1. Request
        ↓
  2. Cache lookup
        ↓
  3. HIT → plaintext cache (temporary)
        ↓
  4. MISS
        ↓
  5. Database
        ↓
  6. Cache
- Then:
  1. encrypted credential
        ↓
  2. AES-GCM decrypt
        ↓
  3. temporary plaintext
- The cache must never log credential contents.
- Document that this cache is process-local and therefore does not synchronize between multiple Gateway instances.

## Request pipeline 

- The intended protected request pipeline is:
   1. Claude CLI
        ↓
   2. HTTP request
        ↓
   3. JWT middleware
        ↓
   4. resolve user
        ↓
   5. resolve team/company
        ↓
   6. attach identity to request context
        ↓
   7. credential resolver
        ↓
   8. cache lookup
        ↓
   9. DB on cache miss
        ↓
   10. AES-256-GCM decrypt
        ↓
   11. provider credential
        ↓
   12. provider Authorization header
        ↓
   13. transport
        ↓
   14. Anthropic
- The provider credential should be injected only into the outbound provider request.
- Do NOT mutate the incoming client request's Authorization header in a way that destroys the Tokenly JWT.
- There are two different credentials:
  1. Incoming: Authorization: Bearer <Tokenly JWT>
  2. Outgoing: Authorization: Bearer <Anthropic API Key>
- Keep those concepts separate.

## Logging

- Implement safe request logging.
- Log things such as:
  1. method
  2. path
  3. status
  4. duration
  5. user ID
  6. team ID
  7. provider
  8. cache hit/miss
- Never log:
  1. Tokenly JWT
  2. Anthropic API key
  3. ENCRYPTION_KEY
  4. JWT_SECRET
  5. request Authorization header
  6. provider Authorization header
  7. plaintext credential
- If headers need to be inspected during local development, redact sensitive headers explicitly.
- Example:
  1. Authorization: REDACTED
- Do not create a debug endpoint that dumps all request headers without redaction.

## Server and route structure

- Keep HTTP composition in:
  1. internal/server/
- Authentication logic remains in:
  1. internal/auth/
- The server package wires middleware and handlers together.
- Conceptually:
  server
  ├── public
  │    └── /health
  │
  └── protected
      └── /api/v1/*
             ↓
        auth middleware
- /api/v1/login is public.
- All other /api/v1/* routes require authentication unless there is an explicitly documented reason otherwise.
- Do not create a separate generic middleware/ package unless there are multiple unrelated middleware systems that justify it.

## What not to implement 

- Do NOT introduce:
  - Redis
  - Kafka
  - message queues
  - Kubernetes
  - KMS/HSM
  - generic dependency injection framework
  - ORM
  - generic CRUD generator
  - microservices
  - background credential refresh
  - distributed cache
  - complex rate limiting
  - circuit breakers
  - retries unless required by the existing provider implementation
  - elaborate telemetry
  - unnecessary interfaces everywhere
- The MVP should remain small.