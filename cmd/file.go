package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/nullify/slack-cli/internal/output"
	"github.com/nullify/slack-cli/internal/slack"
	"github.com/nullify/slack-cli/internal/urlparse"
	"github.com/spf13/cobra"
)

var fileCmd = &cobra.Command{
	Use:   "file",
	Short: "Inspect, upload, and download Slack file attachments",
}

var fileInfoCmd = &cobra.Command{
	Use:   "info <file-id>",
	Short: "Fetch metadata for a Slack file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getClient()
		if err != nil {
			return err
		}
		info, err := slack.GetFileInfo(cmd.Context(), client, args[0])
		if err != nil {
			return err
		}
		return output.PrintJSON(info)
	},
}

var fileDownloadCmd = &cobra.Command{
	Use:   "download <file-id>",
	Short: "Download a Slack file to stdout or --output <path>",
	Long: `Download a Slack file attachment using its file ID.

The file content is written to stdout by default, or to a file with --output.
File metadata (name, mimetype, size) is printed to stderr so stdout stays clean for piping.

Examples:
  slack-cli file download F12345678 > image.png
  slack-cli file download F12345678 --output image.png`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		outputPath, _ := cmd.Flags().GetString("output")

		client, err := getClient()
		if err != nil {
			return err
		}

		info, err := slack.GetFileInfo(cmd.Context(), client, args[0])
		if err != nil {
			return err
		}
		if info.URLPrivate == "" {
			return fmt.Errorf("file %s has no downloadable URL (it may be a snippet or external file)", args[0])
		}

		fmt.Fprintf(os.Stderr, "downloading: %s (%s, %d bytes)\n", info.Name, info.Mimetype, info.Size)

		body, _, err := client.Download(cmd.Context(), info.URLPrivate)
		if err != nil {
			return err
		}
		defer body.Close()

		var dst io.Writer = os.Stdout
		if outputPath != "" {
			f, err := os.Create(outputPath)
			if err != nil {
				return fmt.Errorf("creating output file: %w", err)
			}
			defer f.Close()
			dst = f
		}

		if _, err := io.Copy(dst, body); err != nil {
			return fmt.Errorf("writing file content: %w", err)
		}

		if outputPath != "" {
			fmt.Fprintf(os.Stderr, "saved to: %s\n", outputPath)
		}
		return nil
	},
}

var fileUploadCmd = &cobra.Command{
	Use:   "upload <channel> <file-path>",
	Short: "Upload a file to a Slack channel or thread",
	Long: `Upload a local file to a Slack channel using the files.getUploadURLExternal API.

The channel can be a channel ID (C...), channel name (#general), or user ID for DMs.

Examples:
  slack-cli file upload C01234ABC ./report.pdf --message "Here is the report"
  slack-cli file upload "#general" ./image.png --thread-ts 1234567890.123456
  slack-cli file upload C01234ABC ./data.csv --title "Q1 Data" --message "See attached"`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		message, _ := cmd.Flags().GetString("message")
		threadTS, _ := cmd.Flags().GetString("thread-ts")
		title, _ := cmd.Flags().GetString("title")
		filename, _ := cmd.Flags().GetString("filename")

		client, err := getClient()
		if err != nil {
			return err
		}

		target := urlparse.ParseMsgTarget(args[0])
		channelID, err := resolveTargetToChannel(cmd, client, target)
		if err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "uploading: %s\n", args[1])

		info, err := slack.UploadFile(cmd.Context(), client, slack.UploadOpts{
			FilePath:  args[1],
			ChannelID: channelID,
			Filename:  filename,
			Title:     title,
			Message:   message,
			ThreadTS:  threadTS,
		})
		if err != nil {
			return err
		}

		return output.PrintJSON(info)
	},
}

func init() {
	rootCmd.AddCommand(fileCmd)
	fileCmd.AddCommand(fileInfoCmd, fileDownloadCmd, fileUploadCmd)

	fileDownloadCmd.Flags().String("output", "", "Write file content to this path instead of stdout")

	fileUploadCmd.Flags().String("message", "", "Optional message to post alongside the file")
	fileUploadCmd.Flags().String("thread-ts", "", "Thread root ts to upload into a thread")
	fileUploadCmd.Flags().String("title", "", "Display title for the file (defaults to filename)")
	fileUploadCmd.Flags().String("filename", "", "Override the filename sent to Slack (defaults to basename of file-path)")
}
