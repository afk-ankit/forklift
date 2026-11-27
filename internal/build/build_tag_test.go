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

func TestIncrementTag_rollsAtNine(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"v-dev-0.0.9", "v-dev-0.1.0"},
		{"v-dev-0.0.99", "v-dev-0.1.0"},
		{"v-dev-0.1.9", "v-dev-0.2.0"},
		{"v-dev-0.9.9", "v-dev-1.0.0"},
		{"v-release-1-0.0.9", "v-release-1-0.1.0"},
	}
	for _, tc := range cases {
		got, err := IncrementTag(tc.in, "dev")
		if err != nil {
			t.Fatal(err)
		}
		if got != tc.want {
			t.Fatalf("IncrementTag(%q) = %q, want %q", tc.in, got, tc.want)
		}
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
