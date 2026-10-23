package main

import (
	"flag"
	"fmt"
	"slices"
	"strings"
	"time"

	"sentinovo.ai/bizInt/internal/ado"
)

// globalFlags holds the three flags every read command accepts (D-20). The
// values are carried in one struct so a command body reads them by name rather
// than juggling pointers returned by eight separate registrations.
type globalFlags struct {
	asJSON   bool
	timeout  time.Duration
	projects stringList
	team     string // scrum verbs only (scrumCommands)
	at       string // sprint items only: ado.SnapshotLive, SnapshotBaseline or SnapshotClose
}

// scrumCommands are the verbs that run against configured teams. They alone
// register --team and run selectTeams before reading (TEAM-01, TEAM-04).
var scrumCommands = map[string]bool{
	"sprint current": true,
	"sprint items":   true,
	"teams members":  true,
	"teams process":  true,
}

// configuredProjects is the D-09 allow-list every --project value is checked
// against. run() assigns it from cfg.Projects only AFTER config validation
// returns, exactly as it does readBudget, so the list the check trusts is the
// one the validator accepted.
//
// The zero value is the fail-closed one: nil or empty rejects every supplied
// --project value. An unset list must never mean "allow anything". Tests set it
// through withConfiguredProjects.
//
// Matching is exact and case-sensitive (RESEARCH assumption A2). ADO itself
// treats project names case-insensitively, but a near-miss must exit 3 rather
// than land a read or a write on a project the caller did not name exactly.
var configuredProjects []string

// filterFlags holds the per-verb filter flags. Unlike globalFlags these are
// registered only on the verbs whose payload supports them, because a flag that
// parses but cannot mean anything is worse than a flag that does not exist: an
// agent reading --help would believe `releases list --hide-done` filters.
//
// Every field's zero value is "off" (CLI-06, D-18). This is not incidental — a
// read verb must never apply a filter the caller did not request. The work
// items TUI defaults assigned-to-me and hide-done to on because that is the
// useful human view, but a CLI that hides data the caller did not ask to hide
// hands an agent a filtered subset carrying complete:true, which is exactly the
// failure this project exists to prevent.
//
// identity is not a flag. It is resolved from the client's cached identity by
// the command closure and carried here so the post-filter has one input struct.
type filterFlags struct {
	search       string
	assignedToMe bool
	hideDone     bool
	hideTasks    bool
	repos        stringList
	identity     string
}

// filterSupport records which filter flags a verb registers. It is keyed on the
// same `noun verb` string as the dispatch table so the two cannot disagree
// about which verbs exist, and a verb absent from this map simply registers no
// filter flags.
//
// Deliberately not read from config: the same command must return the same data
// on two machines, and config-driven filter defaults are hostile to agent
// reproducibility.
type filterSupport struct {
	search       bool
	assignedToMe bool
	hideDone     bool
	hideTasks    bool
	repo         bool
}

var filterSupportByCommand = map[string]filterSupport{
	"workitems list": {search: true, assignedToMe: true, hideDone: true, hideTasks: true},
	"prs list":       {search: true, repo: true},
	"branches list":  {search: true, repo: true},
	"releases list":  {search: true},
	"pipelines runs": {search: true},
}

// writeSupport records which write flags a verb registers. It mirrors
// filterSupport exactly: keyed on the same `noun verb` string as the dispatch
// table so the two cannot disagree about which verbs exist, and a verb absent
// from this map registers no write flags at all.
//
// Membership of this map is also the definition of "is a write verb", which is
// what makes --confirm's presence a structural property rather than a
// convention: TestConfirmRegisteredOnEveryWriteVerbAndNoReadVerb asserts the
// two agree for every registered command.
//
// --confirm is deliberately NOT a global flag. flags.go's own rule (see
// filterFlags) is that a flag which parses but cannot mean anything is worse
// than one that does not exist: an agent reading `prs list --help` must not
// find a --confirm that cannot confirm anything.
type writeSupport struct {
	confirm bool
	state   bool
	effort  bool
	comment bool
	branch  bool
	repo    bool
}

