package ado

import "strings"

// doneStates is the single definition of "done" in this repository. It moved
// here out of internal/ui/workitems so the CLI and the TUI cannot drift on the
// rule (CLI-06). A second copy in a view package is the duplication D-17 exists
// to remove.
var doneStates = map[string]bool{
	"Done":     true,
	"Closed":   true,
	"Resolved": true,
	"Removed":  true,
}

// IsDoneState reports whether a work item state counts as done or resolved.
// The comparison is exact-match on the state string, not a case-folded or
// prefix match — a process template with a "Done (Deferred)" state is a
// different state, and silently folding it into Done would hide items a caller
// asked to see. Implements D-17.
func IsDoneState(state string) bool {
	return doneStates[state]
}

// IsTaskType reports whether a work item is a Task. Exact-match for the same
// reason as IsDoneState: "Task" and "Testing Task" are different types, and
// prefix matching would hide the second. Implements D-17.
func IsTaskType(itemType string) bool {
	return itemType == "Task"
}

// MatchesIdentity reports whether an item assigned to email belongs to
// identity. An empty identity keeps everything rather than matching nothing:
// identity resolution is deliberately non-fatal (Phase 1 D-12), so an empty
// cache is a legitimate state, and a match-nothing rule would render an empty
// view that is indistinguishable from an empty backlog.
//
// Comparison uses strings.EqualFold rather than lowercasing both sides and
// comparing, preserving the existing call-site semantics exactly. Implements
// D-17 and D-18.
func MatchesIdentity(email, identity string) bool {
	if identity == "" {
		return true
	}
	return strings.EqualFold(email, identity)
}

// MatchesProject reports whether project is in the configured set. An empty or
// nil slice disables the filter rather than matching nothing — the rejected
// alternative, treating an empty set as "match none", would silently empty the
// view for any user with no projects configured, and would make a CLI read with
// no --project flag return zero rows. It is also what gives D-18's
// "the CLI defaults to no filtering" a free implementation: the CLI passes an
// empty slice. Implements D-17 and D-18.
func MatchesProject(project string, projects []string) bool {
	return matchesAny(project, projects)
}

// MatchesRepository reports whether repository is in the configured set, with
// the same empty-means-all rule as MatchesProject. This predicate replaces
// three byte-identical copies: the inline block in GetBranches, the one in the
// pull requests view, and the CLI's. Implements D-17 and D-18.
func MatchesRepository(repository string, repositories []string) bool {
	return matchesAny(repository, repositories)
}

// matchesAny is the shared empty-means-all, case-insensitive set membership
// test behind MatchesProject and MatchesRepository. It is unexported because
// the two named predicates are the vocabulary callers should read; a single
// generic MatchesSet would let a call site filter repositories against a
// project list with no compile error.
func matchesAny(value string, set []string) bool {
	if len(set) == 0 {
		return true
	}
	for _, s := range set {
		if strings.EqualFold(value, s) {
			return true
		}
	}
	return false
}

// MatchesSearch reports whether any of fields contains the free-text query.
//
// RED stub — implemented in the GREEN commit.
func MatchesSearch(query string, fields ...string) bool {
	return false
}
