package config

import (
	"forklift/internal/structures"
	"strings"
	"testing"
)

func TestValidateSlack_bothOrNeither(t *testing.T) {
	if err := ValidateSlack("", ""); err != nil {
		t.Fatalf("empty pair should be ok, got %v", err)
	}
	if err := ValidateSlack("xoxb-1", "#builds"); err != nil {
		t.Fatalf("both set should be ok, got %v", err)
	}
	if err := ValidateSlack("xoxb-1", ""); err == nil {
		t.Fatal("token without channel should error")
	}
	if err := ValidateSlack("", "#builds"); err == nil {
		t.Fatal("channel without token should error")
	}
}

func TestSlackConfigured(t *testing.T) {
	if SlackConfigured(structures.Config{}) {
		t.Fatal("empty config should not be configured")
	}
	if SlackConfigured(structures.Config{SlackBotToken: "xoxb-1"}) {
		t.Fatal("token only should not be configured")
	}
	if !SlackConfigured(structures.Config{SlackBotToken: "xoxb-1", SlackChannel: "#builds"}) {
		t.Fatal("token+channel should be configured")
	}
}

func TestValidateSlack_errorMentionsBoth(t *testing.T) {
	err := ValidateSlack("xoxb-1", "")
	if err == nil || !strings.Contains(err.Error(), "both") {
		t.Fatalf("expected error mentioning both fields, got %v", err)
	}
}
