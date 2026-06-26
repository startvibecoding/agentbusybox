package process

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/agentbusybox/pkg/applet"
)

func init() {
	applet.Register(&applet.Applet{Name: "ps", Short: "Report process status", Func: runPs})
	applet.Register(&applet.Applet{Name: "minips", Short: "Report process status", Func: runPs})
}

func runPs(args []string) int {
	showAll := false
	// wide format
	wide := false

	for _, a := range args[1:] {
		if a == "aux" || a == "-aux" {
			showAll = true
			continue
		}
		if a == "-A" || a == "-e" {
			showAll = true
			continue
		}
		if a == "-w" {
			wide = true
			continue
		}
		if !strings.HasPrefix(a, "-") {
			showAll = true
		}
	}
	_ = wide

	if runtime.GOOS == "linux" {
		return psLinux(showAll)
	}

	fmt.Printf("  PID TTY          TIME CMD\n")
	fmt.Printf("%5d ?        00:00:00 %s\n", os.Getpid(), os.Args[0])
	return 0
}

func psLinux(showAll bool) int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		fmt.Fprintf(os.Stderr, "ps: %v\n", err)
		return 1
	}

	type procInfo struct {
		pid     int
		user    string
		cpu     string
		mem     string
		vsz     string
		rss     string
		tty     string
		stat    string
		start   string
		time    string
		command string
	}

	procs := []procInfo{}

	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}

		comm, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
		if err != nil {
			continue
		}

		stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
		if err != nil {
			continue
		}

		statParts := strings.Fields(string(stat))
		state := ""
		if len(statParts) > 2 {
			state = statParts[2]
		}

		procs = append(procs, procInfo{
			pid:     pid,
			command: strings.TrimSpace(string(comm)),
			tty:     "?",
			stat:    state,
			time:    "00:00:00",
		})
	}

	sort.Slice(procs, func(i, j int) bool { return procs[i].pid < procs[j].pid })

	if showAll {
		fmt.Printf("USER       PID %%CPU %%MEM    VSZ   RSS TTY      STAT START   TIME COMMAND\n")
		for _, p := range procs {
			fmt.Printf("%-10s %5d  0.0  0.0 %6s %5s %-8s %-4s %s %s %s\n",
				"root", p.pid, "0", "0", p.tty, p.stat, p.start, p.time, p.command)
		}
	} else {
		fmt.Printf("  PID TTY          TIME CMD\n")
		for _, p := range procs {
			fmt.Printf("%5d %-8s %s %s\n", p.pid, p.tty, p.time, p.command)
		}
	}
	return 0
}

func init() {
	applet.Register(&applet.Applet{Name: "kill", Short: "Send signals to processes", Func: runKill})
}

func runKill(args []string) int {
	signal := syscall.SIGTERM

	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "kill: missing operand\n")
		return 1
	}

	pids := []int{}
	for _, a := range args[1:] {
		if strings.HasPrefix(a, "-") {
			// Parse signal number/name
			sig := a[1:]
			switch sig {
			case "9":
				signal = syscall.SIGKILL
			case "15":
				signal = syscall.SIGTERM
			case "1":
				signal = syscall.SIGHUP
			case "2":
				signal = syscall.SIGINT
			case "HUP":
				signal = syscall.SIGHUP
			case "INT":
				signal = syscall.SIGINT
			case "KILL":
				signal = syscall.SIGKILL
			case "TERM":
				signal = syscall.SIGTERM
			}
			continue
		}
		pid, err := strconv.Atoi(a)
		if err != nil {
			fmt.Fprintf(os.Stderr, "kill: invalid pid '%s'\n", a)
			return 1
		}
		pids = append(pids, pid)
	}

	exitCode := 0
	for _, pid := range pids {
		p, err := os.FindProcess(pid)
		if err != nil {
			fmt.Fprintf(os.Stderr, "kill: (%d) - No such process\n", pid)
			exitCode = 1
			continue
		}
		if err := p.Signal(signal); err != nil {
			fmt.Fprintf(os.Stderr, "kill: (%d) - %v\n", pid, err)
			exitCode = 1
		}
	}
	return exitCode
}

func init() {
	applet.Register(&applet.Applet{Name: "top", Short: "Display process information", Func: runTop})
}

