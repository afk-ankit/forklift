package build

import "testing"

func TestIncrementTag_devSequence(t *testing.T) {
	got, err := IncrementTag("v-dev-0.0.4", "dev")
	if err != nil {
		t.Fatal(err)
	}
	if got != "v-dev-0.0.5" {
		t.Fatalf("got %q, want v-dev-0.0.5", got)
	}
}

func TestIncrementTag_emptyStartsAtOne(t *testing.T) {
	got, err := IncrementTag("", "dev")
	if err != nil {
		t.Fatal(err)
	}
	if got != "v-dev-0.0.1" {
		t.Fatalf("got %q, want v-dev-0.0.1", got)
	}
}
