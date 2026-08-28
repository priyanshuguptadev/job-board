# Implementation Plan & Milestones

## 1. Directory Structure

```text
.
├── cmd/
│   └── server/
│       └── main.go           # Application entrypoint & CLI commands (run, migrate)
├── internal/
│   ├── api/
│   │   ├── middleware/       # Auth (API key), CORS, rate limiting, logging
│   │   ├── v1/               # Public and Admin HTTP handlers
│   │   └── router.go         # Route registrations and OpenAPI mount
│   ├── config/               # Environment configuration loader and validation
│   ├── domain/               # Core entities, interfaces, and DTOs
│   ├── service/              # Business logic (Jobs, Applications, Webhooks)
│   ├── storage/              # S3-compatible client implementation
│   ├── store/
│   │   └── postgres/         # PostgreSQL repositories & SQL queries
│   └── webhook/              # Async outbound webhook delivery worker & HMAC signer
├── migrations/               # SQL migration files (.up.sql / .down.sql)
├── docs/                     # OpenAPI 3.0 YAML/JSON spec & Swagger assets
├── docker-compose.yml        # Turnkey deployment with PostgreSQL
├── Dockerfile                # Multi-stage production container build
├── .env.example              # Template for environment variables
└── go.mod
```

---

## 2. Implementation Milestones

### Milestone 1: Project Setup & Scaffolding
- Initialize Go module (`go mod init ...`).
- Set up configuration loader supporting environment variables with sensible defaults.
- Implement structured logging and graceful shutdown mechanism.
- Set up embedded database migrations with `golang-migrate`.

### Milestone 2: Database Layer & Migrations
- Write PostgreSQL DDL migrations for:
  - `jobs`
  - `applications`
  - `application_notes`
  - `webhook_subscriptions`
  - `api_keys`
- Implement repository methods for CRUD operations, pagination, and JSONB filtering.

### Milestone 3: S3 Storage & Attachment Handling
- Implement S3 client using `aws-sdk-go-v2` with support for custom endpoints (MinIO, R2, GCS).
- Implement resume file validation (MIME type check, file size limits).
- Implement presigned download URL generation with time-based expiration.

### Milestone 4: Authentication & Middleware
- Implement API key generation and SHA256 hashing utility.
- Build `ApiKeyAuth` middleware supporting dual scopes: `public` (`jb_pub_...`) and `admin` (`jb_sec_...`).
- Add CORS and in-memory IP-based rate-limiting middleware (`golang.org/x/time/rate`) for public endpoints.

### Milestone 5: Public Career API Endpoints
- Implement `GET /v1/public/jobs` with filtering by department, location, employment type.
- Implement `GET /v1/public/jobs/{slug_or_id}` returning job details and custom application field definitions.
- Implement `GET /v1/public/departments`.
- Implement `POST /v1/public/jobs/{job_id}/apply` accepting multipart form data (resume file + custom field JSON answers).

### Milestone 6: Admin ATS & Candidate Management
- Implement Admin CRUD endpoints for jobs (create, update status, archive).
- Implement candidate pipeline endpoints: list applications by job, fetch details, transition stages (`applied` -> `screening` -> `interviewing` -> `offer` -> `hired`/`rejected`).
- Implement review notes endpoints (`POST` and `GET` notes).
- Implement resume download presigned URL endpoint.

### Milestone 7: Outbound Webhook Worker
- Implement webhook subscriber registry.
- Build async background worker to dispatch events (`job.published`, `application.created`, `application.stage_updated`).
- Implement HMAC-SHA256 signature header (`X-JobBoard-Signature`) and delivery retry logic.

### Milestone 8: OpenAPI Spec, Docs & Packaging
- Define OpenAPI 3.0 specification.
- Embed Swagger UI served at `/docs` and raw spec at `/openapi.json`.
- Create multi-stage `Dockerfile` and `docker-compose.yml`.
- Add CLI seed command for bootstrapping initial admin and public API keys.
