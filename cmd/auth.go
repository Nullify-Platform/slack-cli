package cmd

import (
	"fmt"
	"strings"

	"github.com/nullify/slack-cli/internal/auth"
	"github.com/nullify/slack-cli/internal/output"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage Slack authentication",
}

var authTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Verify credentials by calling auth.test",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getClient()
		if err != nil {
			return err
		}
		resp, err := client.Call(cmd.Context(), "auth.test", nil)
		if err != nil {
			return err
		}
		return output.PrintJSON(resp)
	},
}

var authWhoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show current auth configuration (tokens redacted)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := auth.LoadFromEnv()
		if err != nil {
			return err
		}

		mode := "standard"
		if cfg.Mode == 1 { // AuthBrowser
			mode = "browser"
		}

		info := map[string]string{
			"mode":  mode,
			"token": redact(cfg.Token),
		}
		if cfg.Cookie != "" {
			info["cookie"] = redact(cfg.Cookie)
		}
		if cfg.WorkspaceURL != "" {
			info["workspace_url"] = cfg.WorkspaceURL
		}

		return output.PrintJSON(info)
	},
}

func redact(s string) string {
	if len(s) <= 8 {
		return strings.Repeat("*", len(s))
	}
	return s[:8] + "..." + fmt.Sprintf("(%d chars)", len(s))
}

func init() {
	rootCmd.AddCommand(authCmd)
	authCmd.AddCommand(authTestCmd)
	authCmd.AddCommand(authWhoamiCmd)
}