var writeSupportByCommand = map[string]writeSupport{
	// Each entry declares exactly the flags its payload can honour. A flag that
	// parses but cannot mean anything is worse than one that does not exist, so
	// `prs approve --state Closed` is a parse error rather than a silently
	// ignored instruction (threat T-04-15).
	"workitems set-state": {confirm: true, state: true, effort: true},
	"prs approve":         {confirm: true, repo: true},
	"prs complete":        {confirm: true, repo: true},
	"releases approve":    {confirm: true, comment: true},
	"releases reject":     {confirm: true, comment: true},
	"pipelines queue":     {confirm: true, branch: true},
}

// writeFlags holds the per-verb write flags, exactly as filterFlags does for
// reads.
//
// The three effort values are pointers so "omitted" and "zero" stay
// distinguishable all the way to the JSON-Patch builder (D-17, D-18). Never
// give them a float64 + omitempty shape anywhere between the flag and the
// patch document: that silently drops `--remaining 0`, which is the inverse
// bug and the one a closing Task actually needs.
type writeFlags struct {
	confirm bool
	state   string

	originalEstimate *float64
	remaining        *float64
	completed        *float64

	// The plain float64 destinations the flag package writes into. They are
	// meaningless on their own — a zero here is indistinguishable from an
	// omitted flag, which is the entire bug. promoteEffort turns the ones the
	// caller actually set into the pointers above.
	originalEstimateVal float64
	remainingVal        float64
	completedVal        float64

	comment string
	branch  string
	repo    stringList
}

// defaultTimeout is the zero value meaning "no CLI-imposed deadline": the read
// then runs under the configured fetch budget. A non-zero value replaces that
// budget for this invocation only, and the replacement lands after config
// validation, because the validator rejects a fetch budget at or below the poll
// interval and a CLI flag must not trip a config-file check (D-P6).
const defaultTimeout time.Duration = 0

// stringList collects a repeatable, also comma-separated, flag value. It backs
// both --project and --repo. Both spellings are accepted because an agent
// composing a command line and a human typing one reach for different ones, and
// rejecting either would be a gratuitous failure mode.
type stringList []string

func (p *stringList) String() string { return strings.Join(*p, ",") }

func (p *stringList) Set(value string) error {
	for _, part := range strings.Split(value, ",") {
		if part = strings.TrimSpace(part); part != "" {
			*p = append(*p, part)
		}
	}
	return nil
}

