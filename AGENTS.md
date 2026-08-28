# Job Board API — Agent Guidelines & Instructions

## 1. Project Mission & Invariants
An open-source, single-tenant, headless Job Board REST API written in **Go (1.22+)**, backed by **PostgreSQL (15+)** and **S3-compatible object storage**.

### Non-Negotiable Invariants:
1. **Single-Tenant Model**: Do not add multi-tenant organization identifiers or cross-tenant routing logic.
2. **Scoped Authentication**:
   - `jb_pub_...` (Public): Read published jobs & departments, submit candidate applications.
   - `jb_sec_...` (Admin): Full CRUD on jobs, candidates, pipeline stages, notes, and webhooks.
3. **Standard Error Envelope**: Always return errors using the standard `{ "error": { "code": "...", "message": "...", "details": [...] } }` schema.
4. **Resumes in S3 Only**: Binary files are never stored in PostgreSQL or served directly through the API; use S3 object storage with presigned URLs.

---

## 2. Rule Map & Deep References
Detailed architectural, schema, and API specifications are decomposed into `.agents/rules/`. Read these files when working on their respective domains:

- **Architecture & Topology**: [`.agents/rules/architecture.md`](file:///.agents/rules/architecture.md)
- **Database DDL & Models**: [`.agents/rules/dbschema.md`](file:///.agents/rules/dbschema.md)
- **API Spec & Routes**: [`.agents/rules/api.md`](file:///.agents/rules/api.md)
- **Environment & CLI**: [`.agents/rules/config.md`](file:///.agents/rules/config.md)
- **Error Standards**: [`.agents/rules/errors.md`](file:///.agents/rules/errors.md)
- **Webhook Delivery & HMAC**: [`.agents/rules/webhooks.md`](file:///.agents/rules/webhooks.md)
- **Implementation Milestones**: [`.agents/rules/implementation.md`](file:///.agents/rules/implementation.md)
- **Testing & QA**: [`.agents/rules/testing.md`](file:///.agents/rules/testing.md)
- **Git & GitHub Standards**: [`.agents/rules/git.md`](file:///.agents/rules/git.md)

---

## 3. Code Conventions & Standards
- **Idiomatic Go**: Use standard library `net/http` idioms with `chi` router. Avoid heavy ORMs; use standard `database/sql` or `pgx` with clean repository interfaces.
- **Dynamic Fields**: Store job custom questions and application answers as PostgreSQL `JSONB`.
- **Testing**: Write table-driven unit tests alongside implementations (`*_test.go`).
- **Security**: Never log raw API keys or store unhashed keys in the database. Compute HMAC-SHA256 for all outbound webhook dispatches.

---

## 4. Common Commands
```bash
# Start server (with auto-migrations)
go run ./cmd/server server

# Generate API keys
go run ./cmd/server keygen --name "Admin" --scope admin

# Run test suite
go test -v ./...
```
