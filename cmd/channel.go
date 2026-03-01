package cmd

import (
	"strings"

	"github.com/nullify/slack-cli/internal/output"
	"github.com/nullify/slack-cli/internal/slack"
	"github.com/nullify/slack-cli/internal/urlparse"
	"github.com/spf13/cobra"
)

var channelCmd = &cobra.Command{
	Use:   "channel",
	Short: "List conversations, create channels, and manage invites",
}

var channelListCmd = &cobra.Command{
	Use:   "list",
	Short: "List conversations",
	RunE: func(cmd *cobra.Command, args []string) error {
		all, _ := cmd.Flags().GetBool("all")
		user, _ := cmd.Flags().GetString("user")
		limit, _ := cmd.Flags().GetInt("limit")
		cursor, _ := cmd.Flags().GetString("cursor")

		client, err := getClient()
		if err != nil {
			return err
		}

		// Resolve user if needed
		userID := ""
		if user != "" {
			userID, err = slack.ResolveUserID(cmd.Context(), client, user)
			if err != nil {
				return err
			}
		}

		result, err := slack.ListChannels(cmd.Context(), client, slack.ListChannelsOpts{
			All:             all,
			UserID:          userID,
			Limit:           limit,
			Cursor:          cursor,
			ExcludeArchived: true,
		})
		if err != nil {
			return err
		}

		return output.PrintJSON(result)
	},
}

var channelNewCmd = &cobra.Command{
	Use:   "new",
	Short: "Create a new channel",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		private, _ := cmd.Flags().GetBool("private")

		client, err := getClient()
		if err != nil {
			return err
		}

		ch, err := slack.CreateChannel(cmd.Context(), client, name, private)
		if err != nil {
			return err
		}

		return output.PrintJSON(ch)
	},
}

var channelInviteCmd = &cobra.Command{
	Use:   "invite",
	Short: "Invite users to a channel",
	RunE: func(cmd *cobra.Command, args []string) error {
		channel, _ := cmd.Flags().GetString("channel")
		usersFlag, _ := cmd.Flags().GetString("users")

		client, err := getClient()
		if err != nil {
			return err
		}

		// Resolve channel
		channelID, err := slack.ResolveChannelID(cmd.Context(), client, channel)
		if err != nil {
			return err
		}

		// Resolve each user
		userInputs := strings.Split(usersFlag, ",")
		var userIDs []string
		for _, u := range userInputs {
			u = strings.TrimSpace(u)
			if u == "" {
				continue
			}
			if urlparse.IsUserID(u) {
				userIDs = append(userIDs, u)
			} else {
				id, err := slack.ResolveUserID(cmd.Context(), client, u)
				if err != nil {
					return err
				}
				userIDs = append(userIDs, id)
			}
		}

		result, err := slack.InviteToChannel(cmd.Context(), client, channelID, userIDs)
		if err != nil {
			return err
		}

		return output.PrintJSON(result)
	},
}

func init() {
	rootCmd.AddCommand(channelCmd)
	channelCmd.AddCommand(channelListCmd, channelNewCmd, channelInviteCmd)

	channelListCmd.Flags().String("user", "", "User ID, @handle, or email")
	channelListCmd.Flags().Bool("all", false, "List all workspace conversations")
	channelListCmd.Flags().Int("limit", 100, "Max conversations per page")
	channelListCmd.Flags().String("cursor", "", "Pagination cursor")

	channelNewCmd.Flags().String("name", "", "Channel name")
	_ = channelNewCmd.MarkFlagRequired("name")
	channelNewCmd.Flags().Bool("private", false, "Create as private channel")

	channelInviteCmd.Flags().String("channel", "", "Channel ID or #name")
	_ = channelInviteCmd.MarkFlagRequired("channel")
	channelInviteCmd.Flags().String("users", "", "Comma-separated user IDs, @handles, or emails")
	_ = channelInviteCmd.MarkFlagRequired("users")
}
