package main

import (
	"bytes"
	"context"
	"encoding/json"
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
		projects   []string
		resolveErr error
		wantOK     bool
		wantCode   int
		wantCalls  int
		wantTeams  int
		wantStderr string
	}{
		{name: "all configured teams by default", wantOK: true, wantCode: exitOK, wantCalls: 1, wantTeams: 3},
		{name: "--team narrows to one", team: "A/Web", wantOK: true, wantCode: exitOK, wantCalls: 1, wantTeams: 1},
		{name: "--project narrows the selection", projects: []string{"B"}, wantOK: true, wantCode: exitOK, wantCalls: 1, wantTeams: 1},
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

			teams, code, ok := selectTeams(context.Background(), c, tt.team, tt.projects)
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

// scrumFake resolves every selected team and serves fixed scrum reads.
func scrumFake(t *testing.T) *fakeReadClient {
	calls := 0
	var selected map[string][]string
	c := resolveRecorder(t, &calls, &selected, nil)
	c.GetCurrentSprintsFunc = func(_ context.Context, teams []ado.Team) (ado.Result[ado.Sprint], error) {
		r := emptyResult[ado.Sprint]()
		for i, tm := range teams {
			r.Items = append(r.Items, ado.Sprint{Project: tm.Project, Team: tm.Team, StartDate: "2026-10-0" + string(rune('1'+i)), FinishDate: "2026-10-14"})
		}
		if len(teams) > 1 {
			r.Findings = []ado.Finding{{Kind: ado.FindingCadenceMismatch, Teams: []string{"A/Core", "A/Web"}, Detail: "teams have different current sprints"}}
		}
		return r, nil
	}
	c.GetTeamMembersFunc = func(_ context.Context, teams []ado.Team) (ado.Result[ado.TeamMember], error) {
		r := emptyResult[ado.TeamMember]()
		for _, tm := range teams {
			r.Items = append(r.Items, ado.TeamMember{Project: tm.Project, Team: tm.Team, ID: "g", DisplayName: "Devs", IsGroup: true})
		}
		return r, nil
	}
	c.GetProcessFieldsFunc = func(_ context.Context, teams []ado.Team) (ado.Result[ado.ProcessFields], error) {
		r := emptyResult[ado.ProcessFields]()
		r.Failures = []ado.Failure{{Project: "B", Team: "Mobile", Operation: "resolve process fields", Kind: ado.FailureOther, Err: errors.New("rank field (Order) is not mapped")}}
		r.Complete = false
		return r, nil
	}
	return c
}

// TestTeamFlagOnlyOnScrumVerbs: --team parses only where it can mean
// something, like every other per-verb flag (flags.go).
func TestTeamFlagOnlyOnScrumVerbs(t *testing.T) {
	for _, name := range commandKeys() {
		fs, _, _, _ := newFlagSet(name)
		if got := fs.Lookup("team") != nil; got != scrumCommands[name] {
			t.Errorf("%s registers --team = %v, want %v", name, got, scrumCommands[name])
		}
	}
}

func TestScrumVerbsEndToEnd(t *testing.T) {
	withReadBudget(t, time.Minute)

	t.Run("sprint current --json carries caps and findings", func(t *testing.T) {
		withScrumConfig(t, scrumTestConfig())
		stdout, _ := captureOutput(t)
		if code := dispatch(context.Background(), scrumFake(t), []string{"sprint", "current", "--json"}); code != exitOK {
			t.Fatalf("exit = %d, want 0", code)
		}
		var env struct {
			Items    []ado.Sprint  `json:"items"`
			Findings []ado.Finding `json:"findings"`
			Complete bool          `json:"complete"`
			Scope    struct {
				Caps config.Caps `json:"caps"`
			} `json:"scope"`
		}
		if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
			t.Fatalf("unmarshal %s: %v", stdout, err)
		}
		if len(env.Items) != 3 || len(env.Findings) != 1 || !env.Complete || env.Scope.Caps.MaxTeams != 25 {
			t.Errorf("envelope = %+v", env)
		}
		for _, s := range env.Items {
			if s.Team == "" {
				t.Errorf("item without team: %+v", s)
			}
		}
	})

	t.Run("human mode prints scope caps and the finding", func(t *testing.T) {
		withScrumConfig(t, scrumTestConfig())
		_, stderr := captureOutput(t)
		dispatch(context.Background(), scrumFake(t), []string{"sprint", "current"})
		if !strings.Contains(stderr.String(), "caps max_teams=25") ||
			!strings.Contains(stderr.String(), "finding: cadence_mismatch: teams have different current sprints") {
			t.Errorf("stderr = %q", stderr)
		}
	})

	t.Run("--team narrows the read", func(t *testing.T) {
		withScrumConfig(t, scrumTestConfig())
		stdout, _ := captureOutput(t)
		if code := dispatch(context.Background(), scrumFake(t), []string{"teams", "members", "--json", "--team", "B/Mobile"}); code != exitOK {
			t.Fatalf("exit = %d", code)
		}
		var env struct {
			Items []ado.TeamMember `json:"items"`
		}
		_ = json.Unmarshal(stdout.Bytes(), &env)
		if len(env.Items) != 1 || env.Items[0].Team != "Mobile" || !env.Items[0].IsGroup {
			t.Errorf("items = %+v", env.Items)
		}
	})

	t.Run("over the cap exits 3 before reading", func(t *testing.T) {
		cfg := scrumTestConfig()
		cfg.Caps.MaxTeams = 1
		withScrumConfig(t, cfg)
		captureOutput(t)
		c := &fakeReadClient{t: t} // any call fails the test
		if code := dispatch(context.Background(), c, []string{"sprint", "current"}); code != exitUnknown {
			t.Errorf("exit = %d, want 3", code)
		}
	})

	t.Run("teams process partial failure exits 1", func(t *testing.T) {
		withScrumConfig(t, scrumTestConfig())
		_, stderr := captureOutput(t)
		if code := dispatch(context.Background(), scrumFake(t), []string{"teams", "process"}); code != exitPartial {
			t.Errorf("exit = %d, want 1", code)
		}
		if !strings.Contains(stderr.String(), "warning: B/Mobile: resolve process fields failed (other)") {
			t.Errorf("stderr = %q", stderr)
		}
	})
}

