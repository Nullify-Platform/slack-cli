package cmd

import (
	"github.com/nullify/slack-cli/internal/output"
	"github.com/nullify/slack-cli/internal/slack"
	"github.com/spf13/cobra"
)

var userCmd = &cobra.Command{
	Use:   "user",
	Short: "Workspace user directory",
}

var userListCmd = &cobra.Command{
	Use:   "list",
	Short: "List users in the workspace",
	RunE: func(cmd *cobra.Command, args []string) error {
		limit, _ := cmd.Flags().GetInt("limit")
		cursor, _ := cmd.Flags().GetString("cursor")
		includeBots, _ := cmd.Flags().GetBool("include-bots")

		client, err := getClient()
		if err != nil {
			return err
		}

		result, err := slack.ListUsers(cmd.Context(), client, slack.ListUsersOpts{
			Limit:       limit,
			Cursor:      cursor,
			IncludeBots: includeBots,
		})
		if err != nil {
			return err
		}

		return output.PrintJSON(result)
	},
}

var userGetCmd = &cobra.Command{
	Use:   "get <user>",
	Short: "Get user by ID, @handle, or email",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getClient()
		if err != nil {
			return err
		}

		user, err := slack.GetUser(cmd.Context(), client, args[0])
		if err != nil {
			return err
		}

		return output.PrintJSON(user)
	},
}

func init() {
	rootCmd.AddCommand(userCmd)
	userCmd.AddCommand(userListCmd, userGetCmd)

	userListCmd.Flags().Int("limit", 200, "Max users")
	userListCmd.Flags().String("cursor", "", "Pagination cursor")
	userListCmd.Flags().Bool("include-bots", false, "Include bot users")
}