// newFlagSet builds the FlagSet for one subcommand and registers the global
// flags into it, plus whichever filter flags that verb supports. Registering
// through a single function is the whole point of D-20: stdlib gives no
// compile-time check against eight independent declarations, and the drift it
// prevents is `prs list` accepting --timeout while `releases approvals`
// silently does not.
//
// ContinueOnError rather than the default ExitOnError (D-22): stdlib's default
// prints usage and exits 2, which is now `auth failure` and would be a lie.
//
// SetOutput is called explicitly rather than left to the default so a future
// change cannot silently redirect flag errors to stdout and break OUT-03. This
// is RESEARCH assumption A2 closed by construction and pinned by a test rather
// than trusted.
func newFlagSet(name string) (*flag.FlagSet, *globalFlags, *filterFlags, *writeFlags) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderrWriter)

	gf := &globalFlags{}
	fs.BoolVar(&gf.asJSON, "json", false,
		"emit the raw result envelope as one JSON document on stdout")
	fs.DurationVar(&gf.timeout, "timeout", defaultTimeout,
		"deadline for the whole read, e.g. 30s; 0s means the built-in default budget")
	// The allow-list check lives in the registration itself, so every read and
	// write verb inherits it and it runs at the earliest moment the values
	// exist (D-09). A rejection makes fs.Parse fail, which runRead and runWrite
	// already map to exitUnknown: nothing ran. Fail closed across the whole
	// value: no part is kept unless every part is configured.
	fs.Func("project",
		"restrict the read to a project; repeatable or comma-separated",
		func(value string) error {
			var parts stringList
			_ = parts.Set(value) // never fails: it only splits and trims
			for _, part := range parts {
				if slices.Contains(configuredProjects, part) {
					continue
				}
				if len(configuredProjects) == 0 {
					return fmt.Errorf("not a configured project: %q (no projects are configured)", part)
				}
				return fmt.Errorf("not a configured project: %q (configured: %s)",
					part, strings.Join(configuredProjects, ", "))
			}
			gf.projects = append(gf.projects, parts...)
			return nil
		})

	if scrumCommands[name] {
		fs.StringVar(&gf.team, "team", "",
			"restrict the read to one configured team, written project/team; default is every configured team")
	}

	gf.at = ado.SnapshotLive
	if name == "sprint items" {
		// Validated as it parses, so a bad value exits 3 before any call.
		fs.Func("at", "which snapshot to read: live (default), baseline (current sprint start, 00:00 UTC) or close (last closed sprint, day after its finish, 00:00 UTC)",
			func(v string) error {
				if v != ado.SnapshotLive && v != ado.SnapshotBaseline && v != ado.SnapshotClose {
					return fmt.Errorf("--at must be live, baseline or close; got %q", v)
				}
				gf.at = v
				return nil
			})
	}

	ff := &filterFlags{}
	sup := filterSupportByCommand[name]
	if sup.search {
		fs.StringVar(&ff.search, "search", "",
			"keep only items matching this free-text query; empty disables the filter")
	}
	if sup.assignedToMe {
		fs.BoolVar(&ff.assignedToMe, "assigned-to-me", false,
			"keep only items assigned to the authenticated identity (off by default)")
	}
	if sup.hideDone {
		fs.BoolVar(&ff.hideDone, "hide-done", false,
			"also drop Resolved items client-side; Done, Closed and Removed are always excluded by Azure DevOps (off by default)")
	}
	if sup.hideTasks {
		fs.BoolVar(&ff.hideTasks, "hide-tasks", false,
			"drop work items of type Task (off by default)")
	}
	if sup.repo {
		fs.Var(&ff.repos, "repo",
			"restrict the read to a repository; repeatable or comma-separated")
	}

	wf := &writeFlags{}
	wsup := writeSupportByCommand[name]
	if wsup.confirm {
		fs.BoolVar(&wf.confirm, "confirm", false,
			"actually perform the write; without it the command reports the diff and changes nothing")
	}
	if wsup.state {
		fs.StringVar(&wf.state, "state", "",
			"the state to set; required, and never inferred from configuration")
	}

	if wsup.effort {
		fs.Float64Var(&wf.originalEstimateVal, "original-estimate", 0,
			"set the work item's original estimate in hours; omitted leaves it unchanged")
		fs.Float64Var(&wf.remainingVal, "remaining", 0,
			"set the work item's remaining work in hours; omitted leaves it unchanged, 0 sets it to zero")
		fs.Float64Var(&wf.completedVal, "completed", 0,
			"set the work item's completed work in hours; omitted leaves it unchanged")
	}

	if wsup.comment {
		fs.StringVar(&wf.comment, "comment", "",
			"comment recorded with the decision in Azure DevOps; optional, empty by default")
	}

	// No default value, and none is inferred from the definition: --branch is
	// the substance of a queue, so omitting it is a parse error (P-04).
	if wsup.branch {
		fs.StringVar(&wf.branch, "branch", "",
			"branch to queue the pipeline run on; required, never defaulted from the definition")
	}

	// The write --repo is a stringList only so it reuses the read path's
	// spelling and parsing; its arity is checked as exactly one, because
	// fanning a mutation across repositories is meaningless (D-24).
	if wsup.repo {
		fs.Var(&wf.repo, "repo",
			"the repository holding the pull request; exactly one value required")
	}

	fs.Usage = func() { usageVerb(name, fs) }

	return fs, gf, ff, wf
}

// promoteEffort turns the effort flags the caller ACTUALLY SET into non-nil
// pointers, leaving the rest nil. It must be called immediately after a
// successful fs.Parse and before anything reads the pointers.
//
// fs.Visit walks only the flags that were set, unlike fs.VisitAll which walks
// every registered flag. That distinction is the whole mechanism: it is why
// `--remaining 0` is a real zero rather than an omission, and why an omitted
// `--remaining` never reaches ADO as an operation at all. A value comparison
// cannot do this job — zero-because-asked and zero-by-default are the same
// float64 — and neither can a `float64` with `omitempty`, which drops the
// explicit zero that closing a task actually needs.
func (wf *writeFlags) promoteEffort(fs *flag.FlagSet) {
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })

	if set["original-estimate"] {
		wf.originalEstimate = &wf.originalEstimateVal
	}
	if set["remaining"] {
		wf.remaining = &wf.remainingVal
	}
	if set["completed"] {
		wf.completed = &wf.completedVal
	}
}
