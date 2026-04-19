# Trivia

A self-hosted Kubernetes trivia platform where hosts create and run live quiz sessions and players join via QR code — no account required to play. Built as both a real product and a hands-on Kubernetes learning project.

---

## What's been built so far

The Go API skeleton is in place and wired up end-to-end:

- **Server bootstrap** (`cmd/server/main.go`) — connects to Postgres and Redis on startup with health checks, registers all routes, and shuts down gracefully on SIGINT/SIGTERM.
- **Router** — Chi with standard middleware (request ID, real IP, structured logger, recoverer, 30s timeout). Health/readiness probes at `/healthz` and `/readyz`. Prometheus metrics at `/metrics`.
- **Auth middleware** — Auth0 JWT validation via `lestrrat-go/jwx`. Authenticated routes are grouped; the WebSocket endpoint is intentionally public so players can join without an account.
- **Config** — environment-driven config loader (`internal/config`).
- **Billing interface** — `EntitlementChecker` interface defined in `internal/billing`. `NoopChecker` is the current implementation (always grants access). Handlers call this interface — swapping in a Stripe-backed implementation later requires no changes at call sites.
- **Game service** (`internal/game`) — skeleton wired into the router.
- **User service** (`internal/user`) — skeleton wired into the server.
- **WebSocket hub** (`internal/realtime`) — room management scaffold registered on the router.
- **Database layer** — full sqlc-generated store (`internal/store`) covering all tables: `users`, `question_banks`, `questions`, `games`, `game_players`, `answers`, `subscriptions`.
- **Migrations** — 8 sequential migrations (goose) covering the complete schema, including a `subscriptions` table that's created now but left empty until billing is implemented.
- **sqlc queries** — raw SQL query files for all entities in `queries/`.

The frontend (`apps/web`) and all infrastructure/deploy manifests are next in the build sequence.

---

## Stack

| Layer | Technology |
|---|---|
| Backend | Go 1.23, Chi router, coder/websocket |
| Database | Neon Postgres (sqlc + pgx, goose migrations) |
| Cache / realtime state | Redis (in-cluster StatefulSet) |
| Auth | Auth0 (SPA app for Next.js, API audience for Go) |
| Frontend | Next.js (App Router, @auth0/nextjs-auth0) |
| Observability | Prometheus (`promhttp`), structured JSON logging (`slog`) |
| Hosting | DigitalOcean Kubernetes (DOKS) |
| GitOps | Argo CD (app-of-apps, Kustomize overlays) |
| Ingress | ingress-nginx + cert-manager (Let's Encrypt) |
| Secrets | External Secrets Operator or sealed-secrets |
| Payments (deferred) | Stripe Billing |

---

## Repo structure

```
trivia/
  apps/
    api/                    # Go backend
      cmd/server/           # main entrypoint
      internal/
        auth/               # Auth0 JWT middleware
        billing/            # EntitlementChecker interface + NoopChecker
        config/             # env-driven config
        game/               # game lifecycle, questions, scoring
        realtime/           # WebSocket hub, room management
        store/              # sqlc-generated DB layer
        user/               # host account service
      migrations/           # goose SQL migrations (0001–0008)
      queries/              # raw SQL for sqlc
    web/                    # Next.js frontend (not yet started)
  deploy/
    k8s/                    # Kustomize base + overlays (dev, prod, gke)
    argocd/                 # Argo CD Application definitions
  docs/
```

---

## Data model

**Postgres** (persistent):

- `users` — host accounts, keyed by Auth0 `sub`
- `question_banks` — reusable question sets owned by a host
- `questions` — belongs to a bank; supports `text` and `multiple_choice` types; choices stored as JSONB
- `games` — a play instance with a 6-char join code, status, and current question index
- `game_players` — display name, join time, score per game
- `answers` — one row per player per question, used for post-game review
- `subscriptions` — table exists now, stays empty until Stripe billing is live

**Redis** (ephemeral live state, flushed to Postgres on game end):

- `game:{code}:state` — current question, phase (lobby/question/reveal/scoreboard), deadline
- `game:{code}:players` — set of connected players
- `game:{code}:answers:{qid}` — hash of player → answer
- `game:{code}:scores` — sorted set for the live leaderboard

---

## Realtime design

One WebSocket hub per API pod, rooms keyed by game code. Message directions:

- **Host → server:** start game, advance question, end game
- **Player → server:** join, submit answer
- **Server → room:** question revealed, timer tick, answer accepted, scoreboard update, game ended

Starting single-replica. Scaling out later uses Redis Pub/Sub fan-out — an additive change, not a rewrite. Sticky sessions on ingress cover the interim.

---

## Getting started (local dev)

### Prerequisites

- Go 1.23+
- A running Postgres instance (or a [Neon](https://neon.tech) branch)
- A running Redis instance
- An Auth0 tenant with an SPA app and an API configured

### Run the API

```bash
cd apps/api
cp .env.example .env
# Fill in DATABASE_URL, REDIS_ADDR, AUTH0_DOMAIN, AUTH0_AUDIENCE
make run
```

### Run migrations

```bash
make migrate
```

### Regenerate sqlc

```bash
make sqlc
```

See `apps/api/README.md` for full details.

---

## What's next

Following the planned build sequence:

1. **Basic HTTP API** — question bank CRUD, game create/list endpoints
2. **Next.js host dashboard** — Auth0 login, hitting the Go API
3. **WebSocket game loop** — join flow, questions, answers, scoring, reveal
4. **QR code generation** — join page for players
5. **Kubernetes manifests** — Kustomize base + overlays, test on local k3d/kind, deploy to DOKS
6. **Argo CD + observability** — kube-prometheus-stack (Prometheus, Grafana, Alertmanager), optionally Loki
7. **GitHub Actions CI/CD** — test → build → push to registry; Argo Image Updater handles deploys
8. **Stripe billing** — implement `EntitlementChecker` backed by Stripe webhooks + `subscriptions` table

---

## Kubernetes layout

**Namespaces:** `trivia` (app workloads), `platform` (ingress, cert-manager, monitoring), `argocd`

**Workloads in `trivia`:**
- `api` Deployment (1 replica to start, HPA later)
- `web` Deployment
- `redis` StatefulSet with PVC
- Ingress: `/api/*` → api service, everything else → web

**GitOps:** Argo CD app-of-apps — a root Application points at `deploy/argocd/`, which contains child Applications per workload. Push to `main` → cluster reconciles.

**Kustomize:** `base/`, `overlays/dev/`, `overlays/prod/`. A future `overlays/gke/` (~20 lines of patches) handles the migration to GKE Autopilot.

---

## Migration path to GKE

Swap DOKS → GKE Autopilot/Standard, in-cluster Redis → Memorystore. Neon stays (or migrates to Cloud SQL). Manifests are nearly identical — add `overlays/gke/` and point Argo CD at the new cluster.

---

## Economics

Planned pricing: **$0.99/month** or **$9.99/year**. Break-even on a ~$36/mo DOKS + load balancer baseline is roughly 40 paid subscribers. A free tier (e.g. 1 game/month) is under consideration to lower the conversion barrier when billing goes live.
