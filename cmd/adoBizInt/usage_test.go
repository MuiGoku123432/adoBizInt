package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// TestFlagsRegisteredOnEverySubcommand pins D-20: the shared registrar is what
// prevents `prs list` accepting --timeout while `releases approvals` silently
// does not. Every verb is run with all three global flags set.
func TestFlagsRegisteredOnEverySubcommand(t *testing.T) {
	for _, key := range wantCommands {
		t.Run(key, func(t *testing.T) {
			_, stderr := captureOutput(t)

			args := append(strings.Split(key, " "),
				"--json", "--timeout", "5s", "--project", "alpha,beta", "--project", "gamma")
			if code := dispatch(context.Background(), allReadsFake(t), args); code != exitOK {
				t.Fatalf("dispatch(%q) = %d, want %d (stderr: %q)", args, code, exitOK, stderr.String())
			}
		})
	}
}

// TestProjectListAccumulates pins both accepted spellings of --project.
func TestProjectListAccumulates(t *testing.T) {
	_, _ = captureOutput(t)

	fs, gf, _ := newFlagSet("prs list")
	if err := fs.Parse([]string{"--project", "alpha, beta", "--project", "gamma"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := []string{"alpha", "beta", "gamma"}
	if got := []string(gf.projects); len(got) != len(want) {
		t.Fatalf("projects = %v, want %v", got, want)
	} else {
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("projects[%d] = %q, want %q", i, got[i], want[i])
			}
		}
	}
}

// TestHelpNeedsNoCredentials closes the trap in config.go:151 — Validate aborts
// on an empty PAT, so help must be resolved before config is loaded.
func TestHelpNeedsNoCredentials(t *testing.T) {
	t.Setenv("ADO_PAT", "")
	t.Setenv("ADO_ORG_URL", "")
	t.Setenv("HOME", t.TempDir()) // no config file exists under a fresh home
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	tests := []struct {
		name string
		args []string
		want string
	}{
		{"root", []string{"--help"}, "Nouns:"},
		{"noun", []string{"prs", "--help"}, "Verbs:"},
		{"verb", []string{"prs", "list", "--help"}, "-json"},
		{"short form", []string{"-h"}, "Nouns:"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr := captureOutput(t)

			code, handled := handleUsage(tt.args)
			if !handled {
				t.Fatalf("handleUsage(%q) reported unhandled, want handled", tt.args)
			}
			if code != exitOK {
				t.Errorf("handleUsage(%q) = %d, want %d", tt.args, code, exitOK)
			}
			if !strings.Contains(stderr.String(), tt.want) {
				t.Errorf("help output %q does not contain %q", stderr.String(), tt.want)
			}
			if stdout.Len() != 0 {
				t.Errorf("help wrote %q to stdout; OUT-03 reserves stdout for parseable output", stdout.String())
			}
		})
	}
}

// TestHelpIsDeterministic pins byte-identical help across invocations, so an
// agent diffing two runs learns only about real changes.
func TestHelpIsDeterministic(t *testing.T) {
	runOnce := func(args []string) []byte {
		_, stderr := captureOutput(t)
		if _, handled := handleUsage(args); !handled {
			t.Fatalf("handleUsage(%q) reported unhandled", args)
		}
		return append([]byte(nil), stderr.Bytes()...)
	}

	for _, args := range [][]string{{"--help"}, {"pipelines", "--help"}, {"prs", "list", "--help"}} {
		first, second := runOnce(args), runOnce(args)
		if !bytes.Equal(first, second) {
			t.Errorf("help for %q differed across runs:\nfirst:  %q\nsecond: %q", args, first, second)
		}
	}
}

// TestUsageErrorsExitThree pins D-09 and D-22: nothing ran, so the outcome is
// unknown, and stdout stays untouched.
func TestUsageErrorsExitThree(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"unknown noun", []string{"widgets"}},
		{"unknown verb", []string{"prs", "nonsense"}},
		{"bare noun", []string{"prs"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr := captureOutput(t)

			code, handled := handleUsage(tt.args)
			if !handled {
				t.Fatalf("handleUsage(%q) reported unhandled, want handled", tt.args)
			}
			if code != exitUnknown {
				t.Errorf("handleUsage(%q) = %d, want %d", tt.args, code, exitUnknown)
			}
			if stdout.Len() != 0 {
				t.Errorf("wrote %q to stdout, want nothing", stdout.String())
			}
			if stderr.Len() == 0 {
				t.Errorf("wrote nothing to stderr, want usage")
			}
		})
	}
}

// TestBadFlagExitsThreeWithCleanStdout pins the flag-parse error path. Stdlib's
// default would print usage and exit 2, which is now `auth failure`.
func TestBadFlagExitsThreeWithCleanStdout(t *testing.T) {
	stdout, stderr := captureOutput(t)

	// The fake has no closures set: reaching a read fails the test, proving the
	// parse error short-circuits before any Azure DevOps call.
	code := dispatch(context.Background(), &fakeReadClient{t: t}, []string{"prs", "list", "--nonsense"})

	if code != exitUnknown {
		t.Errorf("dispatch(prs list --nonsense) = %d, want %d", code, exitUnknown)
	}
	if stdout.Len() != 0 {
		t.Errorf("wrote %q to stdout, want nothing", stdout.String())
	}
	if !strings.Contains(stderr.String(), "Usage: adoBizInt prs list") {
		t.Errorf("stderr = %q, want the verb usage", stderr.String())
	}
}

// TestHandleUsagePassesValidSubcommandsThrough proves a real read request is
// not swallowed by the usage layer.
func TestHandleUsagePassesValidSubcommandsThrough(t *testing.T) {
	for _, key := range wantCommands {
		args := append(strings.Split(key, " "), "--json")
		if _, handled := handleUsage(args); handled {
			t.Errorf("handleUsage(%q) reported handled, want passthrough to dispatch", args)
		}
	}
}
