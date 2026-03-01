package slack

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/nullify/slack-cli/internal/api"
	"github.com/nullify/slack-cli/internal/types"
	"github.com/nullify/slack-cli/internal/urlparse"
)

const defaultConversationTypes = "public_channel,private_channel"

// ListChannelsOpts configures channel listing.
type ListChannelsOpts struct {
	All             bool
	UserID          string
	Limit           int
	Cursor          string
	Types           string
	ExcludeArchived bool
}

// ChannelListResult is the output of ListChannels.
type ChannelListResult struct {
	Channels   []types.CompactChannel `json:"channels"`
	NextCursor string                 `json:"next_cursor,omitempty"`
}

// ListChannels lists conversations.
// Uses users.conversations (default) or conversations.list (when All=true).
func ListChannels(ctx context.Context, client *api.Client, opts ListChannelsOpts) (*ChannelListResult, error) {
	if opts.Limit <= 0 {
		opts.Limit = 100
	}
	if opts.Limit > 1000 {
		opts.Limit = 1000
	}
	if opts.Types == "" {
		opts.Types = defaultConversationTypes
	}

	method := "users.conversations"
	if opts.All {
		method = "conversations.list"
	}

	params := map[string]string{
		"types":            opts.Types,
		"limit":            strconv.Itoa(opts.Limit),
		"exclude_archived": "true",
	}
	if !opts.ExcludeArchived {
		params["exclude_archived"] = "false"
	}
	if opts.Cursor != "" {
		params["cursor"] = opts.Cursor
	}
	if opts.UserID != "" && !opts.All {
		params["user"] = opts.UserID
	}

	resp, err := client.Call(ctx, method, params)
	if err != nil {
		return nil, err
	}

	rawChannels := api.GetSlice(resp["channels"])
	result := &ChannelListResult{}
	for _, ch := range rawChannels {
		if m, ok := ch.(map[string]interface{}); ok {
			result.Channels = append(result.Channels, parseRawChannel(m))
		}
	}

	result.NextCursor = api.ExtractCursor(resp)
	return result, nil
}

// GetChannelInfo calls conversations.info and returns details.
func GetChannelInfo(ctx context.Context, client *api.Client, channelID string) (*types.CompactChannel, error) {
	resp, err := client.Call(ctx, "conversations.info", map[string]string{
		"channel": channelID,
	})
	if err != nil {
		return nil, err
	}

	raw := api.GetMap(resp["channel"])
	if raw == nil {
		return nil, fmt.Errorf("no channel data in response")
	}

	ch := parseRawChannel(raw)
	return &ch, nil
}

// ResolveChannelID converts a channel name or ID to a channel ID.
func ResolveChannelID(ctx context.Context, client *api.Client, input string) (string, error) {
	kind, value := urlparse.NormalizeChannelInput(input)
	if kind == "id" {
		return value, nil
	}

	// Try search.messages with in:#name for fast single-call lookup
	resp, err := client.Call(ctx, "search.messages", map[string]string{
		"query": "in:#" + value,
		"count": "1",
	})
	if err == nil {
		if msgs := api.GetMap(resp["messages"]); msgs != nil {
			if matches := api.GetSlice(msgs["matches"]); len(matches) > 0 {
				if m, ok := matches[0].(map[string]interface{}); ok {
					if ch := api.GetMap(m["channel"]); ch != nil {
						if id := api.GetStringFromMap(ch, "id"); id != "" {
							return id, nil
						}
					}
				}
			}
		}
	}

	// Fallback: paginate conversations.list
	cursor := ""
	for {
		params := map[string]string{
			"types":            defaultConversationTypes,
			"limit":            "200",
			"exclude_archived": "true",
		}
		if cursor != "" {
			params["cursor"] = cursor
		}

		resp, err := client.Call(ctx, "conversations.list", params)
		if err != nil {
			return "", fmt.Errorf("listing channels to resolve %q: %w", input, err)
		}

		rawChannels := api.GetSlice(resp["channels"])
		for _, ch := range rawChannels {
			if m, ok := ch.(map[string]interface{}); ok {
				name := api.GetStringFromMap(m, "name")
				if strings.EqualFold(name, value) {
					return api.GetStringFromMap(m, "id"), nil
				}
			}
		}

		cursor = api.ExtractCursor(resp)
		if cursor == "" {
			break
		}
	}

	return "", fmt.Errorf("could not resolve channel: %s", input)
}

// OpenDM opens a DM channel with a user.
func OpenDM(ctx context.Context, client *api.Client, userID string) (string, error) {
	resp, err := client.Call(ctx, "conversations.open", map[string]string{
		"users": userID,
	})
	if err != nil {
		return "", err
	}

	ch := api.GetMap(resp["channel"])
	if ch == nil {
		return "", fmt.Errorf("no channel in conversations.open response")
	}
	return api.GetStringFromMap(ch, "id"), nil
}

// CreateChannel creates a new channel.
func CreateChannel(ctx context.Context, client *api.Client, name string, isPrivate bool) (*types.CompactChannel, error) {
	params := map[string]string{
		"name": name,
	}
	if isPrivate {
		params["is_private"] = "true"
	}

	resp, err := client.Call(ctx, "conversations.create", params)
	if err != nil {
		return nil, err
	}

	raw := api.GetMap(resp["channel"])
	if raw == nil {
		return nil, fmt.Errorf("no channel in conversations.create response")
	}
	ch := parseRawChannel(raw)
	return &ch, nil
}

// InviteResult contains the result of channel invitation.
type InviteResult struct {
	Invited        []string `json:"invited,omitempty"`
	AlreadyInChannel []string `json:"already_in_channel,omitempty"`
	Errors         []string `json:"errors,omitempty"`
}

// InviteToChannel invites users to a channel.
func InviteToChannel(ctx context.Context, client *api.Client, channelID string, userIDs []string) (*InviteResult, error) {
	result := &InviteResult{}

	// Slack's conversations.invite accepts comma-separated user IDs
	_, err := client.Call(ctx, "conversations.invite", map[string]string{
		"channel": channelID,
		"users":   strings.Join(userIDs, ","),
	})
	if err != nil {
		if apiErr, ok := err.(*api.SlackAPIError); ok {
			if apiErr.Code == "already_in_channel" {
				result.AlreadyInChannel = userIDs
				return result, nil
			}
		}
		return nil, err
	}

	result.Invited = userIDs
	return result, nil
}

func parseRawChannel(raw map[string]interface{}) types.CompactChannel {
	ch := types.CompactChannel{
		ID:   api.GetStringFromMap(raw, "id"),
		Name: api.GetStringFromMap(raw, "name"),
	}

	numMembers := api.GetIntFromMap(raw, "num_members")
	if numMembers > 0 {
		ch.NumMembers = numMembers
	}

	if isPrivate := api.GetBoolFromMap(raw, "is_private"); isPrivate {
		ch.IsPrivate = boolPtr(true)
	}
	if isIM := api.GetBoolFromMap(raw, "is_im"); isIM {
		ch.IsIM = boolPtr(true)
	}
	if isMPIM := api.GetBoolFromMap(raw, "is_mpim"); isMPIM {
		ch.IsMPIM = boolPtr(true)
	}

	if topic := api.GetMap(raw["topic"]); topic != nil {
		ch.Topic = api.GetStringFromMap(topic, "value")
	}
	if purpose := api.GetMap(raw["purpose"]); purpose != nil {
		ch.Purpose = api.GetStringFromMap(purpose, "value")
	}

	return ch
}

func boolPtr(b bool) *bool {
	return &b
}