func runTop(args []string) int {
	for i := 0; i < 5; i++ {
		fmt.Print("\033[2J\033[H") // Clear screen
		fmt.Printf("top - %s up 0 days, load average: 0.00, 0.00, 0.00\n", time.Now().Format("15:04:05"))
		fmt.Printf("Tasks: 1 total, 1 running\n")
		fmt.Printf("%%Cpu(s): 0.0 us, 0.0 sy, 0.0 ni, 100.0 id\n")
		fmt.Printf("MiB Mem: 0.0 total, 0.0 free, 0.0 used\n\n")
		fmt.Printf("  PID USER      PR  NI    VIRT    RES    SHR S  %%CPU  %%MEM     TIME+ COMMAND\n")

		if runtime.GOOS == "linux" {
			entries, _ := os.ReadDir("/proc")
			for _, entry := range entries {
				pid, err := strconv.Atoi(entry.Name())
				if err != nil {
					continue
				}
				comm, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
				if err != nil {
					continue
				}
				fmt.Printf("%5d root      20   0       0      0      0 R   0.0   0.0   0:00.00 %s\n",
					pid, strings.TrimSpace(string(comm)))
			}
		} else {
			fmt.Printf("%5d root      20   0       0      0      0 R   0.0   0.0   0:00.00 %s\n",
				os.Getpid(), os.Args[0])
		}

		time.Sleep(time.Second)
	}
	return 0
}

func init() {
	applet.Register(&applet.Applet{Name: "uptime", Short: "Show how long the system has been running", Func: runUptime})
}

func runUptime(args []string) int {
	if runtime.GOOS == "linux" {
		data, err := os.ReadFile("/proc/uptime")
		if err == nil {
			var uptime float64
			fmt.Sscanf(string(data), "%f", &uptime)
			days := int(uptime) / 86400
			hours := (int(uptime) % 86400) / 3600
			mins := (int(uptime) % 3600) / 60
			fmt.Printf(" %s up %d days, %d:%02d, load average: 0.00, 0.00, 0.00\n",
				time.Now().Format("15:04:05"), days, hours, mins)
			return 0
		}
	}
	fmt.Printf(" %s up 0 days, 0:00, load average: 0.00, 0.00, 0.00\n",
		time.Now().Format("15:04:05"))
	return 0
}

func init() {
	applet.Register(&applet.Applet{Name: "free", Short: "Display memory usage", Func: runFree})
}

func runFree(args []string) int {
	human := false
	for _, a := range args[1:] {
		if a == "-h" {
			human = true
		}
	}

	if runtime.GOOS == "linux" {
		data, err := os.ReadFile("/proc/meminfo")
		if err == nil {
			mem := parseMeminfo(string(data))
			total := mem["MemTotal"]
			free := mem["MemFree"]
			avail := mem["MemAvailable"]
			buffers := mem["Buffers"]
			cached := mem["Cached"]
			swapTotal := mem["SwapTotal"]
			swapFree := mem["SwapFree"]

			fmt.Printf("              total        used        free      shared  buff/cache   available\n")
			if human {
				fmt.Printf("Mem:    %10s %10s %10s %10s %10s %10s\n",
					formatKB(total), formatKB(total-free-cached), formatKB(free),
					"0", formatKB(buffers+cached), formatKB(avail))
				fmt.Printf("Swap:   %10s %10s %10s\n",
					formatKB(swapTotal), formatKB(swapTotal-swapFree), formatKB(swapFree))
			} else {
				fmt.Printf("Mem:    %10d %10d %10d %10d %10d %10d\n",
					total, total-free-cached, free, 0, buffers+cached, avail)
				fmt.Printf("Swap:   %10d %10d %10d\n",
					swapTotal, swapTotal-swapFree, swapFree)
			}
			return 0
		}
	}

	fmt.Printf("              total        used        free\n")
	fmt.Printf("Mem:             0           0           0\n")
	return 0
}

func parseMeminfo(data string) map[string]int64 {
	result := make(map[string]int64)
	for _, line := range strings.Split(data, "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			val = strings.TrimSuffix(val, " kB")
			var n int64
			fmt.Sscanf(val, "%d", &n)
			result[key] = n
		}
	}
	return result
}

