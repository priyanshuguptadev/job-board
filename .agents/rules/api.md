# API Specification & Authentication

## 1. Authentication & Scopes

All API endpoints are protected using Scoped API Keys passed via the `X-API-Key` header or `Authorization: Bearer <key>`.

| Key Scope | Prefix | Purpose | Allowed Endpoints |
| :--- | :--- | :--- | :--- |
| **Public** | `jb_pub_` | Used by public career frontends / static sites | `GET /v1/public/*`, `POST /v1/public/jobs/{job_id}/apply` |
| **Admin** | `jb_sec_` | Full administrative control (recruiting tools, ATS, internal scripts) | `ALL /v1/admin/*`, `ALL /v1/public/*` |

---

## 2. API Endpoints

### 2.1 Public Career Endpoints (`jb_pub_...`)

#### `GET /v1/public/jobs`
List all published jobs.
- **Query Params**:
  - `department` (string, optional)
  - `location` (string, optional)
  - `employment_type` (string, optional: `full_time`, `part_time`, `contract`, `internship`)
  - `page` (int, default: 1)
  - `limit` (int, default: 20, max: 100)
- **Response**: Array of published job summary objects with pagination metadata.

#### `GET /v1/public/jobs/{slug_or_id}`
Retrieve complete details of a published job listing including its custom dynamic application schema.
- **Response**: Single job object including `custom_fields` array.

#### `GET /v1/public/departments`
List distinct departments with active published job openings.
- **Response**: `{"departments": ["Engineering", "Product", "Design", ...]}`

#### `POST /v1/public/jobs/{job_id}/apply`
Submit a candidate application for a specific job.
- **Content-Type**: `multipart/form-data`
- **Form Fields**:
  - `candidate_name` (string, required)
  - `candidate_email` (string, required)
  - `candidate_phone` (string, optional)
  - `linkedin_url` (string, optional)
  - `resume` (file, required, PDF/DOCX/DOC, max 10MB)
  - `custom_answers` (JSON string, optional, matches job's `custom_fields`)
- **Response**: `201 Created` with application ID and confirmation message.

---

### 2.2 Admin Endpoints (`jb_sec_...`)

#### Job Management
- `POST /v1/admin/jobs`: Create a new job listing (status can be `draft` or `published`).
- `GET /v1/admin/jobs`: List all jobs with filtering by status (`draft`, `published`, `archived`), department, and pagination.
- `GET /v1/admin/jobs/{id}`: Get full job details.
- `PATCH /v1/admin/jobs/{id}`: Update job details, salary range, custom question fields, or status.
- `DELETE /v1/admin/jobs/{id}`: Archive or permanently delete a job.

#### Candidate & Pipeline Management
- `GET /v1/admin/jobs/{id}/applications`: List all applications for a job, filterable by hiring stage.
- `GET /v1/admin/applications/{id}`: Get application details, custom answers, and timeline.
- `GET /v1/admin/applications/{id}/resume`: Returns a short-lived presigned S3 download URL for the resume.
- `PATCH /v1/admin/applications/{id}/stage`: Update candidate hiring stage:
  - Valid stages: `applied`, `screening`, `interviewing`, `offer`, `hired`, `rejected`.
  - Body: `{"stage": "interviewing", "rejected_reason": ""}`
- `POST /v1/admin/applications/{id}/notes`: Add an internal review note.
- `GET /v1/admin/applications/{id}/notes`: Retrieve all review notes for an application.

#### Webhooks Management
- `POST /v1/admin/webhooks`: Register a webhook URL and select events.
- `GET /v1/admin/webhooks`: List all webhook subscriptions.
- `DELETE /v1/admin/webhooks/{id}`: Delete a webhook subscription.
- `POST /v1/admin/webhooks/{id}/test`: Trigger a ping/test payload to the webhook endpoint.

---

### 2.3 System & Observability Endpoints
- `GET /healthz`: Basic liveness/readiness probe (verifies database connectivity).
- `GET /docs`: Embedded Swagger UI documentation.
- `GET /openapi.json`: OpenAPI 3.0 specification in JSON format.
