package slack

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	"github.com/nullify/slack-cli/internal/api"
	"github.com/nullify/slack-cli/internal/types"
)

// ChannelHistoryOpts configures FetchChannelHistory.
type ChannelHistoryOpts struct {
	ChannelID        string
	Limit            int
	Oldest           string
	Latest           string
	IncludeReactions bool
	MaxBodyChars     int // default 8000, -1 for unlimited
}

// FetchMessage fetches a single message by channel + ts.
func FetchMessage(ctx context.Context, client *api.Client, channelID, messageTS, threadTSHint string, includeReactions bool, maxBodyChars int) (*types.CompactMessage, error) {
	if maxBodyChars == 0 {
		maxBodyChars = 8000
	}

	// If we have a thread_ts hint, try conversations.replies first
	if threadTSHint != "" {
		resp, err := client.Call(ctx, "conversations.replies", map[string]string{
			"channel": channelID,
			"ts":      threadTSHint,
			"limit":   "200",
		})
		if err == nil {
			msgs := api.GetSlice(resp["messages"])
			for _, m := range msgs {
				if raw, ok := m.(map[string]interface{}); ok {
					if api.GetStringFromMap(raw, "ts") == messageTS {
						return parseRawMessage(channelID, raw, maxBodyChars, includeReactions), nil
					}
				}
			}
		}
	}

	// Try conversations.history with inclusive=true for exact ts
	resp, err := client.Call(ctx, "conversations.history", map[string]string{
		"channel":   channelID,
		"latest":    messageTS,
		"oldest":    messageTS,
		"inclusive": "true",
		"limit":     "1",
	})
	if err != nil {
		return nil, err
	}

	msgs := api.GetSlice(resp["messages"])
	if len(msgs) > 0 {
		if raw, ok := msgs[0].(map[string]interface{}); ok {
			return parseRawMessage(channelID, raw, maxBodyChars, includeReactions), nil
		}
	}

	// Try as thread root
	resp, err = client.Call(ctx, "conversations.replies", map[string]string{
		"channel": channelID,
		"ts":      messageTS,
		"limit":   "1",
	})
	if err != nil {
		return nil, fmt.Errorf("message not found: %s in %s", messageTS, channelID)
	}

	msgs = api.GetSlice(resp["messages"])
	if len(msgs) > 0 {
		if raw, ok := msgs[0].(map[string]interface{}); ok {
			return parseRawMessage(channelID, raw, maxBodyChars, includeReactions), nil
		}
	}

	return nil, fmt.Errorf("message not found: %s in %s", messageTS, channelID)
}

// FetchChannelHistory fetches recent messages from a channel.
func FetchChannelHistory(ctx context.Context, client *api.Client, opts ChannelHistoryOpts) ([]types.CompactMessage, error) {
	if opts.Limit <= 0 {
		opts.Limit = 25
	}
	if opts.Limit > 200 {
		opts.Limit = 200
	}
	if opts.MaxBodyChars == 0 {
		opts.MaxBodyChars = 8000
	}

	params := map[string]string{
		"channel": opts.ChannelID,
		"limit":   strconv.Itoa(opts.Limit),
	}
	if opts.Oldest != "" {
		params["oldest"] = opts.Oldest
	}
	if opts.Latest != "" {
		params["latest"] = opts.Latest
	}

	resp, err := client.Call(ctx, "conversations.history", params)
	if err != nil {
		return nil, err
	}

	rawMsgs := api.GetSlice(resp["messages"])
	var messages []types.CompactMessage
	for _, m := range rawMsgs {
		if raw, ok := m.(map[string]interface{}); ok {
			messages = append(messages, *parseRawMessage(opts.ChannelID, raw, opts.MaxBodyChars, opts.IncludeReactions))
		}
	}

	// Sort chronologically (oldest first) - Slack returns newest first
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].TS < messages[j].TS
	})

	return messages, nil
}

// FetchThread fetches all replies in a thread.
func FetchThread(ctx context.Context, client *api.Client, channelID, threadTS string, includeReactions bool, maxBodyChars int) ([]types.CompactMessage, error) {
	if maxBodyChars == 0 {
		maxBodyChars = 8000
	}

	var messages []types.CompactMessage
	cursor := ""

	for {
		params := map[string]string{
			"channel": channelID,
			"ts":      threadTS,
			"limit":   "200",
		}
		if cursor != "" {
			params["cursor"] = cursor
		}

		resp, err := client.Call(ctx, "conversations.replies", params)
		if err != nil {
			return nil, err
		}

		rawMsgs := api.GetSlice(resp["messages"])
		for _, m := range rawMsgs {
			if raw, ok := m.(map[string]interface{}); ok {
				messages = append(messages, *parseRawMessage(channelID, raw, maxBodyChars, includeReactions))
			}
		}

		cursor = api.ExtractCursor(resp)
		if cursor == "" {
			break
		}
	}

	// Sort chronologically
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].TS < messages[j].TS
	})

	return messages, nil
}

