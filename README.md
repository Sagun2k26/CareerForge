# CareerForge — AI Career Growth Platform

An AI-powered platform that helps engineers go from *"here is my resume"* to *"here is exactly what I need to learn, practise, and rewrite to land the role I want."*

Upload a resume, pick a target role, and the platform parses your experience, finds your skill gaps, generates a personalised learning roadmap, runs scored mock interviews, serves a role-specific question bank with semantic search, and rewrites your resume against a real job description.

Built as a **Go modular monolith** with a **Next.js** frontend. It runs end-to-end with **zero API keys** thanks to a built-in mock LLM provider, and swaps to real Anthropic or OpenAI models by setting a single environment variable.

---

## Features

1. **Resume upload + parsing** — PDF/DOCX upload, text extraction, and LLM-structured parsing (skills, experience, education) processed asynchronously by an in-process worker pool.
2. **Skill-gap analysis** — compares parsed resume skills against a target role's required skills and produces matched/missing skills with a gap score.
3. **Learning roadmap** — generates an ordered, trackable roadmap to close the gaps, with per-item `todo`/`done` progress.
4. **Mock interviews** — stateful, multi-turn interviews grounded in the role, with per-answer scoring and a final summary.
5. **Question bank + RAG** — role-specific questions with **semantic search** (vector embeddings) and on-demand model answers.
6. **JD-tailored resume rewrite** — rewrites a parsed resume against a pasted job description.
7. **Auth** — email/password registration and login with JWT-protected APIs.

---

## Architecture

```
                         ┌──────────────────────────┐
                         │   Next.js 14 (App Router) │
                         │   Tailwind · TypeScript   │
                         └────────────┬─────────────┘
                                      │  REST + JWT (Bearer)
                                      ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      Go modular monolith (chi)                        │
│                                                                       │
│   middleware: RequestID · RealIP · Recoverer · Timeout · RateLimit    │
│                                                                       │
│   ┌────────┐ ┌────────┐ ┌────────┐ ┌──────────┐ ┌──────────────┐      │
│   │  user  │ │ resume │ │  role  │ │ analysis │ │ questionbank │ …    │
│   └────────┘ └────────┘ └────────┘ └──────────┘ └──────────────┘      │
│   each domain = repository.go + service.go + handler.go               │
│                                                                       │
│   shared services:                                                    │
│   ┌──────────┐  ┌──────────────┐  ┌──────────────┐  ┌───────────┐     │
│   │   auth   │  │  llm.Client  │  │  rag.Engine  │  │  worker   │     │
│   │  (JWT)   │  │ (provider-   │  │ (chromem-go  │  │  (async   │     │
│   │          │  │  agnostic)   │  │  vector DB)  │  │  pool)    │     │
│   └──────────┘  └──────┬───────┘  └──────────────┘  └───────────┘     │
└────────────────────────┼──────────────────────────────────────────────┘
                         │                          │
            ┌────────────┴───────────┐     ┌────────┴────────┐
            ▼            ▼            ▼     ▼                 ▼
       ┌────────┐  ┌──────────┐ ┌────────┐ ┌──────────────────────┐
       │  Mock  │  │Anthropic │ │ OpenAI │ │  PostgreSQL (pgx/v5)  │
       │provider│  │ Messages │ │  Chat  │ │  migrations on boot   │
       └────────┘  └──────────┘ └────────┘ └──────────────────────┘
```

Each business domain is a self-contained package following a strict
`repository.go` (data access) → `service.go` (business logic) → `handler.go`
(HTTP) split. Cross-cutting concerns (`auth`, `llm`, `rag`, `worker`,
`database`, `httpx`, `config`) live in shared packages and are injected from
`internal/server`.

---

## Tech stack

| Layer        | Choice                                                            |
|--------------|-------------------------------------------------------------------|
| Backend      | Go 1.22, chi v5 router, JWT (HS256), bcrypt                       |
| Database     | PostgreSQL via pgx/v5 (pgxpool), idempotent SQL migrations        |
| LLM          | Provider-agnostic client — Mock (default), Anthropic, OpenAI      |
| Vector / RAG | chromem-go embeddable vector store                                |
| Async        | In-process worker pool (buffered channel + goroutines)            |
| Frontend     | Next.js 14 (App Router), TypeScript, Tailwind CSS, lucide-react   |
| CI           | GitHub Actions (`go vet/build/test`, `npm run build`)             |
| Packaging    | Multi-stage Dockerfile + docker-compose (Postgres + API)          |

---

## Getting started

### Option A — Docker (fastest)

```bash
cd backend
docker compose up --build
```

This starts Postgres and the API (`http://localhost:8080`). Migrations run and
seed data loads automatically on boot. The default LLM provider is the mock, so
no API keys are required.

Then run the frontend:

```bash
cd frontend
cp .env.local.example .env.local   # points at http://localhost:8080
npm install
npm run dev                        # http://localhost:3000
```

### Option B — Run locally

**Backend**

```bash
cd backend
cp .env.example .env               # adjust DATABASE_URL if needed
go mod tidy                        # generates go.sum
go run ./cmd/server
```

Requires a reachable Postgres. Default DSN:
`postgres://postgres:postgres@localhost:5432/career?sslmode=disable`.

**Frontend**

```bash
cd frontend
cp .env.local.example .env.local
npm install
npm run dev
```

Open `http://localhost:3000`, register an account, and upload a resume.

---

## Environment variables

**Backend** (`backend/.env`)

