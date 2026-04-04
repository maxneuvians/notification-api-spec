# Behavioral Spec: Smoke Tests

## Processed Files
- [x] tests_smoke/smoke/test_admin_csv.py
- [x] tests_smoke/smoke/test_admin_one_off.py
- [x] tests_smoke/smoke/test_api_bulk.py
- [x] tests_smoke/smoke/test_api_one_off.py
- [x] tests_smoke/smoke_test.py
- [x] tests_smoke/smoke/common.py
- [x] tests_smoke/send_many.py

---

## Overview

The smoke tests exercise a live Notify API instance (staging or production) end-to-end to confirm that core notification delivery paths are operational. They are not unit or integration tests — they make real HTTP requests, push real jobs through the system, and poll until delivery is confirmed or a timeout is reached.

Each test sends at least one email and one SMS notification and waits for the status to reach `delivered`. Four delivery paths are covered:

| Path | Auth | Batch size |
|---|---|---|
| Admin one-off | JWT (admin client) | 1 |
| Admin CSV upload | JWT (admin client) | configurable (`JOB_SIZE`, default 2) |
| API one-off | API key | 1 |
| API bulk (v2) | API key | configurable (`JOB_SIZE`, default 2) |

An additional variant of the API one-off test exercises file attachments (attached and link modes) for email.

The `send_many.py` utility is a separate load-generation tool that drives very large CSV-based sends (default 50,000 per job) using the same admin CSV path, without checking for delivery success.

---

## Common Infrastructure

Source: `tests_smoke/smoke/common.py`

### `Config` (environment-driven)

All configuration is loaded from environment variables (with `.env` file support via `python-dotenv`).

| Attribute | Env var | Default |
|---|---|---|
| `API_HOST_NAME` | `SMOKE_API_HOST_NAME` | `http://localhost:6011` |
| `IS_LOCAL` | _(derived)_ | `True` if `localhost` in host |
| `ADMIN_CLIENT_USER_NAME` | _(hardcoded)_ | `notify-admin` |
| `ADMIN_CLIENT_SECRET` | `SMOKE_ADMIN_CLIENT_SECRET` | `local_app` |
| `POLL_TIMEOUT` | `SMOKE_POLL_TIMEOUT` | `120` (seconds) |
| `AWS_REGION` | _(hardcoded)_ | `ca-central-1` |
| `CSV_UPLOAD_BUCKET_NAME` | `SMOKE_CSV_UPLOAD_BUCKET_NAME` | `notification-canada-ca-staging-csv-upload` |
| `AWS_ACCESS_KEY_ID` | `SMOKE_AWS_ACCESS_KEY_ID` | _(none; OIDC assumed if absent)_ |
| `AWS_SECRET_ACCESS_KEY` | `SMOKE_AWS_SECRET_ACCESS_KEY` | _(none; OIDC assumed if absent)_ |
| `SERVICE_ID` | `SMOKE_SERVICE_ID` | _(required)_ |
| `USER_ID` | `SMOKE_USER_ID` | _(required for admin paths)_ |
| `EMAIL_TO` | `SMOKE_EMAIL_TO` | `internal.test@cds-snc.ca` |
| `SMS_TO` | `SMOKE_SMS_TO` | `+16135550123` |
| `EMAIL_TEMPLATE_ID` | `SMOKE_EMAIL_TEMPLATE_ID` | _(required)_ |
| `SMS_TEMPLATE_ID` | `SMOKE_SMS_TEMPLATE_ID` | _(required)_ |
| `API_KEY` | `SMOKE_API_KEY` | _(required for API paths)_ |
| `JOB_SIZE` | `SMOKE_JOB_SIZE` | `2` |

AWS credentials default to the ambient session (OIDC role assumption in CI). Static key/secret credentials are supported for local use.

### Enumerations

- **`Notification_type`**: `EMAIL = "email"`, `SMS = "sms"`
- **`Attachment_type`**: `NONE = "none"`, `ATTACHED = "attach"`, `LINK = "link"`

### Helper functions

