package cmd

import (
	"os"
	"strings"
	"testing"
)

func TestResolveMessageTextDashStdin(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = orig }()

	go func() {
		w.WriteString("hello from stdin\n")
		w.Close()
	}()

	got, err := resolveMessageText(messageSendCmd, []string{"D0AHB4FLY75", "-"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(got) != "hello from stdin" {
		t.Fatalf("got %q, want stdin content", got)
	}
}
