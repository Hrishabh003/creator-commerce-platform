# Creator Commerce Operations Platform

A full-stack operations workspace for a small online store: manage products, orders, customers, support tickets, and email campaigns from one place, with an AI-assisted support workflow where **a human always approves the reply**.

**Status:** in development. This is a focused reference build of the core flows, not a production system.

**Stack:** Go · GraphQL · PostgreSQL · TypeScript · React (Vite) · Expo / React Native · GCP-ready (container-friendly, config via env vars)

## What it does

| Area | Implemented |
| --- | --- |
| Support (AI-assisted) | Create a ticket → it is categorized, relevant knowledge-base articles are retrieved, and a reply is drafted for an operator to edit, approve, or reject |
| Orders | List orders with customer details |
| Products | List products, update stock |
| Campaigns | List campaigns and their status |
| Web | React admin dashboard with all four areas |
| Mobile | Expo companion app: approve or reject AI drafts, browse orders |

## Architecture

```
React web (Vite) ─┐
                  ├─►  shared/ (typed GraphQL client + types)  ─►  Go GraphQL API ─► PostgreSQL
Expo mobile ──────┘                                                   │
                                                                      └─► AI pipeline: categorize → retrieve (Postgres FTS) → draft
```

- `backend/` Go API. GraphQL schema, PostgreSQL access, embedded SQL migrations, AI pipeline.
- `web/` React + TypeScript admin dashboard.
- `mobile/` Expo app.
- `shared/` the API client and TypeScript types imported by both `web/` and `mobile/`, so a schema change is fixed in one place.

## Design decisions

- **Human in the loop.** `createTicket` saves the draft as `pending`. Nothing is sent until `reviewDraft(approve: true)`. Only pending drafts can be reviewed, and approving marks the ticket as replied inside one transaction.
- **Grounded drafts.** Retrieval uses PostgreSQL full-text search (`tsvector` + GIN index) over knowledge-base articles. The drafter only receives those articles, and the system prompt tells the model to hand off to a teammate when the context does not answer the question.
- **Works without an API key.** With no `ANTHROPIC_API_KEY`, a template drafter quotes the retrieved articles. With a key, Claude drafts the reply, and any provider error falls back to the template so ticket intake never fails.
- **No N+1 queries.** List queries load related customers and drafts with one batched query each (`WHERE id = ANY($1)`) and attach them in memory.
- **Plain SQL migrations.** Files in `backend/internal/db/migrations` are embedded in the binary and applied once each, in order, inside transactions.

## Run it locally

Requirements: Go 1.22+, Node 20+, Docker.

```bash
# 1. Database
docker compose up -d

# 2. API (applies migrations and seed data on start)
cd backend
go mod tidy          # first run only; generates go.sum
go test ./...
go run ./cmd/server  # http://localhost:8080/graphql

# 3. Web dashboard
cd ../web
npm install
npm run dev          # http://localhost:5173

# 4. Mobile app
cd ../mobile
npm install
npx expo install --fix   # aligns dependency versions with your Expo SDK
npx expo start
```

For a physical phone, set `EXPO_PUBLIC_API_URL` to `http://<your-computer-LAN-IP>:8080/graphql`. Copy `.env.example` for all variables. To use Claude for drafts, set `ANTHROPIC_API_KEY`.

### Try the API

```bash
curl -s localhost:8080/graphql -H 'content-type: application/json' -d '{
  "query": "mutation { createTicket(customerId: 1, subject: \"Charged twice\", body: \"I think I was charged twice for my order, can I get a refund?\") { id category drafts { status sources body } } }"
}'
```

## GraphQL API

Queries: `products`, `orders(status, limit)`, `tickets(status, limit)`, `ticket(id)`, `campaigns`
Mutations: `createTicket`, `reviewDraft`, `updateStock`

## Tests

`go test ./...` covers ticket categorization, keyword extraction, and the template drafter. CI (`.github/workflows/ci.yml`) runs `go vet`, `go test`, a TypeScript check, and the web build.

## Roadmap

- Authentication and per-operator permissions
- Order and customer detail pages, order creation
- Campaign composer and scheduling
- Replace the keyword categorizer with an LLM classifier and compare accuracy on labeled tickets
- Embedding-based retrieval (pgvector) alongside full-text search
- Deployment to GCP (Cloud Run + Cloud SQL)
