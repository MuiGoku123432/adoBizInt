package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"sentinovo.ai/bizInt/internal/ado"
)

// TestFlagsRegisteredOnEverySubcommand pins D-20: the shared registrar is what
// prevents `prs list` accepting --timeout while `releases approvals` silently
// does not. Every verb is run with all three global flags set.
func TestFlagsRegisteredOnEverySubcommand(t *testing.T) {
	withConfiguredProjects(t, "alpha", "beta", "gamma")
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
	withConfiguredProjects(t, "alpha", "beta", "gamma")
	_, _ = captureOutput(t)

	fs, gf, _, _ := newFlagSet("prs list")
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

// TestHelpTokenAsFlagValueIsNotHelp pins D-07, D-08 and D-09 (audit W2): a help
// token that stdlib flag reads as a flag's value is that value, so the request
// it belongs to runs rather than being silently replaced by help.
func TestHelpTokenAsFlagValueIsNotHelp(t *testing.T) {
	withConfiguredProjects(t) // no names: the pre-config state handleUsage runs in

	for _, args := range [][]string{
		{"workitems", "list", "--search", "help"},
		{"workitems", "list", "--search", "-h"},
		{"workitems", "list", "--search", "--help"},
		{"workitems", "list", "--search=-help"},
		{"workitems", "list", "--search", "-help", "--json"},
		{"workitems", "list", "--json", "help"},
		{"workitems", "list", "help"},
		{"workitems", "list", "--HELP"},
		{"workitems", "list", "--search"},
		{"workitems", "list"},
		{"releases", "approve", "77", "--project", "P", "--comment", "-h", "--confirm"},
		{"pipelines", "queue", "7", "--project", "P", "--branch", "-h"},
		{"workitems", "list", "--project", "nope", "--search", "help"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			stdout, stderr := captureOutput(t)
			if _, handled := handleUsage(args); handled {
				t.Errorf("handleUsage(%q) reported handled, want passthrough to dispatch", args)
			}
			if stdout.Len() != 0 || stderr.Len() != 0 {
				t.Errorf("handleUsage(%q) wrote stdout %q and stderr %q, want nothing: the probe is silent",
					args, stdout.String(), stderr.String())
			}
		})
	}

	t.Run("search help runs the read", func(t *testing.T) {
		withConfiguredProjects(t, "one")
		stdout, stderr := captureOutput(t)

		calls := 0
		c := &fakeReadClient{t: t,
			GetWorkItemsFunc: func(context.Context, []string) (ado.Result[ado.WorkItem], error) {
				calls++
				return ado.Result[ado.WorkItem]{Items: []ado.WorkItem{
					{ID: 7, Title: "help desk outage", Project: "one", State: "Active"},
					{ID: 8, Title: "Beta task", Project: "one", State: "Active"},
				}, Failures: []ado.Failure{}, Complete: true}, nil
			}}

		args := []string{"workitems", "list", "--search", "help", "--json"}
		if _, handled := handleUsage(args); handled {
			t.Fatalf("handleUsage(%q) reported handled; the read never ran (stderr: %q)", args, stderr.String())
		}
		if code := dispatch(context.Background(), c, args); code != exitOK {
			t.Fatalf("dispatch(%q) = %d, want %d (stderr: %q)", args, code, exitOK, stderr.String())
		}
		if got := decodeWorkItemIDs(t, stdout); !equalInts(got, []int{7}) {
			t.Errorf("emitted IDs = %v, want [7]", got)
		}
		if calls != 1 {
			t.Errorf("GetWorkItems called %d times, want exactly 1", calls)
		}
	})

	t.Run("search help with no match still runs the read", func(t *testing.T) {
		withConfiguredProjects(t, "one")
		stdout, stderr := captureOutput(t)

		calls := 0
		c := &fakeReadClient{t: t,
			GetWorkItemsFunc: func(context.Context, []string) (ado.Result[ado.WorkItem], error) {
				calls++
				return ado.Result[ado.WorkItem]{Items: []ado.WorkItem{
					{ID: 8, Title: "Beta task", Project: "one", State: "Active"},
				}, Failures: []ado.Failure{}, Complete: true}, nil
			}}

		args := []string{"workitems", "list", "--search", "help"}
		if _, handled := handleUsage(args); handled {
			t.Fatalf("handleUsage(%q) reported handled; the read never ran", args)
		}
		if code := dispatch(context.Background(), c, args); code != exitOK {
			t.Fatalf("dispatch(%q) = %d, want %d (stderr: %q)", args, code, exitOK, stderr.String())
		}
		if stdout.Len() != 0 {
			t.Errorf("stdout = %q, want nothing for a search with no match", stdout.String())
		}
		if calls != 1 {
			t.Errorf("GetWorkItems called %d times, want exactly 1", calls)
		}
	})

	t.Run("unconfigured project is still refused", func(t *testing.T) {
		withConfiguredProjects(t, "one")
		stdout, stderr := captureOutput(t)

		args := []string{"workitems", "list", "--project", "nope", "--search", "help"}
		if _, handled := handleUsage(args); handled {
			t.Fatalf("handleUsage(%q) reported handled, want passthrough to dispatch", args)
		}
		// No closure set: reaching a read fails the subtest.
		if code := dispatch(context.Background(), &fakeReadClient{t: t}, args); code != exitUnknown {
			t.Errorf("dispatch(%q) = %d, want %d", args, code, exitUnknown)
		}
		if !strings.Contains(stderr.String(), "not a configured project") {
			t.Errorf("stderr = %q, want the allow-list refusal", stderr.String())
		}
		if stdout.Len() != 0 {
			t.Errorf("stdout = %q, want nothing", stdout.String())
		}
	})

	t.Run("--HELP is an unknown flag", func(t *testing.T) {
		stdout, _ := captureOutput(t)

		args := []string{"workitems", "list", "--HELP"}
		if code := dispatch(context.Background(), &fakeReadClient{t: t}, args); code != exitUnknown {
			t.Errorf("dispatch(%q) = %d, want %d", args, code, exitUnknown)
		}
		if stdout.Len() != 0 {
			t.Errorf("stdout = %q, want nothing", stdout.String())
		}
	})
}

