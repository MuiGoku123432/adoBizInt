package main

import (
	"flag"
	"fmt"
	"sort"
	"strings"
)

// helpTokens are the spellings that request usage rather than a read.
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
	fmt.Fprintf(stderrWriter, "\nRun 'adoBizInt <noun> --help' for a noun's verbs.\n")
}

// usageNoun lists one noun's verbs (D-21 level two). A bare noun failing with
// no guidance is the dead end D-21 rejected two-level help to avoid.
func usageNoun(noun string) {
	_, byNoun := nounVerbs()

	fmt.Fprintf(stderrWriter, "Usage: adoBizInt %s <verb> [flags]\n", noun)
	fmt.Fprintf(stderrWriter, "\nVerbs:\n")
	for _, verb := range byNoun[noun] {
		fmt.Fprintf(stderrWriter, "  %s\n", verb)
	}
	fmt.Fprintf(stderrWriter, "\nRun 'adoBizInt %s <verb> --help' for a verb's flags.\n", noun)
}

// usageVerb prints one subcommand's flag list (D-21 level three). It renders
// the FlagSet's own defaults, so a flag added to the shared registrar appears
// here with no second edit.
func usageVerb(name string, fs *flag.FlagSet) {
	fmt.Fprintf(stderrWriter, "Usage: adoBizInt %s [flags]\n", name)
	fmt.Fprintf(stderrWriter, "\nFlags:\n")
	fs.PrintDefaults()
}

// handleUsage resolves every request that must not reach Azure DevOps: help at
// any of the three levels, an unknown noun, an unknown verb, and a bare noun.
//
// It runs before config.Load and cfg.Validate in run(). config.go:151 aborts
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

	// A valid subcommand: verb-level help is handled here rather than by the
	// FlagSet, so it too runs before config is loaded.
	for _, arg := range args[2:] {
		if helpTokens[arg] {
			fs, _, _ := newFlagSet(noun + " " + verb)
			usageVerb(noun+" "+verb, fs)
			return exitOK, true
		}
	}

	return exitOK, false
}
