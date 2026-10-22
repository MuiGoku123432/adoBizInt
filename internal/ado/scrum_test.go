package ado

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/webapi"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/work"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
)

func boolPtr(b bool) *bool { return &b }

var scrumTeams = []Team{
	{Project: "P", Team: "Core", TeamID: "id-core"},
	{Project: "P", Team: "Web", TeamID: "id-web"},
}

// TestGetTeamMembersFlagsGroups covers TEAM-02: a user, an IsContainer group
// and an aadgp. descriptor group are all present, keyed by id, and both
// groups are flagged.
func TestGetTeamMembersFlagsGroups(t *testing.T) {
	members := []webapi.TeamMember{
		{Identity: &webapi.IdentityRef{Id: strPtr("u1"), DisplayName: strPtr("Ann"), Descriptor: strPtr("aad.xyz")}, IsTeamAdmin: boolPtr(true)},
		{Identity: &webapi.IdentityRef{Id: strPtr("g1"), DisplayName: strPtr("Old group"), IsContainer: boolPtr(true)}},
		{Identity: &webapi.IdentityRef{Id: strPtr("g2"), DisplayName: strPtr("Entra group"), Descriptor: strPtr("aadgp.abc")}},
	}
	c := &Client{coreClient: &fakeCoreAPI{t: t, GetTeamMembersFunc: func(_ context.Context, args core.GetTeamMembersWithExtendedPropertiesArgs) (*[]webapi.TeamMember, error) {
		if *args.TeamId != "id-core" || args.Top == nil || args.Skip == nil {
			t.Fatalf("args = %+v", args)
		}
		return &members, nil
	}}}
	result, _ := c.GetTeamMembers(context.Background(), scrumTeams[:1])
	if len(result.Items) != 3 || !result.Complete {
		t.Fatalf("result = %+v", result)
	}
	got := map[string]TeamMember{}
	for _, m := range result.Items {
		got[m.ID] = m
	}
	if got["u1"].IsGroup || !got["u1"].IsTeamAdmin || !got["g1"].IsGroup || !got["g2"].IsGroup {
		t.Errorf("members = %+v", got)
	}
	if got["g2"].Team != "Core" || got["g2"].Project != "P" {
		t.Errorf("member team/project = %+v", got["g2"])
	}
}

func TestGetTeamMembersFailureIsPerTeam(t *testing.T) {
	c := &Client{coreClient: &fakeCoreAPI{t: t, GetTeamMembersFunc: func(_ context.Context, args core.GetTeamMembersWithExtendedPropertiesArgs) (*[]webapi.TeamMember, error) {
		if *args.TeamId == "id-core" {
			return nil, wrappedErrorValue(500, "x")
		}
		return &[]webapi.TeamMember{}, nil
	}}}
	result, _ := c.GetTeamMembers(context.Background(), scrumTeams)
	if result.Complete || len(result.Failures) != 1 || result.Failures[0].Team != "Core" {
		t.Errorf("result = %+v", result)
	}
}

func adoDate(y int, m time.Month, d int) *azuredevops.Time {
	return &azuredevops.Time{Time: time.Date(y, m, d, 0, 0, 0, 0, time.UTC)}
}

func iteration(start, finish *azuredevops.Time) work.TeamSettingsIteration {
	id := uuid.New()
	return work.TeamSettingsIteration{Id: &id, Name: strPtr("Sprint 7"), Path: strPtr(`P\Sprint 7`),
		Attributes: &work.TeamIterationAttributes{StartDate: start, FinishDate: finish}}
}

func sprintClient(t *testing.T, byTeam map[string][]work.TeamSettingsIteration) *Client {
	return &Client{workClient: &fakeWorkAPI{t: t, GetTeamIterationsFunc: func(_ context.Context, args work.GetTeamIterationsArgs) (*[]work.TeamSettingsIteration, error) {
		if args.Timeframe == nil || *args.Timeframe != "current" {
			t.Fatalf("timeframe = %v, want current", args.Timeframe)
		}
		its := byTeam[*args.Team]
		return &its, nil
	}}}
}

