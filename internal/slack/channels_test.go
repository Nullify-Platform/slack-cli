package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nullify/slack-cli/internal/api"
	"github.com/nullify/slack-cli/internal/types"
)

func fakeConversationsList(t *testing.T, pages [][]string) *httptest.Server {
	t.Helper()
	call := 0
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if call >= len(pages) {
			t.Fatalf("unexpected extra call %d to conversations.list", call+1)
		}
		names := pages[call]
		call++

		channels := make([]map[string]any, len(names))
		for i, n := range names {
			channels[i] = map[string]any{"id": fmt.Sprintf("C%d", i), "name": n}
		}

		nextCursor := ""
		if call < len(pages) {
			nextCursor = fmt.Sprintf("cursor-%d", call)
		}

		resp := map[string]any{
			"ok":                true,
			"channels":          channels,
			"response_metadata": map[string]any{"next_cursor": nextCursor},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func testClient(baseURL string) *api.Client {
	return api.NewClient(&types.AuthConfig{
		Mode:         types.AuthBrowser,
		Token:        "xoxc-test",
		Cookie:       "xoxd-test",
		WorkspaceURL: baseURL,
	})
}

func TestListChannels_AllAutoPaginatesAcrossShardedPages(t *testing.T) {
	srv := fakeConversationsList(t, [][]string{
		{"general", "nul-team"},
		{"internal-acme", "internal-globex"},
		{"nul-random"},
	})
	defer srv.Close()

	result, err := ListChannels(context.Background(), testClient(srv.URL), ListChannelsOpts{All: true})
	if err != nil {
		t.Fatalf("ListChannels: %v", err)
	}

	if len(result.Channels) != 5 {
		t.Fatalf("expected 5 channels merged across pages, got %d: %+v", len(result.Channels), result.Channels)
	}
	if result.NextCursor != "" {
		t.Fatalf("expected no next_cursor after exhausting pages, got %q", result.NextCursor)
	}
	if result.Truncated {
		t.Fatalf("expected Truncated=false when pagination completes within the cap")
	}

	found := false
	for _, ch := range result.Channels {
		if ch.Name == "internal-acme" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected internal-acme in merged results, got %+v", result.Channels)
	}
}

func TestListChannels_ExplicitCursorFetchesSinglePage(t *testing.T) {
	srv := fakeConversationsList(t, [][]string{
		{"general"},
		{"internal-acme"},
	})
	defer srv.Close()

	result, err := ListChannels(context.Background(), testClient(srv.URL), ListChannelsOpts{All: true, Cursor: "some-cursor"})
	if err != nil {
		t.Fatalf("ListChannels: %v", err)
	}

	if len(result.Channels) != 1 {
		t.Fatalf("expected exactly 1 page of channels with explicit cursor, got %d", len(result.Channels))
	}
}

func TestListChannels_TruncatesAtPageCap(t *testing.T) {
	old := maxAutoPaginatePages
	maxAutoPaginatePages = 2
	defer func() { maxAutoPaginatePages = old }()

	srv := fakeConversationsList(t, [][]string{
		{"a"}, {"b"}, {"c"},
	})
	defer srv.Close()

	result, err := ListChannels(context.Background(), testClient(srv.URL), ListChannelsOpts{All: true})
	if err != nil {
		t.Fatalf("ListChannels: %v", err)
	}

	if !result.Truncated {
		t.Fatalf("expected Truncated=true when the page cap is hit before exhaustion")
	}
	if result.NextCursor == "" {
		t.Fatalf("expected a resumable NextCursor when truncated")
	}
	if len(result.Channels) != 2 {
		t.Fatalf("expected 2 channels (one per allowed page) before truncation, got %d", len(result.Channels))
	}
}
