package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/nullify/slack-cli/internal/api"
	"github.com/nullify/slack-cli/internal/output"
	"github.com/nullify/slack-cli/internal/slack"
	"github.com/nullify/slack-cli/internal/types"
	"github.com/nullify/slack-cli/internal/urlparse"
	"github.com/spf13/cobra"
)

var messageCmd = &cobra.Command{
	Use:   "message",
	Short: "Read/write Slack messages",
}

var messageGetCmd = &cobra.Command{
	Use:   "get <target>",
	Short: "Fetch a single Slack message",
	Long: `Fetch a single message by Slack URL, #channel with --ts, or channel ID with --ts.

Target can be:
  - A Slack message URL (https://workspace.slack.com/archives/C.../p...)
  - #channel-name (requires --ts)
  - A channel ID like C01234ABC (requires --ts)`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ts, _ := cmd.Flags().GetString("ts")
		threadTS, _ := cmd.Flags().GetString("thread-ts")
		maxBody, _ := cmd.Flags().GetInt("max-body-chars")
		includeReactions, _ := cmd.Flags().GetBool("include-reactions")

		target := urlparse.ParseMsgTarget(args[0])
		client, err := getClient()
		if err != nil {
			return err
		}

		channelID, messageTS, err := resolveTargetToChannelAndTS(cmd, client, target, ts, threadTS)
		if err != nil {
			return err
		}

		msg, err := slack.FetchMessage(cmd.Context(), client, channelID, messageTS, threadTS, includeReactions, maxBody)
		if err != nil {
			return err
		}

		return output.PrintJSON(map[string]interface{}{"message": msg})
	},
}

var messageListCmd = &cobra.Command{
	Use:   "list <target>",
	Short: "List channel messages or thread replies",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		threadTS, _ := cmd.Flags().GetString("thread-ts")
		ts, _ := cmd.Flags().GetString("ts")
		limit, _ := cmd.Flags().GetInt("limit")
		oldest, _ := cmd.Flags().GetString("oldest")
		latest, _ := cmd.Flags().GetString("latest")
		maxBody, _ := cmd.Flags().GetInt("max-body-chars")
		includeReactions, _ := cmd.Flags().GetBool("include-reactions")
		includeThreads, _ := cmd.Flags().GetBool("include-threads")

		target := urlparse.ParseMsgTarget(args[0])
		client, err := getClient()
		if err != nil {
			return err
		}

		channelID, err := resolveTargetToChannel(cmd, client, target)
		if err != nil {
			return err
		}

		// Determine if we should fetch a thread
		effectiveThreadTS := threadTS
		if effectiveThreadTS == "" && ts != "" {
			effectiveThreadTS = ts
		}
		if effectiveThreadTS == "" && target.Kind == types.TargetURL && target.Ref != nil {
			if target.Ref.ThreadTSHint != "" {
				effectiveThreadTS = target.Ref.ThreadTSHint
			} else {
				effectiveThreadTS = target.Ref.MessageTS
			}
		}

		if effectiveThreadTS != "" {
			messages, err := slack.FetchThread(cmd.Context(), client, channelID, effectiveThreadTS, includeReactions, maxBody)
			if err != nil {
				return err
			}
			return output.PrintJSON(map[string]interface{}{
				"channel_id": channelID,
				"thread_ts":  effectiveThreadTS,
				"messages":   messages,
			})
		}

		// Activity mode: find new top-level messages AND new thread replies
		if includeThreads && oldest != "" {
			newMessages, threadUpdates, err := slack.FetchChannelActivity(cmd.Context(), client, slack.ChannelHistoryOpts{
				ChannelID:        channelID,
				Limit:            limit,
				Oldest:           oldest,
				Latest:           latest,
				IncludeReactions: includeReactions,
				MaxBodyChars:     maxBody,
			})
			if err != nil {
				return err
			}
			result := map[string]interface{}{
				"channel_id": channelID,
				"messages":   newMessages,
			}
			if len(threadUpdates) > 0 {
				result["thread_updates"] = threadUpdates
			}
			return output.PrintJSON(result)
		}

		// Channel history mode
		messages, err := slack.FetchChannelHistory(cmd.Context(), client, slack.ChannelHistoryOpts{
			ChannelID:        channelID,
			Limit:            limit,
			Oldest:           oldest,
			Latest:           latest,
			IncludeReactions: includeReactions,
			MaxBodyChars:     maxBody,
		})
		if err != nil {
			return err
		}

		return output.PrintJSON(map[string]interface{}{
			"channel_id": channelID,
			"messages":   messages,
		})
	},
}