**`rows_to_csv(rows)`**
Converts a list of lists into a CSV string using Python's `csv.writer`. Used to build the in-memory CSV payloads for bulk and CSV-upload tests.

**`job_line(data, number_of_lines, prefix)`**
Generator that yields `[data, "{prefix} {n}"]` rows — i.e., the same recipient address repeated `number_of_lines` times with an incrementing personalisation variable.

**`single_succeeded(uri, use_jwt)`**
Polls a single notification status endpoint once per second for up to `POLL_TIMEOUT` seconds. Returns `True` when `status == "delivered"`. In local mode also accepts any non-failure status. Returns `False` on `permanent-failure` or timeout.
- JWT auth used when `use_jwt=True` (admin paths); API key auth used otherwise.

**`job_succeeded(service_id, job_id)`**
Polls `GET /service/{service_id}/job/{job_id}` once per second until `job_status == "finished"`. Returns `True` when all statistics entries have `status == "delivered"` (or all are non-failure in local mode). Returns `False` on any `permanent-failure` or timeout.

**`s3upload(service_id, data)`**
Generates a UUID as the upload ID, then stores the CSV string at `service-{service_id}-notify/{upload_id}.csv` in `CSV_UPLOAD_BUCKET_NAME` with AES-256 server-side encryption. Returns the upload ID.

**`set_metadata_on_csv_upload(service_id, upload_id, **kwargs)`**
Performs an S3 `copy_from` on the uploaded object (copy-to-self) to set custom S3 metadata: `notification_count`, `template_id`, `valid`, `original_file_name`. This is the mechanism by which the API/worker discovers job parameters.

---

## Smoke Test Scenarios

### `test_admin_one_off`

**Endpoint(s) exercised**
- `POST /service/{service_id}/send-notification` (admin internal endpoint)
- `GET /service/{service_id}/notifications/{notification_id}` (polling)

**What it does**
1. Creates a JWT token signed with `ADMIN_CLIENT_SECRET` for client ID `notify-admin`.
2. POSTs to the internal admin send-notification endpoint with:
   - `to`: recipient address (`EMAIL_TO` or `SMS_TO`)
   - `template_id`: the appropriate template for the notification type
   - `created_by`: `USER_ID`
   - `personalisation`: `{"var": "smoke test admin one off"}`
3. In non-local mode, polls the returned notification ID until delivered.

**What it asserts**
- The POST returns HTTP 201.
- The notification eventually reaches `status == "delivered"` within `POLL_TIMEOUT` seconds.

**Configuration**
`SERVICE_ID`, `USER_ID`, `ADMIN_CLIENT_SECRET`, `EMAIL_TO` / `SMS_TO`, `EMAIL_TEMPLATE_ID` / `SMS_TEMPLATE_ID`.

---

### `test_admin_csv`

**Endpoint(s) exercised**
- S3 bucket `CSV_UPLOAD_BUCKET_NAME` (direct write via boto3)
- `POST /service/{service_id}/job` (admin internal endpoint)
- `GET /service/{service_id}/job/{job_id}` (polling)

**What it does**
1. Builds an in-memory CSV with a header row (`email address,var` or `phone number,var`) and `JOB_SIZE` data rows, all addressed to the same recipient with incrementing personalisation values prefixed with `"smoke test admin csv"`.
2. Uploads the CSV to S3 under `service-{service_id}-notify/{upload_id}.csv` (AES-256 encrypted).
3. Copies the S3 object over itself to attach metadata: `notification_count=1`, `template_id`, `valid=True`, `original_file_name="smoke_test.csv"`.
4. Creates a JWT token and POSTs to `/service/{service_id}/job` with `{"id": upload_id, "created_by": USER_ID}`.
5. In non-local mode, polls the job until all notifications are delivered.

**What it asserts**
- The job POST returns HTTP 201.
- All job statistics entries eventually show `status == "delivered"`.

