package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"

	"sentinovo.ai/bizInt/internal/ado"
	"sentinovo.ai/bizInt/internal/config"
)

// withScrumConfig sets scrumConfig for one test, standing in for the
// assignment run() makes after config validation.
func withScrumConfig(t *testing.T, cfg *config.Config) {
	t.Helper()
	prev := scrumConfig
	scrumConfig = cfg
	t.Cleanup(func() { scrumConfig = prev })
}

func scrumTestConfig() *config.Config {
	return &config.Config{
		Projects: config.StringSlice{"A", "B"},
		Teams:    map[string]config.StringSlice{"A": {"Core", "Web"}, "B": {"Mobile"}},
		Caps:     config.Caps{MaxTeams: 25, MaxSprintHistory: 12, MaxBacklogDepth: 500},
	}
}

// resolveRecorder echoes the selection back as teams and records whether
// ResolveTeams was called and with what.
func resolveRecorder(t *testing.T, called *int, got *map[string][]string, err error) *fakeReadClient {
	return &fakeReadClient{t: t, ResolveTeamsFunc: func(_ context.Context, projects []string, selected map[string][]string) ([]ado.Team, error) {
		*called++
		*got = selected
		if err != nil {
			return nil, err
		}
		var teams []ado.Team
		for _, p := range projects {
			for _, name := range selected[p] {
				teams = append(teams, ado.Team{Project: p, Team: name, TeamID: p + "-" + name})
			}
		}
		return teams, nil
	}}
}

func TestSelectTeams(t *testing.T) {
	withReadBudget(t, time.Minute)
	status := func(code int) error {
		return azuredevops.WrappedError{StatusCode: &code, Message: new(string)}
	}

	tests := []struct {
		name       string
		mutate     func(*config.Config)
		team       string
		resolveErr error
		wantOK     bool
		wantCode   int
		wantCalls  int
		wantTeams  int
		wantStderr string
	}{
		{name: "all configured teams by default", wantOK: true, wantCode: exitOK, wantCalls: 1, wantTeams: 3},
		{name: "--team narrows to one", team: "A/Web", wantOK: true, wantCode: exitOK, wantCalls: 1, wantTeams: 1},
		{name: "unconfigured --team", team: "A/Nope", wantCode: exitUnknown, wantStderr: `not a configured team: "A/Nope"`},
		{name: "--team match is exact", team: "a/web", wantCode: exitUnknown, wantStderr: "not a configured team"},
		{name: "no teams configured", mutate: func(c *config.Config) { c.Teams = nil }, wantCode: exitUnknown,
			wantStderr: "at least one team is required for scrum verbs"},
		{name: "over max_teams makes no call", mutate: func(c *config.Config) { c.Caps.MaxTeams = 2 }, wantCode: exitUnknown,
			wantStderr: "request covers 3 teams, over max_teams 2"},
		{name: "--team brings selection under the cap", mutate: func(c *config.Config) { c.Caps.MaxTeams = 1 }, team: "B/Mobile",
			wantOK: true, wantCode: exitOK, wantCalls: 1, wantTeams: 1},
		{name: "unknown team in ADO", resolveErr: &config.ConfigError{Field: "teams", Message: `unknown team "A/Web"`},
			wantCode: exitUnknown, wantCalls: 1, wantStderr: `unknown team "A/Web"`},
		{name: "auth rejection names the scope", resolveErr: status(401), wantCode: exitAuth, wantCalls: 1,
			wantStderr: "Project and Team (Read)"},
		{name: "other listing error", resolveErr: errors.New("boom"), wantCode: exitUnknown, wantCalls: 1,
			wantStderr: "team validation failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, stderr := captureOutput(t)
			cfg := scrumTestConfig()
			if tt.mutate != nil {
				tt.mutate(cfg)
			}
			withScrumConfig(t, cfg)
			calls := 0
			var selected map[string][]string
			c := resolveRecorder(t, &calls, &selected, tt.resolveErr)

			teams, code, ok := selectTeams(context.Background(), c, tt.team)
			if ok != tt.wantOK || code != tt.wantCode {
				t.Fatalf("selectTeams() = ok %v code %d, want ok %v code %d (stderr %q)", ok, code, tt.wantOK, tt.wantCode, stderr)
			}
			if calls != tt.wantCalls {
				t.Errorf("ResolveTeams calls = %d, want %d", calls, tt.wantCalls)
			}
			if len(teams) != tt.wantTeams {
				t.Errorf("teams = %+v, want %d", teams, tt.wantTeams)
			}
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr = %q, want it to contain %q", stderr, tt.wantStderr)
			}
		})
	}
}

// TestTeamsListNeedsNoTeamsConfig: discovery runs on the v1 path, so it works
// with scrumConfig unset and renders every team the read returns.
func TestTeamsListNeedsNoTeamsConfig(t *testing.T) {
	withReadBudget(t, time.Minute)
	withScrumConfig(t, nil)
	stdout, _ := captureOutput(t)
	c := allReadsFake(t)
	c.GetTeamsFunc = func(context.Context, []string) (ado.Result[ado.Team], error) {
		r := emptyResult[ado.Team]()
		r.Items = []ado.Team{{Project: "A", Team: "Core", TeamID: "id-1"}}
		return r, nil
	}
	if code := dispatch(context.Background(), c, []string{"teams", "list"}); code != exitOK {
		t.Fatalf("teams list exit = %d, want %d", code, exitOK)
	}
	if !strings.Contains(stdout.String(), "Core") || !strings.Contains(stdout.String(), "id-1") {
		t.Errorf("stdout = %q, want the team row", stdout)
	}
}
