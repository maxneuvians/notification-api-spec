## Requirements

### Requirement: AwsPinpointClient — phone normalisation
`AwsPinpointClient.send_sms` SHALL normalise the recipient `to` parameter before building the boto call. A 10-digit number (no country code) SHALL have `+1` prepended. An empty string SHALL raise `ValueError("No valid numbers found for SMS delivery")`.

#### Scenario: 10-digit Canadian/US number gets +1 prefix
- **WHEN** `to = "6135550100"` (10 digits, no `+`)
- **THEN** the boto call uses `DestinationPhoneNumber = "+16135550100"`

#### Scenario: Empty to-address raises ValueError
- **WHEN** `to = ""`
- **THEN** `ValueError("No valid numbers found for SMS delivery")` is raised and no boto call is made

---

### Requirement: AwsPinpointClient — origination pool selection
The client SHALL select the `OriginationIdentity` for the boto call based on template ID, service ID, and `sending_vehicle`.

#### Scenario: Default case uses DEFAULT_POOL_ID
- **WHEN** `sending_vehicle` is `None` or `long_code` and template is not in `AWS_PINPOINT_SC_TEMPLATE_IDS`
- **THEN** `OriginationIdentity = AWS_PINPOINT_DEFAULT_POOL_ID`

#### Scenario: Template in SC list AND NOTIFY_SERVICE_ID uses SC_POOL_ID
- **WHEN** `template_id` is in `AWS_PINPOINT_SC_TEMPLATE_IDS` and `service_id == NOTIFY_SERVICE_ID`
- **THEN** `OriginationIdentity = AWS_PINPOINT_SC_POOL_ID`

#### Scenario: sending_vehicle=short_code uses SC_POOL_ID
- **WHEN** `sending_vehicle = SmsSendingVehicles("short_code")`
- **THEN** `OriginationIdentity = AWS_PINPOINT_SC_POOL_ID`

#### Scenario: International (non-+1) number omits OriginationIdentity and DryRun
- **WHEN** `to` starts with `+` but not `+1` (e.g., `+442071234567`)
- **THEN** the boto call does NOT include `OriginationIdentity` or `DryRun` fields

---

### Requirement: AwsPinpointClient — dry run for external test number
When the recipient matches `Config.EXTERNAL_TEST_NUMBER`, the client SHALL set `DryRun = true` in the boto call so no actual SMS is sent by the provider.

#### Scenario: External test number sets DryRun=True in boto call
- **WHEN** `to` equals `Config.EXTERNAL_TEST_NUMBER`
- **THEN** the boto params include `DryRun = True`

---

### Requirement: AwsPinpointClient — dedicated sender uses _dedicated_client
When `sender` is a long-code number and `FF_USE_PINPOINT_FOR_DEDICATED = true`, the client SHALL use the dedicated Pinpoint client (`_dedicated_client`) with `OriginationIdentity = sender`. The default `_client` SHALL NOT be called.

#### Scenario: Dedicated sender routes to _dedicated_client
- **WHEN** `sender` is a long-code number and `FF_USE_PINPOINT_FOR_DEDICATED = true`
- **THEN** `_dedicated_client.send_text_message` is called with `OriginationIdentity = sender`; `_client` is not called

#### Scenario: Dedicated sender with FF disabled still uses default pool
- **WHEN** `sender` is a long-code number and `FF_USE_PINPOINT_FOR_DEDICATED = false`
- **THEN** `_client.send_text_message` is called with `DEFAULT_POOL_ID`

---

### Requirement: AwsPinpointClient — always sets required static fields
Every Pinpoint `send_text_message` call SHALL include `MessageType = "TRANSACTIONAL"` and `ConfigurationSetName = <config_set_name>`.

#### Scenario: MessageType and ConfigurationSetName always present
- **WHEN** any `send_sms` call is made
- **THEN** the boto params include `MessageType = "TRANSACTIONAL"` and `ConfigurationSetName = <configured value>`

---

### Requirement: AwsPinpointClient — opted-out number returns "opted_out" string
When Pinpoint returns a `ConflictException` with `Reason = "DESTINATION_PHONE_NUMBER_OPTED_OUT"`, the client SHALL return the string `"opted_out"` without raising an exception.

#### Scenario: OPTED_OUT ConflictException returns "opted_out" string
- **WHEN** Pinpoint raises `ConflictException` with `Reason = "DESTINATION_PHONE_NUMBER_OPTED_OUT"`
- **THEN** `send_sms` returns the string `"opted_out"` and no exception is propagated

---

### Requirement: AwsPinpointClient — other ConflictException raises PinpointConflictException
Any `ConflictException` with a Reason other than `DESTINATION_PHONE_NUMBER_OPTED_OUT` SHALL be re-raised as `PinpointConflictException` wrapping the original exception.

#### Scenario: Non-opted-out ConflictException raises PinpointConflictException
- **WHEN** Pinpoint raises `ConflictException` with any Reason other than OPTED_OUT
- **THEN** `PinpointConflictException` is raised wrapping the original boto exception

---

### Requirement: AwsPinpointClient — ValidationException raises PinpointValidationException
A Pinpoint `ValidationException` SHALL be re-raised as `PinpointValidationException` wrapping the original, preserving the `Reason` field for the caller to use as `feedback_reason`.

