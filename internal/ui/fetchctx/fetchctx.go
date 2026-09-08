// Package fetchctx derives the fan-out deadline every read command in the TUI
// runs under.
//
// It exists because all six views need the identical five-line
// "capture the budget, wrap Background in a WithTimeout, defer cancel" block,
// which is well past the rule of three. Extracting it also means no view file
// mentions context.Background() on a read path at all, so the absence of an
// unbounded read context is visible by inspection rather than by reading six
// closures (D-09, threat T-02-04).
//
// Write call sites deliberately do not use this: they keep their own contexts
// and are Phase 4's scope.
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
func New(budget time.Duration) (context.Context, context.CancelFunc) {
	if budget <= 0 {
		budget = defaultBudget
	}
	return context.WithTimeout(context.Background(), budget)
}
