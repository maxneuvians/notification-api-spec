## 1. Newsletter Undocumented Routes (C2 fix)

- [ ] 1.1 Implement `POST /newsletter/update-language/{subscriber_id}` — parse and validate `language` against `["en", "fr"]` (400 on invalid value), call `AirtableClient.UpdateSubscriberLanguage`, return 204; register with admin JWT middleware
- [ ] 1.2 Implement `GET /newsletter/send-latest/{subscriber_id}` — Airtable subscriber lookup, `LatestNewsletterTemplate` fetch (sort `-Created at`, limit 1), trigger newsletter send to subscriber's email; return 200 or 404 with distinct messages for missing subscriber vs missing template
- [ ] 1.3 Implement `GET /newsletter/find-subscriber?email=` — require `email` query param (400 if absent), call `AirtableClient.GetSubscriberByEmail`, return 200 with full subscriber JSON shape or 404
- [ ] 1.4 Implement `GET /platform-stats/send-method-stats-by-service` — query `ft_billing` grouped by `service_id` and `key_type`; aggregate into `{service_id, api_count, bulk_count, team_count}` JSON list; return 200
- [ ] 1.5 Write integration tests for all 4 routes using mock `AirtableClient` and mock DB; assert correct status codes, response shapes, and that admin JWT is required (401 without it) for each route