// FetchChannelActivity returns new top-level messages AND new thread replies
// since the given oldest timestamp. It scans recent channel history to find
// threads with latest_reply > oldest, then fetches those thread replies.
func FetchChannelActivity(ctx context.Context, client *api.Client, opts ChannelHistoryOpts) ([]types.CompactMessage, []types.ThreadUpdate, error) {
	if opts.Limit <= 0 {
		opts.Limit = 50
	}
	if opts.Limit > 200 {
		opts.Limit = 200
	}
	if opts.MaxBodyChars == 0 {
		opts.MaxBodyChars = 8000
	}
	oldest := opts.Oldest

	// Step 1: Fetch channel history WITHOUT oldest filter to get recent thread parents.
	scanParams := map[string]string{
		"channel": opts.ChannelID,
		"limit":   strconv.Itoa(opts.Limit),
	}
	if opts.Latest != "" {
		scanParams["latest"] = opts.Latest
	}

	resp, err := client.Call(ctx, "conversations.history", scanParams)
	if err != nil {
		return nil, nil, err
	}

	rawMsgs := api.GetSlice(resp["messages"])
	var newMessages []types.CompactMessage
	var threadUpdates []types.ThreadUpdate

	for _, m := range rawMsgs {
		raw, ok := m.(map[string]interface{})
		if !ok {
			continue
		}

		ts := api.GetStringFromMap(raw, "ts")
		latestReply := api.GetStringFromMap(raw, "latest_reply")
		replyCount := api.GetIntFromMap(raw, "reply_count")

		// New top-level message
		if oldest == "" || ts > oldest {
			newMessages = append(newMessages, *parseRawMessage(opts.ChannelID, raw, opts.MaxBodyChars, opts.IncludeReactions))
		}

		// Thread with new replies (parent may be old, but replies are new)
		if replyCount > 0 && latestReply != "" && oldest != "" && latestReply > oldest {
			// Fetch only new replies using oldest parameter
			threadReplies, err := fetchThreadRepliesSince(ctx, client, opts.ChannelID, ts, oldest, opts.IncludeReactions, opts.MaxBodyChars)
			if err != nil {
				continue
			}
			if len(threadReplies) > 0 {
				parentMsg := parseRawMessage(opts.ChannelID, raw, 200, false) // short preview
				update := types.ThreadUpdate{
					ThreadTS:     ts,
					ParentAuthor: parentMsg.Author,
					NewReplies:   threadReplies,
				}
				if parentMsg.Content != "" {
					preview := parentMsg.Content
					if len(preview) > 200 {
						preview = preview[:200] + "..."
					}
					update.ParentPreview = preview
				}
				threadUpdates = append(threadUpdates, update)
			}
		}
	}

	// Sort new messages chronologically
	sort.Slice(newMessages, func(i, j int) bool {
		return newMessages[i].TS < newMessages[j].TS
	})

	return newMessages, threadUpdates, nil
}

// fetchThreadRepliesSince fetches thread replies newer than oldest timestamp.
func fetchThreadRepliesSince(ctx context.Context, client *api.Client, channelID, threadTS, oldest string, includeReactions bool, maxBodyChars int) ([]types.CompactMessage, error) {
	var replies []types.CompactMessage
	cursor := ""

	for {
		params := map[string]string{
			"channel": channelID,
			"ts":      threadTS,
			"oldest":  oldest,
			"limit":   "200",
		}
		if cursor != "" {
			params["cursor"] = cursor
		}

		resp, err := client.Call(ctx, "conversations.replies", params)
		if err != nil {
			return nil, err
		}

		rawMsgs := api.GetSlice(resp["messages"])
		for _, m := range rawMsgs {
			if raw, ok := m.(map[string]interface{}); ok {
				msgTS := api.GetStringFromMap(raw, "ts")
				// Skip the thread parent itself, only include actual replies
				if msgTS == threadTS {
					continue
				}
				// Only include replies newer than oldest
				if msgTS > oldest {
					replies = append(replies, *parseRawMessage(channelID, raw, maxBodyChars, includeReactions))
				}
			}
		}

		cursor = api.ExtractCursor(resp)
		if cursor == "" {
			break
		}
	}

	sort.Slice(replies, func(i, j int) bool {
		return replies[i].TS < replies[j].TS
	})

	return replies, nil
}

