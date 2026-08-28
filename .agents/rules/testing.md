# Testing & Quality Assurance Strategy

## 1. Testing Pyramid

| Layer | Focus Area | Tools / Libraries |
| :--- | :--- | :--- |
| **Unit Tests** | Domain validation, custom field schema validator, API key hashing, HMAC signature generator | Standard `testing` package, `github.com/stretchr/testify` |
| **Store & Repo Tests** | PostgreSQL queries, JSONB indexing, constraint verification | Real PostgreSQL via `testcontainers-go` or local docker test DB |
| **HTTP API Integration** | Public & admin endpoint contracts, auth middleware, multipart upload handling | `net/http/httptest`, mock S3 storage |
| **End-to-End Flow** | Job creation -> public list -> candidate application -> stage transition -> webhook dispatch | Go test suite running against test environment |

---

## 2. Testing Conventions

- Place unit tests adjacent to implementation files (e.g. `auth_test.go` beside `auth.go`).
- Use table-driven test patterns standard in Go.
- Storage interfaces must allow injecting an in-memory or mock S3 client for fast, deterministic unit and API tests.
