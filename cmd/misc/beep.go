package misc

import (
	"fmt"
	"os"

	"github.com/agentbusybox/pkg/applet"
)

func init() {
	applet.Register(&applet.Applet{Name: "beep", Short: "Beep once", Func: runBeep})
}

func runBeep(args []string) int {
	freq := 440   // -f FREQ
	length := 200 // -l LEN
	delay := 0    // -d DELAY
	reps := 1     // -r REPS
	count := 1    // -n COUNT

	for i := 1; i < len(args); i++ {
		a := args[i]
		switch a {
		case "-f":
			if i+1 < len(args) {
				i++
				fmt.Sscanf(args[i], "%d", &freq)
			}
		case "-l":
			if i+1 < len(args) {
				i++
				fmt.Sscanf(args[i], "%d", &length)
			}
		case "-d":
			if i+1 < len(args) {
				i++
				fmt.Sscanf(args[i], "%d", &delay)
			}
		case "-r":
			if i+1 < len(args) {
				i++
				fmt.Sscanf(args[i], "%d", &reps)
			}
		case "-n":
			if i+1 < len(args) {
				i++
				fmt.Sscanf(args[i], "%d", &count)
			}
		}
	}

	_ = delay
	_ = count

	// Try console ioctl first (Linux)
	if f, err := os.OpenFile("/dev/console", os.O_WRONLY, 0); err == nil {
		defer f.Close()
		for i := 0; i < reps; i++ {
			// KDMKTONE ioctl: frequency in Hz, duration in ms
			// Not portable, just print as fallback
			fmt.Fprintf(f, "\a")
		}
		return 0
	}

	// Fallback: just print bell character
	for i := 0; i < reps; i++ {
		fmt.Print("\a")
	}
	_ = freq
	_ = length
	return 0
}
