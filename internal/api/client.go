package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/nullify/slack-cli/internal/types"
)

const (
	defaultBaseURL = "https://slack.com"
	userAgent      = "slack-cli/0.1.0"
	maxRetries     = 3
	maxRetryDelay  = 30 * time.Second
)

// SlackAPIError represents an error returned by the Slack API.
type SlackAPIError struct {
	Method  string
	Code    string
	Message string
}

func (e *SlackAPIError) Error() string { return e.Message }

// Client is the Slack API HTTP client.
type Client struct {
	auth       *types.AuthConfig
	httpClient *http.Client
	baseURL    string
}

// NewClient constructs a Client from an AuthConfig.
func NewClient(auth *types.AuthConfig) *Client {
	baseURL := defaultBaseURL
	if auth.Mode == types.AuthBrowser && auth.WorkspaceURL != "" {
		baseURL = auth.WorkspaceURL
	}
	return &Client{
		auth:       auth,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    baseURL,
	}
}

// Call executes a Slack API method and returns the parsed response.
func (c *Client) Call(ctx context.Context, method string, params map[string]string) (types.APIResponse, error) {
	apiURL := c.baseURL + "/api/" + method

	form := url.Values{}
	if c.auth.Mode == types.AuthBrowser {
		form.Set("token", c.auth.Token)
	}
	for k, v := range params {
		if v != "" {
			form.Set(k, v)
		}
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(form.Encode()))
		if err != nil {
			return nil, fmt.Errorf("building request for %s: %w", method, err)
		}

		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("User-Agent", userAgent)

		if c.auth.Mode == types.AuthBrowser {
			req.Header.Set("Cookie", "d="+url.QueryEscape(c.auth.Cookie))
			req.Header.Set("Origin", "https://app.slack.com")
		} else {
			req.Header.Set("Authorization", "Bearer "+c.auth.Token)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("HTTP error calling %s: %w", method, err)
			continue
		}

		if resp.StatusCode == 429 && attempt < maxRetries {
			retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"), 5)
			delay := time.Duration(retryAfter) * time.Second
			if delay > maxRetryDelay {
				delay = maxRetryDelay
			}
			fmt.Fprintf(os.Stderr, "Rate limited on %s, retrying in %v...\n", method, delay)
			resp.Body.Close()
			time.Sleep(delay)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("reading response from %s: %w", method, err)
		}

		var result types.APIResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("parsing JSON from %s: %w (status %d)", method, err, resp.StatusCode)
		}

		if ok, exists := result["ok"]; !exists || ok != true {
			errCode := GetString(result["error"])
			if errCode == "" {
				errCode = "unknown_error"
			}
			return result, &SlackAPIError{
				Method:  method,
				Code:    errCode,
				Message: fmt.Sprintf("slack API error calling %s: %s", method, errCode),
			}
		}

		return result, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("max retries exceeded for %s", method)
}

// UploadToURL POSTs raw file content to a pre-signed Slack upload URL.
// The upload URL itself is authorization — no Slack auth headers are sent.
func (c *Client) UploadToURL(ctx context.Context, uploadURL string, r io.Reader, size int64, contentType string) error {
	req, err := http.NewRequestWithContext(ctx, "POST", uploadURL, r)
	if err != nil {
		return fmt.Errorf("building upload request: %w", err)
	}
	req.ContentLength = size
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("uploading file: %w", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("upload failed with HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// Download performs an authenticated GET on a private Slack URL (e.g. url_private)
// and returns the response body. The caller must close it.
func (c *Client) Download(ctx context.Context, privateURL string) (io.ReadCloser, string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", privateURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("building download request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Authorization", "Bearer "+c.auth.Token)
	if c.auth.Mode == types.AuthBrowser {
		req.Header.Set("Cookie", "d="+url.QueryEscape(c.auth.Cookie))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("downloading file: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, "", fmt.Errorf("download failed with HTTP %d", resp.StatusCode)
	}
	return resp.Body, resp.Header.Get("Content-Type"), nil
}

func parseRetryAfter(header string, defaultSec int) int {
	if header == "" {
		return defaultSec
	}
	n, err := strconv.Atoi(header)
	if err != nil || n < 1 {
		return defaultSec
	}
	return n
}

// --- Type-safe accessors for untyped JSON ---

// GetString safely extracts a string from an interface{}.
func GetString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// GetFloat safely extracts a float64 from an interface{}.
func GetFloat(v interface{}) float64 {
	if f, ok := v.(float64); ok {
		return f
	}
	return 0
}

// GetInt safely extracts an int from a JSON number (float64).
func GetInt(v interface{}) int {
	if f, ok := v.(float64); ok {
		return int(f)
	}
	return 0
}

// GetBool safely extracts a bool from an interface{}.
func GetBool(v interface{}) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

// GetMap safely extracts a map from an interface{}.
func GetMap(v interface{}) map[string]interface{} {
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	return nil
}

// GetSlice safely extracts a slice from an interface{}.
func GetSlice(v interface{}) []interface{} {
	if s, ok := v.([]interface{}); ok {
		return s
	}
	return nil
}

// GetStringFromMap extracts a string field from a map.
func GetStringFromMap(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	return GetString(m[key])
}

// GetIntFromMap extracts an int field from a map.
func GetIntFromMap(m map[string]interface{}, key string) int {
	if m == nil {
		return 0
	}
	return GetInt(m[key])
}

// GetBoolFromMap extracts a bool field from a map.
func GetBoolFromMap(m map[string]interface{}, key string) bool {
	if m == nil {
		return false
	}
	return GetBool(m[key])
}

// GetMapFromMap extracts a nested map field.
func GetMapFromMap(m map[string]interface{}, key string) map[string]interface{} {
	if m == nil {
		return nil
	}
	return GetMap(m[key])
}

// GetSliceFromMap extracts a slice field from a map.
func GetSliceFromMap(m map[string]interface{}, key string) []interface{} {
	if m == nil {
		return nil
	}
	return GetSlice(m[key])
}

// ExtractCursor gets the next_cursor from response_metadata.
func ExtractCursor(resp types.APIResponse) string {
	meta := GetMap(resp["response_metadata"])
	if meta == nil {
		return ""
	}
	return GetString(meta["next_cursor"])
}
