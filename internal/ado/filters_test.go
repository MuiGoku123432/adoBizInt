package ado

import (
	"strings"
	"testing"
)

func TestFilterIsDoneState(t *testing.T) {
	tests := []struct {
		name  string
		state string
		want  bool
	}{
		{"done is done", "Done", true},
		{"closed is done", "Closed", true},
		{"resolved is done", "Resolved", true},
		{"removed is done", "Removed", true},
		{"active is not done", "Active", false},
		{"new is not done", "New", false},
		{"empty state is not done", "", false},
		// The check is exact-match on the state string, as it is today.
		{"lowercase done is not matched", "done", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsDoneState(tt.state); got != tt.want {
				t.Errorf("IsDoneState(%q) = %v, want %v", tt.state, got, tt.want)
			}
		})
	}
}

func TestFilterIsTaskType(t *testing.T) {
	tests := []struct {
		name     string
		itemType string
		want     bool
	}{
		{"task is a task", "Task", true},
		{"bug is not a task", "Bug", false},
		{"user story is not a task", "User Story", false},
		{"empty type is not a task", "", false},
		{"lowercase task is not matched", "task", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsTaskType(tt.itemType); got != tt.want {
				t.Errorf("IsTaskType(%q) = %v, want %v", tt.itemType, got, tt.want)
			}
		})
	}
}

func TestFilterMatchesIdentity(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		identity string
		want     bool
	}{
		{"exact match kept", "a@example.com", "a@example.com", true},
		{"case-insensitive match kept", "A@Example.COM", "a@example.com", true},
		{"non-matching email dropped", "b@example.com", "a@example.com", false},
		// Identity resolution is non-fatal (Phase 1 D-12), so the cache can
		// legitimately be empty. An empty identity is a no-op, never a
		// match-nothing.
		{"empty identity keeps non-matching email", "b@example.com", "", true},
		{"empty identity keeps empty email", "", "", true},
		{"unassigned item dropped when identity is set", "", "a@example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchesIdentity(tt.email, tt.identity); got != tt.want {
				t.Errorf("MatchesIdentity(%q, %q) = %v, want %v", tt.email, tt.identity, got, tt.want)
			}
		})
	}
}

func TestFilterMatchesProject(t *testing.T) {
	tests := []struct {
		name     string
		project  string
		projects []string
		want     bool
	}{
		// An empty configured slice disables the filter (D-18). Without this,
		// a caller who requested no filter would receive nothing at all.
		{"nil slice keeps everything", "Alpha", nil, true},
		{"empty slice keeps an item a non-matching slice would drop", "Alpha", []string{}, true},
		{"exact match kept", "Alpha", []string{"Alpha", "Beta"}, true},
		{"case-insensitive match kept", "ALPHA", []string{"alpha"}, true},
		{"non-member dropped", "Gamma", []string{"Alpha", "Beta"}, false},
		{"empty project name dropped by a non-empty slice", "", []string{"Alpha"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchesProject(tt.project, tt.projects); got != tt.want {
				t.Errorf("MatchesProject(%q, %v) = %v, want %v", tt.project, tt.projects, got, tt.want)
			}
		})
	}
}

func TestFilterMatchesRepository(t *testing.T) {
	tests := []struct {
		name         string
		repo         string
		repositories []string
		want         bool
	}{
		{"nil slice keeps everything", "api", nil, true},
		{"empty slice keeps an item a non-matching slice would drop", "api", []string{}, true},
		{"exact match kept", "api", []string{"api", "web"}, true},
		{"case-insensitive match kept", "API", []string{"api"}, true},
		{"non-member dropped", "docs", []string{"api", "web"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchesRepository(tt.repo, tt.repositories); got != tt.want {
				t.Errorf("MatchesRepository(%q, %v) = %v, want %v", tt.repo, tt.repositories, got, tt.want)
			}
		})
	}
}

// TestFilterMatchesRepositoryEqualsInlineBranchesRule pins the promoted
// predicate against the rule it replaces inside GetBranches (branches.go:92).
// The two were verified byte-identical before promotion; this asserts they stay
// that way rather than trusting the reading.
func TestFilterMatchesRepositoryEqualsInlineBranchesRule(t *testing.T) {
	repoNames := []string{"api", "API", "web", "docs", ""}
	configured := [][]string{
		nil,
		{},
		{"api"},
		{"API", "web"},
		{"nothing-matches"},
	}

	for _, repositories := range configured {
		validRepos := make(map[string]bool, len(repositories))
		for _, r := range repositories {
			validRepos[strings.ToLower(r)] = true
		}
		for _, name := range repoNames {
			// The inline rule skips (drops) when this is true.
			inlineDrops := len(validRepos) > 0 && !validRepos[strings.ToLower(name)]
			if got := MatchesRepository(name, repositories); got == inlineDrops {
				t.Errorf("MatchesRepository(%q, %v) = %v, inline rule drops = %v",
					name, repositories, got, inlineDrops)
			}
		}
	}
}
