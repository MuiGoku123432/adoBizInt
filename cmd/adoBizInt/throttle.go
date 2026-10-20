package main

import (
	"fmt"
	"io"
	"time"
)

// renderThrottleSummary writes one line to the stderr writer when the run was
// paced by ADO's rate-limit headers or re-issued a rate-limited read, and
// nothing when it was neither: "throttled by ADO: 4 waits, 31s total, 1 retry"
// (D-05). It runs once, after dispatch returns, never per event, because a
// throttled fan-out comes in bursts and one line is what an agent parses
// (D-06). stdout and the JSON envelope are untouched: a paced read that
// succeeded is complete and correct, and adoBizInt.log carries the details.
//
// Only counters are printed, so no server-controlled text reaches the
// terminal. The total is whole seconds, the unit the D-08 rate_limit message
// uses, so a parser never meets Duration's "1m30s" form.
func renderThrottleSummary(w io.Writer, waits int64, waited time.Duration, retries int64) {
	if waits == 0 && retries == 0 {
		return
	}
	noun := func(n int64, one, many string) string {
		if n == 1 {
			return one
		}
		return many
	}
	_, _ = fmt.Fprintf(w, "throttled by ADO: %d %s, %ds total, %d %s\n",
		waits, noun(waits, "wait", "waits"),
		int64(waited.Round(time.Second)/time.Second),
		retries, noun(retries, "retry", "retries"))
}