// TestCurrentSprintCivilDatesAcrossZones covers TEAM-03: the civil dates do
// not move under a zone west (Chicago) or east (Kolkata) of UTC.
func TestCurrentSprintCivilDatesAcrossZones(t *testing.T) {
	for _, zone := range []string{"America/Chicago", "Asia/Kolkata"} {
		t.Run(zone, func(t *testing.T) {
			loc, err := time.LoadLocation(zone)
			if err != nil {
				t.Skipf("zone %s unavailable: %v", zone, err)
			}
			prev := time.Local
			time.Local = loc
			t.Cleanup(func() { time.Local = prev })

			c := sprintClient(t, map[string][]work.TeamSettingsIteration{
				"id-core": {iteration(adoDate(2026, 10, 5), adoDate(2026, 10, 16))},
			})
			result, _ := c.GetCurrentSprints(context.Background(), scrumTeams[:1])
			if len(result.Items) != 1 {
				t.Fatalf("result = %+v", result)
			}
			s := result.Items[0]
			if s.StartDate != "2026-10-05" || s.FinishDate != "2026-10-16" || s.Team != "Core" || s.IterationID == "" {
				t.Errorf("sprint = %+v", s)
			}
		})
	}
}

func TestCurrentSprintPerTeamFailures(t *testing.T) {
	tests := []struct {
		name string
		its  []work.TeamSettingsIteration
		want string
	}{
		{"no current iteration", nil, "no current iteration"},
		{"two current iterations", []work.TeamSettingsIteration{
			iteration(adoDate(2026, 10, 5), adoDate(2026, 10, 16)), iteration(adoDate(2026, 10, 5), adoDate(2026, 10, 16)),
		}, "2 current iterations"},
		{"null start date", []work.TeamSettingsIteration{iteration(nil, adoDate(2026, 10, 16))}, "no start or finish date"},
		{"null attributes", []work.TeamSettingsIteration{{Name: strPtr("x")}}, "no start or finish date"},
		{"finish before start", []work.TeamSettingsIteration{iteration(adoDate(2026, 10, 16), adoDate(2026, 10, 5))}, "finishes before"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := sprintClient(t, map[string][]work.TeamSettingsIteration{"id-core": tt.its})
			result, _ := c.GetCurrentSprints(context.Background(), scrumTeams[:1])
			if len(result.Items) != 0 || len(result.Failures) != 1 || result.Complete {
				t.Fatalf("result = %+v, want one failure and no sprint", result)
			}
			f := result.Failures[0]
			if f.Team != "Core" || f.Operation != opGetCurrentSprint || !strings.Contains(f.Err.Error(), tt.want) {
				t.Errorf("failure = %+v (err %v), want %q", f, f.Err, tt.want)
			}
		})
	}
}

// TestCadenceMismatchFinding covers TEAM-05: different sprint dates produce one
// finding and leave the read complete; equal dates produce none.
func TestCadenceMismatchFinding(t *testing.T) {
	t.Run("different dates", func(t *testing.T) {
		c := sprintClient(t, map[string][]work.TeamSettingsIteration{
			"id-core": {iteration(adoDate(2026, 10, 5), adoDate(2026, 10, 16))},
			"id-web":  {iteration(adoDate(2026, 10, 12), adoDate(2026, 10, 23))},
		})
		result, _ := c.GetCurrentSprints(context.Background(), scrumTeams)
		if !result.Complete || len(result.Findings) != 1 {
			t.Fatalf("result = %+v, want complete with one finding", result)
		}
		f := result.Findings[0]
		if f.Kind != FindingCadenceMismatch || f.Project != "P" || len(f.Teams) != 2 ||
			!strings.Contains(f.Detail, "2026-10-05..2026-10-16 (P/Core)") {
			t.Errorf("finding = %+v", f)
		}
	})
	t.Run("same dates", func(t *testing.T) {
		c := sprintClient(t, map[string][]work.TeamSettingsIteration{
			"id-core": {iteration(adoDate(2026, 10, 5), adoDate(2026, 10, 16))},
			"id-web":  {iteration(adoDate(2026, 10, 5), adoDate(2026, 10, 16))},
		})
		result, _ := c.GetCurrentSprints(context.Background(), scrumTeams)
		if len(result.Findings) != 0 || result.Findings == nil {
			t.Errorf("findings = %#v, want empty non-nil", result.Findings)
		}
	})
}

