## Source Files
- spec/behavioral-spec/misc-gaps.md (C2 gap routes)
- spec/api-surface.md

---

## Context
Validation finding C2 identified 4 routes present in the Python production codebase but absent from `api-surface.md`. All 4 require admin JWT.

---

## Route 1: POST /newsletter/update-language/{subscriber_id}

### Request
- Path param: `subscriber_id` (Airtable record ID)
- Body: `{"language": "en" | "fr"}`
- Auth: admin JWT (`requires_admin_auth`)

### Behavior
- Validate `language` is in `["en", "fr"]`; invalid value → 400
- Call `AirtableClient.UpdateSubscriberLanguage(subscriber_id, language)`
- Return 204 No Content on success

### Error conditions
| Condition | Response |
|-----------|----------|
| Missing/invalid `language` value | 400 |
| Subscriber not found in Airtable | 404 |
| No admin JWT | 401 |

---

## Route 2: GET /newsletter/send-latest/{subscriber_id}

### Request
- Path param: `subscriber_id`
- Auth: admin JWT

### Behavior
- Look up subscriber in Airtable by `subscriber_id`
- Retrieve latest newsletter template via `LatestNewsletterTemplate.get_latest_newsletter_templates()` (sort `-Created at`, max_records=1, returns EN + FR template IDs)
- Trigger newsletter send to subscriber's email address using the template for their language preference
- Return 200

### Error conditions
| Condition | Response |
|-----------|----------|
| Subscriber not found | 404 |
| No newsletter templates configured | 404 |
| No admin JWT | 401 |

---

## Route 3: GET /newsletter/find-subscriber

### Request
- Query param: `?email=<email_address>` (required)
- Auth: admin JWT

### Behavior
- Calls `NewsletterSubscriber.from_email(email)` formula-based lookup (`{Email} = 'email'`)
- Returns subscriber record on success → 200
- Subscriber not found (Airtable `HTTPError(404)`) → 404

### Response shape (200)
```json
{
  "id": "<airtable_record_id>",
  "email": "<email>",
  "language": "en" | "fr",
  "status": "unconfirmed" | "subscribed" | "unsubscribed",
  "created_at": "<datetime>",
  "confirmed_at": "<datetime | null>",
  "unsubscribed_at": "<datetime | null>",
  "has_resubscribed": true | false
}
```

### Error conditions
| Condition | Response |
|-----------|----------|
| Missing `email` query param | 400 |
| Subscriber not found | 404 |
| No admin JWT | 401 |

---

## Route 4: GET /platform-stats/send-method-stats-by-service

### Request
- No path/query params
- Auth: admin JWT

### Behavior
- Queries `ft_billing` table grouped by `service_id` and `key_type`
- `key_type` values: `"normal"` (API send), `"team"` (admin/team send), `"test"` (test key)
- Returns per-service breakdown as JSON list

### Response shape (200)
```json
[
  {
    "service_id": "<uuid>",
    "api_count": <int>,
    "bulk_count": <int>,
    "team_count": <int>
  }
]
```

### Error conditions
| Condition | Response |
|-----------|----------|
| No admin JWT | 401 |

---

## Business Rules
- All 4 routes require `requires_admin_auth`; unauthenticated requests → 401
- `update-language` validates language strictly against `["en", "fr"]` before any Airtable call
- `find-subscriber` does not support listing all subscribers; `email` param is mandatory
- `send-latest` uses the Airtable `LatestNewsletterTemplate` (not a request body template ID)
- `platform-stats` reads from `ft_billing` only — no Airtable dependency
- Depends on: `external-client-integrations` (Airtable client), `authentication-middleware` (admin JWT)
