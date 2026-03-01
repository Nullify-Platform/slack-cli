package slack

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/nullify/slack-cli/internal/api"
	"github.com/nullify/slack-cli/internal/types"
)

// SearchOpts configures SearchSlack.
type SearchOpts struct {
	Query           string
	Kind            string // "messages", "files", "all"
	Channels        []string
	User            string
	After           string // YYYY-MM-DD
	Before          string // YYYY-MM-DD
	Limit           int    // default 20, max 100
	MaxContentChars int    // default 4000
}

// SearchResult is the output of SearchSlack.
type SearchResult struct {
	Messages []types.CompactMessage `json:"messages,omitempty"`
	Files    []SearchFileResult    `json:"files,omitempty"`
}

// SearchFileResult represents a file search result.
type SearchFileResult struct {
	ID        string `json:"id,omitempty"`
	Title     string `json:"title,omitempty"`
	Mimetype  string `json:"mimetype,omitempty"`
	Filetype  string `json:"filetype,omitempty"`
	Name      string `json:"name,omitempty"`
	Permalink string `json:"permalink,omitempty"`
	Size      int    `json:"size,omitempty"`
}

// SearchSlack performs a search across messages and/or files.
func SearchSlack(ctx context.Context, client *api.Client, opts SearchOpts) (*SearchResult, error) {
	if opts.Limit <= 0 {
		opts.Limit = 20
	}
	if opts.Limit > 100 {
		opts.Limit = 100
	}
	if opts.MaxContentChars == 0 {
		opts.MaxContentChars = 4000
	}

	query := buildSearchQuery(opts)
	result := &SearchResult{}

	if opts.Kind == "messages" || opts.Kind == "all" {
		msgs, err := searchMessages(ctx, client, query, opts.Limit, opts.MaxContentChars)
		if err != nil {
			return nil, err
		}
		result.Messages = msgs
	}

	if opts.Kind == "files" || opts.Kind == "all" {
		files, err := searchFiles(ctx, client, query, opts.Limit)
		if err != nil {
			return nil, err
		}
		result.Files = files
	}

	return result, nil
}

func buildSearchQuery(opts SearchOpts) string {
	parts := []string{opts.Query}

	for _, ch := range opts.Channels {
		ch = strings.TrimPrefix(ch, "#")
		parts = append(parts, "in:#"+ch)
	}
	if opts.User != "" {
		user := strings.TrimPrefix(opts.User, "@")
		parts = append(parts, "from:@"+user)
	}
	if opts.After != "" {
		parts = append(parts, "after:"+opts.After)
	}
	if opts.Before != "" {
		parts = append(parts, "before:"+opts.Before)
	}

	return strings.Join(parts, " ")
}

func searchMessages(ctx context.Context, client *api.Client, query string, limit, maxContentChars int) ([]types.CompactMessage, error) {
	var messages []types.CompactMessage
	page := 1
	remaining := limit

	for remaining > 0 {
		count := remaining
		if count > 100 {
			count = 100
		}

		resp, err := client.Call(ctx, "search.messages", map[string]string{
			"query": query,
			"count": strconv.Itoa(count),
			"page":  strconv.Itoa(page),
		})
		if err != nil {
			return nil, err
		}

		msgsObj := api.GetMap(resp["messages"])
		if msgsObj == nil {
			break
		}

		matches := api.GetSlice(msgsObj["matches"])
		if len(matches) == 0 {
			break
		}

		for _, m := range matches {
			if raw, ok := m.(map[string]interface{}); ok {
				channelID := ""
				if ch := api.GetMap(raw["channel"]); ch != nil {
					channelID = api.GetStringFromMap(ch, "id")
				}
				messages = append(messages, *parseRawMessage(channelID, raw, maxContentChars, false))
			}
		}

		// Check if there are more pages
		paging := api.GetMap(msgsObj["paging"])
		if paging == nil {
			break
		}
		totalPages := api.GetIntFromMap(paging, "pages")
		if page >= totalPages {
			break
		}

		remaining -= len(matches)
		page++
	}

	return messages, nil
}

func searchFiles(ctx context.Context, client *api.Client, query string, limit int) ([]SearchFileResult, error) {
	var files []SearchFileResult
	page := 1
	remaining := limit

	for remaining > 0 {
		count := remaining
		if count > 100 {
			count = 100
		}

		resp, err := client.Call(ctx, "search.files", map[string]string{
			"query": query,
			"count": strconv.Itoa(count),
			"page":  strconv.Itoa(page),
		})
		if err != nil {
			return nil, err
		}

		filesObj := api.GetMap(resp["files"])
		if filesObj == nil {
			break
		}

		matches := api.GetSlice(filesObj["matches"])
		if len(matches) == 0 {
			break
		}

		for _, f := range matches {
			if raw, ok := f.(map[string]interface{}); ok {
				files = append(files, SearchFileResult{
					ID:        api.GetStringFromMap(raw, "id"),
					Title:     api.GetStringFromMap(raw, "title"),
					Mimetype:  api.GetStringFromMap(raw, "mimetype"),
					Filetype:  api.GetStringFromMap(raw, "filetype"),
					Name:      api.GetStringFromMap(raw, "name"),
					Permalink: api.GetStringFromMap(raw, "permalink"),
					Size:      api.GetIntFromMap(raw, "size"),
				})
			}
		}

		// Check pagination
		paging := api.GetMap(filesObj["paging"])
		if paging == nil {
			break
		}
		totalPages := api.GetIntFromMap(paging, "pages")
		if page >= totalPages {
			break
		}

		remaining -= len(matches)
		page++
	}

	return files, nil
}

// ValidateDate checks YYYY-MM-DD format.
func ValidateDate(s string) error {
	if len(s) != 10 || s[4] != '-' || s[7] != '-' {
		return fmt.Errorf("invalid date format %q, expected YYYY-MM-DD", s)
	}
	return nil
}
