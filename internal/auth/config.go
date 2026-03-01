package auth

import (
	"fmt"
	"os"
	"strings"

	"github.com/nullify/slack-cli/internal/types"
)

// LoadFromEnv reads SLACK_TOKEN, SLACK_COOKIE_D, SLACK_WORKSPACE_URL from
// the environment and returns a validated AuthConfig.
func LoadFromEnv() (*types.AuthConfig, error) {
	token := os.Getenv("SLACK_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("SLACK_TOKEN environment variable is required")
	}

	cookie := os.Getenv("SLACK_COOKIE_D")
	workspaceURL := strings.TrimRight(os.Getenv("SLACK_WORKSPACE_URL"), "/")

	cfg := &types.AuthConfig{
		Token:        token,
		Cookie:       cookie,
		WorkspaceURL: workspaceURL,
	}

	if strings.HasPrefix(token, "xoxc-") {
		cfg.Mode = types.AuthBrowser
		if cookie == "" {
			return nil, fmt.Errorf("SLACK_COOKIE_D environment variable is required for xoxc tokens")
		}
		if workspaceURL == "" {
			return nil, fmt.Errorf("SLACK_WORKSPACE_URL environment variable is required for browser auth (xoxc tokens)")
		}
	} else if strings.HasPrefix(token, "xoxb-") || strings.HasPrefix(token, "xoxp-") {
		cfg.Mode = types.AuthStandard
	} else {
		return nil, fmt.Errorf("SLACK_TOKEN must start with xoxb-, xoxp-, or xoxc- (got %s...)", token[:min(8, len(token))])
	}

	return cfg, nil
}
