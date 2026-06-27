package process

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/agentbusybox/pkg/applet"
)

// --- iostat ---
func init() {
	applet.Register(&applet.Applet{Name: "iostat", Short: "Report CPU and I/O statistics", Func: runIostat})
}

func runIostat(args []string) int {
	interval := 0
	count := 1
	flags := []string{}
	posArgs := []string{}

	for _, a := range args[1:] {
		if strings.HasPrefix(a, "-") {
			flags = append(flags, a)
		} else {
			posArgs = append(posArgs, a)
		}
	}
	if len(posArgs) >= 1 {
		fmt.Sscanf(posArgs[0], "%d", &interval)
	}
	if len(posArgs) >= 2 {
		fmt.Sscanf(posArgs[1], "%d", &count)
	}
	if interval == 0 {
		count = 1
	}

	for i := 0; i < count; i++ {
		if runtime.GOOS == "linux" {
			printIostat()
		} else {
			fmt.Fprintf(os.Stderr, "iostat: not supported on this platform\n")
			return 1
		}
		if i < count-1 && interval > 0 {
			time.Sleep(time.Duration(interval) * time.Second)
		}
	}
	return 0
}

func printIostat() {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		fmt.Fprintf(os.Stderr, "iostat: %v\n", err)
		return
	}

	// CPU stats
	fmt.Printf("avg-cpu:  %%user   %%nice %%system %%iowait  %%steal   %%idle\n")
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "cpu ") {
			parts := strings.Fields(line)
			if len(parts) >= 8 {
				user, _ := strconv.ParseFloat(parts[1], 64)
				nice, _ := strconv.ParseFloat(parts[2], 64)
				sys, _ := strconv.ParseFloat(parts[3], 64)
				idle, _ := strconv.ParseFloat(parts[4], 64)
				iowait, _ := strconv.ParseFloat(parts[5], 64)
				irq, _ := strconv.ParseFloat(parts[6], 64)
				softirq, _ := strconv.ParseFloat(parts[7], 64)
				total := user + nice + sys + idle + iowait + irq + softirq
				if total == 0 {
					total = 1
				}
				fmt.Printf("          %5.1f    %5.1f    %5.1f    %5.1f    %5.1f    %5.1f\n",
					user/total*100, nice/total*100, sys/total*100, iowait/total*100, 0.0, idle/total*100)
			}
		}
	}

	// Disk stats
	data2, err := os.ReadFile("/proc/diskstats")
	if err != nil {
		return
	}
	fmt.Printf("\nDevice             tps    kB_read/s    kB_wrtn/s    kB_read    kB_wrtn\n")
	for _, line := range strings.Split(string(data2), "\n") {
		parts := strings.Fields(line)
		if len(parts) >= 14 {
			dev := parts[2]
			reads, _ := strconv.ParseInt(parts[5], 10, 64)
			readSectors, _ := strconv.ParseInt(parts[6], 10, 64)
			writes, _ := strconv.ParseInt(parts[9], 10, 64)
			writeSectors, _ := strconv.ParseInt(parts[10], 10, 64)
			if reads > 0 || writes > 0 {
				tps := reads + writes
				fmt.Printf("%-18s %4d    %10.1f    %10.1f %10d %10d\n",
					dev, tps, float64(readSectors)/2.0, float64(writeSectors)/2.0, readSectors/2, writeSectors/2)
			}
		}
	}
}

// --- lsof ---
func init() {
	applet.Register(&applet.Applet{Name: "lsof", Short: "List open files", Func: runLsof})
}

