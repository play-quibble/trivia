# Quibble

A self-hosted Kubernetes trivia platform where hosts create and run live quiz sessions and players join via a game code — no account required to play. Built as both a real product and a hands-on Kubernetes learning project.

---

## How it works

### Creating content

1. **Create a question bank** — a reusable library of questions. Each question is either a text answer (accepts a list of valid spellings) or multiple choice (2–6 options, one marked correct). Questions are assigned point values and can be reordered.

2. **Build a quiz** — a quiz is a curated programme of rounds. Add rounds in order, then open each round's question picker to choose questions from any of your banks. Questions within a round can be reordered with the up/down controls.

### Running a game

3. **Launch a game** — from the quiz builder (or the Games page), click *Launch Game* / *Create Game*. This creates a game record in the database and generates a 6-character join code. You are taken straight to the **host panel**.

4. **Players join** — players navigate to the app's root URL, enter the join code and a display name, and are placed in the lobby. No account is required. The host panel shows players as they arrive.

5. **Start the game** — the host clicks *Start Game*. The first round begins.

6. **Release questions one at a time** — within each round the host clicks *Release Question* to reveal questions to players one at a time. All previously released questions for the current round remain visible to players so they can answer at their own pace. The host panel shows a live answer count per question.

7. **End the round → review answers** — when all questions have been released the host clicks *End Round → Review*. The host sees a review screen listing every player's answer for each question. Any answer marked incorrect can be flipped to correct with *Mark ✓* (useful for free-text questions with unexpected-but-valid spellings).

8. **Release scores** — the host clicks *Release Scores*. Each player receives a per-question result (prompt, correct answer, their answer, points earned) and a round total. A leaderboard is shown after every round.

9. **Repeat or end** — the host starts the next round or ends the game. A final leaderboard is shown to all players when the game is over.

---

## What's built

- **Go API** (`apps/api`) — Chi router, Auth0 JWT middleware, full CRUD for banks, questions, quizzes, rounds, and games. WebSocket hub manages live game rooms with a phase-based state machine (lobby → question → round review → leaderboard → ended).
- **Next.js frontend** (`apps/web`) — App Router, server actions keep the auth token server-side. Host dashboard covers banks, quiz builder, and games list. Host game panel and player game panel driven by WebSocket messages.
- **Database** — 11 goose migrations on Neon Postgres, sqlc-generated query layer.
- **Realtime** — single-hub WebSocket architecture; per-question submission tracking; host-only review messages; per-player score payloads at round end.

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
        game/               # bank/question/quiz/game HTTP handlers + business logic
        realtime/           # WebSocket hub, room state machine
        store/              # sqlc-generated DB layer
        user/               # host account service
      migrations/           # goose SQL migrations (0001–0011)
      queries/              # raw SQL for sqlc
    web/                    # Next.js frontend (App Router)
      src/
        app/
          (host)/           # authenticated host routes: banks, quizzes, games
          (player)/         # unauthenticated player routes: join, play
        components/         # QuizBuilder, HostGame, PlayerGame, Navbar, …
        lib/api/            # server-side API clients (auth token never leaves server)
        types/              # shared TypeScript types
  deploy/
    k8s/                    # Kustomize base + overlays (dev, prod, gke)
    argocd/                 # Argo CD Application definitions
  docs/
```

---

## Data model

**Postgres** (persistent):

- `users` — host accounts, keyed by Auth0 `sub`
- `question_banks` — reusable question libraries owned by a host
- `questions` — belongs to a bank; `text` or `multiple_choice` type; choices and accepted answers stored as JSONB
- `quizzes` — a host-curated programme of rounds; one quiz can be played multiple times
- `quiz_rounds` — ordered rounds within a quiz, each with an optional title
- `quiz_round_questions` — ordered join table linking questions to a round; position is explicit so hosts can reorder
- `games` — a live play instance; linked to a quiz (or a legacy bank); holds a 6-char join code, status, and round/question index
- `game_players` — display name, join time, running score per game
- `answers` — one row per player per question, used for post-game review
- `subscriptions` — table exists now, stays empty until Stripe billing is live

**In-memory hub** (per API process, per room):

Live game state lives in the WebSocket hub while a game is in progress. The hub tracks the current phase, which questions have been released, and all player submissions for the active round so the host can review and override answers before scores are published.

---

## Realtime design

One WebSocket hub per API pod, rooms keyed by game code. Each room is a state machine with phases:

`lobby` → `question` → `round_review` → `leaderboard` → *(next round or)* `ended`

Message directions:

- **Host → server:** `start_game`, `release_question`, `end_round`, `override_answer`, `release_scores`, `start_next_round`, `end_game`
- **Player → server:** `submit_answer` (includes `question_id` so multiple questions can be open simultaneously)
- **Server → all players:** `question_released`, `answer_accepted`, `scoreboard_update`, `round_ended`, `round_leaderboard`, `game_ended`
- **Server → individual player:** `round_scores` (per-player result payload with each question's outcome)
- **Server → host only:** `round_review` (all player answers for review/override before scores are released)

Starting single-replica. Scaling out later uses Redis Pub/Sub fan-out — an additive change, not a rewrite. Sticky sessions on ingress cover the interim.

---

## Getting started (local dev)

### Prerequisites

- Go 1.23+
- Node 20+
- A running Postgres instance (or a [Neon](https://neon.tech) branch)
- A running Redis instance
- An Auth0 tenant with an SPA app and an API configured

### Run the API

```bash
cd apps/api
cp .env.example .env
# Fill in DATABASE_URL, REDIS_ADDR, AUTH0_DOMAIN, AUTH0_AUDIENCE
# DEV_AUTH_TOKEN can be any string for local dev — the frontend uses it to bypass Auth0
make migrate-up
make run
```

### Run the frontend

```bash
cd apps/web
cp .env.local.example .env.local
# Fill in AUTH0_* vars and API_URL=http://localhost:8080
# Set DEV_AUTH_TOKEN to the same value used in the API
npm install
npm run dev
```

### Useful API make targets

```bash
make migrate-up      # apply all pending migrations
make migrate-status  # show current migration state
make sqlc            # regenerate store from queries/
make lint            # run golangci-lint
```

---

## What's next

1. **QR code generation** — display a scannable code on the host panel so players can join without typing the game code
2. **Kubernetes manifests** — Kustomize base + overlays, test on local k3d/kind, deploy to DOKS
3. **Argo CD + observability** — kube-prometheus-stack (Prometheus, Grafana, Alertmanager), optionally Loki
4. **GitHub Actions CI/CD** — test → build → push to registry; Argo Image Updater handles deploys
5. **Stripe billing** — implement `EntitlementChecker` backed by Stripe webhooks + `subscriptions` table

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
