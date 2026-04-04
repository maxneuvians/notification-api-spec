## 1. Cache-Clear and Healthcheck Handlers

- [ ] 1.1 Implement `POST /cache/clear` handler in `internal/handler/admin/`: check for valid cache-clear JWT (`RequireCacheClear` middleware); iterate every pattern in `CACHE_KEYS_ALL` and call `redis_store.delete_cache_keys_by_pattern`; return `201 {"result": "ok"}` on success; catch Redis exceptions and return `500 {"error": "Unable to clear the cache"}`; write tests: keys deleted on success, Redis error returns 500, admin JWT rejected with 401
- [ ] 1.2 Implement `GET /` and `GET /_status` healthcheck handlers: no auth; query DB version; return `200 {"status": "ok", "db_version", "commit_sha", "build_time", "current_time_utc"}`; write tests: status `ok`, DB version present, no auth required
- [ ] 1.3 Implement `GET /status/live-service-and-organisation-counts` (admin JWT): query services with `active=True AND restricted=False AND count_as_live=True`; derive org count as distinct org IDs from qualifying services; return `{"organisations": <int>, "services": <int>}`; write tests: only qualifying services counted, no-org service counts in services, trial/inactive/count-as-live-false excluded

## 2. SRE Tool Handlers

- [ ] 2.1 Implement `RequireSRE` middleware in `internal/middleware/`: validate SRE JWT from request header; return `401` on missing/invalid token; write tests: valid SRE JWT passes, admin JWT rejected with 401, missing token rejected
- [ ] 2.2 Implement `GET /sre-tools/live-service-and-organisation-counts` handler: wire `RequireSRE` middleware; delegate to the same counting function as `GET /status/live-service-and-organisation-counts`; return `{"organisations": <int>, "services": <int>}`; write test: SRE JWT returns 200, admin JWT returns 401

## 3. Cypress Helper Handlers

- [ ] 3.1 Implement `POST /cypress/create_user/<email_suffix>` handler in `internal/handler/admin/cypress/`: check `NOTIFY_ENVIRONMENT == "production"` first (before auth) → return `403`; validate `email_suffix` is alphanumeric (reject non-alphanumeric with `400`); require Cypress JWT; create both users (`notify-ui-tests+ag_<suffix>@cds-snc.ca` regular, `notify-ui-tests+ag_<suffix>_admin@cds-snc.ca` admin); return `201 {"regular": {...}, "admin": {...}}`; write tests: valid suffix creates both users, non-alphanumeric suffix returns 400, production returns 403 before auth check, missing Cypress JWT returns 401
- [ ] 3.2 Implement `GET /cypress/cleanup` handler: same production guard and Cypress JWT auth; delete test users with `created_at` older than 30 days; return `201 {"message": "Clean up complete"}`; write tests: stale users deleted, recent users preserved, production returns 403

## 4. Events Handler