func runLsof(args []string) int {
	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "lsof: not supported on this platform\n")
		return 1
	}

	entries, err := os.ReadDir("/proc")
	if err != nil {
		fmt.Fprintf(os.Stderr, "lsof: %v\n", err)
		return 1
	}

	fmt.Printf("%-8s %5s %4s %-10s %10s %8s %s\n", "COMMAND", "PID", "FD", "TYPE", "DEVICE", "SIZE", "NODE", "NAME")
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		comm, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
		if err != nil {
			continue
		}
		cmdName := strings.TrimSpace(string(comm))

		fdDir := fmt.Sprintf("/proc/%d/fd", pid)
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			continue
		}
		for _, fd := range fds {
			fdNum := fd.Name()
			link, err := os.Readlink(fmt.Sprintf("%s/%s", fdDir, fdNum))
			if err != nil {
				continue
			}

			// Determine type
			fdType := "REG"
			device := ""
			size := ""
			node := ""

			switch {
			case link == "/dev/null" || link == "/dev/zero" || link == "/dev/full":
				fdType = "CHR"
				device = "1,3"
			case strings.HasPrefix(link, "socket:"):
				fdType = "IPv4"
				inode := strings.TrimSuffix(strings.TrimPrefix(link, "socket:["), "]")
				node = inode
			case strings.HasPrefix(link, "pipe:"):
				fdType = "FIFO"
				inode := strings.TrimSuffix(strings.TrimPrefix(link, "pipe:["), "]")
				node = inode
			case strings.HasPrefix(link, "[anon]"):
				fdType = "REG"
			default:
				fdType = "REG"
				if info, err := os.Stat(link); err == nil {
					if info.IsDir() {
						fdType = "DIR"
					}
					size = fmt.Sprintf("%d", info.Size())
				}
			}

			// Get file stat for device/inode
			if st, err := os.Stat(link); err == nil {
				if sys, ok := st.Sys().(*syscall.Stat_t); ok {
					device = fmt.Sprintf("%d,%d", sys.Dev>>8, sys.Dev&0xff)
					node = fmt.Sprintf("%d", sys.Ino)
				}
			}

			fmt.Printf("%-8s %5s %4s %-10s %10s %8s %8s %s\n",
				cmdName, pid, fdNum, fdType, device, size, node, link)
		}
	}
	return 0
}

// --- killall5 ---
func init() {
	applet.Register(&applet.Applet{Name: "killall5", Short: "Send signal to all processes", Func: runKillall5})
}

func runKillall5(args []string) int {
	signal := syscall.SIGTERM
	for _, a := range args[1:] {
		if strings.HasPrefix(a, "-") {
			switch a[1:] {
			case "9":
				signal = syscall.SIGKILL
			case "15":
				signal = syscall.SIGTERM
			}
		}
	}
	if runtime.GOOS == "linux" {
		entries, _ := os.ReadDir("/proc")
		myPid := os.Getpid()
		for _, entry := range entries {
			pid, err := strconv.Atoi(entry.Name())
			if err != nil || pid == myPid {
				continue
			}
			p, err := os.FindProcess(pid)
			if err != nil {
				continue
			}
			p.Signal(signal)
		}
		return 0
	}
	return 1
}

// --- pstree ---
func init() {
	applet.Register(&applet.Applet{Name: "pstree", Short: "Show process tree", Func: runPstree})
}

func runPstree(args []string) int {
	pid := 0
	showPids := false
	for _, a := range args[1:] {
		if a == "-p" || a == "--show-pids" {
			showPids = true
			continue
		}
		if !strings.HasPrefix(a, "-") {
			fmt.Sscanf(a, "%d", &pid)
		}
	}
	if pid == 0 {
		pid = 1
	}

	if runtime.GOOS == "linux" {
		tree := buildProcessTree(pid, showPids)
		fmt.Println(tree)
		return 0
	}
	fmt.Printf("init\n")
	return 0
}

func buildProcessTree(pid int, showPids bool) string {
	comm, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
	if err != nil {
		return ""
	}
	name := strings.TrimSpace(string(comm))
	if showPids {
		name = fmt.Sprintf("%s(%d)", name, pid)
	}

	children := getChildren(pid)
	if len(children) == 0 {
		return name
	}

	result := name + "---"
	for i, child := range children {
		childTree := buildProcessTree(child, showPids)
		if i == 0 {
			result += childTree
		} else {
			result += "\n" + strings.Repeat(" ", len(name)+3) + childTree
		}
	}
	return result
}

