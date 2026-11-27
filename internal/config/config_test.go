package config

import (
	"encoding/json"
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

func TestCreateTagEnabled_defaultsOn(t *testing.T) {
	if !CreateTagEnabled(structures.Config{}) {
		t.Fatal("unset create_tag should create tags")
	}
	off := false
	if CreateTagEnabled(structures.Config{CreateTag: &off}) {
		t.Fatal("explicit false should skip tags")
	}
	on := true
	if !CreateTagEnabled(structures.Config{CreateTag: &on}) {
		t.Fatal("explicit true should create tags")
	}
}

func TestCreateTag_falseSurvivesJSON(t *testing.T) {
	off := false
	data, err := json.Marshal(structures.Config{CreateTag: &off})
	if err != nil {
		t.Fatal(err)
	}
	var cfg structures.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if CreateTagEnabled(cfg) {
		t.Fatal("create_tag false was lost in json")
	}
}

func TestValidateSlack_errorMentionsBoth(t *testing.T) {
	err := ValidateSlack("xoxb-1", "")
	if err == nil || !strings.Contains(err.Error(), "both") {
		t.Fatalf("expected error mentioning both fields, got %v", err)
	}
}
