package main

import (
	"flag"
	"strings"
	"time"
)

// globalFlags holds the three flags every read command accepts (D-20). The
// values are carried in one struct so a command body reads them by name rather
// than juggling pointers returned by eight separate registrations.
type globalFlags struct {
	asJSON   bool
	timeout  time.Duration
	projects stringList
}

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
	// 04-01 is the tracer: one verb, state only. 04-03 expands this map to the
	// remaining six verbs and turns on effort, comment, branch and repo.
	"workitems set-state": {confirm: true, state: true, effort: true},
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
	fs.Var(&gf.projects, "project",
		"restrict the read to a project; repeatable or comma-separated")

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
			"drop items in a done or resolved state (off by default)")
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

// hasEffort reports whether the caller named any effort field.
func (wf *writeFlags) hasEffort() bool {
	return wf.originalEstimate != nil || wf.remaining != nil || wf.completed != nil
}
