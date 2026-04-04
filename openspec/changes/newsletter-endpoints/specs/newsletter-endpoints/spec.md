## Requirements

### Requirement: POST /newsletter/update-language/{subscriber_id}
`POST /newsletter/update-language/{subscriber_id}` SHALL accept a `language` field, validate it against the `["en", "fr"]` enum, update the Airtable subscriber record, and return HTTP 204.

#### Scenario: Language updated to French
- **WHEN** `POST /newsletter/update-language/{id}` is called with `{"language": "fr"}` and admin JWT
- **THEN** calls `AirtableClient.UpdateSubscriberLanguage(subscriber_id, "fr")` and returns HTTP 204

#### Scenario: Language updated to English
- **WHEN** `POST /newsletter/update-language/{id}` is called with `{"language": "en"}` and admin JWT
- **THEN** calls `AirtableClient.UpdateSubscriberLanguage(subscriber_id, "en")` and returns HTTP 204

#### Scenario: Invalid language value returns 400
- **WHEN** `POST /newsletter/update-language/{id}` is called with `{"language": "es"}` (not in enum)
- **THEN** returns HTTP 400 with a validation error before any Airtable call is made

#### Scenario: Missing language field returns 400
- **WHEN** `POST /newsletter/update-language/{id}` is called with an empty or missing body
- **THEN** returns HTTP 400

#### Scenario: Subscriber not found returns 404
- **WHEN** `POST /newsletter/update-language/{id}` is called and the Airtable client returns a 404 error
- **THEN** returns HTTP 404

#### Scenario: Unauthenticated request returns 401
- **WHEN** `POST /newsletter/update-language/{id}` is called without an admin JWT
- **THEN** returns HTTP 401

---

### Requirement: GET /newsletter/send-latest/{subscriber_id}
`GET /newsletter/send-latest/{subscriber_id}` SHALL retrieve the subscriber from Airtable, fetch the latest newsletter template, trigger the newsletter send, and return HTTP 200.

#### Scenario: Latest newsletter dispatched to subscriber
- **WHEN** `GET /newsletter/send-latest/{id}` is called with admin JWT and the subscriber and template exist
- **THEN** looks up the subscriber, retrieves the most recent template from `LatestNewsletterTemplate`, triggers the newsletter send, and returns HTTP 200

#### Scenario: Subscriber not found returns 404
- **WHEN** `GET /newsletter/send-latest/{id}` is called and the subscriber does not exist in Airtable
- **THEN** returns HTTP 404 with an error message indicating the subscriber was not found

#### Scenario: No newsletter templates returns 404
- **WHEN** `GET /newsletter/send-latest/{id}` is called and `LatestNewsletterTemplate.get_latest_newsletter_templates()` raises HTTPError 404
- **THEN** returns HTTP 404 with an error message indicating no templates are configured

#### Scenario: Unauthenticated request returns 401
- **WHEN** `GET /newsletter/send-latest/{id}` is called without an admin JWT
- **THEN** returns HTTP 401

---

### Requirement: GET /newsletter/find-subscriber
`GET /newsletter/find-subscriber` SHALL accept a required `email` query parameter, look up the subscriber in Airtable, and return the subscriber record.

#### Scenario: Subscriber found by email address
- **WHEN** `GET /newsletter/find-subscriber?email=user@gc.ca` is called with admin JWT
- **THEN** returns HTTP 200 with the subscriber record including `id`, `email`, `language`, `status`, `created_at`, `confirmed_at`, `unsubscribed_at`, `has_resubscribed`

#### Scenario: status values reflected correctly
- **WHEN** the Airtable record has `status = "subscribed"` and `language = "fr"`
- **THEN** the response body contains `"status": "subscribed"` and `"language": "fr"`

#### Scenario: Subscriber not found returns 404
- **WHEN** `GET /newsletter/find-subscriber?email=unknown@gc.ca` is called and Airtable returns no records
- **THEN** returns HTTP 404

#### Scenario: Missing email query param returns 400
- **WHEN** `GET /newsletter/find-subscriber` is called without the `email` query parameter
- **THEN** returns HTTP 400

#### Scenario: Unauthenticated request returns 401
- **WHEN** `GET /newsletter/find-subscriber?email=user@gc.ca` is called without an admin JWT
- **THEN** returns HTTP 401

---

### Requirement: GET /platform-stats/send-method-stats-by-service
`GET /platform-stats/send-method-stats-by-service` SHALL query `ft_billing` grouped by `service_id` and `key_type`, and return per-service send-method counts.

#### Scenario: Stats returned for all services
- **WHEN** `GET /platform-stats/send-method-stats-by-service` is called with admin JWT
- **THEN** returns HTTP 200 with a JSON array where each entry contains `service_id`, `api_count`, `bulk_count`, and `team_count`

#### Scenario: Empty result when ft_billing has no data
- **WHEN** `GET /platform-stats/send-method-stats-by-service` is called and `ft_billing` is empty
- **THEN** returns HTTP 200 with an empty JSON array `[]`

#### Scenario: key_type normal maps to api_count
- **WHEN** a service has 5 rows in `ft_billing` with `key_type = "normal"`
- **THEN** the response entry for that service has `api_count = 5`

#### Scenario: Unauthenticated request returns 401
- **WHEN** `GET /platform-stats/send-method-stats-by-service` is called without an admin JWT
- **THEN** returns HTTP 401
