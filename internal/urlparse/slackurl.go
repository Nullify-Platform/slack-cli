package urlparse

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/nullify/slack-cli/internal/types"
)

var (
	channelIDRe = regexp.MustCompile(`^[CDG][A-Z0-9]{8,}$`)
	userIDRe    = regexp.MustCompile(`^U[A-Z0-9]{8,}$`)
)

// ParseSlackMessageURL parses a Slack message URL into its components.
// URL format: https://workspace.slack.com/archives/<channel>/p<13digits>
// Timestamp conversion: split digits at len-6: "seconds.micros"
func ParseSlackMessageURL(input string) (*types.SlackMessageRef, error) {
	u, err := url.Parse(input)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	if !strings.HasSuffix(u.Hostname(), ".slack.com") {
		return nil, fmt.Errorf("not a Slack URL: %s", u.Hostname())
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 3 || parts[0] != "archives" {
		return nil, fmt.Errorf("unsupported Slack URL path: %s", u.Path)
	}

	channelID := parts[1]
	pDigits := parts[2]

	if !strings.HasPrefix(pDigits, "p") || len(pDigits) < 8 {
		return nil, fmt.Errorf("invalid message ID in URL: %s", pDigits)
	}

	digits := pDigits[1:]
	if len(digits) < 7 {
		return nil, fmt.Errorf("message ID too short: %s", pDigits)
	}

	// Convert to Slack timestamp format: split at len-6
	splitAt := len(digits) - 6
	messageTS := digits[:splitAt] + "." + digits[splitAt:]

	workspaceURL := u.Scheme + "://" + u.Host

	ref := &types.SlackMessageRef{
		WorkspaceURL: workspaceURL,
		ChannelID:    channelID,
		MessageTS:    messageTS,
		Raw:          input,
	}

	// Check for thread_ts query parameter
	if threadTS := u.Query().Get("thread_ts"); threadTS != "" {
		ref.ThreadTSHint = threadTS
	}

	return ref, nil
}

// IsChannelID returns true if the input matches [CDG][A-Z0-9]{8,}.
func IsChannelID(s string) bool {
	return channelIDRe.MatchString(s)
}

// IsUserID returns true if the input matches U[A-Z0-9]{8,}.
func IsUserID(s string) bool {
	return userIDRe.MatchString(s)
}

// ParseMsgTarget determines whether input is a URL, channel, or user reference.
func ParseMsgTarget(input string) *types.MsgTarget {
	// Try URL first
	if strings.Contains(input, "slack.com/archives/") {
		ref, err := ParseSlackMessageURL(input)
		if err == nil {
			return &types.MsgTarget{Kind: types.TargetURL, Ref: ref}
		}
	}

	// Check for user ID
	if IsUserID(input) {
		return &types.MsgTarget{Kind: types.TargetUser, UserID: input}
	}

	// Everything else is a channel reference
	return &types.MsgTarget{Kind: types.TargetChannel, Channel: input}
}

// NormalizeChannelInput strips leading # from channel names and classifies the input.
func NormalizeChannelInput(input string) (kind string, value string) {
	cleaned := strings.TrimPrefix(input, "#")
	if IsChannelID(cleaned) {
		return "id", cleaned
	}
	return "name", cleaned
}
