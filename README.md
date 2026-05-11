# Orchid

**Visual AI agent workflow builder** — drag-and-drop canvas for composing LLM-powered automations. Connect 200+ integrations, run Python/JS inline, branch on conditions, loop over data, and deploy agents that execute in parallel on a DAG engine.

![Tech Stack](https://img.shields.io/badge/Go-1.25-00ADD8?style=flat&logo=go) ![React](https://img.shields.io/badge/React-19-61DAFB?style=flat&logo=react) ![License](https://img.shields.io/badge/license-MIT-green?style=flat)

---

## What it does

- **Build agents visually** — drag blocks onto a canvas and wire them together. Every connection is a data flow edge.
- **LLM blocks with tool access** — give an AI block a prompt and a set of tools; it reasons, calls tools, and returns structured output.
- **Deterministic blocks** — HTTP requests, if/else branches, for-each loops, data transforms, filters, sorts, merges, deduplication — no LLM token cost.
- **200+ pre-built tools** — Slack, GitHub, Notion, Gmail, Airtable, Shopify, Stripe, databases, file processing, image generation, and more.
- **Real-time execution** — WebSocket stream shows block-by-block progress as your workflow runs.
- **Multi-provider LLM** — plug in OpenAI, Anthropic, Google, Groq, Ollama, or any OpenAI-compatible provider.

---

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                    Frontend (React 19)               │
│  React Flow canvas · Zustand state · Tailwind CSS   │
│         AMOLED dark theme · WebSocket client        │
└──────────────────────┬──────────────────────────────┘
                       │ HTTP / WebSocket
┌──────────────────────▼──────────────────────────────┐
│                  Backend (Go / Fiber v2)             │
│  JWT auth · Google OAuth · REST API · WS hub        │
│                                                     │
│  ┌───────────────────────────────────────────────┐  │
│  │              DAG Execution Engine             │  │
│  │  Topological sort → parallel block dispatch  │  │
│  │  Retries · circuit breaker · execution log   │  │
│  └───────────────────────────────────────────────┘  │
│                                                     │
│  MySQL (schema) · MongoDB (executions) · Redis      │
└─────────────────────────────────────────────────────┘
```

---

## Workflow blocks

| Category | Blocks |
|---|---|
| **AI** | `llm_inference` (agent with tools), `code_block` (tool call, no LLM) |
| **Flow control** | `if_condition`, `switch`, `for_each`, `sub_agent` |
| **HTTP / API** | `http_request` (bearer / API key / basic auth) |
| **Data** | `transform`, `filter`, `sort`, `limit`, `aggregate`, `merge`, `deduplicate` |
| **Code** | `inline_code` (Python or JavaScript) |
| **I/O** | `variable`, `wait` |
| **Triggers** | `webhook_trigger`, `schedule_trigger` |

### Condition operators
`eq · neq · contains · not_contains · gt · lt · gte · lte · is_empty · not_empty · is_true · is_false · starts_with · ends_with`

### Aggregate functions
`count · sum · avg · min · max · first · last · concat · collect`

### Merge modes
`append · merge_by_key · combine_all`

---

## Integrations (200+ tools)

| Category | Services |
|---|---|
| **Messaging** | Slack, Discord, Microsoft Teams, Telegram, Google Chat |
| **Email** | Gmail (Composio), SendGrid, Mailchimp, Brevo, Twilio |
| **Productivity** | Notion, Airtable, ClickUp, Trello, Jira, Linear, Asana |
| **Calendar** | Google Calendar, Calendly |
| **Cloud storage** | Google Drive, Amazon S3, file upload/download |
| **Code & DevOps** | GitHub, GitLab, Netlify |
| **Social** | Twitter/X, LinkedIn, YouTube |
| **CRM & Sales** | HubSpot, LeadSquared, Referral Monk |
| **Meetings** | Zoom, Google Meet |
| **Databases** | MongoDB, Redis, spreadsheets, CSV/Excel |
| **AI & Media** | Image generation, image editing, describe image, transcribe audio, ML trainer, data analyst |
| **Documents** | PDF reader, Word/HTML-to-PDF, presentation builder |
| **Web** | Web scraper, image search, REST API tester |
| **Finance** | Shopify, Mixpanel, PostHog, Dodo Payments |
| **Blockchain** | 0G storage |
| **Misc** | Canva (Composio), math, time, Python runner, ask-user, webhook sender |

---

## Tech stack

### Backend
- **Go 1.25** + **Fiber v2** — low-allocation HTTP, middleware pipeline
- **MySQL 8** — providers, models, API keys, credentials, agent/workflow schema
- **MongoDB 7** — execution history, logs
- **Redis 7** — session cache, OAuth exchange codes, rate limiting
- **JWT** (golang-jwt/v5) — access tokens (15 min) + refresh tokens (7 days, HTTP-only cookie)
- **Google OAuth 2.0** — exchange-code pattern; JWT never in URL
- **Prometheus** — metrics endpoint at `/metrics`
- **WebSocket** — live execution streaming via `gofiber/contrib/websocket`
- **Chromedp** — headless Chrome for web scraping and HTML-to-PDF

### Frontend
- **React 19** + **TypeScript** + **Vite 7**
- **React Flow** — node-based workflow canvas
- **Zustand** — global state (auth, agents, UI)
- **Tailwind CSS** — AMOLED dark theme, rose-pink `#e91e63` accent
- **Framer Motion** — animations

### Infrastructure
- **Docker Compose** — 5 services: frontend (nginx), backend, MySQL, MongoDB, Redis
- **Multi-stage Dockerfiles** — minimal production images
- **nginx** — SPA routing, static asset serving, health endpoint

---

## Quick start

### Prerequisites
- Docker and Docker Compose
- A Google OAuth client ID/secret *(optional — email/password auth works without it)*

### 1. Clone and configure

```bash
git clone https://github.com/sujal7103/ORCHID.git
cd ORCHID
cp .env.example .env
```

Edit `.env` and set at minimum:

```env
JWT_SECRET=<random-64-char-string>
ENCRYPTION_MASTER_KEY=<random-32-char-hex>
```

### 2. Run

```bash
docker compose up -d
```

Open [http://localhost:3000](http://localhost:3000).

The first user to register automatically becomes admin.

### 3. Add an LLM provider

Log in → **Admin → Providers** → add your API key for OpenAI, Anthropic, Google, or any compatible endpoint.

---

## Environment variables

| Variable | Default | Description |
|---|---|---|
| `JWT_SECRET` | **required** | Signs all JWTs |
| `ENCRYPTION_MASTER_KEY` | **required** | AES key for stored credentials |
| `FRONTEND_PORT` | `3000` | Host port for the UI |
| `BACKEND_PORT` | `3001` | Host port for the API |
| `MYSQL_USER` | `clara` | MySQL username |
| `MYSQL_PASSWORD` | *(set in .env)* | MySQL password — **change before deploying** |
| `MYSQL_DATABASE` | `orchid` | MySQL database name |
| `MYSQL_ROOT_PASSWORD` | *(set in .env)* | MySQL root password — **change before deploying** |
| `JWT_ACCESS_TOKEN_EXPIRY` | `15m` | Access token lifetime |
| `JWT_REFRESH_TOKEN_EXPIRY` | `168h` | Refresh token lifetime (7 days) |
| `ALLOWED_ORIGINS` | `http://localhost:3000,http://localhost:5173` | CORS origins |
| `FRONTEND_URL` | `http://localhost:3000` | Used in OAuth redirect |
| `BACKEND_URL` | `http://localhost:3001` | Used in OAuth callback |
| `GOOGLE_CLIENT_ID` | *(empty)* | Enable Google sign-in |
| `GOOGLE_CLIENT_SECRET` | *(empty)* | Enable Google sign-in |
| `VITE_API_BASE_URL` | `http://localhost:3001` | API URL seen by browser |
| `VITE_WS_URL` | `ws://localhost:3001` | WebSocket URL seen by browser |
| `SEARXNG_URL` | *(empty)* | Self-hosted search engine |
| `E2B_API_KEY` | *(empty)* | Sandboxed code execution |
| `COMPOSIO_API_KEY` | *(empty)* | Composio integration hub |

---

## Google OAuth setup

1. Go to [Google Cloud Console](https://console.cloud.google.com/) → **APIs & Services → Credentials**.
2. Create an **OAuth 2.0 Client ID** (Web application).
3. Add authorized redirect URI: `http://localhost:3001/api/auth/google/callback` (or your `BACKEND_URL`).
4. Set `GOOGLE_CLIENT_ID` and `GOOGLE_CLIENT_SECRET` in `.env`.

Orchid uses the **exchange-code pattern**: the JWT is stored in Redis under a one-time opaque code (60 s TTL) — it never appears in the browser URL.

---

## Production deployment

### Checklist

- [ ] Set strong random values for `JWT_SECRET` (64+ chars) and `ENCRYPTION_MASTER_KEY` (32-byte hex).
- [ ] Set `MYSQL_ROOT_PASSWORD` / `MYSQL_PASSWORD` to non-defaults.
- [ ] Point `FRONTEND_URL` and `BACKEND_URL` to your real domain.
- [ ] Set `ALLOWED_ORIGINS` to your production frontend URL only.
- [ ] Put TLS termination in front (nginx reverse proxy or a load balancer).
- [ ] Enable Redis persistence (AOF is already on in the default config).
- [ ] Back up the MySQL and MongoDB volumes.

### Scaling

- The backend is stateless; run multiple replicas behind a load balancer.
- Redis is required for JWT exchange codes and rate limiting — share one Redis across replicas.
- MongoDB holds execution history; it can grow large — set a TTL index or archive old runs.

---

## API overview

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/auth/register` | Create account |
| `POST` | `/api/auth/login` | Email/password login |
| `POST` | `/api/auth/refresh` | Rotate refresh token |
| `POST` | `/api/auth/logout` | Revoke refresh token |
| `GET` | `/api/auth/me` | Current user |
| `GET` | `/api/auth/google` | Redirect to Google |
| `GET` | `/api/auth/google/callback` | Google OAuth callback |
| `POST` | `/api/auth/google/exchange` | Exchange opaque code → JWT |
| `GET` | `/api/agents` | List agents |
| `POST` | `/api/agents` | Create agent |
| `GET` | `/api/agents/:id` | Get agent |
| `PUT` | `/api/agents/:id` | Update agent |
| `DELETE` | `/api/agents/:id` | Delete agent |
| `GET` | `/api/workflows` | List workflows |
| `POST` | `/api/workflows` | Create workflow |
| `POST` | `/api/workflows/:id/execute` | Run workflow |
| `GET` | `/api/executions/:id` | Execution status |
| `WS` | `/ws/executions/:id` | Live execution stream |
| `GET` | `/api/tools` | List available tools |
| `GET` | `/api/providers` | List LLM providers |
| `POST` | `/api/admin/providers` | Add provider |
| `GET` | `/api/admin/models` | List models |
| `GET` | `/metrics` | Prometheus metrics |
| `GET` | `/health` | Health check |

---

## Development

### Backend

```bash
cd backend
go run ./cmd/server
```

Requires MySQL, MongoDB, and Redis running locally (or use `docker compose up mongodb redis mysql -d`).

### Frontend

```bash
cd frontend
npm install
npm run dev
```

Runs at `http://localhost:5173` with `VITE_API_BASE_URL=http://localhost:3001`.

### Database migrations

Migrations run automatically on backend startup from `backend/migrations/migrations/`:

| File | Description |
|---|---|
| `001_initial_schema.sql` | Core tables: users, agents, workflows, providers, models |
| `002_global_tiers.sql` | Usage tiers and limits |
| `003_mcp_connections_columns.sql` | MCP server connection metadata |
| `004_device_tokens.sql` | Push notification device tokens |

---

## License

MIT
