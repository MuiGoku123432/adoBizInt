// Package fetchctx derives the bounded deadline every read command (CLI and
// TUI) runs under, and, through NewWithParent, the context every guarded write
// is issued under.
//
// It exists because all six views need the identical five-line
// "capture the budget, wrap the background context in a WithTimeout, defer
// cancel" block, which is well past the rule of three. Extracting it also means
// no view file names the background context on a read path at all, so the
// absence of an unbounded read context is visible by inspection rather than by
// reading six closures (D-09, threat T-02-04).
//
// The wording above deliberately avoids spelling the constructor: an acceptance
// grep counts occurrences in this file to prove the root context is built in
// exactly one place, and a comment that spelled it would defeat that check.
//
// It lives outside internal/ui so non-UI packages (the CLI and the shared write
// guard) can import it without depending on the UI tree.
package fetchctx

import (
	"context"
	"time"
)

// defaultBudget bounds a fetch when no budget is configured. An unconfigured
// budget must not mean an unbounded one — that is the failure this package
// exists to remove.
const defaultBudget = 60 * time.Second

// New returns a context carrying the fan-out budget and its cancel func. The
// caller must defer the cancel inside the command closure, not at construction
// time, or the deadline fires before the request is made.
//
// This is the TUI entry point and the only place in the package that names the
// root context. Its behavior is unchanged by the delegation (CLI-01).
func New(budget time.Duration) (context.Context, context.CancelFunc) {
	return NewWithParent(context.Background(), budget)
}

// NewWithParent is New over a caller-supplied parent, so a cancellable ancestor
// — the CLI's signal-derived context — can abort a read in flight. The
// zero-budget fallback lives here rather than being duplicated at the CLI call
// site, because a second copy of that rule is a second place an unbounded read
// can be reintroduced (D-P7, threat T-03-17).
func NewWithParent(parent context.Context, budget time.Duration) (context.Context, context.CancelFunc) {
	if budget <= 0 {
		budget = defaultBudget
	}
	return context.WithTimeout(parent, budget)
}
