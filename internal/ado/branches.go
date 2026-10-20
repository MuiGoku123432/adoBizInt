package ado

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"

	"sentinovo.ai/bizInt/internal/logging"
)

// Branch represents a repository branch for display
type Branch struct {
	Name           string    `json:"name"`
	Project        string    `json:"project"`
	RepositoryID   string    `json:"repository_id"`
	RepositoryName string    `json:"repository_name"`
	DefaultBranch  string    `json:"default_branch"` // Repository default branch, used as the PR target
	IsDefault      bool      `json:"is_default"`
	AheadCount     int       `json:"ahead_count"`
	BehindCount    int       `json:"behind_count"`
	LastCommitDate time.Time `json:"last_commit_date"`
	LastCommitBy   string    `json:"last_commit_by"`
	CommitID       string    `json:"commit_id"`
}

// opGetRepositories labels the per-project repository listing in any Failure
// this read records. Per-repository branch-stat failures build their own label
// from opGetBranchStats so a user reading the warning line can tell which
// repository is broken.
const (
	opGetRepositories = "get repositories"
	opGetBranchStats  = "get branch stats"
)

// emptyRepositoryStatus and emptyRepositoryCode together identify the answer
// ADO gives a branch-stats call on a repository with no branches. It is
// suppressed rather than recorded: naming every empty repository in the org on
// the branches view would train the user to ignore the exact channel D-13
// depends on.
//
// D-06a: the live capture of 2026-10-12 returned status 400, typeKey
// ArgumentException, errorCode 0, and a message beginning "VS403403: Cannot
// find any branches for the ... repository." Neither TypeKey (generic) nor
// ErrorCode (0) distinguishes it, so the leading ADO error identifier is
// matched, anchored at the start of the message and only in conjunction with
// the structural 400. That capture falsified assumption A4 (an empty
// repository answers 404).
//
// D-07 fixed the failure kind set at four and explicitly deferred a not-found
// kind; this is the site where such a kind would have earned its keep. If it
// comes back, it comes back from the deferred-ideas list in 02-CONTEXT.md.
const (
	emptyRepositoryStatus = 400
	emptyRepositoryCode   = "VS403403:"
)

// disabledRepositoryStatus and disabledRepositoryTypeKey identify a disabled
// repository (D-06b). GetRepositories still enumerates disabled repositories,
// the 2020 SDK's GitRepository carries no isDisabled field to skip them by, and
// their branch-stats call answers 404 with typeKey GitRepositoryNotFoundException
// (live capture, 2026-10-12: 2 Kinetic, 128 Process). The repository was listed
// moments earlier, so "not found" here means disabled; the structured typeKey is
// matched, never the message. The old blanket 404 rule hid these by accident.
const (
	disabledRepositoryStatus  = 404
	disabledRepositoryTypeKey = "GitRepositoryNotFoundException"
)

// unreadableRepository reports whether a branch-stats error is an empty
// repository (D-06a) or a disabled one (D-06b), neither of which is a broken
// read. Every other error, including any 401/403, is recorded by the caller.
func unreadableRepository(err error) bool {
	wrapped, ok := wrappedErrorOf(err)
	if !ok || wrapped.StatusCode == nil {
		return false
	}
	switch *wrapped.StatusCode {
	case emptyRepositoryStatus:
		return strings.HasPrefix(err.Error(), emptyRepositoryCode)
	case disabledRepositoryStatus:
		return wrapped.TypeKey != nil && *wrapped.TypeKey == disabledRepositoryTypeKey
	}
	return false
}

// GetBranches fetches branches for every repository in the specified projects,
// sorted by most recent commit first. When repositories is non-empty only those
// repository names are fetched. Per-project and per-repository failures are
// reported in the returned envelope.
func (c *Client) GetBranches(ctx context.Context, projects, repositories []string) (Result[Branch], error) {
	log := logging.Logger()

	targetProjects := projects
	if len(targetProjects) == 0 {
		targetProjects = c.projects
	}

	var allBranches []Branch
	var failures []Failure

	for _, project := range targetProjects {
		// Record the remaining projects as unreached rather than issuing calls
		// that are already doomed (D-12).
		if err := ctx.Err(); err != nil {
			failures = appendFailure(failures, project, opGetRepositories, err)
			continue
		}

		repos, err := c.gitClient.GetRepositories(ctx, git.GetRepositoriesArgs{Project: &project})
		if err != nil {
			log.Error("Failed to get repositories", "project", project, "error", err)
			// The loop still visits every configured project: D-08 rejected
			// short-circuiting on auth failure explicitly.
			failures = appendFailure(failures, project, opGetRepositories, err)
			continue
		}
		if repos == nil {
			continue
		}

		for _, repo := range *repos {
			if repo.Name == nil || repo.Id == nil {
				continue
			}
			if !MatchesRepository(*repo.Name, repositories) {
				continue
			}

			repoID := repo.Id.String()
			defaultBranch := ""
			if repo.DefaultBranch != nil {
				defaultBranch = strings.TrimPrefix(*repo.DefaultBranch, "refs/heads/")
			}

			stats, err := c.gitClient.GetBranches(ctx, git.GetBranchesArgs{
				RepositoryId: &repoID,
				Project:      &project,
			})
			if err != nil {
				// Empty repositories have no branch stats to report
				log.Warn("Failed to get branch stats", "project", project, "repository", *repo.Name, "error", err)
				// Empty (D-06a) and disabled (D-06b) repositories are not broken
				// reads; anything else, including a 401/403 or a timeout, is recorded.
				if !unreadableRepository(err) {
					failures = appendFailure(failures, project, opGetBranchStats+" for "+*repo.Name, err)
				}
				continue
			}
			if stats == nil {
				continue
			}

			for _, s := range *stats {
				if s.Name == nil {
					continue
				}
				b := Branch{
					Name:           *s.Name,
					Project:        project,
					RepositoryID:   repoID,
					RepositoryName: *repo.Name,
					DefaultBranch:  defaultBranch,
					IsDefault:      *s.Name == defaultBranch,
				}
				if s.AheadCount != nil {
					b.AheadCount = *s.AheadCount
				}
				if s.BehindCount != nil {
					b.BehindCount = *s.BehindCount
				}
				if s.Commit != nil {
					if s.Commit.CommitId != nil {
						b.CommitID = *s.Commit.CommitId
					}
					if s.Commit.Committer != nil {
						if s.Commit.Committer.Date != nil {
							b.LastCommitDate = s.Commit.Committer.Date.Time
						}
						if s.Commit.Committer.Name != nil {
							b.LastCommitBy = *s.Commit.Committer.Name
						}
					}
				}
				allBranches = append(allBranches, b)
			}
		}
	}

	// Newest first so freshly created branches surface at the top
	sort.Slice(allBranches, func(i, j int) bool {
		return allBranches[i].LastCommitDate.After(allBranches[j].LastCommitDate)
	})

	log.Info("Branches fetched", "total_count", len(allBranches), "failure_count", len(failures))
	return newResult(allBranches, failures), nil
}
