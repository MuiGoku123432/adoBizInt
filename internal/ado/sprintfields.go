package ado

import (
	"html"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
)

// Pure parsing of batch-read work items into SprintItem: no calls to ADO.

func sprintItemFromFields(team Team, pf ProcessFields, f map[string]interface{}) SprintItem {
	str := func(k string) string { s, _ := f[k].(string); return s }
	num := func(k string) *float64 {
		if v, ok := f[k].(float64); ok {
			return &v
		}
		return nil
	}
	it := SprintItem{
		Project:          team.Project,
		Team:             team.Team,
		Type:             str("System.WorkItemType"),
		Title:            str("System.Title"),
		State:            str("System.State"),
		AreaPath:         str("System.AreaPath"),
		IterationPath:    str("System.IterationPath"),
		Points:           num(pf.PointsField),
		RemainingWork:    num(fieldRemaining),
		CompletedWork:    num(fieldCompleted),
		OriginalEstimate: num(fieldOriginal),
		Tasks:            []SprintItem{},
	}
	it.StateCategory = pf.StateCategories[it.Type][it.State]
	it.Done = it.StateCategory == stateCategoryCompleted
	if p, ok := f["System.Parent"].(float64); ok {
		id := int(p)
		it.ParentID = &id
	}
	if a, ok := f["System.AssignedTo"].(map[string]interface{}); ok {
		it.AssignedTo, _ = a["displayName"].(string)
	}
	it.AcceptanceCriteriaEmpty = acceptanceCriteriaEmpty(str(fieldAC))
	return it
}

// nestTasks puts each Task under its parent when the parent is in the same
// set; a task whose parent is not stays top level with its parent_id, so
// nothing is dropped (DATA-03). Order follows the query.
func nestTasks(order []int, byID map[int]*SprintItem) []SprintItem {
	nested := map[int]bool{}
	for _, id := range order {
		it := byID[id]
		if !IsTaskType(it.Type) || it.ParentID == nil {
			continue
		}
		if parent := byID[*it.ParentID]; parent != nil && !IsTaskType(parent.Type) {
			parent.Tasks = append(parent.Tasks, *it)
			nested[id] = true
		}
	}
	out := []SprintItem{}
	for _, id := range order {
		if !nested[id] {
			out = append(out, *byID[id])
		}
	}
	return out
}

// pullRequestIDs parses PR ids from ArtifactLink relations, whose url is
// vstfs:///Git/PullRequestId/<project>%2F<repo>%2F<id>. Other relations,
// attachments included, are ignored (10-RESEARCH Q3).
func pullRequestIDs(rels []workitemtracking.WorkItemRelation) []int {
	ids := []int{}
	for _, r := range rels {
		u := deref(r.Url)
		if deref(r.Rel) != "ArtifactLink" || !strings.HasPrefix(strings.ToLower(u), "vstfs:///git/pullrequestid/") {
			continue
		}
		i := strings.LastIndex(strings.ToUpper(u), "%2F")
		if i < 0 {
			continue
		}
		if id, err := strconv.Atoi(u[i+3:]); err == nil && !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	return ids
}

var htmlTag = regexp.MustCompile(`<[^>]*>`)

// acceptanceCriteriaEmpty reports whether acceptance criteria hold no text
// once tags are removed, entities decoded (&nbsp; included) and whitespace,
// Unicode included, trimmed (DATA-06).
func acceptanceCriteriaEmpty(s string) bool {
	text := html.UnescapeString(htmlTag.ReplaceAllString(s, " "))
	return strings.TrimFunc(text, unicode.IsSpace) == ""
}
