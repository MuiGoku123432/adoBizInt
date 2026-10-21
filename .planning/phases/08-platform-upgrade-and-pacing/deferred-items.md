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
