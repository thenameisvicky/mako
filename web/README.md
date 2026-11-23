# Mako dashboard

Next.js app for the Mako proxy: shared Claude Code sessions, usage, policy and
token budgets.

Styling is one plain stylesheet, `src/app/globals.css`, whose tokens and base
components mirror the Tokenly landing page (`tokenly/landing/index.css`). Change
the theme there first, then copy it here. There is no CSS framework.

```bash
cp .env.example .env.local     # point at your Mako proxy
npm install
npm run dev                    # http://localhost:3000
```

The proxy must list this app's origin in its `CORS_ALLOWED_ORIGINS`, or the
browser will block every request.

Full documentation lives in the [repository README](../README.md#web-dashboard).

## Troubleshooting

**`__webpack_modules__[moduleId] is not a function`**

`next dev` and `next build` share the `.next/` directory. Running a build while
the dev server is live rewrites the chunks underneath it, and the running server's
module graph no longer matches what is on disk. Stop the dev server, clear the
directory, and start again:

```bash
rm -rf .next
npm run dev
```

**`Cannot reach the Mako proxy at http://localhost:8080`**

The proxy is not running, or its `CORS_ALLOWED_ORIGINS` does not list this
app's origin. The browser hides CORS detail from JavaScript, so the two look
identical from here. Check the proxy log for `[CORS] blocked origin`, and test
the preflight from *your actual port* (Next.js moves to 3001, 3002, ... when
3000 is taken):

```bash
curl -i -X OPTIONS http://localhost:8080/api/v1/login \\
  -H "Origin: http://localhost:3002" \\
  -H "Access-Control-Request-Method: POST"
# expect 204 AND an Access-Control-Allow-Origin header echoing your origin.
# A 204 with no such header means the origin is blocked.
```

Setting `CORS_ALLOWED_ORIGINS=http://localhost:*` on the proxy covers every
local port.

**`Database query failed` on sign in**

The database is behind the current schema. Apply the migrations from the
repository root:

```bash
set -a && source .env && set +a
go run ./cmd/migrate
```
