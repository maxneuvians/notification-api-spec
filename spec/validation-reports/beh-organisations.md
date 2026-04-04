# Validation Report: behavioral-spec/organisations.md
Date: 2026-04-04

## Summary
- **Contracts in spec**: ~15
- **CONFIRMED**: 12
- **DISCREPANCIES**: 0
- **UNCOVERED**: 4 (MOU notifications, domain edge cases, invite URL customization)
- **Risk items**: 4

## Verdict
**PASS** — All core organisation contracts verified. MOU notification test is skipped (medium risk). Domain cross-org semantics ambiguous.

---

## Confirmed

- GET /organisations: all (active + inactive), ordered active-first then alphabetical ✅
- GET /organisations/{id}: full detail with all nullable fields ✅
- GET /organisations/by-domain: 200 with org, 404 if unregistered, 400 if domain contains @ ✅
- POST /organisations: crown must be boolean (null → error `"crown None is not of type boolean"`), organisation_type required ✅
- Duplicate name → 400 `"Organisation name already exists"` ✅
- POST .../domains: full domain replacement semantics; updating other fields does NOT clear domains ✅
- POST .../services: links service, copies org's `organisation_type` and `crown`; PT org sets 3-day retention ✅
- GET .../services: sorted (active alphabetical, then inactive alphabetical) ✅
- POST .../users/{user_id}: adds user; returned user includes org in organisations array ✅
- GET .../users: active users only ✅
- POST .../invite: status=pending, deliver_email to notify-internal-tasks, reply_to_text = inviter email ✅
- GET /organisations/unique: 200 true/false; case-insensitive; own name always returns true; both params required ✅

---

## Discrepancies
None found.

---

## Uncovered Contracts

1. **MOU notification emails** — setting `agreement_signed: true` should send template email; test is `@pytest.mark.skip` — not validated
2. **GET by-domain cross-org semantics** — test has xfail for querying domain owned by a different org; spec is ambiguous about whether this should 200 or 404
3. **Duplicate domain in same list (different case)** — raises `IntegrityError`; marked xfail in test
4. **invite_link_host override** — basic test exists but edge cases not exhaustive

---

## RISK Items for Go Implementors

### 🔴 CRITICAL
**1. Domain cross-org semantics ambiguous** — `GET /organisations/by-domain` with a domain registered to a different org: should it return that org or 404? Test is xfail with no resolution. Clarify with product before Go implementation.

### 🟡 MEDIUM
**2. MOU notification emails not tested** — Template differentiation based on `agreement_signed_on_behalf_of_name` presence; implement carefully.

**3. Case-insensitive organisation name and domain handling** — Requires correct SQL collation everywhere, not only at REST validation layer.

**4. Service organisation_type propagation on link** — Linking a service to an org automatically overwrites the service's `organisation_type`. Go implementation must perform this cascade explicitly.