var messageSendCmd = &cobra.Command{
	Use:   "send <target> [text]",
	Short: "Send a message (optionally into a thread)",
	Long: `Send a message to a channel, user, or thread.

The message body can be supplied three ways (checked in this order):
  - --file <path>  read the body from a file ('-' means stdin)
  - <text>         a positional argument
  - stdin          piped input when no text argument is given

Prefer --file or stdin for multi-line or formatted messages: passing the body
as a shell argument means the shell interprets backticks, $, and quotes, which
silently corrupts Slack mrkdwn. A quoted heredoc avoids this entirely:

  slack-cli message send C01234ABC - <<'EOF'
  :rotating_light: *Alert* with ` + "`code`" + ` and 'quotes' kept literal
  EOF`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		threadTS, _ := cmd.Flags().GetString("thread-ts")

		text, err := resolveMessageText(cmd, args, 1)
		if err != nil {
			return err
		}

		target := urlparse.ParseMsgTarget(args[0])
		client, err := getClient()
		if err != nil {
			return err
		}

		channelID, err := resolveTargetToChannel(cmd, client, target)
		if err != nil {
			return err
		}

		// If target is a URL, reply in that thread
		if target.Kind == types.TargetURL && target.Ref != nil && threadTS == "" {
			if target.Ref.ThreadTSHint != "" {
				threadTS = target.Ref.ThreadTSHint
			} else {
				threadTS = target.Ref.MessageTS
			}
		}

		msg, err := slack.SendMessage(cmd.Context(), client, channelID, text, threadTS)
		if err != nil {
			return err
		}

		return output.PrintJSON(msg)
	},
}