func getChildren(ppid int) []int {
	children := []int{}
	if runtime.GOOS != "linux" {
		return children
	}
	entries, _ := os.ReadDir("/proc")
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
		if err != nil {
			continue
		}
		parts := strings.Fields(string(stat))
		if len(parts) >= 4 {
			parent, _ := strconv.Atoi(parts[3])
			if parent == ppid {
				children = append(children, pid)
			}
		}
	}
	return children
}

// --- pwdx ---
func init() {
	applet.Register(&applet.Applet{Name: "pwdx", Short: "Print current working directory of a process", Func: runPwdx})
}

func runPwdx(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "pwdx: missing pid\n")
		return 1
	}
	for _, a := range args[1:] {
		pid := a
		if runtime.GOOS == "linux" {
			link, err := os.Readlink(fmt.Sprintf("/proc/%s/cwd", pid))
			if err != nil {
				fmt.Fprintf(os.Stderr, "pwdx: %s: %v\n", pid, err)
				return 1
			}
			fmt.Printf("%s: %s\n", pid, link)
		}
	}
	return 0
}

// --- mpstat ---
func init() {
	applet.Register(&applet.Applet{Name: "mpstat", Short: "Processor statistics", Func: runMpstat})
}

func runMpstat(args []string) int {
	interval := 0
	count := 1
	posArgs := []string{}
	for _, a := range args[1:] {
		if !strings.HasPrefix(a, "-") {
			posArgs = append(posArgs, a)
		}
	}
	if len(posArgs) >= 1 {
		fmt.Sscanf(posArgs[0], "%d", &interval)
	}
	if len(posArgs) >= 2 {
		fmt.Sscanf(posArgs[1], "%d", &count)
	}
	if interval == 0 {
		count = 1
	}

	for i := 0; i < count; i++ {
		if runtime.GOOS == "linux" {
			printMpstat()
		} else {
			fmt.Fprintf(os.Stderr, "mpstat: not supported\n")
			return 1
		}
		if i < count-1 && interval > 0 {
			time.Sleep(time.Duration(interval) * time.Second)
		}
	}
	return 0
}

func printMpstat() {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return
	}
	fmt.Printf("%-8s  %%usr  %%nice   %%sys %%iowait  %%irq  %%soft  %%steal  %%guest  %%gnice   %%idle\n", "CPU")
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "cpu") {
			parts := strings.Fields(line)
			if len(parts) >= 8 {
				user, _ := strconv.ParseFloat(parts[1], 64)
				nice, _ := strconv.ParseFloat(parts[2], 64)
				sys, _ := strconv.ParseFloat(parts[3], 64)
				idle, _ := strconv.ParseFloat(parts[4], 64)
				iowait, _ := strconv.ParseFloat(parts[5], 64)
				irq, _ := strconv.ParseFloat(parts[6], 64)
				softirq, _ := strconv.ParseFloat(parts[7], 64)
				total := user + nice + sys + idle + iowait + irq + softirq
				if total == 0 {
					total = 1
				}
				fmt.Printf("%-8s %5.1f  %5.1f  %5.1f    %5.1f  %5.1f  %5.1f    %5.1f    %5.1f    %5.1f    %5.1f\n",
					parts[0], user/total*100, nice/total*100, sys/total*100, iowait/total*100,
					irq/total*100, softirq/total*100, 0.0, 0.0, 0.0, idle/total*100)
			}
		}
	}
}

// --- sysctl ---
func init() {
	applet.Register(&applet.Applet{Name: "sysctl", Short: "Configure kernel parameters at runtime", Func: runSysctl})
}