// TestScrumInterruptDuringValidation: a signal that lands while teams are
// being validated yields the interrupt crash record, as every other read
// does, not a validation error.
func TestScrumInterruptDuringValidation(t *testing.T) {
	withReadBudget(t, time.Minute)
	withScrumConfig(t, scrumTestConfig())
	stdout, stderr := captureOutput(t)
	ctx, cancel := context.WithCancel(context.Background())
	c := &fakeReadClient{t: t, ResolveTeamsFunc: func(rctx context.Context, _ []string, _ map[string][]string) ([]ado.Team, error) {
		cancel()
		return nil, rctx.Err()
	}}
	if code := dispatch(ctx, c, []string{"sprint", "current", "--json"}); code != exitUnknown {
		t.Errorf("exit = %d, want 3", code)
	}
	if !strings.Contains(stdout.String(), `"interrupt"`) {
		t.Errorf("stdout = %q, want the interrupt crash record", stdout)
	}
	if strings.Contains(stderr.String(), "team validation failed") {
		t.Errorf("stderr = %q, an interrupt is not a validation failure", stderr)
	}
}

// TestDefaultCapRefuses26Teams is AC9 as the spec states it: 26 configured
// teams under the default cap of 25 exit 3 with zero ADO calls.
func TestDefaultCapRefuses26Teams(t *testing.T) {
	withReadBudget(t, time.Minute)
	cfg := scrumTestConfig()
	cfg.Teams = map[string]config.StringSlice{"A": {}, "B": {"Mobile"}}
	for i := 0; i < 25; i++ {
		cfg.Teams["A"] = append(cfg.Teams["A"], "T"+string(rune('a'+i)))
	}
	withScrumConfig(t, cfg)
	_, stderr := captureOutput(t)
	if code := dispatch(context.Background(), &fakeReadClient{t: t}, []string{"teams", "members"}); code != exitUnknown {
		t.Errorf("exit = %d, want 3", code)
	}
	if !strings.Contains(stderr.String(), "request covers 26 teams, over max_teams 25") {
		t.Errorf("stderr = %q", stderr)
	}
}

// TestHumanOutputSanitizesADOTeamNames: team names come from ADO, so an escape
// sequence in one never reaches the terminal raw.
func TestHumanOutputSanitizesADOTeamNames(t *testing.T) {
	var b bytes.Buffer
	renderFindings(&b, []ado.Finding{{Kind: ado.FindingCadenceMismatch, Detail: "x (P/\x1b[31mRed\nTeam)"}})
	renderFailures(&b, []ado.Failure{{Project: "P", Team: "\x1b[2JEvil", Operation: "op", Kind: ado.FailureOther}})
	if strings.ContainsRune(b.String(), '\x1b') || strings.Count(b.String(), "\n") != 2 {
		t.Errorf("output = %q", b.String())
	}
}

