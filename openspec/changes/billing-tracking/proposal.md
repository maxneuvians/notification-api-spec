## Why

Billing tracks SMS fragment usage against annual and daily limits. This change implements the billing REST endpoints, the nightly fact-table aggregation workers, annual SMS limit management, quarterly usage email generation, and the `ft_billing` and `ft_notification_status` write paths.

## What Changes

- `internal/handler/billing/` — GET monthly/yearly billing, GET/PUT annual SMS fragment limit, GET notification statistics for a service, GET usage data endpoints
- `internal/worker/reporting/` — `create_nightly_billing.go`, `create_nightly_notification_status.go`, `timeout_sending_notifications.go`, `delete_notifications.go`, `delete_verify_codes.go`, `run_scheduled_jobs.go`
- `internal/worker/scheduled/` — quarterly data insert and quarterly email beat entries (all 4 including the missing Q4 entry — **C6 fix**)
- Beat schedule: nightly tasks (various UTC hours), quarterly tasks at the 4 quarter-start dates including `send-quarterly-email-q4` at `0 23 2 4 *` (the missing entry from the Python implementation)

## Capabilities

### New Capabilities

- `billing-tracking`: Billing REST endpoints, nightly fact-table aggregation, annual SMS limit management, quarterly usage emails with C6 fix

### Modified Capabilities

## Non-goals

- Notification send and limit enforcement (covered in send changes)
- Provider rate records (covered in `provider-integration`)

## Impact

- Requires `data-model-migrations` (billing, annual_billing, ft_billing repositories)
- **C6 fix**: `send-quarterly-email-q4` beat entry added (`0 23 2 4 *`); Python bug not replicated in Go