**Configuration**
`SERVICE_ID`, `USER_ID`, `ADMIN_CLIENT_SECRET`, `AWS_REGION`, `CSV_UPLOAD_BUCKET_NAME`, `EMAIL_TO` / `SMS_TO`, `EMAIL_TEMPLATE_ID` / `SMS_TEMPLATE_ID`, `JOB_SIZE`. AWS credentials via OIDC or `AWS_ACCESS_KEY_ID` + `AWS_SECRET_ACCESS_KEY`.

---

### `test_api_one_off`

**Endpoint(s) exercised**
- `POST /v2/notifications/email` or `POST /v2/notifications/sms`
- Notification URI returned in the response body (polling)

**What it does**

*No-attachment variant (email and SMS):*
1. Builds a payload with `email_address` / `phone_number`, `template_id`, and `personalisation: {"var": "smoke test api one off"}`.
2. POSTs to `/v2/notifications/{notification_type}` using `ApiKey-v1 {API_KEY}` authorization.
3. In non-local mode, polls `response.json()["uri"]` until delivered.

*File-attached variant (email only, `Attachment_type.ATTACHED`):*
1. Adds `personalisation.application_file` with a base64-encoded file payload (`"aGkgdGhlcmU="` = `"hi there"`), filename `test_file.txt`, and `sending_method: "attach"`.
2. Same POST and polling steps.

*File-link variant (email only, `Attachment_type.LINK`):*
1. Sets `personalisation.var` to the file object dict (base64 content, filename, `sending_method: "link"`).
2. Same POST and polling steps.

SMS with any attachment type is explicitly rejected with a printed error and early return (no API call made).

**What it asserts**
- The POST returns HTTP 201.
- The notification eventually reaches `status == "delivered"` via the URI returned in the response.

**Configuration**
`API_KEY`, `EMAIL_TO` / `SMS_TO`, `EMAIL_TEMPLATE_ID` / `SMS_TEMPLATE_ID`.

---

### `test_api_bulk`

**Endpoint(s) exercised**
- `POST /v2/notifications/bulk`
- `GET /service/{service_id}/job/{job_id}` (polling)

**What it does**
1. Builds an in-memory CSV with a header row and `JOB_SIZE` rows, all addressed to the same recipient with personalisation values prefixed with `"smoke test api bulk"`.
2. POSTs to `/v2/notifications/bulk` using `ApiKey-v1 {API_KEY}` authorization with:
   - `name`: timestamp-based bulk job name (`"My bulk name {utcnow().isoformat()}"`)
   - `template_id`: the appropriate template
   - `csv`: the CSV string
3. In non-local mode, polls the job ID from `response.json()["data"]["id"]` until all notifications are delivered.

**What it asserts**
- The POST returns HTTP 201.
- All job statistics entries eventually show `status == "delivered"`.

**Configuration**
`API_KEY`, `SERVICE_ID`, `EMAIL_TO` / `SMS_TO`, `EMAIL_TEMPLATE_ID` / `SMS_TEMPLATE_ID`, `JOB_SIZE`.

---

## Test Orchestration (`smoke_test.py`)

The top-level runner executes all tests in the following order for **both** `EMAIL` and `SMS` notification types:

1. `test_admin_one_off`
2. `test_admin_csv` _(unless `--nocsv` flag set)_
3. `test_api_one_off` (no attachment)
4. `test_api_bulk`

After the loop, if `--nofiles` is not set, two additional email-only `test_api_one_off` calls execute:

5. `test_api_one_off` (email, `Attachment_type.ATTACHED`)
6. `test_api_one_off` (email, `Attachment_type.LINK`)

**CLI flags**

| Flag | Effect |
|---|---|
| `-l` / `--local` | Skip delivery polling; expect manual verification |
| `--nofiles` | Skip the file-attachment email variants |
| `--nocsv` | Skip the admin CSV upload tests |

Any test failure calls `exit(1)`, aborting the entire run immediately.

---

## `send_many.py` — Bulk Load Utility

This is not a smoke test; it is a load-generation and performance-test tool that reuses the same admin CSV upload path.

