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

// GetBranches fetches branches for every repository in the specified projects,
// sorted by most recent commit first. When repositories is non-empty only those
// repository names are fetched.
func (c *Client) GetBranches(ctx context.Context, projects, repositories []string) ([]Branch, error) {
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

	for _, project := range targetProjects {
		repos, err := c.gitClient.GetRepositories(ctx, git.GetRepositoriesArgs{Project: &project})
		if err != nil {
			log.Error("Failed to get repositories", "project", project, "error", err)
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

	log.Info("Branches fetched", "total_count", len(allBranches))
	return allBranches, nil
}
