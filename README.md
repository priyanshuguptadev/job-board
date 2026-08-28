# Headless Job Board REST API

A high-performance, single-tenant, headless Job Board & Applicant Tracking System (ATS) REST API written in **Go (1.22+)**, backed by **PostgreSQL (15+)** and **S3-compatible object storage**.

Designed for zero-friction self-hosting and effortless integration with custom career pages, internal dashboards, and automation workflows.

---

## Architecture

```mermaid
flowchart TD
    subgraph ClientLayer ["Client Layer"]
        PublicWeb["Public Careers Page<br/>(Uses jb_pub_... key)"]
        AdminDashboard["Internal Admin / ATS / Scripts<br/>(Uses jb_sec_... key)"]
    end

    subgraph ServerLayer ["Job Board API Server (Go)"]
        AuthMW["API Key Auth Middleware<br/>(Public vs Secret Scope)"]
        Router["HTTP Router (Chi)"]
        
        subgraph ServicesLayer ["Services"]
            JobSvc["Job Service"]
            AppSvc["Application & Pipeline Service"]
            WebhookSvc["Outbound Webhook Worker"]
            StorageSvc["S3 Storage Client"]
        end
        
        OpenAPI["Swagger UI & OpenAPI 3.0<br/>(/docs, /openapi.json)"]
    end

    subgraph InfraLayer ["Infrastructure"]
        Postgres[("PostgreSQL<br/>Relational + JSONB")]
        S3Bucket[("S3 / MinIO / R2<br/>Resume Attachments")]
        ExternalWebhooks["External Webhook Subscribers<br/>(Slack, Zapier, ATS, etc.)"]
    end

    PublicWeb -->|Read Jobs & Apply| AuthMW
    AdminDashboard -->|Manage Jobs & Pipeline| AuthMW
    AuthMW --> Router
    Router --> JobSvc
    Router --> AppSvc
    Router --> OpenAPI
    JobSvc --> Postgres
    AppSvc --> Postgres
    AppSvc --> StorageSvc
    StorageSvc --> S3Bucket
    AppSvc -.->|Trigger Event| WebhookSvc
    WebhookSvc -->|HMAC Signed POST| ExternalWebhooks
```

---

## Key Features

* **Scoped API Key Auth**:
  * `jb_pub_...` (Public): Read published jobs & departments, submit candidate applications.
  * `jb_sec_...` (Admin): Full CRUD on job postings, candidate pipelines, reviewer notes, and webhook subscriptions.
* **Dynamic Application Forms**: Define custom form fields (URLs, text, selects, numbers) per job using PostgreSQL `JSONB`.
* **S3-Compatible Resume Storage**: Upload resumes directly to AWS S3, Cloudflare R2, MinIO, or Google Cloud Storage with short-lived presigned download links.
* **Outbound Webhook Worker**: Asynchronous event dispatch (`job.published`, `application.created`, `application.stage_updated`) with **HMAC-SHA256 signatures** and exponential retries.
* **In-Memory Rate Limiting**: Token bucket rate limiter (`golang.org/x/time/rate`) per client IP for public endpoints—no Redis required.
* **Interactive OpenAPI & Swagger**: Built-in Swagger UI served at `/docs` and raw spec at `/openapi.json`.
* **Turnkey Operations**: Embedded database migrations via `golang-migrate` and built-in CLI for key generation and server management.

---

## Tech Stack

| Component | Technology | Rationale |
| :--- | :--- | :--- |
| **Language & Runtime** | Go 1.22+ | Single static binary, low memory footprint (~20MB), high throughput |
| **Router** | `chi` | Lightweight, idiomatic `net/http` compatibility |
| **Database** | PostgreSQL 15+ | ACID compliance with `JSONB` for dynamic custom forms |
| **Migrations** | `golang-migrate` (embedded) | Auto-migrates database on startup or via CLI |
| **Object Storage** | AWS S3 SDK (`aws-sdk-go-v2`) | Compatible with AWS S3, MinIO, Cloudflare R2, and GCS |
| **Rate Limiting** | `golang.org/x/time/rate` | In-memory token bucket per client IP with zero external dependencies |
| **Docs** | OpenAPI 3.0 & Swagger UI | Embedded UI at `/docs` and raw spec at `/openapi.json` |

