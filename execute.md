# Execution Order

## Phase 1 — Foundation

1. `go-project-setup` — module layout, chi router, sqlc config, pkg/crypto, pkg/signing

## Phase 2 — Data layer

2. `data-model-migrations` — all repository packages (typed Go DB access for every domain)

## Phase 3 — Auth + Provider *(parallel)*

3. `authentication-middleware`
4. `provider-integration`

## Phase 4 — Core API surface *(parallel)*

5. `service-management`
6. `template-management`
7. `send-email-notifications`
8. `send-sms-notifications`

## Phase 5 — User layer + Delivery workers *(parallel)*

9. `user-management`
10. `notification-delivery-pipeline`

## Phase 6 — Organisation + Post-delivery workers *(parallel)*

11. `organisation-management`
12. `notification-receipt-callbacks`
13. `billing-tracking`
14. `bulk-send-jobs`
15. `inbound-sms`

## Phase 7 — Admin + Integrations *(parallel)*

16. `platform-admin-features`
17. `external-client-integrations`

## Phase 8 — Stub endpoints + Integration consumers *(parallel)*

18. `letter-stub-endpoints`
19. `newsletter-endpoints`

## Phase 9 — QA

20. `smoke-test-suite`