func formatKB(kb int64) string {
	if kb >= 1048576 {
		return fmt.Sprintf("%.1fG", float64(kb)/1048576)
	}
	if kb >= 1024 {
		return fmt.Sprintf("%.1fM", float64(kb)/1024)
	}
	return fmt.Sprintf("%dK", kb)
}

func init() {
	applet.Register(&applet.Applet{Name: "pgrep", Short: "Look up processes by name", Func: runPgrep})
	applet.Register(&applet.Applet{Name: "pkill", Short: "Kill processes by name", Func: runPkill})
}

func runPgrep(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "pgrep: missing pattern\n")
		return 1
	}
	pattern := args[1]
	exitCode := 1

	if runtime.GOOS == "linux" {
		entries, _ := os.ReadDir("/proc")
		for _, entry := range entries {
			pid, err := strconv.Atoi(entry.Name())
			if err != nil {
				continue
			}
			comm, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
			if err != nil {
				continue
			}
			name := strings.TrimSpace(string(comm))
			if strings.Contains(name, pattern) {
				fmt.Println(pid)
				exitCode = 0
			}
		}
	}
	return exitCode
}

func runPkill(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "pkill: missing pattern\n")
		return 1
	}
	pattern := args[1]
	exitCode := 1

	if runtime.GOOS == "linux" {
		entries, _ := os.ReadDir("/proc")
		for _, entry := range entries {
			pid, err := strconv.Atoi(entry.Name())
			if err != nil {
				continue
			}
			comm, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
			if err != nil {
				continue
			}
			name := strings.TrimSpace(string(comm))
			if strings.Contains(name, pattern) {
				p, _ := os.FindProcess(pid)
				p.Signal(syscall.SIGTERM)
				exitCode = 0
			}
		}
	}
	return exitCode
}

func init() {
	applet.Register(&applet.Applet{Name: "pmap", Short: "Report memory map of a process", Func: runPmap})
}

func runPmap(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "pmap: missing pid\n")
		return 1
	}
	pid := args[1]
	if runtime.GOOS == "linux" {
		data, err := os.ReadFile(fmt.Sprintf("/proc/%s/maps", pid))
		if err != nil {
			fmt.Fprintf(os.Stderr, "pmap: %v\n", err)
			return 1
		}
		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		totalSize := int64(0)
		for scanner.Scan() {
			parts := strings.Fields(scanner.Text())
			if len(parts) >= 5 {
				fmt.Printf("%-34s %8s %s\n", parts[0], parts[1], parts[5])
			}
		}
		fmt.Printf(" total %8dK\n", totalSize)
		return 0
	}
	fmt.Fprintf(os.Stderr, "pmap: not supported on this platform\n")
	return 1
}

func init() {
	applet.Register(&applet.Applet{Name: "pidof", Short: "Find the process ID of a running program", Func: runPidof})
}

func runPidof(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "pidof: missing program name\n")
		return 1
	}
	name := args[1]
	exitCode := 1

	if runtime.GOOS == "linux" {
		entries, _ := os.ReadDir("/proc")
		for _, entry := range entries {
			pid, err := strconv.Atoi(entry.Name())
			if err != nil {
				continue
			}
			comm, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
			if err != nil {
				continue
			}
			if strings.TrimSpace(string(comm)) == name {
				fmt.Printf("%d ", pid)
				exitCode = 0
			}
		}
		if exitCode == 0 {
			fmt.Println()
		}
	}
	return exitCode
}

func init() {
	applet.Register(&applet.Applet{Name: "vmstat", Short: "Report virtual memory statistics", Func: runVmstat})
}

func runVmstat(args []string) int {
	count := 1
	if len(args) > 1 {
		fmt.Sscanf(args[1], "%d", &count)
	}

	for i := 0; i < count; i++ {
		if runtime.GOOS == "linux" {
			data, _ := os.ReadFile("/proc/stat")
			_ = data
			// Simplified output
			fmt.Printf("procs -----------memory---------- ---swap-- -----io---- -system-- ------cpu-----\n")
			fmt.Printf(" r  b   swpd   free   buff  cache   si   so    bi    bo   in   cs us sy id wa st\n")
			fmt.Printf(" 0  0      0      0      0      0    0    0     0     0    0    0  0  0 100  0  0\n")
		}
		if i < count-1 {
			time.Sleep(time.Second)
		}
	}
	return 0
}

// --- fuser --- moved to fuser.go ---