---

## Quick Start

### 1. Prerequisites
* **Go 1.22+**
* **PostgreSQL 15+**
* **S3-compatible Object Storage** (AWS S3, MinIO, Cloudflare R2, etc.)

### 2. Environment Setup
Copy the example environment file and adjust your credentials:
```bash
cp .env.example .env
```

Key environment variables:
```dotenv
SERVER_PORT=8080
DATABASE_URL=postgres://postgres:postgres@localhost:5432/jobboard?sslmode=disable
S3_BUCKET=jobboard-resumes
S3_REGION=us-east-1
S3_ACCESS_KEY_ID=your-key-id
S3_SECRET_ACCESS_KEY=your-secret-key
# Set if using MinIO or local emulators:
# S3_ENDPOINT=http://localhost:9000
# S3_FORCE_PATH_STYLE=true
```

### 3. Generate API Keys
Bootstrap your administrative and public API keys:
```bash
# Generate Admin Key (Full access)
go run ./cmd/server keygen --name "Admin Console" --scope admin
# Outputs: jb_sec_xxxxxxxxxxxxxxxxxxxx

# Generate Public Key (Careers page)
go run ./cmd/server keygen --name "Careers Website" --scope public
# Outputs: jb_pub_xxxxxxxxxxxxxxxxxxxx
```

### 4. Run the Server
```bash
go run ./cmd/server server
```
The server will automatically apply pending database migrations and start listening on `:8080`.

---

## API Overview

### Public Endpoints (`X-API-Key: jb_pub_...`)
* `GET /v1/public/jobs` — List published jobs (filterable by department, location, employment type).
* `GET /v1/public/jobs/{slug_or_id}` — Get job details and custom application form schema.
* `GET /v1/public/departments` — List active departments with job openings.
* `POST /v1/public/jobs/{job_id}/apply` — Submit application with resume (`multipart/form-data`).

### Admin Endpoints (`X-API-Key: jb_sec_...`)
* `POST /v1/admin/jobs` — Create a job posting (`draft` or `published`).
* `PATCH /v1/admin/jobs/{id}` — Update job details, salary range, or status.
* `GET /v1/admin/jobs/{id}/applications` — List applications filterable by stage.
* `PATCH /v1/admin/applications/{id}/stage` — Advance candidate stage (`screening`, `interviewing`, `offer`, `hired`, `rejected`).
* `GET /v1/admin/applications/{id}/resume` — Get presigned S3 download URL for resume.
* `POST /v1/admin/applications/{id}/notes` — Add internal interviewer review note.
* `POST /v1/admin/webhooks` — Subscribe to outbound webhook events.

### Observability & Documentation
* `GET /healthz` — Liveness/readiness health check.
* `GET /docs` — Interactive Swagger UI.
* `GET /openapi.json` — Raw OpenAPI 3.0 specification.

---

## Webhook Verification

Outbound webhooks include signature headers for cryptographic verification:
```http
X-JobBoard-Signature: sha256=<hex_digest>
X-JobBoard-Timestamp: 1787910300
```
Signature payload format: `t=${X-JobBoard-Timestamp}.${raw_body_json}` using HMAC-SHA256 with the subscription's secret token.

---

## Testing

Run the test suite:
```bash
go test -v ./...
```

---

## Project Layout

```text
.
├── cmd/
│   └── server/               # Application entrypoint & CLI subcommands
├── internal/
│   ├── api/                  # HTTP routes, handlers, and middleware
│   ├── config/               # Environment configuration loader
│   ├── domain/               # Core entity definitions and DTOs
│   ├── service/              # Business logic (Jobs, Applications, Webhooks)
│   ├── storage/              # S3 storage client
│   ├── store/postgres/       # PostgreSQL repositories & SQL queries
│   └── webhook/              # Async webhook dispatcher & HMAC signer
├── migrations/               # PostgreSQL schema migrations
└── docs/                     # OpenAPI 3.0 specifications
```
