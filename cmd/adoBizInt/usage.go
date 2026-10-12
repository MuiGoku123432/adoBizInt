package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"
)

// helpTokens are the spellings that request usage when they stand as the noun
// or the verb. Inside a verb's own arguments stdlib flag decides instead, so a
// help token there is help only in flag position (D-07).
var helpTokens = map[string]bool{
	"--help": true,
	"-help":  true,
	"-h":     true,
	"help":   true,
}

// nounVerbs derives the noun tree from the dispatch table, so a verb registered
// in exactly one place is discoverable in help without a second list to keep in
// sync. Both the noun order and each noun's verb order are sorted, so help
// output is byte-identical across runs.
func nounVerbs() ([]string, map[string][]string) {
	byNoun := map[string][]string{}
	for _, key := range commandKeys() {
		noun, verb, _ := strings.Cut(key, " ")
		byNoun[noun] = append(byNoun[noun], verb)
	}
	nouns := make([]string, 0, len(byNoun))
	for noun := range byNoun {
		sort.Strings(byNoun[noun])
		nouns = append(nouns, noun)
	}
	sort.Strings(nouns)
	return nouns, byNoun
}

// verbSummaries is the one-line description shown beside each verb at the noun
// level. Every write verb's text states the dry-run default explicitly, because
// D-06 means there is no prompt to discover it at runtime and D-04 means
// --confirm is the entire mechanism: an agent that reads help and not source
// must learn from this line alone that the verb previews by default.
//
// A verb missing from this map renders with no description rather than failing,
// so adding a read verb cannot break help. TestEveryWriteVerbAppearsInNounUsage
// enforces the stricter rule for writes.
var verbSummaries = map[string]string{
	"branches list":         "list branches",
	"dashboard counts":      "summary counts across the configured projects",
	"pipelines definitions": "list pipeline definitions",
	"pipelines runs":        "list recent pipeline runs",
	"prs list":              "list pull requests",
	"releases approvals":    "list pending release approvals",
	"releases list":         "list releases",
	"workitems list":        "list work items",

	"workitems set-state": "set a work item's state; previews the diff by default, writes only with --confirm",
	"prs approve":         "approve a pull request; previews the diff by default, writes only with --confirm",
	"prs complete":        "complete (merge) a pull request; previews the diff by default, writes only with --confirm",
	"releases approve":    "approve a pending release approval; previews the diff by default, writes only with --confirm",
	"releases reject":     "reject a pending release approval; previews the diff by default, writes only with --confirm",
	"pipelines queue":     "queue a pipeline run on --branch; previews by default, queues only with --confirm",
}

// Usage text is a diagnostic and goes to stderr at every level, never to
// stdout. OUT-03 makes stdout the parseable channel: a caller piping
// `adoBizInt prs list --json` into a parser must not have to distinguish a
// document from a help page, and that holds even when help was what was asked
// for.

// usageRoot lists the nouns (D-21 level one).
func usageRoot() {
	nouns, byNoun := nounVerbs()

	fmt.Fprintf(stderrWriter, "Usage: adoBizInt <noun> <verb> [flags]\n")
	fmt.Fprintf(stderrWriter, "\nRun with no arguments to start the interactive TUI.\n")
	fmt.Fprintf(stderrWriter, "\nNouns:\n")
	for _, noun := range nouns {
		fmt.Fprintf(stderrWriter, "  %-12s %s\n", noun, strings.Join(byNoun[noun], ", "))
	}

	// Every write verb previews by default. Saying so once at the root means an
	// agent scanning the top-level page learns it without opening a verb.
	fmt.Fprintf(stderrWriter, "\nWrite verbs report the change they would make and do nothing;\n")
	fmt.Fprintf(stderrWriter, "pass --confirm to actually perform the write.\n")

	// The exit-code contract (D-06, D-08, D-09). These integers are published:
	// agents and CI jobs branch on them.
	fmt.Fprintf(stderrWriter, "\nExit codes:\n")
	fmt.Fprintf(stderrWriter, "  0  the command completed and nothing failed\n")
	fmt.Fprintf(stderrWriter, "  1  a read did not reach every configured project, or Azure DevOps refused a write\n")
	fmt.Fprintf(stderrWriter, "  2  at least one failure was an authentication rejection\n")
	fmt.Fprintf(stderrWriter, "  3  nothing ran: usage error, config error, client error\n")
	fmt.Fprintf(stderrWriter, "  4  already in the requested state, nothing written\n")
	fmt.Fprintf(stderrWriter, "  5  the write was issued and its outcome could not be established\n")

	fmt.Fprintf(stderrWriter, "\nRun 'adoBizInt <noun> --help' for a noun's verbs.\n")
}

