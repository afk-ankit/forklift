package cmd

import "testing"

func TestBackgroundPollMessage(t *testing.T) {
	got := backgroundPollMessage("v-dev-0.0.5")
	want := "Polling tag v-dev-0.0.5 in the background…"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
