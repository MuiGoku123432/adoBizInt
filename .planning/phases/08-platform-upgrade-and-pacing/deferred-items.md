# Phase 8 Deferred Items

Out-of-scope discoveries logged during execution. Not fixed in the plan that found them.

| Found in | File | Item | Why deferred |
|----------|------|------|--------------|
| 08-02 Task 1 | `internal/ado/attachments_test.go:434` | The comment inside `TestNoRawHTTPInADO` ("Only struct fields: newConnection legitimately returns the connection...") names `newConnection`, which 08-02 deleted. The struct-field-only rule is still correct; only the example in the comment is stale. | The comment sits inside the `TestNoRawHTTPInADO` function body, which D-11 requires to stay byte-identical through Phase 8. Reword it when that lock lifts. |
