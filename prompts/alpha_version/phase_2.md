# Tokenly gateway - controlled implementation vibe coding plan phase 2

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

## API router & per module route registration

- Per-Module routes.go: Each package (like auth, telemetry, proxy) keeps its route logic, HTTP paths, and method matching isolated in its own package folder (e.g. internal/auth/routes.go).
- The main router.go file which has the all the routes registred across the modules This file is solely responsible for creating the http.ServeMux, registering all the modular handlers, and wrapping them in middleware.
- The main server.go file is now only responsible for starting / stopping the listeners and wiriing up dependencies and scaling horizontally with cores from CPU

## High speed hot path optimizations

- Hot path in tokenly is API key replacing and proxying.
- There must not be > 1ms latency caused by tokenly-gateway. This is the metric measured in telementry
- Hot path only reads from cache (In-Memory).
- If the credential/quota is not in the cache (a Cache Miss), perform the DB lookup, save it to the cache, and then proceed. Once cached, subsequent requests will read in <0.01ms.
- Database reads should be handled in the background (if needed) to avoid blocking the hot path.
- Use sync.Pool for request-scoped allocations like request ID generation, buffer creation, and header management to minimize GC pressure.
- Telementry saving in DB too must not be awaited here blocking the hot path. Save in background go routine or channel to be processed and saved later.
- Hot path modules are cache, requestctx, proxy and middleware that wrap them.
- Share the HTTP2 connection or what ever connection established with claude server across requests to reduce latency and overhead.
- The path where claude CLI communucates with claude server via tokenly-gateway must not have any latency.
- To minimize garbage collection spikes (which hurt p99 latency), reuse objects instead of creating new ones on every request.
- The Solution: Use Go's sync.Pool for things like temporary buffers or custom structs:
- "Configure a shared http.Client with a custom http.Transport setting MaxIdleConns and MaxIdleConnsPerHost to 100+ to ensure connections to the upstream host are pooled and kept alive."
- "When streaming the request/response bodies, use io.CopyBuffer instead of io.Copy, and fetch the buffer slice from a sync.Pool of []byte (32KB size) to eliminate allocation overhead during payload copy."
- "Ensure the in-memory cache stores decrypted/unencrypted API keys directly to eliminate decryption CPU overhead on cache hits, but ensure they are wiped/cleared securely if credentials are deleted."