# Database Schema & Domain Models

## 1. Schema Overview

The database is PostgreSQL 15+ utilizing native UUIDs, Enums, Timestamps with timezone, Foreign Key constraints with cascades, and `JSONB` for flexible schema attributes (such as dynamic form fields and candidate answers).

---

## 2. Table Definitions

### 2.1 `api_keys`
Stores hashed API keys for authentication.

```sql
CREATE TYPE api_key_scope AS ENUM ('public', 'admin');

CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    key_hash VARCHAR(64) NOT NULL UNIQUE, -- SHA256 hex digest
    key_prefix VARCHAR(16) NOT NULL,      -- e.g. 'jb_pub_' or 'jb_sec_'
    scope api_key_scope NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ
);

CREATE INDEX idx_api_keys_hash ON api_keys (key_hash);
```

---

### 2.2 `jobs`
Stores job postings.

```sql
CREATE TYPE job_status AS ENUM ('draft', 'published', 'archived');
CREATE TYPE employment_type AS ENUM ('full_time', 'part_time', 'contract', 'internship');

CREATE TABLE jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug VARCHAR(255) NOT NULL UNIQUE,
    title VARCHAR(255) NOT NULL,
    department VARCHAR(100) NOT NULL,
    location VARCHAR(150) NOT NULL,
    employment_type employment_type NOT NULL DEFAULT 'full_time',
    experience_level VARCHAR(50),
    salary_min NUMERIC(12, 2),
    salary_max NUMERIC(12, 2),
    salary_currency VARCHAR(3) DEFAULT 'USD',
    description_markdown TEXT NOT NULL,
    status job_status NOT NULL DEFAULT 'draft',
    custom_fields JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ
);

CREATE INDEX idx_jobs_status ON jobs (status);
CREATE INDEX idx_jobs_department ON jobs (department);
CREATE INDEX idx_jobs_slug ON jobs (slug);
```

#### Custom Fields JSONB Schema:
```json
[
  {
    "id": "github_url",
    "label": "GitHub Profile URL",
    "type": "url",
    "required": false
  },
  {
    "id": "years_experience",
    "label": "Years of Go experience",
    "type": "number",
    "required": true
  },
  {
    "id": "sponsorship",
    "label": "Do you require visa sponsorship?",
    "type": "select",
    "options": ["Yes", "No"],
    "required": true
  }
]
```

---

### 2.3 `applications`
Stores candidate job applications.

```sql
CREATE TYPE application_stage AS ENUM ('applied', 'screening', 'interviewing', 'offer', 'hired', 'rejected');

CREATE TABLE applications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    candidate_name VARCHAR(255) NOT NULL,
    candidate_email VARCHAR(255) NOT NULL,
    candidate_phone VARCHAR(50),
    linkedin_url VARCHAR(500),
    resume_s3_key VARCHAR(1024) NOT NULL,
    resume_filename VARCHAR(255) NOT NULL,
    custom_answers JSONB NOT NULL DEFAULT '{}'::jsonb,
    stage application_stage NOT NULL DEFAULT 'applied',
    rejected_reason VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_applications_job_id ON applications (job_id);
CREATE INDEX idx_applications_stage ON applications (stage);
CREATE INDEX idx_applications_email ON applications (candidate_email);
```

---

### 2.4 `application_notes`
Internal review notes and feedback left by recruiters and hiring managers.

```sql
CREATE TABLE application_notes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    author_name VARCHAR(255) NOT NULL,
    note_text TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_application_notes_app_id ON application_notes (application_id);
```

---

### 2.5 `webhook_subscriptions`
Stores outbound webhook configurations for event dispatching.

```sql
CREATE TABLE webhook_subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    target_url VARCHAR(1024) NOT NULL,
    secret_token VARCHAR(255) NOT NULL,
    events TEXT[] NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_webhooks_active ON webhook_subscriptions (is_active);
```
