## Why

GC Notify supports 7 messaging providers (AWS SES, SNS, Pinpoint, MMG, Firetext, Loadtesting, DVLA) with priority-based selection and automatic failover. This change implements provider CRUD, the priority/failover selection algorithm, and the provider rate tracking used during delivery.

## What Changes

- `internal/handler/providers/` — GET/PUT provider details, provider stats, provider list
- `internal/service/providers/` — provider selection algorithm (active provider with highest priority), failover on provider error, rate tracking per provider, `InsertProviderDetailsHistory` on every update

## Capabilities

### New Capabilities

- `provider-integration`: Provider CRUD with history writes, priority-based selection, failover logic, per-provider rate tracking

### Modified Capabilities

## Non-goals

- AWS client implementations (covered in `notification-delivery-pipeline`)
- MMG, Firetext, DVLA client implementations (letter stubs, not actively in use)

## Impact

- Requires `data-model-migrations` (provider_details, provider_details_history repositories)
- Provider selection is called from `notification-delivery-pipeline` deliver workers
