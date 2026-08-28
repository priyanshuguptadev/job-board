# Environment Configuration & CLI Tooling

## 1. Environment Variables Specification

The API is fully configurable through environment variables or a `.env` file at the root.

| Variable | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `SERVER_PORT` | int | `8080` | Port for the HTTP server to listen on |
| `SERVER_ENV` | string | `development` | `development`, `staging`, or `production` |
| `SERVER_LOG_LEVEL` | string | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `SERVER_CORS_ALLOWED_ORIGINS` | string | `*` | Comma-separated list of allowed CORS origins |
| `RATE_LIMIT_RPS` | int | `20` | Requests per second limit for public routes (in-memory token bucket per client IP) |
| `RATE_LIMIT_BURST` | int | `50` | Maximum burst allowed for public rate limiting (in-memory token bucket per client IP) |
| `DATABASE_URL` | string | - | PostgreSQL connection URI (e.g., `postgres://user:pass@localhost:5432/jobboard?sslmode=disable`) |
| `DATABASE_MAX_OPEN_CONNS` | int | `25` | Max open connections in pool |
| `DATABASE_MAX_IDLE_CONNS` | int | `10` | Max idle connections in pool |
| `S3_ENDPOINT` | string | `""` | Custom S3 endpoint URL (Required for MinIO, Cloudflare R2, Wasabi; empty for AWS S3) |
| `S3_REGION` | string | `us-east-1` | S3 region |
| `S3_BUCKET` | string | - | S3 bucket name for storing resumes |
| `S3_ACCESS_KEY_ID` | string | - | S3 access key ID |
| `S3_SECRET_ACCESS_KEY` | string | - | S3 secret access key |
| `S3_FORCE_PATH_STYLE` | bool | `false` | Set to `true` for MinIO or local S3 emulators |
| `S3_PRESIGN_EXPIRY_MINUTES` | int | `15` | Expiration time for generated resume download URLs |

---

## 2. Object Storage Provider Matrix

| Provider | `S3_ENDPOINT` | `S3_FORCE_PATH_STYLE` | `S3_REGION` |
| :--- | :--- | :--- | :--- |
| **AWS S3** | *(leave empty)* | `false` | `us-east-1`, `eu-west-1`, etc. |
| **MinIO (Local/Self-hosted)** | `http://minio:9000` | `true` | `us-east-1` |
| **Cloudflare R2** | `https://<account_id>.r2.cloudflarestorage.com` | `false` | `auto` |
| **Google Cloud Storage** | `https://storage.googleapis.com` | `false` | `auto` |

---

## 3. CLI Subcommands & Tooling

The compiled Go binary provides built-in CLI commands for operational workflows:

### 3.1 Start Server
```bash
./jobboard server
```
*Starts the HTTP API server on `$SERVER_PORT` and runs pending database migrations.*

### 3.2 Key Generation (Bootstrapping Admin Key)
```bash
# Generate a new Admin API key
./jobboard keygen --name "Initial Admin Key" --scope admin

# Generate a Public key for frontend integration
./jobboard keygen --name "Careers Website Frontend" --scope public
```
*Outputs the raw unhashed key token once (e.g. `jb_sec_xxxxxxxxxxxx`) and stores the SHA256 hash in the database.*

### 3.3 Database Migrations
```bash
./jobboard migrate up       # Run all pending migrations
./jobboard migrate down 1   # Roll back last migration
./jobboard migrate status   # Print current migration version
```
