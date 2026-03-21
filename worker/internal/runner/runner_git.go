package runner

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

func validateGitHubRepo(repoURL string) error {
	if repoURL == "" {
		return fmt.Errorf("source_repository is required")
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

func cloneRepo(ctx context.Context, dir, repoURL string) error {
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", repoURL, ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
