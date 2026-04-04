# Validation Report: behavioral-spec/users-auth.md
Date: 2026-04-04

## Summary
- **Contracts in spec**: ~25+
- **CONFIRMED**: 22
- **DISCREPANCIES**: 0
- **UNCOVERED**: 4 (deactivation cascade matrix, PT Freshdesk flag, go-live Salesforce, org_notes mapping)
- **Risk items**: 4

## Verdict
**CONDITIONAL PASS** — All core auth/user contracts verified. `FF_PT_SERVICE_SKIP_FRESHDESK` behavior is spec'd but not tested (critical gap).

---

## Confirmed

- POST /user: password hashed, `email_auth` default when `auth_type` omitted, mobile required for sms_auth ✅
- POST /user/{id}: sends notification email/SMS on attribute change, calls `salesforce_client.contact_update` ✅
- Password validation: rejects "Password" with `"Password is not allowed."` ✅
- `sms_auth` requires mobile_number → 400 `"Mobile number must be set if auth_type is set to sms_auth"` ✅
- POST .../verify-password: verifies hash, resets `failed_login_count` on success, increments on failure ✅
- POST .../verify-code: marks code used, sets `logged_in_at`, new `session_id`, resets fail count ✅
- Account lockout at `failed_login_count >= 10`: returns 404 even for correct code ✅
- POST .../verify-2fa: does NOT increment `failed_login_count` on failure (differs from /verify-code) ✅
- POST .../2fa-code: sends 5-digit code, deduplicates within delta window, skips if 10+ unexpired codes ✅
- Cypress E2E bypass: accepts any code when `NOTIFY_ENVIRONMENT=development`, `host=localhost:3000` or `dev.local`, email matches `CYPRESS_EMAIL_PREFIX` ✅
- POST .../deactivate: if sole `manage_settings` holder on live service → service suspended (`suspended_at`, `suspended_by_id`) ✅
- POST .../archive: 400 if sole `manage_settings` holder for any service ✅
- POST .../update-password: creates `LoginEvent` only if `loginData` present ✅
- POST .../permissions/{service_id}: replaces entire permission set ✅
- GET /user/{id}: excludes inactive services from permissions and organisations lists ✅

---

## Discrepancies
None found.

---

## Uncovered Contracts

1. **Deactivation cascade matrix** — Suspension email behavior conditional on service type (trial/live) and remaining member count; partial coverage only
2. **`FF_PT_SERVICE_SKIP_FRESHDESK`** — Province/territory services call `email_freshdesk_ticket_pt_service()` and return 201, not 204; **no test found**
3. **POST .../contact-request go_live_request Salesforce call** — `engagement_update` for go-live requests not explicitly verified
4. **Custom `organisation_notes` → `department_org_name` mapping**: fallback to "Unknown" edge cases not tested

---

## RISK Items for Go Implementors

### 🔴 CRITICAL
**1. `FF_PT_SERVICE_SKIP_FRESHDESK` has no test** — Province/territory Freshdesk path (different endpoint + 201 response) is unverified. Implement and add dedicated test.

### 🟡 MEDIUM
**2. Deactivation cascade logic** — 6-combination matrix (service type × member count × suspension email). Implement all branches and test each.

**3. Cypress bypass must NOT activate in production** — Environment + hostname check must be fail-safe. Add explicit test that production env returns 400 even with correct bypass params.

**4. 2FA vs verify-code `failed_login_count` distinction** — `/verify-2fa` does NOT increment on failure; `/verify-code` does. Critical auth security difference; must be explicit in Go implementation.
