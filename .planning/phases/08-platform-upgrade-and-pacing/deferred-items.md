# Phase 8 Deferred Items

Out-of-scope discoveries logged during execution. Not fixed in the plan that found them.

| Found in | File | Item | Why deferred |
|----------|------|------|--------------|
| 08-02 Task 1 | `internal/ado/attachments_test.go:434` | The comment inside `TestNoRawHTTPInADO` ("Only struct fields: newConnection legitimately returns the connection...") names `newConnection`, which 08-02 deleted. The struct-field-only rule is still correct; only the example in the comment is stale. | The comment sits inside the `TestNoRawHTTPInADO` function body, which D-11 requires to stay byte-identical through Phase 8. Reword it when that lock lifts. |

## From 08-06

- `internal/transport/transport.go` is 366 lines after the D-01 fail-fast (314 after 08-05), over the 300-line CONVENTIONS guideline. Candidate split: move hold, untilNotBefore, observe, retryAfter, rateLimitDelay, logRateLimit and Stats into `internal/transport/pace.go`. Not done in 08-06 because the plan pins `ErrRateLimited` to transport.go.

## From 08-10

- Accepted risk: a double-encoded dot segment after the org passes the escaped-path rule. `https://dev.azure.com/acme/%252e%252e/other/_apis` (org acme) is allowed, because its wire path `/acme/%252e%252e/other/_apis` holds no `%2e`, `%2f` or `%5c` and `path.Clean` leaves it unchanged (probed 2026-10-21). It only names another org if ADO's front end decodes the path twice, and it can only reach the transport through a same-host redirect or a server-supplied URL, since the SDK builds every route with one `url.PathEscape`. Refusing `%25` outright would also refuse a legitimate name containing a literal percent sign. Revisit if ADO is ever shown to double-decode, or if a code path starts following server-supplied links.
- `internal/transport/transport.go` is now 407 lines (366 before 08-10): the CR-01 `asciiLower` helper and the WR-01 escaped-path checks added 41. The pace.go split above is still the candidate; 08-11 also edits this file.

## From the 08 gap closure (08-09 to 08-11)

Review items (08-REVIEW.md) the gap plans deliberately leave alone, with the reason for each.

| Found in | File | Item | Why deferred |
|----------|------|------|--------------|
| 08-11 Task 2 | `internal/transport/transport.go` | 440 lines after 08-11 (407 after 08-10): `minSendBudget`, `maxHold` and their doc comments added 33. Still over the 300-line CONVENTIONS guideline. | The pace.go split proposed under 08-06 is still the candidate. Not done because no gap fix required it, and a move would bury the WR-02/WR-03 behaviour change in a file rename. |
| 08-REVIEW IN-03 | `internal/ado/retry.go:65` | The retry log line names neither the operation nor the resource, so a throttled fan-out read cannot be told apart from the others. | Deferred: not adjacent to a gap, and retry.go is unchanged by the gap plans. |
| 08-REVIEW IN-04 | `internal/ado/retry.go:52-66` | A 429 that carries no pacing header (for example from a proxy) does not move notBefore, so its read retries go out back to back. | Accepted as a conscious decision under D-04: the pacer obeys only explicit instructions and invents no floor. Revisit if the 08-12 smoke or a later phase sees proxy-issued 429s. |
| 08-REVIEW IN-06 | `internal/ado/client.go:176-187` | `areaURL` still describes and implements the empty-list fallback to org_url as the on-prem answer, which D-12 removed. | Deferred: the fallback builds on org_url, which D-12 validates and the host rule allows, so no credential boundary is involved. It is a diagnostic-clarity change to client.go (409 lines) and its discovery tests. |
| 08-REVIEW IN-05 | `internal/ado/attachments.go:102` | Attachment content served from a host outside the org would now be refused as a foreign redirect hop. | Not deferred: checked live by 08-12 checklist item c. |
| 08-REVIEW WR-03 (remainder) | `internal/transport/transport.go` | notBefore is process-wide and never decays: an instruction above maxHold is stored as ADO sent it, so every later request fails fast as rate_limit until it passes or the process restarts. | Accepted: shortening or dropping the instruction would send before the time ADO instructed, which D-04 forbids. maxHold bounds the hold; the refusal is immediate and names the asked wait (D-08). |
