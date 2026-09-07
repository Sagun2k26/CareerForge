# CareerForge — AI Career Growth Platform

CareerForge turns a resume and target role into a structured candidate profile, role-fit assessment, skill-gap analysis, personalized roadmap, RAG-grounded interview preparation, stateful mock interviews, and JD-tailored resume rewrites.

## Architecture

```text
Next.js 14 + TypeScript
          |
       REST/JWT
          v
+-------------------------- Go backend ---------------------------+
| auth / validation / handlers                                  |
|                          |                                     |
|                 Agent Orchestrator                             |
|                          |                                     |
|       +------------------+------------------+                   |
|       |                  |                  |                   |
|  Role-Fit Agent     Skill-Gap Agent    Roadmap Agent           |
|       |                  |                  |                   |
|       +------------------+----------> Interview-Prep Agent      |
|                          |                    |                 |
|                     typed workflow state      |                 |
|                          |                    |                 |
|              +-----------+---------+----------+                 |
|              |                     |                            |
|         LLM providers          RAG / tools                      |
|   Mock / Gemini / OpenAI /     chromem-go                       |
|          Anthropic             question bank                    |
+--------------+---------------------+-----------------------------+
               |                     |
          PostgreSQL          persistent vector index
```

The **Resume Analysis Agent** runs asynchronously when a PDF/DOCX is uploaded: text extraction -> LLM structured profile -> PostgreSQL. The role-specific analysis workflow then consumes that profile. Agents do not call each other directly; the orchestrator passes typed results through shared workflow state.

### Dependency-aware execution

```text
Resume profile
     |
     +--------+--------+
     |                 |
Role-Fit Agent    Skill-Gap Agent       <- run concurrently
                       |
              +--------+--------+
              |                 |
         Roadmap Agent     Interview-Prep Agent   <- run concurrently
                                |
                           RAG retrieval
```

The orchestrator is intentionally a small Go DAG scheduler rather than a separate LangGraph/Python service. The workflow is fixed and small, so goroutines + typed state provide the required parallelism and dependency handling without another runtime or network hop.

## Core features

1. **Resume Analysis Agent** — PDF/DOCX extraction and LLM parsing into structured skills, experience, education and summary, executed asynchronously by a bounded Go worker pool.
2. **Role-Fit Agent** — independent hiring-style fit score, strengths and concerns.
3. **Skill-Gap Agent** — matched/missing skills and readiness score against the selected role.
4. **Roadmap Agent** — dependency-aware personalized learning plan from missing skills.
5. **Interview-Prep Agent + RAG** — semantic retrieval of role/gap-relevant interview questions from an embedded vector store.
6. **Stateful mock interview** — multi-turn transcript persistence, answer scoring and RAG-grounded questions.
7. **Provider-agnostic AI** — one `llm.Provider` interface for Mock, Gemini, OpenAI and Anthropic.
8. **JD-tailored resume** — LLM rewrite constrained to existing candidate experience.
9. **JWT auth + PostgreSQL persistence**.

## Tech stack

| Layer | Choice |
|---|---|
| Frontend | Next.js 14, TypeScript, Tailwind |
| Backend | Go 1.22, chi |
| Database | PostgreSQL, pgx/v5 |
| AI | Gemini API, OpenAI, Anthropic, deterministic Mock |
| RAG | Gemini/OpenAI embeddings + chromem-go vector store |
| Orchestration | lightweight dependency DAG + goroutines + typed workflow state |
| Async | bounded in-process worker pool |
| Auth | JWT + bcrypt |
| Packaging/CI | Docker, docker-compose, GitHub Actions |

## AI provider configuration

The default provider is `mock`, so the app works without external credentials. For real semantic RAG and LLM output, configure a hosted provider.

```env
LLM_PROVIDER=gemini
LLM_MODEL=gemini-3.7-flash
LLM_API_KEY=your_key
```

Gemini generation uses `gemini-3.7-flash`; embeddings use `gemini-embedding-001`. Vector data is stored in a provider/model-specific directory so embeddings with different dimensions are never mixed.

## Main flow

```text
Upload resume
  -> Resume Analysis Agent
  -> structured profile persisted

Choose target role + Analyze
  -> Agent Orchestrator
  -> Role-Fit + Skill-Gap in parallel
  -> Roadmap + RAG Interview Prep in parallel when dependencies are ready
  -> typed outputs aggregated
  -> one transaction persists analysis + roadmap
  -> frontend shows role fit, gaps, RAG questions and roadmap
```

## Local run

```bash
cd backend
cp .env.example .env
docker compose up --build
```

Then:

```bash
cd frontend
cp .env.local.example .env.local
npm install
npm run dev
```

## Tests

```bash
cd backend
go test ./...
```

Tests cover deterministic model fixtures, JSON extraction, embeddings, Gemini request/response wiring, the worker pool, and dependency-aware orchestration.
