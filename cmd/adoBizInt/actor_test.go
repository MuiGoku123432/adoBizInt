package main

import (
	"context"
	"testing"

	writeguard "sentinovo.ai/bizInt/internal/guard"
)

// swapStdinIsTerminal replaces the terminal seam for one test.
func swapStdinIsTerminal(t *testing.T, isTerminal bool) {
	t.Helper()
	prev := stdinIsTerminal
	stdinIsTerminal = func() bool { return isTerminal }
	t.Cleanup(func() { stdinIsTerminal = prev })
}

// TestCLIActorLabel pins the 07-03 checkpoint decision (tty option): an
// interactive stdin is cli-human, anything else is agent, so an agent-run write
// is never recorded as a human action (T-07-09).
func TestCLIActorLabel(t *testing.T) {
	tests := []struct {
		isTerminal bool
		want       string
	}{
		{isTerminal: true, want: writeguard.ActorCLIHuman},
		{isTerminal: false, want: writeguard.ActorAgent},
	}
	for _, tt := range tests {
		swapStdinIsTerminal(t, tt.isTerminal)
		if got := cliActor(); got != tt.want {
			t.Errorf("cliActor() with terminal=%v = %q, want %q", tt.isTerminal, got, tt.want)
		}
	}
}

// TestExecutedCLIWriteRecordsActor pins GUARD-04 end to end on the CLI: the
// record an executed write appends carries the CLI label and user_approved
// false, which stays false until Phase 13 adds --user-approved.
func TestExecutedCLIWriteRecordsActor(t *testing.T) {
	for isTerminal, want := range map[bool]string{true: writeguard.ActorCLIHuman, false: writeguard.ActorAgent} {
		withConfiguredProjects(t, "P")
		sink := tempAuditSink(t)
		_, stderr := captureOutput(t)
		swapStdinIsTerminal(t, isTerminal)

		c := preReadFake(t, "Active")
		c.UpdateWorkItemStateFunc = func(context.Context, string, int, string) error { return nil }

		if code := dispatch(context.Background(), c, setStateArgs("--confirm")); code != exitOK {
			t.Fatalf("executed write = %d, want %d (stderr: %q)", code, exitOK, stderr.String())
		}

		got := decodeAuditRecord(t, assertAuditLineCount(t, sink, 1)[0])
		if got["actor"] != want {
			t.Errorf("terminal=%v: actor = %v, want %q", isTerminal, got["actor"], want)
		}
		if approved, ok := got["user_approved"].(bool); !ok || approved {
			t.Errorf("terminal=%v: user_approved = %#v, want JSON false", isTerminal, got["user_approved"])
		}
	}
}

// TestActorIsNotAFlag pins that no write verb lets a caller choose its own
// actor label (T-07-10). "user-approved" is deliberately not asserted absent:
// Phase 13 adds it.
func TestActorIsNotAFlag(t *testing.T) {
	if len(writeSupportByCommand) != 6 {
		t.Fatalf("write verbs = %d, want 6", len(writeSupportByCommand))
	}
	for verb := range writeSupportByCommand {
		fs, _, _, _ := newFlagSet(verb)
		if fs.Lookup("actor") != nil {
			t.Errorf("%s registers an actor flag", verb)
		}
	}
}
