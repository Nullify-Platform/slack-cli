package cmd

import (
	"github.com/nullify/slack-cli/internal/output"
	"github.com/nullify/slack-cli/internal/slack"
	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search Slack messages and files",
}

func addSearchFlags(cmd *cobra.Command) {
	cmd.Flags().StringSlice("channel", nil, "Channel filter (#name or ID, repeatable)")
	cmd.Flags().String("user", "", "User filter (@name or user ID)")
	cmd.Flags().String("after", "", "Only results after YYYY-MM-DD")
	cmd.Flags().String("before", "", "Only results before YYYY-MM-DD")
	cmd.Flags().Int("limit", 20, "Max results")
	cmd.Flags().Int("max-content-chars", 4000, "Max message content chars (-1 unlimited)")
}

func makeSearchRunE(kind string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		channels, _ := cmd.Flags().GetStringSlice("channel")
		user, _ := cmd.Flags().GetString("user")
		after, _ := cmd.Flags().GetString("after")
		before, _ := cmd.Flags().GetString("before")
		limit, _ := cmd.Flags().GetInt("limit")
		maxContent, _ := cmd.Flags().GetInt("max-content-chars")

		if after != "" {
			if err := slack.ValidateDate(after); err != nil {
				return err
			}
		}
		if before != "" {
			if err := slack.ValidateDate(before); err != nil {
				return err
			}
		}

		client, err := getClient()
		if err != nil {
			return err
		}

		result, err := slack.SearchSlack(cmd.Context(), client, slack.SearchOpts{
			Query:           args[0],
			Kind:            kind,
			Channels:        channels,
			User:            user,
			After:           after,
			Before:          before,
			Limit:           limit,
			MaxContentChars: maxContent,
		})
		if err != nil {
			return err
		}

		return output.PrintJSON(result)
	}
}

func makeSearchCmd(name, short, kind string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name + " <query>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE:  makeSearchRunE(kind),
	}
	addSearchFlags(cmd)
	return cmd
}

func init() {
	rootCmd.AddCommand(searchCmd)
	searchCmd.AddCommand(
		makeSearchCmd("messages", "Search messages", "messages"),
		makeSearchCmd("files", "Search files", "files"),
		makeSearchCmd("all", "Search messages and files", "all"),
	)
}
