package busybox

import (
	"fmt"
	"os"

	"github.com/agentbusybox/pkg/applet"
)

func init() {
	applet.Register(&applet.Applet{Name: "busybox", Short: "BusyBox compatibility dispatcher", Func: runBusybox})
}

func runBusybox(args []string) int {
	if len(args) == 1 {
		fmt.Fprintf(os.Stderr, "Usage: busybox [applet] [args...]\n\nCurrently defined applets:\n")
		applet.List()
		return 0
	}
	switch args[1] {
	case "--list", "-l":
		for _, name := range applet.Names() {
			fmt.Println(name)
		}
		return 0
	case "--help", "-h":
		fmt.Fprintf(os.Stderr, "Usage: busybox [applet] [args...]\n")
		return 0
	}
	a := applet.Get(args[1])
	if a == nil {
		fmt.Fprintf(os.Stderr, "busybox: unknown applet '%s'\n", args[1])
		return 1
	}
	return a.Func(args[1:])
}
