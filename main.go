package main

import (
	"os"

	"github.com/nullify/slack-cli/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
