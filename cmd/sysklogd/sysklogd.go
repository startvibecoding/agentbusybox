package sysklogd

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/agentbusybox/pkg/applet"
)

func init() {
	applet.Register(&applet.Applet{Name: "syslogd", Short: "System logging daemon", Func: runSyslogd})
	applet.Register(&applet.Applet{Name: "klogd", Short: "Kernel log daemon", Func: runKlogd})
	applet.Register(&applet.Applet{Name: "logger", Short: "Write messages to the system log", Func: runLogger})
	applet.Register(&applet.Applet{Name: "logread", Short: "Show messages in syslogd's circular buffer", Func: runLogread})
}

func runSyslogd(args []string) int {
	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "syslogd: not supported\n")
		return 1
	}

	// Open syslog socket
	addr, _ := net.ResolveUnixAddr("unixgram", "/dev/log")
	conn, err := net.ListenUnix("unixgram", addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "syslogd: cannot open /dev/log: %v\n", err)
		return 1
	}
	defer conn.Close()
	os.Chmod("/dev/log", 0666)

	logFile := "/var/log/messages"
	for _, a := range args[1:] {
		if a == "-O" && len(args) > 2 {
			logFile = args[1]
		}
	}

	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "syslogd: cannot open %s: %v\n", logFile, err)
		return 1
	}
	defer f.Close()

	buf := make([]byte, 4096)
	for {
		uc, err := conn.AcceptUnix()
		if err != nil {
			continue
		}
		n, err := uc.Read(buf)
		uc.Close()
		if err != nil {
			continue
		}
		msg := string(buf[:n])
		ts := time.Now().Format("Jan _2 15:04:05")
		line := fmt.Sprintf("%s %s\n", ts, msg)
		f.WriteString(line)
		f.Sync()
	}
}

func runKlogd(args []string) int {
	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "klogd: not supported\n")
		return 1
	}

	// Read kernel messages from /dev/kmsg
	f, err := os.Open("/dev/kmsg")
	if err != nil {
		// Fallback to /proc/kmsg
		f, err = os.Open("/proc/kmsg")
		if err != nil {
			fmt.Fprintf(os.Stderr, "klogd: cannot open kernel log\n")
			return 1
		}
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		ts := time.Now().Format("Jan _2 15:04:05")
		fmt.Printf("%s kernel: %s\n", ts, line)
	}
	return 0
}

func runLogger(args []string) int {
	priority := "user.notice"
	message := ""
	tag := "logger"

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-p":
			if i+1 < len(args) {
				i++
				priority = args[i]
			}
		case "-t":
			if i+1 < len(args) {
				i++
				tag = args[i]
			}
		default:
			if !strings.HasPrefix(args[i], "-") {
				if message != "" {
					message += " "
				}
				message += args[i]
			}
		}
	}

	if message == "" {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			message += scanner.Text()
		}
	}

	// Try to send to /dev/log
	conn, err := net.DialUnix("unixgram", nil, &net.UnixAddr{Name: "/dev/log", Net: "unixgram"})
	if err == nil {
		msg := fmt.Sprintf("<%s>%s: %s", priority, tag, message)
		conn.Write([]byte(msg))
		conn.Close()
		return 0
	}

	// Fallback: print to stdout
	fmt.Printf("%s: %s\n", tag, message)
	return 0
}

func runLogread(args []string) int {
	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "logread: not supported\n")
		return 1
	}

	for _, logfile := range []string{"/var/log/messages", "/var/log/syslog"} {
		data, err := os.ReadFile(logfile)
		if err == nil {
			os.Stdout.Write(data)
			return 0
		}
	}
	fmt.Fprintf(os.Stderr, "logread: no logs available\n")
	return 1
}