- [ ] 4.1 Implement `POST /events` handler in `internal/handler/admin/events/`: require admin JWT; parse `{"event_type": "<str>", "data": {}}` body; call `dao_create_event` (wrap in caller's DB transaction — do not issue a separate commit); return `201 {"data": {"event_type": ..., "data": {...}}}`; write tests: event persisted and echoed, arbitrary nested `data` stored verbatim

## 5. Email Branding CRUD

- [ ] 5.1 Implement `dao_get_email_branding_options`, `dao_get_email_branding_by_id`, `dao_get_email_branding_by_name`, `dao_create_email_branding`, `dao_update_email_branding` in `internal/repository/email_branding/`; `dao_update_email_branding` must coerce falsy values to `NULL`; write round-trip tests for each function including the falsy-to-NULL coercion
- [ ] 5.2 Implement `GET /email-branding` handler: optional `?organisation_id=<uuid>` filter via `dao_get_email_branding_options`; serialise `organisation_id` as `""` when null; return `{"email_branding": [...]}`; write tests: no-filter returns all, org filter restricts results, null org_id serialised as `""`
- [ ] 5.3 Implement `GET /email-branding/<id>` handler: call `dao_get_email_branding_by_id`; return `{"email_branding": {...}}` including all fields with `organisation_id` as `""` when null and `alt_text_fr` as null when absent; return 404 on miss; write tests: known ID returns 200 with all fields, unknown ID returns 404
- [ ] 5.4 Implement `POST /email-branding` (create) handler: validate required `name` and `created_by_id`; default `brand_type` to `custom_logo`, `text` to `name`; check uniqueness via `dao_get_email_branding_by_name` before insert; return `201 {"data": {...}}`; write tests: valid create returns 201, missing name returns 400, duplicate name returns 400, invalid brand_type returns 400, text defaults to name
- [ ] 5.5 Implement `POST /email-branding/<id>` (update) handler: require `updated_by_id`; check name uniqueness before update if `name` is provided; call `dao_update_email_branding`; return `200`; write tests: partial update succeeds, duplicate name returns 400, invalid brand_type returns 400, falsy field stored as NULL

## 6. Letter Branding CRUD

- [ ] 6.1 Implement `dao_get_letter_branding_by_id`, `dao_get_letter_branding_by_name`, `dao_get_all_letter_branding`, `dao_create_letter_branding`, `dao_update_letter_branding` in `internal/repository/letter_branding/`; `dao_get_all_letter_branding` must order by `name ASC`; `dao_update_letter_branding` must return the updated record and coerce falsy values to NULL; write round-trip tests
- [ ] 6.2 Implement `GET /letter-branding` handler: call `dao_get_all_letter_branding`; return JSON array (empty `[]` when none); write tests: multiple brandings ordered by name, empty table returns `[]`
- [ ] 6.3 Implement `GET /letter-branding/<id>` handler: call `dao_get_letter_branding_by_id`; return `200` with serialised record or `404`; write tests: known ID returns 200, unknown ID returns 404
- [ ] 6.4 Implement `POST /letter-branding` (create) handler: validate required `name` and `filename`; call `dao_create_letter_branding`; return `201 {"id": "<uuid>", ...}`; write test: created record is retrievable via GET
- [ ] 6.5 Implement `POST /letter-branding/<id>` (update) handler: call `dao_update_letter_branding`; catch `IntegrityError` on name collision → return `400 {"message": {"name": ["Name already in use"]}}`; return `200` on success; write tests: name update succeeds, filename update succeeds, name collision returns 400

## 7. Complaint Handlers

- [ ] 7.1 Implement `fetch_paginated_complaints` and `fetch_count_of_complaints` queries in `internal/repository/complaint/`; `fetch_paginated_complaints` must joinload `.service` to avoid `DetachedInstanceError`; `fetch_count_of_complaints` must use `America/Toronto` timezone midnight boundaries (TIMEZONE env var); write tests: pagination metadata present, service eager-loaded, count uses correct timezone boundary
- [ ] 7.2 Implement `GET /complaint` handler: require admin JWT; call `fetch_paginated_complaints(page=1)`; include `{"links": {"prev", "next", "last"}}` when PAGE_SIZE exceeded; return `{"complaints": [...]}`; write tests: sorted created_at DESC, empty state returns `{"complaints": []}`, pagination links present when overflow
- [ ] 7.3 Implement `GET /complaint/count` handler: require admin JWT; parse `?start_date` and `?end_date` (both default to today); validate `%Y-%m-%d` format; call `fetch_count_of_complaints`; return integer directly; write tests: count returned as integer, both dates default to today, invalid format returns 400, inclusive day boundaries correct

## 8. Platform Stats Handlers

- [ ] 8.1 Implement financial-year date validation helper `validate_date_range_is_within_a_financial_year` in `internal/platform_stats/`: reject non-date strings (400 "Input must be a date in the format: YYYY-MM-DD"), start > end (400 "Start date must be before end date"), cross-FY span (400 "Date must be in a single financial year."); write tests covering all three error cases and a valid within-year range
- [ ] 8.2 Implement `GET /platform-stats` handler: require admin JWT; parse optional `?start_date` and `?end_date` (default to today); call `fetch_notification_status_totals_for_all_services`; return `{email: {...}, letter: {...}, sms: {...}}`; write tests: channels present in response, defaults to today, invalid date returns 400
- [ ] 8.3 Implement `GET /platform-stats/usage-for-all-services` handler: require admin JWT; validate date range with `validate_date_range_is_within_a_financial_year`; merge SMS billing, letter cost totals, and letter line items into per-service array; serialise `organisation_id` as `""` when null; sort by blank-org-last then org name then service name; write tests: per-service fields present, letter-only service included, cross-FY returns 400, start>end returns 400, non-date returns 400, org sorting correct
- [ ] 8.4 Implement `GET /platform-stats/usage-for-trial-services` handler: require admin JWT; call `fetch_notification_stats_for_trial_services`; return array (may be `[]`); write tests: trial service stats present, non-trial services excluded
- [ ] 8.5 Implement `GET /platform-stats/send-methods-stats-by-service` handler: require admin JWT; parse `?start_date` and `?end_date`; call `send_method_stats_by_service`; return array; write tests: valid range returns results, invalid date returns 400

## 9. Support Handler

- [ ] 9.1 Implement entity lookup queries (`find_user`, `find_service`, `find_template`, `find_job`, `find_notification`) in `internal/repository/support/`: each catches `NoResultFound` and returns `nil`; write tests for each entity type and cross-entity UUID (verifies first-match-wins order)
- [ ] 9.2 Implement `GET /support/find-id` handler: require admin JWT; parse `?ids` (comma/whitespace separated); return `400 {"error": "no ids provided"}` when absent or empty; for each token — validate UUID (return `{"type": "not a uuid"}` inline if invalid); resolve via fixed order user → service → template → job → notification (first match wins); append `{"type": "no result found"}` for unmatched UUIDs; return JSON array with one entry per input token; write tests: user resolved, service resolved, template resolved, job resolved, notification resolved (null api_key_id), non-UUID inline, unknown UUID inline, multiple IDs in order, absent ids returns 400
