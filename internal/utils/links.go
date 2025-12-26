package utils

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/atotto/clipboard"
)

// BuildWorkItemURL constructs the Azure DevOps URL for a work item
func BuildWorkItemURL(orgURL, project string, id int) string {
	return fmt.Sprintf("%s/%s/_workitems/edit/%d", orgURL, project, id)
}

// BuildPRURL constructs the Azure DevOps URL for a pull request
func BuildPRURL(orgURL, project, repoName string, id int) string {
	return fmt.Sprintf("%s/%s/_git/%s/pullrequest/%d", orgURL, project, repoName, id)
}

// BuildPipelineURL constructs the Azure DevOps URL for a pipeline run
func BuildPipelineURL(orgURL, project string, buildID int) string {
	return fmt.Sprintf("%s/%s/_build/results?buildId=%d&view=logs", orgURL, project, buildID)
}

// BuildReleaseURL constructs the Azure DevOps URL for a release
func BuildReleaseURL(orgURL, project string, releaseID int) string {
	return fmt.Sprintf("%s/%s/_release?releaseId=%d", orgURL, project, releaseID)
}

// CopyToClipboard copies the given text to the system clipboard
func CopyToClipboard(text string) error {
	return clipboard.WriteAll(text)
}

// OpenInBrowser opens the given URL in the default browser
func OpenInBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
	return cmd.Start() // Non-blocking
}
