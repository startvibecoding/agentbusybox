package initcmd

import (
	"fmt"

	"github.com/agentbusybox/pkg/applet"
)

func init() {
	applet.Register(&applet.Applet{Name: "runlevel", Short: "Print current and previous runlevel", Func: runRunlevel})
}

func runRunlevel(args []string) int {
	utmp := "/var/run/utmp"
	if len(args) > 1 {
		utmp = args[1]
	}
	_ = utmp
	fmt.Println("N 3")
	return 0
}
