# System Architecture & Tech Stack

## 1. System Overview

A high-performance, single-tenant, headless REST API built with **Go** and backed by **PostgreSQL** and **S3-compatible object storage**. Designed for zero-friction self-hosting via Docker Compose, allowing any organization to deploy their own backend and connect their custom career pages, internal dashboards, and workflow automations.

```mermaid
flowchart TD
    subgraph ClientLayer ["Client Layer"]
        PublicWeb["Public Careers Page<br/>(Uses jb_pub_... key)"]
        AdminDashboard["Internal Admin / ATS / Scripts<br/>(Uses jb_sec_... key)"]
    end

    subgraph ServerLayer ["Job Board API Server (Go)"]
        AuthMW["API Key Auth Middleware<br/>(Public vs Secret Scope)"]
        Router["HTTP Router (Chi/Gin)"]
        
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

## 2. Core Decisions & Tech Stack

| Component | Technology | Rationale |
| :--- | :--- | :--- |
| **Language & Runtime** | Go 1.22+ | Single static binary, low memory footprint (~20MB), high throughput, easy containerization |
| **HTTP Router / Web Framework** | `chi` (or `gin`) | Lightweight, idiomatic `net/http` compatibility, composable middleware |
| **Database** | PostgreSQL 15+ | ACID compliance, `JSONB` for dynamic custom application form fields, indexing |
| **Migrations** | `golang-migrate` (embedded) | Auto-migrates database on startup or via CLI subcommands |
| **Object Storage** | AWS S3 SDK (`aws-sdk-go-v2`) | Compatible with AWS S3, Cloudflare R2, MinIO, Google Cloud Storage, Wasabi |
| **API Documentation** | OpenAPI 3.0 / Swagger UI | Embedded UI at `/docs` and raw spec at `/openapi.json` for client/SDK generation |
| **Rate Limiting** | In-Memory Token Bucket (`golang.org/x/time/rate`) | Zero external dependencies (no Redis required); tracks burst/RPS per client IP in process memory |
| **Deployment** | Docker & Docker Compose | Pre-configured `docker-compose.yml` with PostgreSQL and the API server |
