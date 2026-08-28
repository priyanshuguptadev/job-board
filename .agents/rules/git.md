# Git & GitHub Workflow Standards

This document defines unambiguous guidelines and automated-friendly conventions for Git version control and GitHub pull request / push workflows.

---

## 1. Branching Strategy

| Branch Type | Naming Pattern | Example | Purpose |
| :--- | :--- | :--- | :--- |
| **Main** | `main` | `main` | Production-ready, stable codebase. |
| **Features** | `feat/<short-description>` | `feat/job-crud-api` | New capabilities, endpoints, or features. |
| **Bug Fixes** | `fix/<short-description>` | `fix/webhook-hmac-verify` | Resolving bugs or defects. |
| **Refactoring** | `refactor/<short-description>` | `refactor/s3-storage-client` | Code restructuring without feature changes. |
| **Documentation** | `docs/<short-description>` | `docs/openapi-spec` | Documentation, specs, and README changes. |
| **Chores/CI** | `chore/<short-description>` | `chore/docker-compose-update`| Dependency updates, CI/CD, build tooling. |

### Branch Rules:
1. Always branch off the latest `main` branch:
   ```bash
   git checkout main
   git pull origin main
   git checkout -b feat/your-feature-name
   ```
2. Keep branches short-lived and scoped to a single logical milestone or feature.

---

## 2. Commit Message Standards (Conventional Commits)

All commit messages must adhere to the **Conventional Commits v1.0.0** specification.

### Format:
```text
<type>(<scope>): <subject in imperative mood>

[optional body explaining motivation and context]

[optional footer(s) such as Closes #123 or BREAKING CHANGE: description]
```

### Commit Types:
- `feat`: A new feature or endpoint.
- `fix`: A bug fix.
- `docs`: Documentation changes only.
- `refactor`: Code change that neither fixes a bug nor adds a feature.
- `perf`: Performance improvement.
- `test`: Adding or correcting tests.
- `chore`: Tooling, build config, dependency upgrades.
- `ci`: CI/CD workflow changes.

### Examples:
- `feat(api): add public job listing endpoint with pagination`
- `fix(webhook): correct HMAC signature timestamp formatting`
- `test(store): add postgres job repository integration tests`
- `refactor(storage): extract s3 client interface for mock testing`

### Atomic Commits Rule:
- Each commit must represent a single, atomic logical change.
- Never mix whitespace/formatting fixes with core business logic changes.

---

## 3. Pre-Push Quality & Safety Checklist

Before committing and pushing changes, the following checks **MUST** pass:

1. **Code Formatting & Linting**:
   ```bash
   # Format Go code
   gofmt -s -w .
   # Run Go static analysis
   go vet ./...
   ```
2. **Test Suite Verification**:
   ```bash
   go test -v -race ./...
   ```
3. **Clean Workspace & Secrets Check**:
   - Check `git status` to ensure no sensitive files (`.env`, private keys, local binaries, test dumps) are untracked or staged.
   - Verify `.gitignore` contains all temporary/environment files.

---

## 4. Push & Synchronization Workflow

1. **Rebase against `main` before pushing**:
   ```bash
   git fetch origin
   git rebase origin/main
   ```
2. **Push to Remote**:
   ```bash
   git push -u origin <branch-name>
   ```

---

## 5. Pull Request (PR) Standard

When opening a Pull Request on GitHub:

### PR Title:
Matches conventional commit syntax, e.g. `feat(api): implement application submission endpoint`.

### PR Description Template:
```markdown
## Summary
Brief explanation of the changes introduced in this PR.

## Changes Made
- Added handler for `POST /v1/public/jobs/{job_id}/apply`
- Integrated S3 resume upload validation
- Added unit tests for multipart form parsing

## Verification & Testing
- [x] Unit tests pass: `go test ./...`
- [x] Tested locally against PostgreSQL and MinIO
- [x] Verified error envelope matches standard format

## Related Issues / Milestones
- Resolves Milestone 5 in `.agents/rules/implementation.md`
```

---

## 6. Non-Negotiable Safety Guardrails for Agents

1. **No Force Pushes to `main`**: Never execute `git push --force` or `git push -f` against `main`.
2. **No Secret Leaks**: Never commit API keys, AWS credentials, database passwords, or unhashed secrets.
3. **No Blind Staging**: Avoid `git add .` or `git add -A` when unreviewed temporary files exist; stage files explicitly or verify with `git diff --staged`.
4. **Preserve Untracked Work**: Never run destructive commands like `git reset --hard` or `git clean -fd` without checking if local work will be lost.
