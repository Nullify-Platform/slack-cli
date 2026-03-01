package types

// AuthMode distinguishes standard vs browser auth.
type AuthMode int

const (
	AuthStandard AuthMode = iota // xoxb-*/xoxp-* bearer token
	AuthBrowser                  // xoxc-* token + xoxd-* cookie
)

// AuthConfig holds resolved authentication credentials.
type AuthConfig struct {
	Mode         AuthMode
	Token        string
	Cookie       string // xoxd cookie (browser mode only)
	WorkspaceURL string // required for browser mode
}

// SlackMessageRef represents a parsed Slack message URL.
type SlackMessageRef struct {
	WorkspaceURL string `json:"workspace_url,omitempty"`
	ChannelID    string `json:"channel_id"`
	MessageTS    string `json:"message_ts"`
	ThreadTSHint string `json:"thread_ts_hint,omitempty"`
	Raw          string `json:"raw,omitempty"`
}

// MsgTargetKind discriminates how a user references a message/channel.
type MsgTargetKind int

const (
	TargetURL     MsgTargetKind = iota // Slack message URL
	TargetChannel                      // Channel name or ID
	TargetUser                         // User ID for DM
)

// MsgTarget holds a parsed target reference.
type MsgTarget struct {
	Kind    MsgTargetKind
	Ref     *SlackMessageRef // when Kind == TargetURL
	Channel string           // when Kind == TargetChannel (raw input like "#general" or "C01234")
	UserID  string           // when Kind == TargetUser
}

// APIResponse represents a raw Slack API response.
type APIResponse map[string]interface{}

// CompactMessage is the token-efficient output format for messages.
type CompactMessage struct {
	ChannelID   string           `json:"channel_id,omitempty"`
	TS          string           `json:"ts"`
	ThreadTS    string           `json:"thread_ts,omitempty"`
	ReplyCount  int              `json:"reply_count,omitempty"`
	LatestReply string           `json:"latest_reply,omitempty"`
	Author      *MessageAuthor   `json:"author,omitempty"`
	Content     string           `json:"content,omitempty"`
	Files       []CompactFile    `json:"files,omitempty"`
	Reactions   []CompactReaction `json:"reactions,omitempty"`
}

// ThreadUpdate represents new replies in a thread since a given time.
type ThreadUpdate struct {
	ThreadTS      string           `json:"thread_ts"`
	ParentAuthor  *MessageAuthor   `json:"parent_author,omitempty"`
	ParentPreview string           `json:"parent_preview,omitempty"`
	NewReplies    []CompactMessage `json:"new_replies"`
}

// MessageAuthor identifies who sent a message.
type MessageAuthor struct {
	UserID string `json:"user_id,omitempty"`
	BotID  string `json:"bot_id,omitempty"`
}

// CompactFile is the token-efficient file metadata.
type CompactFile struct {
	Mimetype  string `json:"mimetype,omitempty"`
	Mode      string `json:"mode,omitempty"`
	Name      string `json:"name,omitempty"`
	Title     string `json:"title,omitempty"`
	Permalink string `json:"permalink,omitempty"`
	Size      int    `json:"size,omitempty"`
}

// CompactReaction is the token-efficient reaction format.
type CompactReaction struct {
	Name  string   `json:"name"`
	Users []string `json:"users,omitempty"`
	Count int      `json:"count,omitempty"`
}

// CompactUser is the token-efficient user format.
type CompactUser struct {
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
	RealName    string `json:"real_name,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	Email       string `json:"email,omitempty"`
	Title       string `json:"title,omitempty"`
	TZ          string `json:"tz,omitempty"`
	IsBot       *bool  `json:"is_bot,omitempty"`
	Deleted     *bool  `json:"deleted,omitempty"`
}

// CompactChannel is the token-efficient channel representation.
type CompactChannel struct {
	ID         string `json:"id"`
	Name       string `json:"name,omitempty"`
	IsPrivate  *bool  `json:"is_private,omitempty"`
	IsIM       *bool  `json:"is_im,omitempty"`
	IsMPIM     *bool  `json:"is_mpim,omitempty"`
	NumMembers int    `json:"num_members,omitempty"`
	Topic      string `json:"topic,omitempty"`
	Purpose    string `json:"purpose,omitempty"`
}
