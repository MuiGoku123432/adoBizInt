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
	projects projectList
}

// defaultTimeout is the zero value meaning "no CLI-imposed deadline"; plan
// 03-05 owns turning a non-zero value into a context deadline. The override
// must land after cfg.Validate(), because config.go:159 rejects a FetchTimeout
// at or below PollInterval and a CLI flag must not trip a config-file check.
const defaultTimeout time.Duration = 0

// projectList collects a repeatable, also comma-separated, --project flag.
// Both spellings are accepted because an agent composing a command line and a
// human typing one reach for different ones, and rejecting either would be a
// gratuitous failure mode.
type projectList []string

func (p *projectList) String() string { return strings.Join(*p, ",") }

func (p *projectList) Set(value string) error {
	for _, part := range strings.Split(value, ",") {
		if part = strings.TrimSpace(part); part != "" {
			*p = append(*p, part)
		}
	}
	return nil
}

// newFlagSet builds the FlagSet for one subcommand and registers the global
// flags into it. Registering through a single function is the whole point of
// D-20: stdlib gives no compile-time check against eight independent
// declarations, and the drift it prevents is `prs list` accepting --timeout
// while `releases approvals` silently does not.
//
// ContinueOnError rather than the default ExitOnError (D-22): stdlib's default
// prints usage and exits 2, which is now `auth failure` and would be a lie.
//
// SetOutput is called explicitly rather than left to the default so a future
// change cannot silently redirect flag errors to stdout and break OUT-03. This
// is RESEARCH assumption A2 closed by construction and pinned by a test rather
// than trusted.
func newFlagSet(name string) (*flag.FlagSet, *globalFlags) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderrWriter)

	gf := &globalFlags{}
	fs.BoolVar(&gf.asJSON, "json", false,
		"emit the raw result envelope as one JSON document on stdout")
	fs.DurationVar(&gf.timeout, "timeout", defaultTimeout,
		"deadline for the whole read, e.g. 30s (applied in a later plan)")
	fs.Var(&gf.projects, "project",
		"restrict the read to a project; repeatable or comma-separated")

	fs.Usage = func() { usageVerb(name, fs) }

	return fs, gf
}