var messageEditCmd = &cobra.Command{
	Use:   "edit <target> [text]",
	Short: "Edit an existing message",
	Long: `Edit an existing message. The new body can be supplied via --file <path>
('-' means stdin), a positional argument, or piped stdin (checked in that
order). Prefer --file or stdin for formatted messages so the shell does not
interpret backticks, $, or quotes in the body.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ts, _ := cmd.Flags().GetString("ts")

		text, err := resolveMessageText(cmd, args, 1)
		if err != nil {
			return err
		}

		target := urlparse.ParseMsgTarget(args[0])
		client, err := getClient()
		if err != nil {
			return err
		}

		channelID, messageTS, err := resolveTargetToChannelAndTS(cmd, client, target, ts, "")
		if err != nil {
			return err
		}

		if err := slack.EditMessage(cmd.Context(), client, channelID, messageTS, text); err != nil {
			return err
		}

		return output.PrintJSON(map[string]string{"ok": "true", "channel_id": channelID, "ts": messageTS})
	},
}

var messageDeleteCmd = &cobra.Command{
	Use:   "delete <target>",
	Short: "Delete a message",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ts, _ := cmd.Flags().GetString("ts")

		target := urlparse.ParseMsgTarget(args[0])
		client, err := getClient()
		if err != nil {
			return err
		}

		channelID, messageTS, err := resolveTargetToChannelAndTS(cmd, client, target, ts, "")
		if err != nil {
			return err
		}

		if err := slack.DeleteMessage(cmd.Context(), client, channelID, messageTS); err != nil {
			return err
		}

		return output.PrintJSON(map[string]string{"ok": "true", "channel_id": channelID, "ts": messageTS})
	},
}

var messageReactCmd = &cobra.Command{
	Use:   "react",
	Short: "Add or remove reactions",
}

var messageReactAddCmd = &cobra.Command{
	Use:   "add <target> <emoji>",
	Short: "Add a reaction to a message",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ts, _ := cmd.Flags().GetString("ts")

		target := urlparse.ParseMsgTarget(args[0])
		client, err := getClient()
		if err != nil {
			return err
		}

		channelID, messageTS, err := resolveTargetToChannelAndTS(cmd, client, target, ts, "")
		if err != nil {
			return err
		}

		if err := slack.AddReaction(cmd.Context(), client, channelID, messageTS, args[1]); err != nil {
			return err
		}

		return output.PrintJSON(map[string]string{"ok": "true", "reaction": args[1]})
	},
}

var messageReactRemoveCmd = &cobra.Command{
	Use:   "remove <target> <emoji>",
	Short: "Remove a reaction from a message",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ts, _ := cmd.Flags().GetString("ts")

		target := urlparse.ParseMsgTarget(args[0])
		client, err := getClient()
		if err != nil {
			return err
		}

		channelID, messageTS, err := resolveTargetToChannelAndTS(cmd, client, target, ts, "")
		if err != nil {
			return err
		}

		if err := slack.RemoveReaction(cmd.Context(), client, channelID, messageTS, args[1]); err != nil {
			return err
		}

		return output.PrintJSON(map[string]string{"ok": "true", "reaction": args[1]})
	},
}

// resolveMessageText resolves a message body from, in order of precedence:
// the --file flag ('-' means stdin), the positional text argument at textArg
// ('-' also means stdin there), or piped stdin when neither is given. This
// lets callers avoid passing formatted message bodies as shell arguments,
// where backticks, $, and quotes would otherwise be interpreted by the shell
// and corrupt the message.
func resolveMessageText(cmd *cobra.Command, args []string, textArg int) (string, error) {
	file, _ := cmd.Flags().GetString("file")

	switch {
	case file == "-":
		return readAllStdin()
	case file != "":
		b, err := os.ReadFile(file)
		if err != nil {
			return "", fmt.Errorf("reading message body from %s: %w", file, err)
		}
		return string(b), nil
	case len(args) > textArg && args[textArg] == "-":
		return readAllStdin()
	case len(args) > textArg:
		return args[textArg], nil
	case stdinIsPiped():
		return readAllStdin()
	default:
		return "", fmt.Errorf("no message body: provide a text argument, --file, or pipe via stdin")
	}
}

// stdinIsPiped reports whether stdin is a pipe or file rather than an
// interactive terminal, so we only block on a read when input is actually
// being fed in.
func stdinIsPiped() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice == 0
}

func readAllStdin() (string, error) {
	b, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("reading message body from stdin: %w", err)
	}
	if len(b) == 0 {
		return "", fmt.Errorf("no message body: provide a text argument, --file, or pipe via stdin")
	}
	return string(b), nil
}

// resolveTargetToChannel resolves any target to a channel ID.
func resolveTargetToChannel(cmd *cobra.Command, client *api.Client, target *types.MsgTarget) (string, error) {
	ctx := cmd.Context()
	switch target.Kind {
	case types.TargetURL:
		return target.Ref.ChannelID, nil
	case types.TargetUser:
		return slack.OpenDM(ctx, client, target.UserID)
	case types.TargetChannel:
		return slack.ResolveChannelID(ctx, client, target.Channel)
	default:
		return "", fmt.Errorf("unknown target type")
	}
}

// resolveTargetToChannelAndTS resolves target + optional --ts flag to channel + timestamp.
func resolveTargetToChannelAndTS(cmd *cobra.Command, client *api.Client, target *types.MsgTarget, ts, threadTS string) (string, string, error) {
	ctx := cmd.Context()
	switch target.Kind {
	case types.TargetURL:
		return target.Ref.ChannelID, target.Ref.MessageTS, nil
	case types.TargetChannel:
		if ts == "" {
			return "", "", fmt.Errorf("--ts is required when targeting a channel")
		}
		channelID, err := slack.ResolveChannelID(ctx, client, target.Channel)
		if err != nil {
			return "", "", err
		}
		return channelID, ts, nil
	default:
		return "", "", fmt.Errorf("cannot resolve message timestamp from this target type; use a URL or channel with --ts")
	}
}

func init() {
	rootCmd.AddCommand(messageCmd)
	messageCmd.AddCommand(messageGetCmd, messageListCmd, messageSendCmd, messageEditCmd, messageDeleteCmd, messageReactCmd)
	messageReactCmd.AddCommand(messageReactAddCmd, messageReactRemoveCmd)

	// message get flags
	messageGetCmd.Flags().String("ts", "", "Message timestamp (required with channel target)")
	messageGetCmd.Flags().String("thread-ts", "", "Thread root timestamp hint")
	messageGetCmd.Flags().Int("max-body-chars", 8000, "Max content characters (-1 for unlimited)")
	messageGetCmd.Flags().Bool("include-reactions", false, "Include reactions")

	// message list flags
	messageListCmd.Flags().String("thread-ts", "", "Thread root ts (list thread instead of channel)")
	messageListCmd.Flags().String("ts", "", "Message ts (resolve to its thread)")
	messageListCmd.Flags().Int("limit", 25, "Max messages (max 200)")
	messageListCmd.Flags().String("oldest", "", "Only messages after this ts")
	messageListCmd.Flags().String("latest", "", "Only messages before this ts")
	messageListCmd.Flags().Int("max-body-chars", 8000, "Max content characters (-1 for unlimited)")
	messageListCmd.Flags().Bool("include-reactions", false, "Include reactions")
	messageListCmd.Flags().Bool("include-threads", false, "Also fetch new thread replies (use with --oldest)")

	// message send flags
	messageSendCmd.Flags().String("thread-ts", "", "Thread root ts to reply into")
	messageSendCmd.Flags().String("file", "", "Read message body from a file ('-' for stdin) instead of the text argument")

	// message edit flags
	messageEditCmd.Flags().String("ts", "", "Message ts (required with channel target)")
	messageEditCmd.Flags().String("file", "", "Read new message body from a file ('-' for stdin) instead of the text argument")

	// message delete flags
	messageDeleteCmd.Flags().String("ts", "", "Message ts (required with channel target)")

	// message react flags
	messageReactAddCmd.Flags().String("ts", "", "Message ts (required with channel target)")
	messageReactRemoveCmd.Flags().String("ts", "", "Message ts (required with channel target)")
}
