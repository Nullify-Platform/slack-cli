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

// ListUsersOpts configures ListUsers.
type ListUsersOpts struct {
	Limit       int
	Cursor      string
	IncludeBots bool
}

// UserListResult is the output of ListUsers.
type UserListResult struct {
	Users      []types.CompactUser `json:"users"`
	NextCursor string              `json:"next_cursor,omitempty"`
}

// ListUsers lists workspace users with pagination.
func ListUsers(ctx context.Context, client *api.Client, opts ListUsersOpts) (*UserListResult, error) {
	if opts.Limit <= 0 {
		opts.Limit = 200
	}
	if opts.Limit > 1000 {
		opts.Limit = 1000
	}

	params := map[string]string{
		"limit": strconv.Itoa(opts.Limit),
	}
	if opts.Cursor != "" {
		params["cursor"] = opts.Cursor
	}

	resp, err := client.Call(ctx, "users.list", params)
	if err != nil {
		return nil, err
	}

	rawUsers := api.GetSlice(resp["members"])
	result := &UserListResult{}
	for _, u := range rawUsers {
		if m, ok := u.(map[string]interface{}); ok {
			user := parseRawUser(m)
			// Filter bots unless requested
			if !opts.IncludeBots && user.IsBot != nil && *user.IsBot {
				continue
			}
			// Filter deleted users
			if user.Deleted != nil && *user.Deleted {
				continue
			}
			result.Users = append(result.Users, user)
		}
	}

	result.NextCursor = api.ExtractCursor(resp)
	return result, nil
}

// GetUser fetches a single user by ID, handle, or email.
func GetUser(ctx context.Context, client *api.Client, input string) (*types.CompactUser, error) {
	input = strings.TrimPrefix(input, "@")

	// Direct ID lookup
	if urlparse.IsUserID(input) {
		return getUserByID(ctx, client, input)
	}

	// Email lookup
	if strings.Contains(input, "@") {
		resp, err := client.Call(ctx, "users.lookupByEmail", map[string]string{
			"email": input,
		})
		if err != nil {
			return nil, err
		}
		raw := api.GetMap(resp["user"])
		if raw == nil {
			return nil, fmt.Errorf("no user data in response")
		}
		user := parseRawUser(raw)
		return &user, nil
	}

	// Resolve by handle - get user ID first then fetch full profile
	userID, err := resolveUserByName(ctx, client, input)
	if err != nil {
		return nil, err
	}
	return getUserByID(ctx, client, userID)
}

// ResolveUserID resolves a user identifier to a user ID.
func ResolveUserID(ctx context.Context, client *api.Client, input string) (string, error) {
	input = strings.TrimPrefix(input, "@")

	if urlparse.IsUserID(input) {
		return input, nil
	}

	// Email lookup
	if strings.Contains(input, "@") {
		resp, err := client.Call(ctx, "users.lookupByEmail", map[string]string{
			"email": input,
		})
		if err != nil {
			return "", fmt.Errorf("resolving user email %q: %w", input, err)
		}
		raw := api.GetMap(resp["user"])
		if raw == nil {
			return "", fmt.Errorf("no user data for email %q", input)
		}
		return api.GetStringFromMap(raw, "id"), nil
	}

	return resolveUserByName(ctx, client, input)
}

func getUserByID(ctx context.Context, client *api.Client, userID string) (*types.CompactUser, error) {
	resp, err := client.Call(ctx, "users.info", map[string]string{
		"user": userID,
	})
	if err != nil {
		return nil, err
	}
	raw := api.GetMap(resp["user"])
	if raw == nil {
		return nil, fmt.Errorf("no user data in response")
	}
	user := parseRawUser(raw)
	return &user, nil
}

func resolveUserByName(ctx context.Context, client *api.Client, name string) (string, error) {
	cursor := ""
	for {
		params := map[string]string{
			"limit": "200",
		}
		if cursor != "" {
			params["cursor"] = cursor
		}

		resp, err := client.Call(ctx, "users.list", params)
		if err != nil {
			return "", fmt.Errorf("listing users to resolve %q: %w", name, err)
		}

		rawUsers := api.GetSlice(resp["members"])
		for _, u := range rawUsers {
			if m, ok := u.(map[string]interface{}); ok {
				uName := api.GetStringFromMap(m, "name")
				if strings.EqualFold(uName, name) {
					return api.GetStringFromMap(m, "id"), nil
				}
				// Also check display_name in profile
				if profile := api.GetMap(m["profile"]); profile != nil {
					displayName := api.GetStringFromMap(profile, "display_name")
					if strings.EqualFold(displayName, name) {
						return api.GetStringFromMap(m, "id"), nil
					}
				}
			}
		}

		cursor = api.ExtractCursor(resp)
		if cursor == "" {
			break
		}
	}

	return "", fmt.Errorf("could not resolve user: %s", name)
}

func parseRawUser(raw map[string]interface{}) types.CompactUser {
	user := types.CompactUser{
		ID:   api.GetStringFromMap(raw, "id"),
		Name: api.GetStringFromMap(raw, "name"),
		TZ:   api.GetStringFromMap(raw, "tz"),
	}

	if api.GetBoolFromMap(raw, "is_bot") {
		user.IsBot = boolPtr(true)
	}
	if api.GetBoolFromMap(raw, "deleted") {
		user.Deleted = boolPtr(true)
	}

	if profile := api.GetMap(raw["profile"]); profile != nil {
		user.RealName = api.GetStringFromMap(profile, "real_name")
		user.DisplayName = api.GetStringFromMap(profile, "display_name")
		user.Email = api.GetStringFromMap(profile, "email")
		user.Title = api.GetStringFromMap(profile, "title")
	}

	// Fallback: real_name at top level
	if user.RealName == "" {
		user.RealName = api.GetStringFromMap(raw, "real_name")
	}

	return user
}