func runSysctl(args []string) int {
	showAll := false
	write := false
	files := []string{}
	for _, a := range args[1:] {
		if a == "-a" || a == "-A" {
			showAll = true
			continue
		}
		if a == "-w" {
			write = true
			continue
		}
		if !strings.HasPrefix(a, "-") {
			files = append(files, a)
		}
	}
	_ = write
	if showAll {
		if runtime.GOOS == "linux" {
			filepath.Walk("/proc/sys", func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if !info.IsDir() {
					data, err := os.ReadFile(path)
					if err == nil {
						key := strings.TrimPrefix(path, "/proc/sys/")
						key = strings.ReplaceAll(key, "/", ".")
						fmt.Printf("%s = %s", key, strings.TrimSpace(string(data)))
						fmt.Println()
					}
				}
				return nil
			})
		}
		return 0
	}
	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "sysctl: missing variable\n")
		return 1
	}
	exitCode := 0
	for _, f := range files {
		if idx := strings.Index(f, "="); idx >= 0 {
			key := f[:idx]
			val := f[idx+1:]
			path := "/proc/sys/" + strings.ReplaceAll(key, ".", "/")
			if err := os.WriteFile(path, []byte(val+"\n"), 0644); err != nil {
				fmt.Fprintf(os.Stderr, "sysctl: %s: %v\n", key, err)
				exitCode = 1
			}
		} else {
			path := "/proc/sys/" + strings.ReplaceAll(f, ".", "/")
			data, err := os.ReadFile(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "sysctl: %s: %v\n", f, err)
				exitCode = 1
				continue
			}
			fmt.Printf("%s = %s\n", f, strings.TrimSpace(string(data)))
		}
	}
	return exitCode
}

// --- nmeter ---
func init() {
	applet.Register(&applet.Applet{Name: "nmeter", Short: "Monitor system in a wide format", Func: runNmeter})
}

func runNmeter(args []string) int {
	for i := 0; i < 10; i++ {
		if runtime.GOOS == "linux" {
			load, _ := os.ReadFile("/proc/loadavg")
			fmt.Printf("load: %s", strings.TrimSpace(string(load)))
			fmt.Println()
		}
		time.Sleep(time.Second)
	}
	return 0
}

// --- powertop ---
func init() {
	applet.Register(&applet.Applet{Name: "powertop", Short: "Power consumption monitor", Func: runPowertop})
}

func runPowertop(args []string) int {
	// Read CPU frequency and power info from sysfs
	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "powertop: not supported\n")
		return 1
	}

	for i := 0; i < 5; i++ {
		fmt.Print("\033[2J\033[H")
		fmt.Printf("PowerTOP - %s\n\n", time.Now().Format("15:04:05"))

		// CPU frequency
		cpus, _ := filepath.Glob("/sys/devices/system/cpu/cpu[0-9]*/cpufreq/scaling_cur_freq")
		if len(cpus) > 0 {
			fmt.Printf("CPU frequency:\n")
			for _, cpu := range cpus {
				data, _ := os.ReadFile(cpu)
				freq := strings.TrimSpace(string(data))
				name := filepath.Base(filepath.Dir(filepath.Dir(cpu)))
				fmt.Printf("  %s: %s kHz\n", name, freq)
			}
		}

		// Battery
		batteries, _ := filepath.Glob("/sys/class/power_supply/BAT*/capacity")
		if len(batteries) > 0 {
			fmt.Printf("\nBattery:\n")
			for _, bat := range batteries {
				data, _ := os.ReadFile(bat)
				cap := strings.TrimSpace(string(data))
				name := filepath.Base(filepath.Dir(bat))
				status, _ := os.ReadFile(filepath.Join(filepath.Dir(bat), "status"))
				fmt.Printf("  %s: %s%% (%s)\n", name, cap, strings.TrimSpace(string(status)))
			}
		}

		time.Sleep(time.Second)
	}
	return 0
}

