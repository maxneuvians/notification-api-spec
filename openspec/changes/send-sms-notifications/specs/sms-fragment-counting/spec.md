## ADDED Requirements

### Requirement: GSM-7 encoding detection
`pkg/smsutil.IsGSM7(r rune) bool` SHALL return `true` if and only if the rune is in the GSM-7 basic charset OR the GSM-7 extension table. The extension table characters are: `{`, `}`, `[`, `]`, `\`, `~`, `|`, `€`, `^`, and the backtick character. All other characters (e.g. emoji, accented letters not in GSM-7 basic set, non-Latin scripts) SHALL return `false`.

#### Scenario: ASCII letters are GSM-7
- **WHEN** `IsGSM7('A')` is called
- **THEN** `true` is returned

#### Scenario: GSM-7 extension char is GSM-7
- **WHEN** `IsGSM7('{')` is called
- **THEN** `true` is returned

#### Scenario: Euro sign is GSM-7 (extension table)
- **WHEN** `IsGSM7('€')` is called
- **THEN** `true` is returned

#### Scenario: Accented e not in GSM-7
- **WHEN** `IsGSM7('é')` is called
- **THEN** `false` is returned

#### Scenario: Emoji not in GSM-7
- **WHEN** `IsGSM7('😀')` is called
- **THEN** `false` is returned

---

### Requirement: Fragment counting for GSM-7 single-part messages
A message where every character passes `IsGSM7` AND the total character count (counting extension table chars as 2) is ≤ 160 SHALL be counted as 1 fragment.

#### Scenario: 160-char all-ASCII message is 1 fragment
- **WHEN** a 160-character message of only ASCII printable characters is counted
- **THEN** `FragmentCount` returns `1`

#### Scenario: 159-char message is 1 fragment
- **WHEN** a 159-character all-GSM-7 message is counted
- **THEN** `FragmentCount` returns `1`

#### Scenario: 1-char GSM-7 message is 1 fragment
- **WHEN** a single character `"A"` is counted
- **THEN** `FragmentCount` returns `1`

#### Scenario: Empty string is 0 fragments
- **WHEN** an empty string `""` is counted
- **THEN** `FragmentCount` returns `0`

---

### Requirement: Fragment counting for GSM-7 multi-part messages
A GSM-7 message exceeding 160 characters SHALL use `ceil(len / 153)` fragments (7 characters per part reserved for UDH header).

#### Scenario: 161-char GSM-7 message is 2 fragments
- **WHEN** a 161-character all-ASCII message is counted
- **THEN** `FragmentCount` returns `2` (ceil(161/153) = 2)

#### Scenario: 153-char GSM-7 message is 1 fragment
- **WHEN** a 153-character GSM-7 message is counted
- **THEN** `FragmentCount` returns `1`

#### Scenario: 306-char GSM-7 message is 2 fragments
- **WHEN** a 306-character all-ASCII message is counted
- **THEN** `FragmentCount` returns `2` (306/153 = 2 exactly)

#### Scenario: 307-char GSM-7 message is 3 fragments
- **WHEN** a 307-character all-ASCII message is counted
- **THEN** `FragmentCount` returns `3` (ceil(307/153) = 3)

---

### Requirement: Fragment counting for UCS-2 single-part messages
A message containing at least one character that is NOT in the GSM-7 charset SHALL use UCS-2 encoding. A UCS-2 message with ≤ 70 characters SHALL be counted as 1 fragment.

#### Scenario: Message with non-GSM7 char uses UCS-2 counting
- **WHEN** the message contains `é` (not in GSM-7 basic charset)
- **THEN** UCS-2 counting is used; a 70-char message `"é" + 69 * "a"` returns `1`

#### Scenario: 70-char UCS-2 message is 1 fragment
- **WHEN** a 70-character message containing one `é` and 69 ASCII chars is counted
- **THEN** `FragmentCount` returns `1`

#### Scenario: Single emoji message is 1 fragment
- **WHEN** a message containing just a single emoji character is counted
- **THEN** `FragmentCount` returns `1`

---

### Requirement: Fragment counting for UCS-2 multi-part messages
A UCS-2 message exceeding 70 characters SHALL use `ceil(len / 67)` fragments (3 characters per part reserved for UDH header in UCS-2).

#### Scenario: 71-char UCS-2 message is 2 fragments
- **WHEN** a 71-character message containing one `é` is counted
- **THEN** `FragmentCount` returns `2` (ceil(71/67) = 2)

#### Scenario: 67-char UCS-2 message is 1 fragment
- **WHEN** a 67-character UCS-2 message is counted
- **THEN** `FragmentCount` returns `1`

#### Scenario: 134-char UCS-2 message is 2 fragments
- **WHEN** a 134-character UCS-2 message is counted
- **THEN** `FragmentCount` returns `2` (134/67 = 2 exactly)

#### Scenario: 135-char UCS-2 message is 3 fragments
- **WHEN** a 135-character UCS-2 message is counted
- **THEN** `FragmentCount` returns `3` (ceil(135/67) = 3)

---

### Requirement: GSM-7 extension characters count as 2 characters
Characters in the GSM-7 extension table (`{`, `}`, `[`, `]`, `\`, `~`, `|`, `€`, `^`, backtick) count as 2 characters towards the fragment length calculation because each requires an escape byte in the GSM-7 encoding. A message using only extension chars will still be GSM-7 (not UCS-2).

#### Scenario: Extension char counts as 2 towards fragment limit
- **WHEN** a message contains 80 `{` characters (each counts as 2 → effective 160 chars)
- **THEN** `FragmentCount` returns `1` (exactly at single-part limit)

#### Scenario: 81 extension chars triggers multi-part GSM-7
- **WHEN** a message contains 81 `{` characters (effective 162 chars > 160)
- **THEN** `FragmentCount` returns `2` (multi-part GSM-7: ceil(162/153) = 2)

#### Scenario: Mixed basic and extension chars accumulates correctly
- **WHEN** a message has 159 basic ASCII chars plus one `{` (extension) = effective 161 chars
- **THEN** `FragmentCount` returns `2` (multi-part GSM-7)

---

### Requirement: Fragment count parity with Python reference implementation
The Go `pkg/smsutil.FragmentCount` SHALL produce identical results to the Python `notifications_utils` fragment count for the same input. Discrepancies cause billing errors.

#### Scenario: Cross-language fixture values match
- **WHEN** a set of fixture messages from the Python test suite is run through `FragmentCount`
- **THEN** the fragment count for every fixture matches the Python-computed value exactly

#### Scenario: All GSM-7 extended charset members individually tested
- **WHEN** each of the 10 extension table characters is tested alone, counting as 2
- **THEN** a 2-char-effective string of one extension char is 1 fragment; an 81-char extension string is 2 fragments