func TestSprintItemsVerb(t *testing.T) {
	withReadBudget(t, time.Minute)
	pts := 3.0
	fake := func(t *testing.T, gotAt *string) *fakeReadClient {
		calls := 0
		var sel map[string][]string
		c := resolveRecorder(t, &calls, &sel, nil)
		c.GetSprintItemsFunc = func(_ context.Context, teams []ado.Team, snapshot string, _ time.Time) (ado.Result[ado.SprintItem], error) {
			*gotAt = snapshot
			r := emptyResult[ado.SprintItem]()
			r.Items = []ado.SprintItem{{Project: "A", Team: "Core", ID: 10, Type: "User Story", State: "Active", Points: &pts, Title: "Story",
				Tasks: []ado.SprintItem{{Project: "A", Team: "Core", ID: 11, Type: "Task", State: "Closed", Title: "Task"}}}}
			r.Scope = &ado.Scope{AsOf: "live", AreaPathsCurrent: true, Teams: []ado.SprintTeamScope{{Project: "A", Team: "Core", IterationID: 7}}}
			return r, nil
		}
		return c
	}

	t.Run("--json keeps sprint scope and adds caps", func(t *testing.T) {
		withScrumConfig(t, scrumTestConfig())
		stdout, _ := captureOutput(t)
		var at string
		if code := dispatch(context.Background(), fake(t, &at), []string{"sprint", "items", "--json", "--at", "baseline"}); code != exitOK {
			t.Fatalf("exit = %d", code)
		}
		if at != "baseline" {
			t.Errorf("snapshot = %q", at)
		}
		var env struct {
			Scope map[string]json.RawMessage `json:"scope"`
		}
		_ = json.Unmarshal(stdout.Bytes(), &env)
		for _, k := range []string{"caps", "as_of", "teams", "area_paths_current"} {
			if env.Scope[k] == nil {
				t.Errorf("scope lacks %s: %s", k, stdout)
			}
		}
	})

	t.Run("default is live and human output indents tasks", func(t *testing.T) {
		withScrumConfig(t, scrumTestConfig())
		stdout, stderr := captureOutput(t)
		var at string
		dispatch(context.Background(), fake(t, &at), []string{"sprint", "items"})
		if at != "live" {
			t.Errorf("snapshot = %q, want live", at)
		}
		if !strings.Contains(stdout.String(), "10\tUser Story\tActive\t3\tStory") || !strings.Contains(stdout.String(), "\t  11\tTask\tClosed\t\tTask") {
			t.Errorf("stdout = %q", stdout)
		}
		if !strings.Contains(stderr.String(), "live snapshot") || !strings.Contains(stderr.String(), "team area paths as configured today") {
			t.Errorf("stderr = %q", stderr)
		}
	})

	t.Run("--team narrows sprint items", func(t *testing.T) {
		withScrumConfig(t, scrumTestConfig())
		captureOutput(t)
		var at string
		c := fake(t, &at)
		var got []ado.Team
		inner := c.GetSprintItemsFunc
		c.GetSprintItemsFunc = func(ctx context.Context, teams []ado.Team, s string, n time.Time) (ado.Result[ado.SprintItem], error) {
			got = teams
			return inner(ctx, teams, s, n)
		}
		dispatch(context.Background(), c, []string{"sprint", "items", "--team", "B/Mobile"})
		if len(got) != 1 || got[0].Team != "Mobile" {
			t.Errorf("teams = %+v", got)
		}
	})

	t.Run("bad --at exits 3 before any call", func(t *testing.T) {
		withScrumConfig(t, scrumTestConfig())
		_, stderr := captureOutput(t)
		if code := dispatch(context.Background(), &fakeReadClient{t: t}, []string{"sprint", "items", "--at", "yesterday"}); code != exitUnknown {
			t.Errorf("exit = %d", code)
		}
		if !strings.Contains(stderr.String(), "--at must be live, baseline or close") {
			t.Errorf("stderr = %q", stderr)
		}
	})

	t.Run("--at only on sprint items", func(t *testing.T) {
		for _, name := range commandKeys() {
			fs, _, _, _ := newFlagSet(name)
			if got := fs.Lookup("at") != nil; got != (name == "sprint items") {
				t.Errorf("%s registers --at = %v", name, got)
			}
		}
	})
}