**What it does**
1. Accepts `--notifications N` (total notifications), `--job_size J` (default 50,000), and `--sms`.
2. Splits the total send into chunks of `job_size`.
3. For each chunk, calls `send_admin_csv()`:
   - Builds a CSV with `job_size` rows all going to the same recipient.
   - Uploads to S3 with AES-256 encryption.
   - Sets S3 metadata with `notification_count=1`, `template_id`, `valid=True`, `original_file_name="Large send {timestamp}.csv"`.
   - POSTs `/service/{service_id}/job` with JWT auth.
4. Sleeps 1 second between chunks.

**What it does NOT do**
It does not poll for delivery success. It is fire-and-forget, intended to observe system throughput under load.

**Configuration**
Same as `test_admin_csv`: `SERVICE_ID`, `USER_ID`, `ADMIN_CLIENT_SECRET`, `CSV_UPLOAD_BUCKET_NAME`, `EMAIL_TEMPLATE_ID` / `SMS_TEMPLATE_ID`, `EMAIL_TO` / `SMS_TO`. AWS credentials via OIDC or explicit keys.

---

## End-to-End Flow Contracts

The smoke tests collectively verify two distinct end-to-end delivery flows through the live API.

### Flow 1: Single notification via admin internal endpoint

```
Test runner
  → POST /service/{id}/send-notification (JWT)
      → API creates notification record
          → Worker picks up notification
              → Provider delivers email/SMS
                  → Callback updates status to "delivered"
  → GET /service/{id}/notifications/{notification_id} (JWT, polled)
      → status == "delivered"
```

**Contract**: A JWT-authenticated admin single-send request results in a delivered notification within `POLL_TIMEOUT` seconds.

---

### Flow 2: Batch job via S3 CSV upload (admin path)

```
Test runner
  → PUT CSV to S3 (boto3, AES-256)
  → Copy S3 object to self with metadata (notification_count, template_id, valid)
  → POST /service/{id}/job (JWT, body: {id: upload_id, created_by: user_id})
      → API creates job record pointing to S3 object
          → Worker reads CSV from S3
              → Worker creates individual notification records
                  → Provider delivers each email/SMS
                      → Callbacks update statuses to "delivered"
  → GET /service/{id}/job/{job_id} (JWT, polled)
      → job_status == "finished"
      → All statistics.status == "delivered"
```

**Contract**: A JWT-authenticated CSV job submission results in all constituent notifications being delivered within `POLL_TIMEOUT` seconds.

---

### Flow 3: Single notification via public API key endpoint

```
Test runner
  → POST /v2/notifications/{email|sms} (ApiKey-v1)
      → API creates notification record
          → Worker picks up notification
              → Provider delivers email/SMS (optionally with file attachment or link)
                  → Callback updates status to "delivered"
  → GET {uri from response body} (ApiKey-v1, polled)
      → status == "delivered"
```

**Contract**: An API-key-authenticated v2 single-send request (with or without a file attachment) results in a delivered notification within `POLL_TIMEOUT` seconds.

---

### Flow 4: Batch job via v2 bulk API endpoint

```
Test runner
  → POST /v2/notifications/bulk (ApiKey-v1, body: {name, template_id, csv})
      → API creates job record, parses inline CSV
          → Worker creates individual notification records
              → Provider delivers each email/SMS
                  → Callbacks update statuses to "delivered"
  → GET /service/{id}/job/{job_id} (JWT, polled)
      → job_status == "finished"
      → All statistics.status == "delivered"
```

**Contract**: An API-key-authenticated v2 bulk send with an inline CSV body results in all constituent notifications being delivered within `POLL_TIMEOUT` seconds. Note: submission uses an API key; polling uses a JWT (admin client), as the job status endpoint is internal.

---

### Delivery polling contract (all flows)

- Polls at 1-second intervals.
- Succeeds on `status == "delivered"`.
- Fails immediately on `status == "permanent-failure"`.
- In local mode, succeeds on any status that does not contain `"fail"`.
- Times out after `POLL_TIMEOUT` seconds (default 120).
- On any failure, pretty-prints the full response body and exits with code 1.
