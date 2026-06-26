package findutils

import (
	"github.com/agentbusybox/cmd/textproc"
	"github.com/agentbusybox/pkg/applet"
)

// --- egrep ---
func init() {
	applet.Register(&applet.Applet{Name: "egrep", Short: "Search for patterns (extended regex)", Func: runEgrep})
	applet.Register(&applet.Applet{Name: "fgrep", Short: "Search for fixed strings", Func: runFgrep})
}

func runEgrep(args []string) int {
	newArgs := append([]string{"grep", "-E"}, args[1:]...)
	return textproc.RunGrep(newArgs)
}

func runFgrep(args []string) int {
	newArgs := append([]string{"grep", "-F"}, args[1:]...)
	return textproc.RunGrep(newArgs)
}
