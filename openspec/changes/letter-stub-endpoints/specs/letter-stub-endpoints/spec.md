## Requirements

### Requirement: GET /service/{id}/letter-contact returns empty list
`GET /service/{id}/letter-contact` SHALL return HTTP 200 with `{"data": []}` without touching the database.

#### Scenario: Letter contact list returns empty list
- **WHEN** `GET /service/{id}/letter-contact` is called with admin JWT
- **THEN** returns HTTP 200 with body `{"data": []}`

#### Scenario: Unauthenticated request returns 401
- **WHEN** `GET /service/{id}/letter-contact` is called without admin JWT
- **THEN** returns HTTP 401

---

### Requirement: POST /service/{id}/letter-contact returns stub object
`POST /service/{id}/letter-contact` SHALL return HTTP 200 with a minimal stub letter contact object regardless of request body.

#### Scenario: POST returns stub letter contact object
- **WHEN** `POST /service/{id}/letter-contact` is called with admin JWT
- **THEN** returns HTTP 200 with body `{"data": {"id": "<some_id>", "service_id": "<id>", "contact_block": ""}}`

#### Scenario: Unauthenticated POST returns 401
- **WHEN** `POST /service/{id}/letter-contact` is called without admin JWT
- **THEN** returns HTTP 401

---

### Requirement: GET /service/{id}/letter-contact/{contact_id} returns stub object
`GET /service/{id}/letter-contact/{contact_id}` SHALL return HTTP 200 with a minimal stub letter contact object.

#### Scenario: GET single letter contact returns stub
- **WHEN** `GET /service/{id}/letter-contact/{contact_id}` is called with admin JWT
- **THEN** returns HTTP 200 with a stub letter contact object

#### Scenario: Unauthenticated GET returns 401
- **WHEN** `GET /service/{id}/letter-contact/{contact_id}` is called without admin JWT
- **THEN** returns HTTP 401

---

### Requirement: PUT /service/{id}/letter-contact/{contact_id} returns stub object
`PUT /service/{id}/letter-contact/{contact_id}` SHALL return HTTP 200 with a minimal stub letter contact object regardless of request body.

#### Scenario: PUT returns stub letter contact object
- **WHEN** `PUT /service/{id}/letter-contact/{contact_id}` is called with admin JWT
- **THEN** returns HTTP 200 with a stub letter contact object

#### Scenario: Unauthenticated PUT returns 401
- **WHEN** `PUT /service/{id}/letter-contact/{contact_id}` is called without admin JWT
- **THEN** returns HTTP 401

---

### Requirement: POST /service/{id}/letter-contact/{contact_id}/archive returns stub
`POST /service/{id}/letter-contact/{contact_id}/archive` SHALL return HTTP 200 with a stub letter contact object.

#### Scenario: Archive returns stub
- **WHEN** `POST /service/{id}/letter-contact/{contact_id}/archive` is called with admin JWT
- **THEN** returns HTTP 200 with a stub letter contact object

#### Scenario: Unauthenticated archive returns 401
- **WHEN** `POST /service/{id}/letter-contact/{contact_id}/archive` is called without admin JWT
- **THEN** returns HTTP 401

---

### Requirement: POST /service/{id}/send-pdf-letter returns 400 feature-disabled message
`POST /service/{id}/send-pdf-letter` SHALL return HTTP 400 with the exact error message `"Letters as PDF feature is not enabled"`.

#### Scenario: PDF letter endpoint returns 400 with correct message
- **WHEN** `POST /service/{id}/send-pdf-letter` is called with admin JWT
- **THEN** returns HTTP 400 with body `{"result": "error", "message": "Letters as PDF feature is not enabled"}`

#### Scenario: Message text matches Python verbatim
- **WHEN** any client parses the 400 response body
- **THEN** `message` equals exactly `"Letters as PDF feature is not enabled"` (no trailing period, exact capitalisation)

#### Scenario: Unauthenticated PDF letter returns 401
- **WHEN** `POST /service/{id}/send-pdf-letter` is called without admin JWT
- **THEN** returns HTTP 401

---

### Requirement: GET /letters/returned returns empty list
`GET /letters/returned` SHALL return HTTP 200 with `{"data": []}` without touching the database.

#### Scenario: Returned letters list is empty
- **WHEN** `GET /letters/returned` is called with admin JWT
- **THEN** returns HTTP 200 with body `{"data": []}`

#### Scenario: Unauthenticated returned letters returns 401
- **WHEN** `GET /letters/returned` is called without admin JWT
- **THEN** returns HTTP 401

---

### Requirement: All stub handlers are annotated and database-free
Every stub handler SHALL include the comment `// STUB: letter implementation not in scope` and SHALL NOT execute any database queries or external API calls.

#### Scenario: STUB comment present in handler code
- **WHEN** any of the 7 stub handlers is reviewed in code
- **THEN** the source contains the comment `// STUB: letter implementation not in scope`

#### Scenario: No database call made in any stub
- **WHEN** any of the 7 stub handlers processes a request
- **THEN** no SQL query is executed and no external HTTP call is made