// TestHelpTokenInFlagPositionIsHelp pins D-07 and D-09 (audit W2): a help token
// in flag position still prints verb help with no config loaded, and the help
// is rendered from a real FlagSet, so typed defaults survive.
func TestHelpTokenInFlagPositionIsHelp(t *testing.T) {
	withConfiguredProjects(t) // no names: the pre-config state handleUsage runs in

	for _, args := range [][]string{
		{"workitems", "list", "--project", "P", "--help"},
		{"workitems", "list", "--search", "x", "-h"},
		{"prs", "list", "--json", "-help"},
		{"prs", "list", "--help=x"},
		{"workitems", "set-state", "4821", "--help"},
		{"workitems", "set-state", "--help", "4821"},
		{"releases", "approve", "77", "--project", "P", "-h"},
		{"prs", "list", "foo", "--help"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			stdout, stderr := captureOutput(t)

			code, handled := handleUsage(args)
			if !handled {
				t.Fatalf("handleUsage(%q) reported unhandled, want verb help", args)
			}
			if code != exitOK {
				t.Errorf("handleUsage(%q) = %d, want %d", args, code, exitOK)
			}
			if stdout.Len() != 0 {
				t.Errorf("help wrote %q to stdout, want nothing", stdout.String())
			}
			out := stderr.String()
			for _, want := range []string{"Usage: adoBizInt", "-json", "-timeout duration"} {
				if !strings.Contains(out, want) {
					t.Errorf("help for %q does not contain %q:\n%s", args, want, out)
				}
			}
			if strings.Contains(out, "panic calling String") {
				t.Errorf("help for %q was printed from the value-inert probe:\n%s", args, out)
			}
		})
	}
}

// TestEveryWriteVerbAppearsInNounUsage iterates writeSupportByCommand — the same
// map the dispatcher gates flag registration on — rather than a hand-maintained
// list of verb names. A write verb added to the dispatch table without being
// added to help fails here with no second list to remember to update.
func TestEveryWriteVerbAppearsInNounUsage(t *testing.T) {
	for key := range writeSupportByCommand {
		t.Run(key, func(t *testing.T) {
			noun, verb, found := strings.Cut(key, " ")
			if !found {
				t.Fatalf("write command key %q is not a `noun verb` pair", key)
			}

			_, stderr := captureOutput(t)
			usageNoun(noun)

			if !strings.Contains(stderr.String(), verb) {
				t.Errorf("`adoBizInt %s --help` does not list the verb %q:\n%s", noun, verb, stderr.String())
			}
			// The dry-run default is not discoverable at runtime: D-06 means
			// there is no prompt and D-04 means --confirm is the whole
			// mechanism, so the noun listing has to say so.
			if !strings.Contains(stderr.String(), "--confirm") {
				t.Errorf("noun usage for %q never mentions --confirm:\n%s", noun, stderr.String())
			}
		})
	}
}

// TestWriteVerbUsageDocumentsConfirm pins the verb level: a caller reading
// `adoBizInt prs approve --help` must find the double-dash spelling, not only
// stdlib PrintDefaults' single-dash rendering.
func TestWriteVerbUsageDocumentsConfirm(t *testing.T) {
	for key := range writeSupportByCommand {
		t.Run(key, func(t *testing.T) {
			_, stderr := captureOutput(t)

			fs, _, _, _ := newFlagSet(key)
			usageVerb(key, fs)

			if !strings.Contains(stderr.String(), "--confirm") {
				t.Errorf("verb usage for %q does not contain `--confirm`:\n%s", key, stderr.String())
			}
		})
	}
}

// TestRootUsageDocumentsTheWriteExitCodes pins D-08 and D-09's integers into the
// published help. They are a one-way door; help is where a caller learns them.
func TestRootUsageDocumentsTheWriteExitCodes(t *testing.T) {
	_, stderr := captureOutput(t)
	usageRoot()

	out := stderr.String()
	if !strings.Contains(out, "Exit codes:") {
		t.Fatalf("root usage has no exit-code section:\n%s", out)
	}
	for _, want := range []string{
		"  4  already in the requested state, nothing written",
		"  5  the write was issued and its outcome could not be established",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("root usage does not document %q:\n%s", want, out)
		}
	}
}
