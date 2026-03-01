package cmd

import (
	"github.com/nullify/slack-cli/internal/api"
	"github.com/nullify/slack-cli/internal/auth"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "slack-cli",
	Short: "Slack CLI for AI agents",
	Long:  "Token-efficient Slack automation CLI designed for AI agents. All output is compact JSON.",
	SilenceUsage: true,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

// getClient loads auth from env and creates an API client.
func getClient() (*api.Client, error) {
	cfg, err := auth.LoadFromEnv()
	if err != nil {
		return nil, err
	}
	return api.NewClient(cfg), nil
}

func init() {
	rootCmd.Version = "0.1.0"
}
