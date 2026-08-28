# Error Handling & Response Standards

## 1. Unified Error Envelope

All error responses return a standardized JSON structure across all API endpoints:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "The provided input failed validation checks.",
    "details": [
      {
        "field": "resume",
        "issue": "File exceeds the maximum allowed size of 10MB"
      }
    ]
  }
}
```

---

## 2. Standard Error Codes Catalog

| Error Code | HTTP Status | Description |
| :--- | :--- | :--- |
| `INVALID_API_KEY` | `401 Unauthorized` | Missing, malformed, or invalid API key provided. |
| `FORBIDDEN` | `403 Forbidden` | API key lacks required scope (e.g. public key trying to access admin route). |
| `NOT_FOUND` | `404 Not Found` | Target resource (job, application, webhook) does not exist. |
| `VALIDATION_ERROR` | `422 Unprocessable Entity` | Request payload failed schema, field, or constraint validation. |
| `UNSUPPORTED_MEDIA_TYPE` | `415 Unsupported Media Type` | Resume upload is not an allowed MIME type (PDF, DOC, DOCX). |
| `RATE_LIMIT_EXCEEDED` | `429 Too Many Requests` | Rate limit burst/RPS exceeded on public endpoint. |
| `CONFLICT` | `409 Conflict` | Slug collision or unique constraint violation. |
| `INTERNAL_SERVER_ERROR` | `500 Internal Server Error` | Unexpected backend or database error. |

---

## 3. Success Response Convention

- Single entity queries return the entity object directly:
  `GET /v1/public/jobs/eng-lead` -> `{ "id": "...", "title": "...", ... }`
- Collection queries return items with pagination metadata:
  ```json
  {
    "data": [ ... ],
    "pagination": {
      "page": 1,
      "limit": 20,
      "total_items": 45,
      "total_pages": 3
    }
  }
  ```
- Creation requests return `201 Created` with the created resource and `Location` header.
- Deletion requests return `204 No Content`.
