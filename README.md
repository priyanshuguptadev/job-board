# Headless Job Board & ATS REST API

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-15+-336791?style=flat&logo=postgresql)](https://www.postgresql.org/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

A high-performance, single-tenant, headless Job Board & Applicant Tracking System (ATS) REST API written in **Go (1.22+)**, backed by **PostgreSQL (15+)** and **S3-compatible object storage**.

Designed for zero-friction self-hosting and seamless integration with custom career pages, internal recruiting dashboards, Zapier, Slack, and HR automation workflows.

---

## Architecture & System Flow

```mermaid
flowchart TD
    subgraph ClientLayer ["Client Layer"]
        PublicWeb["Public Careers Frontend<br/>(Uses jb_pub_... key)"]
        AdminDashboard["Internal ATS / Admin Tools<br/>(Uses jb_sec_... key)"]
    end

    subgraph ServerLayer ["Job Board API Server (Go)"]
        AuthMW["API Key Auth Middleware<br/>(Public vs Secret Scope)"]
        RateLimitMW["Token Bucket Rate Limiter<br/>(Per Client IP)"]
        Router["HTTP Router (Chi)"]
        
        subgraph ServicesLayer ["Core Services"]
            JobSvc["Job Service"]
            AppSvc["Application & Pipeline Service"]
            WebhookSvc["Outbound Webhook Dispatcher"]
            StorageSvc["S3 Storage Client"]
        end
        
        OpenAPI["Interactive Swagger UI<br/>(/docs, /openapi.json)"]
    end

    subgraph InfraLayer ["Infrastructure"]
        Postgres[("PostgreSQL 15+<br/>Relational + JSONB")]
        S3Bucket[("S3 / MinIO / R2<br/>Resume Attachments")]
        ExternalWebhooks["External Webhook Subscribers<br/>(Slack, Zapier, Webhooks)"]
    end

    PublicWeb -->|Read Jobs & Apply| RateLimitMW
    RateLimitMW --> AuthMW
    AdminDashboard -->|Manage Jobs, Pipeline & Webhooks| AuthMW
    AuthMW --> Router
    Router --> JobSvc
    Router --> AppSvc
    Router --> OpenAPI
    JobSvc --> Postgres
    AppSvc --> Postgres
    AppSvc --> StorageSvc
    StorageSvc --> S3Bucket
    AppSvc -.->|Trigger Event| WebhookSvc
    WebhookSvc -->|HMAC-SHA256 Signed POST| ExternalWebhooks
```

---

## Key Features

- **Scoped API Key Authentication**:
  - `jb_pub_...` (**Public**): Fetch active job listings, filter departments, and submit candidate applications.
  - `jb_sec_...` (**Admin**): Full CRUD on jobs, candidate pipeline tracking, stage transitions, reviewer notes, and webhook subscriptions.
- **Dynamic Application Forms**: Configure arbitrary custom questions (`text`, `number`, `url`, `select`, `textarea`, `boolean`) per job posting backed by PostgreSQL `JSONB`.
- **Secure S3 Resume Handling**: Validates resumes by magic bytes & MIME types (PDF, DOCX, DOC up to 10MB) and generates short-lived presigned download URLs. Compatible with AWS S3, Cloudflare R2, MinIO, or Google Cloud Storage.
- **Asynchronous Outbound Webhooks**: Dispatches lifecycle events (`job.published`, `application.created`, `application.stage_updated`, `application.rejected`, etc.) with exponential retries and **HMAC-SHA256 signature verification**.
- **In-Memory Rate Limiting**: Built-in token bucket rate limiting (`golang.org/x/time/rate`) per client IP for public endpoints—no Redis dependency.
- **Interactive Documentation**: Embedded Swagger UI at `/docs` and raw OpenAPI 3.0 specification at `/openapi.json`.
- **Zero-Friction Operations**: Auto-runs database migrations on startup using embedded `golang-migrate` scripts, complete with a built-in CLI for key generation and migration management.

---

## Tech Stack

| Layer | Technology | Description |
| :--- | :--- | :--- |
| **Language & Runtime** | Go 1.22+ | Single compiled static binary, low memory footprint (~20MB), high throughput |
| **HTTP Router** | `chi` v5 | Idiomatic `net/http` router with lightweight middleware chaining |
| **Database** | PostgreSQL 15+ | Relational schema with `JSONB` for custom dynamic application schemas |
| **Schema Migrations** | `golang-migrate` | Embedded migration files executed automatically at startup or via CLI |
| **Object Storage** | AWS SDK for Go v2 | S3-compatible client supporting AWS S3, MinIO, Cloudflare R2, and GCS |
| **Rate Limiter** | `golang.org/x/time/rate` | Per-client IP token bucket limiter requiring no external caching tier |
| **API Documentation** | OpenAPI 3.0 & Swagger UI | Self-hosted documentation UI at `/docs` |

---

## Getting Started

### Option A: Turnkey Setup with Docker Compose (Recommended)

The included `docker-compose.yml` spins up PostgreSQL 15, a local MinIO bucket with automatic bucket initialization, and the Job Board API container.

```bash
# 1. Clone the repository
git clone https://github.com/priyanshuguptadev/job-board.git
cd job-board

# 2. Copy the environment file
cp .env.example .env

# 3. Start all services
docker compose up -d
```

The API server will automatically run database migrations and start on `http://localhost:8080`.

To generate your initial API keys:
```bash
# Bootstrap both admin and public keys in one command
docker compose exec api ./jobboard seed
```

---

### Option B: Local Native Go Setup

#### Prerequisites
- **Go 1.22+**
- **PostgreSQL 15+** (running locally or in a container)
- **S3-Compatible Storage** (AWS S3, Cloudflare R2, or local MinIO)

#### 1. Configuration
Create a `.env` file at the root:
```bash
cp .env.example .env
```

Edit `.env` with your PostgreSQL and S3 credentials:
```dotenv
SERVER_PORT=8080
SERVER_ENV=development
SERVER_LOG_LEVEL=info

# Database
DATABASE_URL=postgres://jobboard:jobboard@localhost:5432/jobboard?sslmode=disable

# S3 / MinIO Storage
S3_ENDPOINT=http://localhost:9000
S3_REGION=us-east-1
S3_BUCKET=resumes
S3_ACCESS_KEY_ID=minioadmin
S3_SECRET_ACCESS_KEY=minioadmin
S3_FORCE_PATH_STYLE=true
S3_PRESIGN_EXPIRY_MINUTES=15
```

#### 2. Bootstrap Initial API Keys
```bash
go run ./cmd/server seed
```

This will run pending database migrations and display your Admin (`jb_sec_...`) and Public (`jb_pub_...`) tokens. Save these keys!

#### 3. Start the Server
```bash
go run ./cmd/server server
```

Open [http://localhost:8080/docs](http://localhost:8080/docs) in your browser to explore the interactive Swagger documentation.

---

## CLI & Subcommands Reference

The compiled binary includes subcommands for operations and management:

```text
Usage:
  jobboard server                         Start HTTP server and run auto-migrations
  jobboard migrate up                     Run all pending database migrations
  jobboard migrate down [steps]           Roll back migrations (default: 1 step)
  jobboard migrate status                 Show current migration version
  jobboard keygen --name <n> --scope <s>  Generate an API key (scope: admin | public)
  jobboard seed                           Bootstrap initial admin and public API keys
```

### Examples

```bash
# Start the API server
./jobboard server

# Generate a custom API key for a partner or career frontend
./jobboard keygen --name "Careers Website" --scope public

# Generate an Admin API key for ATS scripts
./jobboard keygen --name "Zapier ATS Sync" --scope admin

# Inspect migration state
./jobboard migrate status
```

---

## Environment Variables

| Variable | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `SERVER_PORT` | `int` | `8080` | Port for the HTTP server to listen on |
| `SERVER_ENV` | `string` | `development` | Environment mode (`development`, `staging`, `production`) |
| `SERVER_LOG_LEVEL` | `string` | `info` | Structured logging level (`debug`, `info`, `warn`, `error`) |
| `SERVER_CORS_ALLOWED_ORIGINS` | `string` | `*` | Comma-separated list of allowed CORS origins |
| `RATE_LIMIT_RPS` | `int` | `20` | Max requests per second per IP for public endpoints |
| `RATE_LIMIT_BURST` | `int` | `50` | Max burst capacity for public rate limiter |
| `DATABASE_URL` | `string` | — | PostgreSQL connection string (`postgres://user:pass@host:5432/db`) |
| `DATABASE_MAX_OPEN_CONNS` | `int` | `25` | Max open database pool connections |
| `DATABASE_MAX_IDLE_CONNS` | `int` | `10` | Max idle database pool connections |
| `S3_ENDPOINT` | `string` | `""` | Custom S3 endpoint URL (set for MinIO, Cloudflare R2, GCS; empty for AWS S3) |
| `S3_REGION` | `string` | `us-east-1` | S3 bucket region (use `auto` for Cloudflare R2 / GCS) |
| `S3_BUCKET` | `string` | — | S3 bucket name for candidate resumes |
| `S3_ACCESS_KEY_ID` | `string` | — | S3 access key ID |
| `S3_SECRET_ACCESS_KEY` | `string` | — | S3 secret access key |
| `S3_FORCE_PATH_STYLE` | `bool` | `false` | Set to `true` for MinIO or path-style S3 emulators |
| `S3_PRESIGN_EXPIRY_MINUTES` | `int` | `15` | Expiration time (in minutes) for presigned resume download links |

---

## API Reference & Interactive Documentation

The API includes a built-in interactive **Swagger UI** and OpenAPI 3.0 specification. Once the server is running locally, navigate to:

- **Interactive Swagger UI**: [http://localhost:8080/docs](http://localhost:8080/docs)
- **Raw OpenAPI 3.0 Spec**: [http://localhost:8080/openapi.json](http://localhost:8080/openapi.json)

Using `/docs`, you can authorize your requests with your API keys (`jb_pub_...` or `jb_sec_...`) and execute requests directly from your browser.

---

### Authentication

Pass your scoped API key via either header:
- `X-API-Key: <token>`
- `Authorization: Bearer <token>`

| Scope | Key Prefix | Permissions |
| :--- | :--- | :--- |
| **Public** | `jb_pub_` | Access public listings, fetch job schemas, and submit applications. |
| **Admin** | `jb_sec_` | Full CRUD on jobs, manage candidates, pipeline stages, notes, and webhooks. |

---

### Standard Error Envelope

All error responses adhere to a consistent JSON structure:
```json
{
  "error": {
    "code": "INVALID_REQUEST",
    "message": "The request payload failed validation.",
    "details": [
      {
        "field": "title",
        "message": "Title is required"
      }
    ]
  }
}
```

---

### Endpoints Overview

#### 1. Public Career Endpoints (`jb_pub_...`)
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/v1/public/jobs` | List published jobs with filtering (`department`, `location`, `employment_type`, pagination) |
| `GET` | `/v1/public/jobs/{slug_or_id}` | Retrieve job details and dynamic `custom_fields` application schema |
| `GET` | `/v1/public/departments` | List distinct departments with active job openings |
| `POST` | `/v1/public/jobs/{job_id}/apply` | Submit application (`multipart/form-data` with resume file and `custom_answers` JSON) |

#### 2. Admin ATS & Pipeline Endpoints (`jb_sec_...`)
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `POST` | `/v1/admin/jobs` | Create a job posting (`draft` or `published`) with custom form fields |
| `GET` | `/v1/admin/jobs` | List all jobs filterable by status (`draft`, `published`, `archived`) and department |
| `GET` | `/v1/admin/jobs/{id}` | Get full job details including application counts |
| `PATCH` | `/v1/admin/jobs/{id}` | Update job metadata, salary range, custom form fields, or status |
| `DELETE`| `/v1/admin/jobs/{id}` | Archive or delete a job |
| `GET` | `/v1/admin/jobs/{id}/applications` | List applications for a job, filterable by hiring stage |
| `GET` | `/v1/admin/applications/{id}` | Get application details, answers, and timeline |
| `PATCH` | `/v1/admin/applications/{id}/stage` | Advance candidate stage (`applied` &rarr; `screening` &rarr; `interviewing` &rarr; `offer` &rarr; `hired`/`rejected`) |
| `GET` | `/v1/admin/applications/{id}/resume` | Generate a short-lived presigned S3 download URL for the resume |
| `POST` | `/v1/admin/applications/{id}/notes` | Add an internal interviewer review note |
| `GET` | `/v1/admin/applications/{id}/notes` | Retrieve all reviewer notes for an application |
| `POST` | `/v1/admin/webhooks` | Register a new webhook subscription URL and target events |
| `GET` | `/v1/admin/webhooks` | List active webhook subscriptions |
| `DELETE`| `/v1/admin/webhooks/{id}` | Remove a webhook subscription |
| `POST` | `/v1/admin/webhooks/{id}/test` | Trigger a test ping event (`webhook.ping`) to the subscriber endpoint |

#### 3. System & Observability
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/healthz` | Health check endpoint reporting API and database connectivity status |
| `GET` | `/docs` | Interactive Swagger UI |
| `GET` | `/openapi.json` | OpenAPI 3.0 specification JSON |

---

## Outbound Webhooks & HMAC Verification

When lifecycle events occur, the server delivers HTTP `POST` requests to subscribed endpoints.

### Supported Events
| Event | Trigger |
| :--- | :--- |
| `job.published` | Job status transitioned to `published` |
| `job.archived` | Job archived or closed |
| `application.created` | Candidate submitted a new application |
| `application.stage_updated` | Application advanced to a new stage |
| `application.rejected` | Candidate rejected with optional reason |
| `webhook.ping` | Test ping sent via `POST /v1/admin/webhooks/{id}/test` |
| `*` | Wildcard: receive all events |

### Webhook Headers
```http
POST /your-webhook-endpoint HTTP/1.1
Host: api.yourcompany.com
Content-Type: application/json
X-JobBoard-Signature: sha256=d3b07384d113edec49eaa6238ad5ff00...
X-JobBoard-Timestamp: 1725100000
```

### Signature Verification Algorithm
1. Extract `X-JobBoard-Signature` (strip `sha256=` prefix) and `X-JobBoard-Timestamp`.
2. Ensure timestamp is within tolerance (e.g., 5 minutes) to prevent replay attacks.
3. Compute `HMAC-SHA256(secret_token, "t=" + timestamp + "." + raw_request_body)`.
4. Compare hex digests using constant-time comparison.

#### Go Verification Example
```go
import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "fmt"
)

func VerifySignature(secret, signatureHex, timestamp string, body []byte) bool {
    payload := fmt.Sprintf("t=%s.%s", timestamp, string(body))
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write([]byte(payload))
    expectedMAC := hex.EncodeToString(mac.Sum(nil))
    return hmac.Equal([]byte(signatureHex), []byte(expectedMAC))
}
```

---

## Testing & Quality Assurance

Run unit and table-driven tests:
```bash
go test -v ./...
```

Run test suite with race detector:
```bash
go test -v -race ./...
```

Run integration tests against a test database:
```bash
TEST_DATABASE_URL="postgres://jobboard:jobboard@localhost:5432/jobboard_test?sslmode=disable" go test -v ./internal/store/postgres/...
```

---

## Project Structure

```text
.
├── cmd/
│   └── server/               # CLI entrypoint (server, migrate, keygen, seed)
├── docs/                     # OpenAPI 3.0 specification & embedded Swagger UI
├── internal/
│   ├── api/                  # HTTP router, error envelopes, and middleware
│   │   ├── middleware/       # API Key auth, CORS, IP rate limiter, structured logger
│   │   └── v1/               # Public and Admin route handlers
│   ├── auth/                 # Scoped API key generation & SHA256 hashing
│   ├── config/               # Environment configuration loader
│   ├── domain/               # Domain entities (Job, Application, Webhook, Note)
│   ├── logger/               # Structured slog JSON logger
│   ├── service/              # Business logic (Jobs, Applications, Webhooks)
│   ├── storage/              # S3/MinIO client & magic-byte resume validator
│   ├── store/postgres/       # PostgreSQL repositories & embedded migrations
│   └── webhook/              # Async worker dispatcher & HMAC signer
├── migrations/               # PostgreSQL schema migration files (.sql)
├── Dockerfile                # Multi-stage container build
├── docker-compose.yml        # Turnkey development setup (API + Postgres + MinIO)
└── go.mod
```

---

## License

This project is open-source under the [MIT License](LICENSE).

