package shell

import (
	"os"
	"os/exec"

	"github.com/agentbusybox/pkg/applet"
)

func init() {
	applet.Register(&applet.Applet{Name: "cttyhack", Short: "Give controlling terminal to a child", Func: runCttyhack})
}

func runCttyhack(args []string) int {
	if len(args) < 2 {
		// Start shell
		cmd := exec.Command("/bin/sh")
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				return exitErr.ExitCode()
			}
			return 1
		}
		return 0
	}
	cmd := exec.Command(args[1], args[2:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		return 1
	}
	return 0
}
