package initcmd

import (
	"fmt"
	"os"
	"runtime"
	"syscall"

	"github.com/agentbusybox/pkg/applet"
)

func init() {
	applet.Register(&applet.Applet{Name: "init", Short: "Init process", Func: runInit, NoFork: true})
	applet.Register(&applet.Applet{Name: "halt", Short: "Halt the system", Func: runHalt, NoFork: true})
	applet.Register(&applet.Applet{Name: "poweroff", Short: "Power off the system", Func: runPoweroff, NoFork: true})
	applet.Register(&applet.Applet{Name: "reboot", Short: "Reboot the system", Func: runReboot, NoFork: true})
	applet.Register(&applet.Applet{Name: "linuxrc", Short: "Linux init", Func: runInit, NoFork: true})
	applet.Register(&applet.Applet{Name: "bootchartd", Short: "Boot charting daemon", Func: runBootchartd})
}

func runInit(args []string) int {
	if runtime.GOOS == "linux" {
		fmt.Println("AgentBusyBox init: starting built-in shell")
		if sh := applet.Get("sh"); sh != nil {
			return sh.Func([]string{"sh"})
		}
		fmt.Fprintf(os.Stderr, "init: built-in shell applet not registered\n")
		return 1
	}
	fmt.Println("AgentBusyBox init: not supported on this platform")
	return 1
}

func runHalt(args []string) int {
	if runtime.GOOS == "linux" {
		fmt.Println("System halted.")
		syscall.Reboot(syscall.LINUX_REBOOT_CMD_POWER_OFF)
		return 0
	}
	fmt.Fprintf(os.Stderr, "halt: not supported on this platform\n")
	return 1
}

func runPoweroff(args []string) int {
	return runHalt(args)
}

func runReboot(args []string) int {
	if runtime.GOOS == "linux" {
		fmt.Println("System rebooting.")
		syscall.Reboot(syscall.LINUX_REBOOT_CMD_RESTART)
		return 0
	}
	fmt.Fprintf(os.Stderr, "reboot: not supported on this platform\n")
	return 1
}

func runBootchartd(args []string) int {
	fmt.Fprintf(os.Stderr, "bootchartd: not yet implemented\n")
	return 1
}
