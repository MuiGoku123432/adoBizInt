package main

import (
	"context"
	"errors"
	"fmt"

	"sentinovo.ai/bizInt/internal/ado"
	"sentinovo.ai/bizInt/internal/config"
	"sentinovo.ai/bizInt/internal/fetchctx"
	"sentinovo.ai/bizInt/internal/logging"
)

// scrumConfig is the validated config the scrum verbs read teams and caps
// from. run() assigns it after Validate, exactly as it does configuredProjects.
// v1 verbs never read it, so they keep working without teams (D-2).
var scrumConfig *config.Config

// selectTeams is the precondition every scrum verb runs before its first read
// (spec section 2-3, 6). In order, and stopping at the first failure:
//
//  1. ValidateScrum: teams present for every project, caps positive (exit 3).
//  2. --team, if set, must name a configured "project/team" exactly, as
//     --project does (D-09); otherwise every configured team is selected.
//  3. max_teams: a selection over the cap is refused before any call (exit 3).
//  4. ResolveTeams checks names against ADO and returns team IDs. An auth
//     rejection exits 2 and names the scope the new calls need; anything else,
//     an unknown team included, exits 3: validation is never partial.
//
// ok is false when the caller must return code without reading.
func selectTeams(ctx context.Context, c client, team string) (teams []ado.Team, code int, ok bool) {
	cfg := scrumConfig
	if err := cfg.ValidateScrum(); err != nil {
		fmt.Fprintf(stderrWriter, "Invalid config: %v\n", err)
		return nil, exitUnknown, false
	}

	selected := map[string][]string{}
	count := 0
	for _, p := range cfg.Projects {
		for _, t := range cfg.TeamsFor(p) {
			if team != "" && team != p+"/"+t {
				continue
			}
			selected[p] = append(selected[p], t)
			count++
		}
	}
	if team != "" && count == 0 {
		fmt.Fprintf(stderrWriter, "not a configured team: %q (use project/team as configured)\n", team)
		return nil, exitUnknown, false
	}
	if count > cfg.Caps.MaxTeams {
		fmt.Fprintf(stderrWriter, "request covers %d teams, over max_teams %d; narrow it with --team or raise caps.max_teams\n",
			count, cfg.Caps.MaxTeams)
		return nil, exitUnknown, false
	}

	resolveCtx, cancel := fetchctx.NewWithParent(ctx, readBudget)
	defer cancel()
	teams, err := c.ResolveTeams(resolveCtx, cfg.Projects, selected)
	if err == nil {
		return teams, exitOK, true
	}
	var cfgErr *config.ConfigError
	switch {
	case errors.As(err, &cfgErr):
		fmt.Fprintf(stderrWriter, "Invalid config: %v\n", err)
	case ado.ClassifyFailure(err) == ado.FailureAuth:
		// Server text stays in the log, never on the terminal (render.go).
		logging.Logger().Error("Team validation refused", "error", err)
		fmt.Fprintf(stderrWriter, "team validation was refused (auth): the PAT needs the Project and Team (Read) scope\n")
		return nil, exitAuth, false
	default:
		logging.Logger().Error("Team validation failed", "error", err)
		fmt.Fprintf(stderrWriter, "team validation failed (%s); see the log for detail\n", ado.ClassifyFailure(err))
	}
	return nil, exitUnknown, false
}