#### Scenario: ValidationException wraps to PinpointValidationException with Reason preserved
- **WHEN** Pinpoint raises `ValidationException` with `Reason = "NO_ORIGINATION_IDENTITIES_FOUND"`
- **THEN** `PinpointValidationException` is raised; the caller can access the `Reason` field from the wrapped exception

---

### Requirement: AwsSesClient — multipart/alternative for emails without attachments
`AwsSesClient.send_email` SHALL construct a `multipart/alternative` MIME message (text/plain + text/html parts) when `attachments` is empty or nil, and call `send_raw_email` with the serialised MIME bytes.

#### Scenario: No-attachment email produces multipart/alternative structure
- **WHEN** `send_email` is called with no attachments
- **THEN** the raw MIME message has a `multipart/alternative` outer part containing `text/plain` and `text/html` sub-parts

#### Scenario: No-attachment email calls send_raw_email
- **WHEN** `send_email` is called with no attachments
- **THEN** `boto3_client.send_raw_email(RawMessage={"Data": <raw bytes>})` is called

---

### Requirement: AwsSesClient — multipart/mixed for emails with attachments
When `attachments` is non-empty, `send_email` SHALL construct a `multipart/mixed` MIME message containing the `multipart/alternative` sub-part plus one attachment part per file. Attachment data SHALL be base64-encoded with `Content-Disposition: attachment`.

#### Scenario: Email with attachments produces multipart/mixed structure
- **WHEN** `send_email` is called with one or more attachments
- **THEN** the MIME outer part is `multipart/mixed`; it contains a `multipart/alternative` sub-part and one attachment part per file

#### Scenario: Attachment data is base64-encoded with Content-Disposition: attachment
- **WHEN** an attachment is added
- **THEN** the MIME part uses `Content-Transfer-Encoding: base64` and `Content-Disposition: attachment`

---

### Requirement: AwsSesClient — reply-to header handling
The client SHALL include the `reply-to` header only when `reply_to_address` is a non-nil, non-empty string. IDN domain parts SHALL be punycode-encoded, and then the full address SHALL be base64-encoded.

#### Scenario: Nil reply_to omits reply-to header
- **WHEN** `reply_to_address = nil`
- **THEN** the raw MIME message does not contain a `reply-to` header

#### Scenario: ASCII reply-to address is included as-is
- **WHEN** `reply_to_address = "support@example.com"` (ASCII domain)
- **THEN** the `reply-to` header contains `support@example.com`

#### Scenario: IDN reply-to domain is punycode-encoded
- **WHEN** `reply_to_address` contains a non-ASCII domain (e.g., `user@例え.jp`)
- **THEN** the domain is converted to its punycode ASCII equivalent before base64 encoding

---

### Requirement: AwsSesClient — to-address IDN punycode encoding
The `to_addresses` list SHALL have any IDN domain parts converted to punycode ASCII before the MIME message is constructed.

#### Scenario: ASCII to-address is unchanged
- **WHEN** `to_addresses = ["recipient@example.com"]`
- **THEN** the MIME `To:` header contains `recipient@example.com` without modification

#### Scenario: IDN to-address is punycode-encoded
- **WHEN** `to_addresses` contains a non-ASCII domain
- **THEN** the domain is converted to its punycode ASCII equivalent

---

### Requirement: AwsSesClient — InvalidParameterValue ClientError maps to InvalidEmailError
When `send_raw_email` raises a `ClientError` with Code `"InvalidParameterValue"`, the client SHALL raise `InvalidEmailError` with the AWS message.

#### Scenario: ClientError InvalidParameterValue raises InvalidEmailError
- **WHEN** SES raises `ClientError` with `Code = "InvalidParameterValue"`
- **THEN** `InvalidEmailError` is raised with the AWS error message

---

### Requirement: AwsSesClient — all other ClientErrors map to AwsSesClientException
Any other `ClientError` from SES SHALL be re-raised as `AwsSesClientException` with the AWS message.

#### Scenario: Generic ClientError raises AwsSesClientException
- **WHEN** SES raises a `ClientError` with any Code other than `"InvalidParameterValue"`
- **THEN** `AwsSesClientException` is raised with the AWS error message

---

### Requirement: punycode_encode_email converts IDN domains to ASCII
The `punycode_encode_email` helper SHALL convert the domain part of an email address from Unicode to ASCII-compatible encoding (ACE). ASCII-only domains SHALL be returned unchanged.

#### Scenario: ASCII domain is returned unchanged
- **WHEN** `punycode_encode_email("user@example.com")` is called
- **THEN** `"user@example.com"` is returned without modification

#### Scenario: Unicode domain is converted to punycode
- **WHEN** `punycode_encode_email("user@例え.jp")` is called
- **THEN** the returned string uses the ACE-prefixed punycode domain (e.g., `"user@xn--r8jz45g.jp"`)

---

### Requirement: AwsPinpointClient and AwsSesClient implement mockable interfaces
Both clients SHALL implement narrow Go interfaces so they can be replaced with mocks in unit tests without any real network calls.

#### Scenario: AwsPinpointClient satisfies SMSSender interface
- **WHEN** `AwsPinpointClient` is constructed
- **THEN** it satisfies the `SMSSender` interface used by deliver-sms workers

#### Scenario: AwsSesClient satisfies EmailSender interface
- **WHEN** `AwsSesClient` is constructed
- **THEN** it satisfies the `EmailSender` interface used by deliver-email workers
