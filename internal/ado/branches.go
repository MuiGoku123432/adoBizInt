package ado

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/microsoft/azure-devops-go-api/azuredevops/git"

	"sentinovo.ai/bizInt/internal/logging"
)

// Branch represents a repository branch for display
type Branch struct {
	Name           string
	Project        string
	RepositoryID   string
	RepositoryName string
	DefaultBranch  string // Repository default branch, used as the PR target
	IsDefault      bool
	AheadCount     int
	BehindCount    int
	LastCommitDate time.Time
	LastCommitBy   string
	CommitID       string
}

// opGetRepositories labels the per-project repository listing in any Failure
// this read records. Per-repository branch-stat failures build their own label
// from opGetBranchStats so a user reading the warning line can tell which
// repository is broken.
const (
	opGetRepositories = "get repositories"
	opGetBranchStats  = "get branch stats"
)

// emptyRepositoryStatus is the status the SDK returns when a repository has no
// branch stats to report. It is suppressed rather than recorded: naming every
// empty repository in the org on the branches view would train the user to
// ignore the exact channel D-13 depends on.
//
// D-07 fixed the failure kind set at four and explicitly deferred a not-found
// kind; this is the site where such a kind would have earned its keep. If it
// comes back, it comes back from the deferred-ideas list in 02-CONTEXT.md.
const emptyRepositoryStatus = 404

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

	validRepos := make(map[string]bool, len(repositories))
	for _, r := range repositories {
		validRepos[strings.ToLower(r)] = true
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
			if len(validRepos) > 0 && !validRepos[strings.ToLower(*repo.Name)] {
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
				// Distinguish by status code (D-06): a not-found response is an
				// ordinary empty repository, not a broken read.
				if status, ok := statusCodeOf(err); !ok || status != emptyRepositoryStatus {
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
