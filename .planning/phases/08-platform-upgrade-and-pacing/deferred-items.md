# Phase 8 Deferred Items

Out-of-scope discoveries logged during execution. Not fixed in the plan that found them.

| Found in | File | Item | Why deferred |
|----------|------|------|--------------|
| 08-02 Task 1 | `internal/ado/attachments_test.go:434` | The comment inside `TestNoRawHTTPInADO` ("Only struct fields: newConnection legitimately returns the connection...") names `newConnection`, which 08-02 deleted. The struct-field-only rule is still correct; only the example in the comment is stale. | The comment sits inside the `TestNoRawHTTPInADO` function body, which D-11 requires to stay byte-identical through Phase 8. Reword it when that lock lifts. |

## From 08-06

- `internal/transport/transport.go` is 366 lines after the D-01 fail-fast (314 after 08-05), over the 300-line CONVENTIONS guideline. Candidate split: move hold, untilNotBefore, observe, retryAfter, rateLimitDelay, logRateLimit and Stats into `internal/transport/pace.go`. Not done in 08-06 because the plan pins `ErrRateLimited` to transport.go.
