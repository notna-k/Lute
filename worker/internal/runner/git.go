package runner

import (
	"fmt"
	"net/url"
	"strings"
)

// validateGitHubRepo accepts an empty URL or an https github.com one.
func validateGitHubRepo(repoURL string) error {
	if repoURL == "" {
		return nil
	}
	u, err := url.Parse(repoURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("only https is allowed (got %q)", u.Scheme)
	}
	host := strings.ToLower(u.Host)
	if host != "github.com" && host != "www.github.com" {
		return fmt.Errorf("only github.com is allowed (got %q)", u.Host)
	}
	return nil
}

// cloneScript clones $1 into the workspace and prints the commit as its last line.
const cloneScript = `git clone --depth 1 "$1" ` + workspaceMount + ` && git -C ` + workspaceMount + ` rev-parse HEAD`

// lastLine is the last non-empty line of s.
func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}