| Variable        | Default                                                              | Notes                                          |
|-----------------|----------------------------------------------------------------------|------------------------------------------------|
| `PORT`          | `8080`                                                               | HTTP listen port                               |
| `DATABASE_URL`  | `postgres://postgres:postgres@localhost:5432/career?sslmode=disable` | Postgres DSN                                   |
| `JWT_SECRET`    | dev value                                                            | **Set a strong secret in production**          |
| `LLM_PROVIDER`  | `mock`                                                               | `mock` \| `anthropic` \| `openai`              |
| `LLM_MODEL`     | `claude-sonnet-4-6`                                                  | Model id for the chosen provider               |
| `LLM_API_KEY`   | *(empty)*                                                            | If empty, the client falls back to the mock    |

**Frontend** (`frontend/.env.local`)

| Variable               | Default                 | Notes                  |
|------------------------|-------------------------|------------------------|
| `NEXT_PUBLIC_API_URL`  | `http://localhost:8080` | Base URL of the Go API |

Setting `LLM_PROVIDER=anthropic` (or `openai`) **with** a valid `LLM_API_KEY`
switches every LLM and embedding call to the real provider — no code changes.

---

## API overview

All routes are prefixed with `/api`. Auth and role routes are public; everything
else requires a `Authorization: Bearer <token>` header.

```
GET    /health
POST   /api/auth/register          { email, password }            → { token, user }
POST   /api/auth/login             { email, password }            → { token, user }
GET    /api/me

GET    /api/roles
GET    /api/roles/{id}

POST   /api/resumes                multipart "file" (PDF/DOCX)
GET    /api/resumes
GET    /api/resumes/{id}
POST   /api/resumes/{id}/tailor    { job_description }

POST   /api/analyses               { resume_id, role_id }
GET    /api/analyses
GET    /api/analyses/{id}
PATCH  /api/analyses/{id}/progress { item_order, status }

GET    /api/questions?role_id=&topic=
GET    /api/questions/search?q=&role_id=
GET    /api/questions/{id}/model-answer?role_name=
POST   /api/questions/{id}/practiced

POST   /api/interviews             { role_id }
GET    /api/interviews
GET    /api/interviews/{id}
POST   /api/interviews/{id}/answer { answer }
```

---

## Testing

```bash
cd backend
go test ./...
```

Tests cover the mock LLM (schema-valid fixtures, deterministic normalized
embeddings, JSON extraction, score round-trips) and the worker pool. CI runs
`go vet`, `go build`, and `go test` on every push and PR.

---

## Technical decisions

**Modular monolith over microservices.** Every domain is an isolated package
with a clean `repository → service → handler` boundary, so the code reads like a
set of services without the operational tax of distributed systems. If a domain
ever needs to be extracted, the seams already exist.

**Provider-agnostic LLM client.** All AI calls go through a single
`llm.Provider` interface (`Complete`, `Embed`, `Name`). The default
`MockProvider` returns schema-correct fixtures keyed off `[task:…]` markers
embedded in each prompt, so the entire product is fully functional with zero API
keys — ideal for local dev, CI, and demos. Swapping to Anthropic or OpenAI is a
config change, not a code change.

**Deterministic mock embeddings for RAG.** The mock provider produces
sha256-derived, L2-normalized 256-dim vectors. Semantic search therefore works
offline and is reproducible in tests, while the real providers transparently
take over when configured.

**In-process worker pool.** Resume parsing is slow (extraction + LLM), so it
runs asynchronously on a bounded goroutine pool. The resume record moves through
`uploaded → parsing → parsed/failed`, and the frontend polls until it settles.
This keeps uploads snappy without introducing a separate queue/broker.

**chromem-go for vectors.** An embeddable, dependency-free vector store keeps the
stack to two processes (API + Postgres) while still demonstrating real
retrieval-augmented generation.

**Idempotent migrations and seeding.** Migrations run on boot with
`IF NOT EXISTS`, and seed data uses deterministic UUIDs (`uuid.NewSHA1`) so the
service can restart safely without duplicating roles or questions.

**JWT + bcrypt.** Stateless auth (HS256) with bcrypt-hashed passwords keeps the
API horizontally scalable and avoids server-side session storage.

---

## Project layout

```
.
├── backend/
│   ├── cmd/server/            # entrypoint (graceful shutdown)
│   ├── internal/
│   │   ├── config/            # env-driven configuration
│   │   ├── database/          # pgxpool connect + migrate
│   │   ├── auth/              # JWT issue + middleware
│   │   ├── httpx/             # JSON helpers + error envelope
│   │   ├── llm/               # provider-agnostic client, mock/anthropic/openai
│   │   ├── rag/               # chromem-go engine
│   │   ├── worker/            # async job pool
│   │   ├── user/ resume/ role/ analysis/ questionbank/ interview/   # domains
│   │   └── server/            # wiring, routing, rate limit, seed
│   ├── migrations/            # SQL applied on boot
│   ├── seed/                  # roles.json + questions.json
│   ├── Dockerfile
│   └── docker-compose.yml
├── frontend/                  # Next.js 14 app
│   ├── app/                   # routes (App Router)
│   ├── components/            # AuthProvider, Nav, UI primitives
│   └── lib/api.ts             # typed API client
└── .github/workflows/ci.yml
```

> **Note:** `backend/go.sum` is generated by `go mod tidy` on first build (and in
> CI) and is intentionally not committed in this initial scaffold.