// SendMessage posts a message to a channel, optionally in a thread.
func SendMessage(ctx context.Context, client *api.Client, channelID, text, threadTS string) (*types.CompactMessage, error) {
	params := map[string]string{
		"channel": channelID,
		"text":    text,
	}
	if threadTS != "" {
		params["thread_ts"] = threadTS
	}

	resp, err := client.Call(ctx, "chat.postMessage", params)
	if err != nil {
		return nil, err
	}

	ts := api.GetString(resp["ts"])
	return &types.CompactMessage{
		ChannelID: channelID,
		TS:        ts,
		ThreadTS:  threadTS,
		Content:   text,
	}, nil
}

// EditMessage updates an existing message.
func EditMessage(ctx context.Context, client *api.Client, channelID, ts, text string) error {
	_, err := client.Call(ctx, "chat.update", map[string]string{
		"channel": channelID,
		"ts":      ts,
		"text":    text,
	})
	return err
}

// DeleteMessage removes a message.
func DeleteMessage(ctx context.Context, client *api.Client, channelID, ts string) error {
	_, err := client.Call(ctx, "chat.delete", map[string]string{
		"channel": channelID,
		"ts":      ts,
	})
	return err
}

// AddReaction adds an emoji reaction to a message.
func AddReaction(ctx context.Context, client *api.Client, channelID, ts, emoji string) error {
	_, err := client.Call(ctx, "reactions.add", map[string]string{
		"channel":   channelID,
		"timestamp": ts,
		"name":      normalizeReactionName(emoji),
	})
	return err
}

// RemoveReaction removes an emoji reaction from a message.
func RemoveReaction(ctx context.Context, client *api.Client, channelID, ts, emoji string) error {
	_, err := client.Call(ctx, "reactions.remove", map[string]string{
		"channel":   channelID,
		"timestamp": ts,
		"name":      normalizeReactionName(emoji),
	})
	return err
}

func parseRawMessage(channelID string, raw map[string]interface{}, maxBodyChars int, includeReactions bool) *types.CompactMessage {
	text := api.GetStringFromMap(raw, "text")
	ts := api.GetStringFromMap(raw, "ts")
	threadTS := api.GetStringFromMap(raw, "thread_ts")
	user := api.GetStringFromMap(raw, "user")
	botID := api.GetStringFromMap(raw, "bot_id")

	content := text
	if maxBodyChars >= 0 && len(content) > maxBodyChars {
		content = content[:maxBodyChars] + "\n..."
	}

	msg := &types.CompactMessage{
		ChannelID:   channelID,
		TS:          ts,
		ThreadTS:    threadTS,
		ReplyCount:  api.GetIntFromMap(raw, "reply_count"),
		LatestReply: api.GetStringFromMap(raw, "latest_reply"),
		Content:     content,
	}

	if user != "" || botID != "" {
		msg.Author = &types.MessageAuthor{UserID: user, BotID: botID}
	}

	// Parse files
	if files := api.GetSlice(raw["files"]); files != nil {
		for _, f := range files {
			if fm, ok := f.(map[string]interface{}); ok {
				msg.Files = append(msg.Files, types.CompactFile{
					Name:      api.GetStringFromMap(fm, "name"),
					Title:     api.GetStringFromMap(fm, "title"),
					Mimetype:  api.GetStringFromMap(fm, "mimetype"),
					Mode:      api.GetStringFromMap(fm, "mode"),
					Permalink: api.GetStringFromMap(fm, "permalink"),
					Size:      api.GetIntFromMap(fm, "size"),
				})
			}
		}
	}

	// Parse reactions
	if includeReactions {
		if reactions := api.GetSlice(raw["reactions"]); reactions != nil {
			for _, r := range reactions {
				if rm, ok := r.(map[string]interface{}); ok {
					name := api.GetStringFromMap(rm, "name")
					var users []string
					if userList := api.GetSlice(rm["users"]); userList != nil {
						for _, u := range userList {
							if s, ok := u.(string); ok {
								users = append(users, s)
							}
						}
					}
					count := api.GetIntFromMap(rm, "count")
					if count == len(users) {
						count = 0 // omit if redundant with users list
					}
					msg.Reactions = append(msg.Reactions, types.CompactReaction{
						Name:  name,
						Users: users,
						Count: count,
					})
				}
			}
		}
	}

	return msg
}

func normalizeReactionName(input string) string {
	// Strip surrounding colons: :thumbsup: -> thumbsup
	name := input
	if len(name) > 2 && name[0] == ':' && name[len(name)-1] == ':' {
		name = name[1 : len(name)-1]
	}
	return name
}
