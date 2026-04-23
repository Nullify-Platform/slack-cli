package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strconv"

	"github.com/nullify/slack-cli/internal/api"
	"github.com/nullify/slack-cli/internal/types"
)

// UploadOpts controls how a file is uploaded to Slack.
type UploadOpts struct {
	FilePath  string
	ChannelID string
	Filename  string // overrides the base name of FilePath
	Title     string
	Message   string
	ThreadTS  string
}

// UploadFile uploads a local file to Slack using the three-step external upload API:
// files.getUploadURLExternal → POST to signed URL → files.completeUploadExternal.
func UploadFile(ctx context.Context, client *api.Client, opts UploadOpts) (*types.FileInfo, error) {
	stat, err := os.Stat(opts.FilePath)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	filename := opts.Filename
	if filename == "" {
		filename = filepath.Base(opts.FilePath)
	}
	title := opts.Title
	if title == "" {
		title = filename
	}

	contentType := mime.TypeByExtension(filepath.Ext(filename))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Step 1: get a pre-signed upload URL and file ID.
	urlResp, err := client.Call(ctx, "files.getUploadURLExternal", map[string]string{
		"filename": filename,
		"length":   strconv.FormatInt(stat.Size(), 10),
	})
	if err != nil {
		return nil, fmt.Errorf("files.getUploadURLExternal: %w", err)
	}
	uploadURL := api.GetString(urlResp["upload_url"])
	fileID := api.GetString(urlResp["file_id"])
	if uploadURL == "" || fileID == "" {
		return nil, fmt.Errorf("files.getUploadURLExternal: missing upload_url or file_id in response")
	}

	// Step 2: stream file content to the pre-signed URL.
	f, err := os.Open(opts.FilePath)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	if err := client.UploadToURL(ctx, uploadURL, f, stat.Size(), contentType); err != nil {
		return nil, err
	}

	// Step 3: complete the upload, optionally sharing to a channel.
	filesParam, err := json.Marshal([]map[string]string{{"id": fileID, "title": title}})
	if err != nil {
		return nil, fmt.Errorf("encoding files param: %w", err)
	}
	completeParams := map[string]string{
		"files":           string(filesParam),
		"channel_id":      opts.ChannelID,
		"initial_comment": opts.Message,
		"thread_ts":       opts.ThreadTS,
	}
	if _, err := client.Call(ctx, "files.completeUploadExternal", completeParams); err != nil {
		return nil, fmt.Errorf("files.completeUploadExternal: %w", err)
	}

	return GetFileInfo(ctx, client, fileID)
}

// GetFileInfo calls files.info and returns compact file metadata.
func GetFileInfo(ctx context.Context, client *api.Client, fileID string) (*types.FileInfo, error) {
	resp, err := client.Call(ctx, "files.info", map[string]string{"file": fileID})
	if err != nil {
		return nil, fmt.Errorf("files.info: %w", err)
	}

	f := api.GetMap(resp["file"])
	if f == nil {
		return nil, fmt.Errorf("files.info: missing file object in response")
	}

	return &types.FileInfo{
		ID:         api.GetStringFromMap(f, "id"),
		Name:       api.GetStringFromMap(f, "name"),
		Title:      api.GetStringFromMap(f, "title"),
		Mimetype:   api.GetStringFromMap(f, "mimetype"),
		Filetype:   api.GetStringFromMap(f, "filetype"),
		Size:       api.GetIntFromMap(f, "size"),
		URLPrivate: api.GetStringFromMap(f, "url_private"),
		Permalink:  api.GetStringFromMap(f, "permalink"),
		Created:    int64(api.GetIntFromMap(f, "created")),
		User:       api.GetStringFromMap(f, "user"),
	}, nil
}
