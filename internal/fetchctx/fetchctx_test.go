package fetchctx

import (
	"context"
	"testing"
	"time"
)

// TestNewWithParentAppliesBudget pins that a positive budget becomes the
// deadline. The tolerance is deliberately wide: the assertion is that the
// budget was applied at all, not that the clock is precise.
func TestNewWithParentAppliesBudget(t *testing.T) {
	const budget = 250 * time.Millisecond

	start := time.Now()
	ctx, cancel := NewWithParent(context.Background(), budget)
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatalf("NewWithParent(bg, %v) returned a context with no deadline", budget)
	}
	got := deadline.Sub(start)
	if got < budget/2 || got > budget*2 {
		t.Errorf("deadline is %v from now, want roughly %v", got, budget)
	}
}

// TestNewWithParentZeroBudgetStillHasDeadline is the load-bearing one. An
// unconfigured budget must fall back to the package default, never to an
// unbounded read — that fallback is the entire reason this package exists, and
// the CLI's --timeout goes through the same door (threat T-03-17).
func TestNewWithParentZeroBudgetStillHasDeadline(t *testing.T) {
	for _, budget := range []time.Duration{0, -1 * time.Second} {
		ctx, cancel := NewWithParent(context.Background(), budget)

		deadline, ok := ctx.Deadline()
		if !ok {
			t.Errorf("NewWithParent(bg, %v) has no deadline; want the default budget", budget)
			cancel()
			continue
		}
		remaining := time.Until(deadline)
		if remaining < defaultBudget/2 || remaining > defaultBudget*2 {
			t.Errorf("NewWithParent(bg, %v) deadline is %v away, want roughly %v",
				budget, remaining, defaultBudget)
		}
		cancel()
	}
}

// TestNewWithParentPropagatesParentCancellation is what makes SIGINT handling
// possible: the CLI derives its read context from a signal-cancelled parent, so
// cancelling that parent must cancel the read.
func TestNewWithParentPropagatesParentCancellation(t *testing.T) {
	parent, cancelParent := context.WithCancel(context.Background())

	ctx, cancel := NewWithParent(parent, time.Hour)
	defer cancel()

	select {
	case <-ctx.Done():
		t.Fatalf("child context was already done before the parent was cancelled")
	default:
	}

	cancelParent()

	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatalf("cancelling the parent did not cancel the child")
	}
	if err := ctx.Err(); err != context.Canceled {
		t.Errorf("ctx.Err() = %v, want context.Canceled", err)
	}
}

// TestNewIsUnchanged guards the six TUI call sites. CLI-01 forbids altering TUI
// behavior, so New must keep producing a deadline-bearing context with no
// cancellable ancestor even though it now delegates.
func TestNewIsUnchanged(t *testing.T) {
	ctx, cancel := New(0)
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatalf("New(0) returned a context with no deadline")
	}
	if remaining := time.Until(deadline); remaining < defaultBudget/2 {
		t.Errorf("New(0) deadline is %v away, want roughly %v", remaining, defaultBudget)
	}

	ctx2, cancel2 := New(150 * time.Millisecond)
	defer cancel2()
	d2, ok := ctx2.Deadline()
	if !ok {
		t.Fatalf("New(150ms) returned a context with no deadline")
	}
	if remaining := time.Until(d2); remaining > time.Second {
		t.Errorf("New(150ms) deadline is %v away, want roughly 150ms", remaining)
	}
}