// usageNoun lists one noun's verbs (D-21 level two). A bare noun failing with
// no guidance is the dead end D-21 rejected two-level help to avoid.
func usageNoun(noun string) {
	_, byNoun := nounVerbs()

	fmt.Fprintf(stderrWriter, "Usage: adoBizInt %s <verb> [flags]\n", noun)
	fmt.Fprintf(stderrWriter, "\nVerbs:\n")
	for _, verb := range byNoun[noun] {
		fmt.Fprintf(stderrWriter, "  %-12s %s\n", verb, verbSummaries[noun+" "+verb])
	}
	fmt.Fprintf(stderrWriter, "\nRun 'adoBizInt %s <verb> --help' for a verb's flags.\n", noun)
}

// usageVerb prints one subcommand's flag list (D-21 level three). It renders
// the FlagSet's own defaults, so a flag added to the shared registrar appears
// here with no second edit.
//
// The write notice is printed in longhand rather than left to PrintDefaults,
// which renders single-dash spellings: an agent copying `-confirm` from help
// would be relying on stdlib's tolerance rather than on the documented surface.
func usageVerb(name string, fs *flag.FlagSet) {
	fmt.Fprintf(stderrWriter, "Usage: adoBizInt %s [flags]\n", name)
	if summary := verbSummaries[name]; summary != "" {
		fmt.Fprintf(stderrWriter, "\n%s\n", summary)
	}
	if writeSupportByCommand[name].confirm {
		fmt.Fprintf(stderrWriter, "\nThis verb previews by default: it reports the change it would make,\n")
		fmt.Fprintf(stderrWriter, "writes nothing, and exits 0. Pass --confirm to perform the write.\n")
		fmt.Fprintf(stderrWriter, "Exits 4 if the change is already applied, 5 if the outcome is unknown.\n")
	}
	fmt.Fprintf(stderrWriter, "\nFlags:\n")
	fs.PrintDefaults()
}

// handleUsage resolves every request that must not reach Azure DevOps: help at
// any of the three levels, an unknown noun, an unknown verb, and a bare noun.
//
// Verb-level help is a help flag in flag position, decided by stdlib parsing,
// so a help token used as a flag's value is the value (D-07, D-08):
// `workitems list --search help` is a search, not a help request.
//
// It still runs before config.Load and cfg.Validate in run() (D-09). config.go:151 aborts
// validation when PAT is empty, so `adoBizInt --help` failing because no
// credential is configured would be hostile — and it is exactly the trap a
// first-time user hits.
//
// The returned bool reports whether the arguments were fully handled; when it
// is false the caller proceeds to load config and dispatch.
func handleUsage(args []string) (int, bool) {
	if len(args) == 0 {
		return exitOK, false
	}

	if helpTokens[args[0]] {
		usageRoot()
		return exitOK, true
	}

	_, byNoun := nounVerbs()
	noun := args[0]
	if _, ok := byNoun[noun]; !ok {
		usageRoot()
		return exitUnknown, true
	}

	// A noun with no verb. Asking for help succeeds; omitting the verb is a
	// usage error, which is exit 3 because nothing ran (D-09, D-22).
	if len(args) == 1 {
		usageNoun(noun)
		return exitUnknown, true
	}

	verb := args[1]
	if helpTokens[verb] {
		usageNoun(noun)
		return exitOK, true
	}

	if _, ok := commands[noun+" "+verb]; !ok {
		usageNoun(noun)
		return exitUnknown, true
	}

	// A valid subcommand. stdlib flag decides flag position versus flag value
	// over the verb's own FlagSet, so a help token consumed as a flag's value is
	// that value (D-07, D-08). This runs before config.Load, so verb help still
	// needs no config or PAT (D-09).
	//
	// The probe applies no value: the --project allow-list is empty before
	// config loads and would reject every value, and the real parse in dispatch
	// enforces it. Usage is printed from a fresh FlagSet, because PrintDefaults
	// over wrapped values loses the type names and emits "panic calling String"
	// lines (RESEARCH Pitfall 3). hoistLeadingPositionals is applied so a target
	// written before the flags does not stop the parse (RESEARCH Pitfall 2).
	name := noun + " " + verb
	probe, _, _, _ := newFlagSet(name)
	probe.SetOutput(io.Discard)
	probe.Usage = func() {}
	probe.VisitAll(func(f *flag.Flag) { f.Value = helpProbeValue{f.Value} })
	if err := probe.Parse(hoistLeadingPositionals(args[2:])); errors.Is(err, flag.ErrHelp) {
		fs, _, _, _ := newFlagSet(name)
		usageVerb(name, fs)
		return exitOK, true
	}

	// Any other outcome, including a parse error, is dispatch's to report
	// after config loads.
	return exitOK, false
}

// helpProbeValue makes a flag value-inert for handleUsage's help probe: Set
// records nothing, so the probe neither applies nor validates a value. It is
// single-use, private to this file, and the only flag.Value wrapper in the
// package.
type helpProbeValue struct{ flag.Value }

func (helpProbeValue) Set(string) error { return nil }

// IsBoolFlag forwards the wrapped flag's arity. Without it every flag would
// take a value, and `--json --help` would read --help as --json's value.
func (v helpProbeValue) IsBoolFlag() bool {
	b, ok := v.Value.(interface{ IsBoolFlag() bool })
	return ok && b.IsBoolFlag()
}
