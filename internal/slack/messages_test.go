package slack

import (
	"strings"
	"testing"
)

func TestParseRawMessage_PlainTextUnaffected(t *testing.T) {
	raw := map[string]interface{}{
		"ts":   "1788613505.366269",
		"text": "hello world",
	}
	msg := parseRawMessage("C123", raw, 8000, false)
	if msg.Content != "hello world" {
		t.Fatalf("got content %q, want %q", msg.Content, "hello world")
	}
}

func TestParseRawMessage_LegacyAttachmentsFallback(t *testing.T) {
	// Mirrors Grafana-style alert posts: empty "text", body lives entirely in
	// legacy "attachments" (title/text/fields/footer).
	raw := map[string]interface{}{
		"ts":   "1788098788.262629",
		"text": "",
		"attachments": []interface{}{
			map[string]interface{}{
				"title":    "Step Function Failure Rate High",
				"text":     "latenthealth-tactical-holomap-sbom-sandbox is failing",
				"fallback": "should not be used, text took priority",
				"fields": []interface{}{
					map[string]interface{}{"title": "Severity", "value": "critical"},
				},
				"footer": "Grafana",
			},
		},
	}
	msg := parseRawMessage("C123", raw, 8000, false)
	if msg.Content == "" {
		t.Fatal("expected content to be populated from attachments, got empty string")
	}
	for _, want := range []string{"Step Function Failure Rate High", "is failing", "Severity: critical", "Grafana"} {
		if !strings.Contains(msg.Content, want) {
			t.Errorf("expected content to contain %q, got: %s", want, msg.Content)
		}
	}
}

func TestParseRawMessage_AttachmentFallbackWhenNoText(t *testing.T) {
	raw := map[string]interface{}{
		"ts":   "1",
		"text": "",
		"attachments": []interface{}{
			map[string]interface{}{
				"fallback": "only a fallback string here",
			},
		},
	}
	msg := parseRawMessage("C123", raw, 8000, false)
	if !strings.Contains(msg.Content, "only a fallback string here") {
		t.Errorf("expected fallback text to be used, got: %s", msg.Content)
	}
}

func TestParseRawMessage_AttachmentNestedBlocks(t *testing.T) {
	// Mirrors the "threat investigation" bot pattern: the real content lives
	// in attachments[0].blocks[], not in any legacy attachment field.
	raw := map[string]interface{}{
		"ts":   "1788613505.366269",
		"text": "A threat investigation has completed",
		"attachments": []interface{}{
			map[string]interface{}{
				"blocks": []interface{}{
					map[string]interface{}{
						"type": "section",
						"text": map[string]interface{}{
							"type": "mrkdwn",
							"text": "CVE-2026-73912/73920 Oracle Helidon RCE, no exposure found at SmithRx",
						},
					},
				},
			},
		},
	}
	msg := parseRawMessage("C123", raw, 8000, false)
	// text is non-empty ("A threat investigation has completed"), so the
	// attachment-fallback path deliberately does not run — that stub text is
	// the current behavior for messages with any plain text at all.
	if msg.Content != "A threat investigation has completed" {
		t.Fatalf("got content %q", msg.Content)
	}
}

func TestParseRawMessage_TopLevelBlocksFallback(t *testing.T) {
	raw := map[string]interface{}{
		"ts":   "1",
		"text": "",
		"blocks": []interface{}{
			map[string]interface{}{
				"type": "section",
				"text": map[string]interface{}{
					"type": "mrkdwn",
					"text": "block kit body",
				},
			},
		},
	}
	msg := parseRawMessage("C123", raw, 8000, false)
	if !strings.Contains(msg.Content, "block kit body") {
		t.Errorf("expected block text to be used, got: %s", msg.Content)
	}
}
