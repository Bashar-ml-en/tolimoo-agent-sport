# Private AI Sports Newsroom

Mobile-first scaffold for a solo sports journalist. The backend owns persistence, API contracts, and future research/orchestration; the Expo team owns the mobile UI. Real Exa, OpenRouter, scheduling, and social publishing are intentionally deferred.

## Prerequisites

- Go 1.23+
- Node.js 20+ and npm
- Expo CLI via the local project command (`npx expo start`)

## Run

```sh
cp backend/.env.example backend/.env
make run-api
curl http://localhost:8080/health

cp mobile/.env.example mobile/.env
cd mobile && npm install && npm start
```

The Go process reads environment variables from its shell; `.env` is a template and is not loaded automatically. Export it with your preferred dotenv tool or `set -a; . backend/.env; set +a`. Expo reads `EXPO_PUBLIC_*` variables from its environment. Never put secrets in the mobile environment.

SQLite is created at `backend/data/newsroom.db`; startup applies the schema and idempotently seeds the three stable assignments. Foreign keys, WAL, busy timeout, and one-process connection limits are enabled.

## Mobile modes

Set `EXPO_PUBLIC_USE_MOCK_API=true` for clearly fictional local fixtures. The app shows a Demo data indicator, and mock review/mutation actions are session-only. Set it to `false` to use the HTTP API; failed real requests are shown as errors and never silently replaced with fixtures.

Physical phones cannot reach a computer backend through `localhost`. Set `EXPO_PUBLIC_API_BASE_URL` to the computer's reachable LAN IP (for example `http://192.168.1.20:8080`) or an HTTPS backend URL, and allow that origin in `CORS_ALLOWED_ORIGINS` where applicable.

## API status

Implemented: `GET /health`, `GET /api/v1/agents`, and `GET /api/v1/agents/{id}/messages` against SQLite. Deferred mutation routes return the documented 501 `not_implemented` error: starting runs, posting messages, and updating drafts.

See [docs/api-contract.md](docs/api-contract.md) and [docs/architecture.md](docs/architecture.md).

## Ownership and verification

Backend ownership includes Go, SQLite, APIs, migrations, scheduling, Exa, OpenRouter, and agent orchestration. The other developer owns the Expo UI and final interaction design.

```sh
make test
cd mobile && npm run typecheck
```
