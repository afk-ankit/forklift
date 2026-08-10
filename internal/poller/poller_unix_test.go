//go:build unix

package poller

import "testing"

func TestNewBackgroundCmd_unixSetsid(t *testing.T) {
	cmd := newBackgroundCmd("/usr/local/bin/forklift", "v-dev-0.0.5")
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.Setsid {
		t.Fatal("expected SysProcAttr.Setsid=true for detach")
	}
}
