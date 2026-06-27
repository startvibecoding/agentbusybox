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
	if runtime.GOOS != "linux" {
		fmt.Println("AgentBusyBox init: not supported on this platform")
		return 1
	}

	fmt.Println("AgentBusyBox init: starting built-in shell")
	if sh := applet.Get("sh"); sh != nil {
		return sh.Func([]string{"sh"})
	}
	fmt.Fprintf(os.Stderr, "init: built-in shell applet not registered\n")
	return 1
}

func runHalt(args []string) int {
	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "halt: not supported on this platform\n")
		return 1
	}

	nowtmp := false
	for _, a := range args[1:] {
		if a == "-n" {
			nowtmp = true
		}
	}
	_ = nowtmp

	// Send SIGTERM to all processes first
	fmt.Println("The system is going down for halt NOW!")
	syscall.Kill(-1, syscall.SIGTERM)

	// Sync filesystems
	syscall.Sync()

	// Halt the system
	err := syscall.Reboot(syscall.LINUX_REBOOT_CMD_POWER_OFF)
	if err != nil {
		fmt.Fprintf(os.Stderr, "halt: %v\n", err)
		return 1
	}
	return 0
}

func runPoweroff(args []string) int {
	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "poweroff: not supported on this platform\n")
		return 1
	}

	fmt.Println("The system is going down for poweroff NOW!")
	syscall.Kill(-1, syscall.SIGTERM)
	syscall.Sync()
	err := syscall.Reboot(syscall.LINUX_REBOOT_CMD_POWER_OFF)
	if err != nil {
		fmt.Fprintf(os.Stderr, "poweroff: %v\n", err)
		return 1
	}
	return 0
}

func runReboot(args []string) int {
	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "reboot: not supported on this platform\n")
		return 1
	}

	nowtmp := false
	for _, a := range args[1:] {
		if a == "-n" {
			nowtmp = true
		}
	}
	_ = nowtmp

	fmt.Println("The system is going down for reboot NOW!")
	syscall.Kill(-1, syscall.SIGTERM)
	syscall.Sync()
	err := syscall.Reboot(syscall.LINUX_REBOOT_CMD_RESTART)
	if err != nil {
		fmt.Fprintf(os.Stderr, "reboot: %v\n", err)
		return 1
	}
	return 0
}

func runBootchartd(args []string) int {
	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "bootchartd: not supported\n")
		return 1
	}

	action := "start"
	if len(args) > 1 {
		action = args[1]
	}

	switch action {
	case "start":
		fmt.Println("bootchartd: starting bootchart collection")
		// Record boot time data
		if data, err := os.ReadFile("/proc/uptime"); err == nil {
			fmt.Printf("boot time: %s", string(data))
		}
	case "stop":
		fmt.Println("bootchartd: stopping bootchart collection")
	case "dump":
		// Dump collected data
		if data, err := os.ReadFile("/proc/stat"); err == nil {
			os.Stdout.Write(data)
		}
	default:
		fmt.Fprintf(os.Stderr, "bootchartd: unknown action '%s'\n", action)
		return 1
	}
	return 0
}