func backlogConfig(typeFields map[string]string, reqTypes []string, states map[string]map[string]string) *work.BacklogConfiguration {
	var refs []workitemtracking.WorkItemTypeReference
	for _, n := range reqTypes {
		refs = append(refs, workitemtracking.WorkItemTypeReference{Name: strPtr(n)})
	}
	var mapped []work.WorkItemTypeStateInfo
	for typ, st := range states {
		mapped = append(mapped, work.WorkItemTypeStateInfo{WorkItemTypeName: strPtr(typ), States: &st})
	}
	return &work.BacklogConfiguration{
		BacklogFields:            &work.BacklogFields{TypeFields: &typeFields},
		RequirementBacklog:       &work.BacklogLevelConfiguration{WorkItemTypes: &refs},
		WorkItemTypeMappedStates: &mapped,
	}
}

// TestGetProcessFields covers TEAM-07 for the three stock process shapes and
// the named failure when a field is missing. One call per project, through
// its first selected team.
func TestGetProcessFields(t *testing.T) {
	shapes := []struct {
		name, points, reqType string
		states                map[string]string
		wantDone              []string
	}{
		{"scrum", "Microsoft.VSTS.Scheduling.Effort", "Product Backlog Item",
			map[string]string{"New": "Proposed", "Committed": "InProgress", "Done": "Completed", "Removed": "Removed"}, []string{"Done"}},
		{"agile", "Microsoft.VSTS.Scheduling.StoryPoints", "User Story",
			map[string]string{"New": "Proposed", "Resolved": "Resolved", "Closed": "Completed"}, []string{"Closed"}},
		{"cmmi", "Microsoft.VSTS.Scheduling.Size", "Requirement",
			map[string]string{"Proposed": "Proposed", "Closed": "Completed"}, []string{"Closed"}},
	}
	for _, sh := range shapes {
		t.Run(sh.name, func(t *testing.T) {
			calls := 0
			c := &Client{workClient: &fakeWorkAPI{t: t, GetBacklogConfigurationsFunc: func(_ context.Context, args work.GetBacklogConfigurationsArgs) (*work.BacklogConfiguration, error) {
				calls++
				if *args.Team != "id-core" {
					t.Errorf("queried team %s, want the first selected team", *args.Team)
				}
				return backlogConfig(map[string]string{"Effort": sh.points, "Order": "Microsoft.VSTS.Common.StackRank"},
					[]string{sh.reqType}, map[string]map[string]string{sh.reqType: sh.states, "Bug": {"Done": "Completed"}}), nil
			}}}
			result, _ := c.GetProcessFields(context.Background(), scrumTeams)
			if calls != 1 || len(result.Items) != 1 || !result.Complete {
				t.Fatalf("calls %d result %+v", calls, result)
			}
			pf := result.Items[0]
			if pf.PointsField != sh.points || pf.RankField != "Microsoft.VSTS.Common.StackRank" {
				t.Errorf("fields = %+v", pf)
			}
			if got := pf.DoneStates[sh.reqType]; len(got) != len(sh.wantDone) || got[0] != sh.wantDone[0] {
				t.Errorf("done states = %v, want %v", got, sh.wantDone)
			}
			if _, ok := pf.DoneStates["Bug"]; ok {
				t.Errorf("done states include a non-requirement type: %v", pf.DoneStates)
			}
		})
	}

	t.Run("missing rank field is a named failure", func(t *testing.T) {
		c := &Client{workClient: &fakeWorkAPI{t: t, GetBacklogConfigurationsFunc: func(context.Context, work.GetBacklogConfigurationsArgs) (*work.BacklogConfiguration, error) {
			return backlogConfig(map[string]string{"Effort": "x"}, []string{"Story"}, map[string]map[string]string{"Story": {"Done": "Completed"}}), nil
		}}}
		result, _ := c.GetProcessFields(context.Background(), scrumTeams)
		if len(result.Failures) != 1 || result.Failures[0].Team != "Core" || !strings.Contains(result.Failures[0].Err.Error(), "Order") {
			t.Errorf("result = %+v", result)
		}
	})
}
