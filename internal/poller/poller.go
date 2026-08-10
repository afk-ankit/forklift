package poller

import (
	"fmt"
	"os"
	"os/exec"
)

func ShouldStart(noPoll bool, tag string) bool {
	return !noPoll && tag != ""
}

func CommandArgs(tag string) []string {
	return []string{"poll", "tag", tag}
}

func newBackgroundCmd(executable, tag string) *exec.Cmd {
	args := CommandArgs(tag)
	cmd := exec.Command(executable, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	cmd.SysProcAttr = sysProcAttr()
	return cmd
}

func StartBackground(tag string) error {
	executable, err := os.Executable()
	if err != nil || executable == "" {
		executable = os.Args[0]
	}
	if executable == "" {
		return fmt.Errorf("could not resolve forklift executable path")
	}
	cmd := newBackgroundCmd(executable, tag)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start background poll: %w", err)
	}
	return nil
}
