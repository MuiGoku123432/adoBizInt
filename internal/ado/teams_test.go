package ado

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"

	"sentinovo.ai/bizInt/internal/config"
)

// pagedTeams serves n teams named T0..T(n-1) per project through offset
// paging, honouring $top and $skip as the server does, and counts calls.
func pagedTeams(t *testing.T, n int, calls *int, fail map[string]error) *fakeCoreAPI {
	return &fakeCoreAPI{t: t, GetTeamsFunc: func(_ context.Context, args core.GetTeamsArgs) (*[]core.WebApiTeam, error) {
		*calls++
		if err := fail[*args.ProjectId]; err != nil {
			return nil, err
		}
		if args.Top == nil || args.Skip == nil {
			t.Fatalf("GetTeams called without explicit $top/$skip")
		}
		var page []core.WebApiTeam
		for i := *args.Skip; i < n && i < *args.Skip+*args.Top; i++ {
			id := uuid.New()
			name := fmt.Sprintf("T%d", i)
			page = append(page, core.WebApiTeam{Id: &id, Name: &name})
		}
		return &page, nil
	}}
}

// TestGetTeamsPagesUntilShortPage covers TEAM-01 discovery: 250 teams arrive
// over three pages (100, 100, 50) and all of them are returned.
func TestGetTeamsPagesUntilShortPage(t *testing.T) {
	calls := 0
	c := &Client{projects: []string{"P"}, coreClient: pagedTeams(t, 250, &calls, nil)}
	result, _ := c.GetTeams(context.Background(), nil)
	if len(result.Items) != 250 || !result.Complete {
		t.Fatalf("got %d teams complete=%v, want 250 complete", len(result.Items), result.Complete)
	}
	if calls != 3 {
		t.Errorf("GetTeams calls = %d, want 3", calls)
	}
	if last := result.Items[249]; last.Team != "T249" || last.TeamID == "" || last.Project != "P" {
		t.Errorf("last team = %+v", last)
	}
}

// TestGetTeamsExactPageMultiple pins termination when the total is an exact
// multiple of the page size: the trailing empty page ends the loop.
func TestGetTeamsExactPageMultiple(t *testing.T) {
	calls := 0
	c := &Client{projects: []string{"P"}, coreClient: pagedTeams(t, 200, &calls, nil)}
	result, _ := c.GetTeams(context.Background(), nil)
	if len(result.Items) != 200 || calls != 3 {
		t.Errorf("got %d teams in %d calls, want 200 in 3", len(result.Items), calls)
	}
}

// TestGetTeamsFailureIsProjectScoped: a failing project is recorded with
// team "" and the other project still returns.
func TestGetTeamsFailureIsProjectScoped(t *testing.T) {
	calls := 0
	fail := map[string]error{"Bad": wrappedErrorValue(401, "no")}
	c := &Client{projects: []string{"Bad", "Good"}, coreClient: pagedTeams(t, 2, &calls, fail)}
	result, _ := c.GetTeams(context.Background(), nil)
	if len(result.Items) != 2 || result.Complete || len(result.Failures) != 1 {
		t.Fatalf("result = %+v", result)
	}
	f := result.Failures[0]
	if f.Project != "Bad" || f.Team != "" || f.Kind != FailureAuth || f.Operation != opGetTeams {
		t.Errorf("failure = %+v", f)
	}
}

func TestResolveTeams(t *testing.T) {
	t.Run("matches across pages ignoring case and carries IDs", func(t *testing.T) {
		calls := 0
		c := &Client{coreClient: pagedTeams(t, 250, &calls, nil)}
		got, err := c.ResolveTeams(context.Background(), []string{"P"}, map[string][]string{"P": {"t3", " T240 "}})
		if err != nil {
			t.Fatalf("ResolveTeams() error = %v", err)
		}
		if len(got) != 2 || got[0].Team != "T3" || got[1].Team != "T240" || got[1].TeamID == "" {
			t.Errorf("resolved = %+v", got)
		}
	})
	t.Run("unknown team is a named config error", func(t *testing.T) {
		calls := 0
		c := &Client{coreClient: pagedTeams(t, 250, &calls, nil)}
		_, err := c.ResolveTeams(context.Background(), []string{"P"}, map[string][]string{"P": {"Nope"}})
		var cfgErr *config.ConfigError
		if !errors.As(err, &cfgErr) || cfgErr.Error() != `teams: unknown team "P/Nope"` {
			t.Errorf("err = %v, want ConfigError for P/Nope", err)
		}
	})
	t.Run("listing error is returned unclassified", func(t *testing.T) {
		calls := 0
		authErr := wrappedErrorValue(401, "no")
		c := &Client{coreClient: pagedTeams(t, 1, &calls, map[string]error{"P": authErr})}
		_, err := c.ResolveTeams(context.Background(), []string{"P"}, map[string][]string{"P": {"T0"}})
		if ClassifyFailure(err) != FailureAuth {
			t.Errorf("err = %v, want an auth-classified error", err)
		}
	})
	t.Run("projects with no selection are not listed", func(t *testing.T) {
		calls := 0
		c := &Client{coreClient: pagedTeams(t, 1, &calls, nil)}
		if _, err := c.ResolveTeams(context.Background(), []string{"P", "Q"}, map[string][]string{"Q": {"T0"}}); err != nil {
			t.Fatal(err)
		}
		if calls != 1 {
			t.Errorf("calls = %d, want 1", calls)
		}
	})
}

// TestGetTeamsStopsWhenPagingDoesNotAdvance: a server that ignores $skip and
// repeats a full page ends the read with an error instead of looping.
func TestGetTeamsStopsWhenPagingDoesNotAdvance(t *testing.T) {
	ids := make([]uuid.UUID, teamsPageSize)
	for i := range ids {
		ids[i] = uuid.New()
	}
	calls := 0
	c := &Client{projects: []string{"P"}, coreClient: &fakeCoreAPI{t: t, GetTeamsFunc: func(context.Context, core.GetTeamsArgs) (*[]core.WebApiTeam, error) {
		calls++
		if calls > 3 {
			t.Fatalf("paging did not stop")
		}
		page := make([]core.WebApiTeam, teamsPageSize)
		for i := range page {
			page[i] = core.WebApiTeam{Id: &ids[i], Name: strPtr("T")}
		}
		return &page, nil
	}}}
	result, _ := c.GetTeams(context.Background(), nil)
	if len(result.Failures) != 1 || !strings.Contains(result.Failures[0].Err.Error(), "did not advance") {
		t.Errorf("result = %+v", result)
	}
}
