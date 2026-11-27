package cmd

import (
	"bytes"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	prev := version
	version = "1.2.3"
	t.Cleanup(func() { version = prev })

	buf := &bytes.Buffer{}
	versionCmd.SetOut(buf)
	versionCmd.Run(versionCmd, nil)

	if got := buf.String(); got != "1.2.3\n" {
		t.Fatalf("got %q, want %q", got, "1.2.3\n")
	}
}