// --- smemcap ---
func init() {
	applet.Register(&applet.Applet{Name: "smemcap", Short: "Collect memory usage data", Func: runSmemcap})
}

func runSmemcap(args []string) int {
	// Read memory info from /proc
	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "smemcap: not supported\n")
		return 1
	}

	// Print memory info in a format similar to smem
	fmt.Printf("%-8s %-8s %-8s %-8s %s\n", "PID", "User", "Swap", "USS", "Command")

	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 1
	}

	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}

		comm, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
		if err != nil {
			continue
		}

		// Read smaps for memory info
		smapsData, err := os.ReadFile(fmt.Sprintf("/proc/%d/smaps", pid))
		if err != nil {
			continue
		}

		var swapTotal, ussTotal int64
		for _, line := range strings.Split(string(smapsData), "\n") {
			if strings.HasPrefix(line, "Swap:") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					val, _ := strconv.ParseInt(parts[1], 10, 64)
					swapTotal += val
				}
			}
			if strings.HasPrefix(line, "Private_Dirty:") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					val, _ := strconv.ParseInt(parts[1], 10, 64)
					ussTotal += val
				}
			}
		}

		fmt.Printf("%-8d %-8s %-8d %-8d %s\n",
			pid, "root", swapTotal, ussTotal, strings.TrimSpace(string(comm)))
	}
	return 0
}

// helper for sysctl Walk
func filepathWalk(path string, fn func(string, os.FileInfo, error) error) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		fullPath := path + "/" + entry.Name()
		if err := fn(fullPath, info, nil); err != nil {
			return err
		}
		if entry.IsDir() {
			filepathWalk(fullPath, fn)
		}
	}
	return nil
}

// --- renice ---
func init() {
	applet.Register(&applet.Applet{Name: "renice", Short: "Alter priority of running processes", Func: runRenice})
}

func runRenice(args []string) int {
	priority := 0
	pids := []int{}
	absolute := false

	i := 1
	for i < len(args) {
		a := args[i]
		if a == "-n" && i+1 < len(args) {
			i++
			fmt.Sscanf(args[i], "%d", &priority)
		} else if a == "-p" {
			// PID mode (default)
		} else if a == "-g" {
			// Process group mode
		} else if a == "-u" {
			// User mode
		} else if strings.HasPrefix(a, "-") && len(a) > 1 {
			// Could be -10, +5, etc.
			fmt.Sscanf(a, "%d", &priority)
		} else if strings.HasPrefix(a, "+") {
			fmt.Sscanf(a[1:], "%d", &priority)
			absolute = false
		} else {
			pid, err := strconv.Atoi(a)
			if err == nil {
				pids = append(pids, pid)
			}
		}
		i++
	}

	if len(pids) == 0 {
		fmt.Fprintf(os.Stderr, "renice: missing pid\n")
		return 1
	}

	if absolute {
		// Set absolute priority
		for _, pid := range pids {
			if err := syscall.Setpriority(syscall.PRIO_PROCESS, pid, priority); err != nil {
				fmt.Fprintf(os.Stderr, "renice: %d: %v\n", pid, err)
				return 1
			}
			fmt.Printf("%d: set priority %d\n", pid, priority)
		}
	} else {
		// Adjust priority
		for _, pid := range pids {
			current, err := syscall.Getpriority(syscall.PRIO_PROCESS, pid)
			if err != nil {
				fmt.Fprintf(os.Stderr, "renice: %d: %v\n", pid, err)
				return 1
			}
			newPrio := current + priority
			if err := syscall.Setpriority(syscall.PRIO_PROCESS, pid, newPrio); err != nil {
				fmt.Fprintf(os.Stderr, "renice: %d: %v\n", pid, err)
				return 1
			}
			fmt.Printf("%d: set priority %d\n", pid, newPrio)
		}
	}
	return 0
}

// --- fuser --- moved to fuser.go ---
