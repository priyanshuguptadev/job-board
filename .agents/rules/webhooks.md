# Outbound Webhook Specification & Security

## 1. Webhook Payload Format

When an event occurs, the API sends an HTTP `POST` request with a JSON payload to all active webhook subscribers subscribed to that event topic:

```json
{
  "id": "evt_01J8N8P0Z04R2G8E9B6R7W8K1L",
  "event": "application.created",
  "created_at": "2026-08-28T11:45:00Z",
  "data": {
    "application_id": "8f03c00c-7729-411a-85d0-9964a7818e9d",
    "job_id": "3b2e5a61-f3b1-4f9e-a89a-0e9e9842c3d1",
    "job_title": "Staff Backend Engineer",
    "job_slug": "staff-backend-engineer",
    "candidate_name": "Jane Doe",
    "candidate_email": "jane@example.com",
    "stage": "applied",
    "custom_answers": {
      "github_url": "https://github.com/janedoe"
    }
  }
}
```

---

## 2. Event Catalog

| Event Name | Trigger Condition | Data Payload |
| :--- | :--- | :--- |
| `job.published` | Job status transitioned to `published` | Job entity |
| `job.archived` | Job status transitioned to `archived` | Job entity |
| `application.created` | New candidate submits application | Application entity & Job metadata |
| `application.stage_updated` | Candidate stage changes (e.g. `applied` -> `interviewing`) | Application ID, old stage, new stage, notes count |
| `application.rejected` | Candidate is marked as rejected | Application ID, rejection reason |
| `webhook.ping` | Test ping triggered via admin API | Test timestamp and message |

---

## 3. Cryptographic Signature & Verification

To ensure webhooks cannot be forged or replayed by malicious third parties, every outbound request includes two verification headers:

```http
POST /webhook-endpoint HTTP/1.1
Host: your-company.com
Content-Type: application/json
X-JobBoard-Signature: sha256=d583b23db17f8b9e6919be6a894676579c882d2c18e19c3b885cf8a9...
X-JobBoard-Timestamp: 1787910300
```

### Signature Generation:
1. Construct the signed string: `t=${X-JobBoard-Timestamp}.${RawRequestBodyJSON}`
2. Compute the HMAC-SHA256 of the string using the webhook's `secret_token`.
3. Prepend `sha256=` to the hex-encoded digest.

### Subscriber Verification Pseudocode:
```python
import hmac, hashlib, time

def verify_webhook(raw_body, timestamp, signature, secret_token):
    # Prevent replay attacks older than 5 minutes
    if abs(time.time() - int(timestamp)) > 300:
        return False
    
    signed_payload = f"t={timestamp}.{raw_body}".encode('utf-8')
    expected_sig = "sha256=" + hmac.new(secret_token.encode('utf-8'), signed_payload, hashlib.sha256).hexdigest()
    return hmac.compare_digest(expected_sig, signature)
```

---

## 4. Delivery & Retry Policy

- **Timeout**: 10 seconds per HTTP POST request.
- **Success Criteria**: HTTP status code `2xx`.
- **Retries**: If the subscriber returns `4xx` (except `410 Gone`), `5xx`, or times out:
  - Retry 1: +30 seconds
  - Retry 2: +5 minutes
  - Retry 3: +30 minutes
- **Deactivation**: If an endpoint fails continuously for 50 attempts or returns `410 Gone`, it is automatically marked `is_active = false`.
