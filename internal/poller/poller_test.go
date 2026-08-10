package poller

import (
	"reflect"
	"testing"
)

func TestShouldStart(t *testing.T) {
	tests := []struct {
		name   string
		noPoll bool
		tag    string
		want   bool
	}{
		{"default with tag", false, "v-dev-0.0.5", true},
		{"no-poll with tag", true, "v-dev-0.0.5", false},
		{"empty tag", false, "", false},
		{"no-poll empty tag", true, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShouldStart(tt.noPoll, tt.tag); got != tt.want {
				t.Fatalf("ShouldStart(%v, %q) = %v, want %v", tt.noPoll, tt.tag, got, tt.want)
			}
		})
	}
}

func TestCommandArgs(t *testing.T) {
	got := CommandArgs("v-dev-0.0.5")
	want := []string{"poll", "tag", "v-dev-0.0.5"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CommandArgs = %#v, want %#v", got, want)
	}
}

func TestNewBackgroundCmd(t *testing.T) {
	cmd := newBackgroundCmd("/usr/local/bin/forklift", "v-dev-0.0.5")
	if cmd.Path != "/usr/local/bin/forklift" {
		t.Fatalf("Path = %q", cmd.Path)
	}
	wantArgs := []string{"/usr/local/bin/forklift", "poll", "tag", "v-dev-0.0.5"}
	if !reflect.DeepEqual(cmd.Args, wantArgs) {
		t.Fatalf("Args = %#v, want %#v", cmd.Args, wantArgs)
	}
	if cmd.Stdout != nil || cmd.Stderr != nil || cmd.Stdin != nil {
		t.Fatal("stdio should be nil (discarded)")
	}
	if cmd.SysProcAttr == nil {
		t.Fatal("expected SysProcAttr to be set for detach")
	}
}
