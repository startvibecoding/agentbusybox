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
	count := 1
	if len(args) > 1 {
		fmt.Sscanf(args[1], "%d", &count)
	}
	for i := 0; i < count; i++ {
		if runtime.GOOS == "linux" {
			data, err := os.ReadFile("/proc/stat")
			if err == nil {
				fmt.Printf("avg-cpu:  %%user   %%nice %%system %%iowait  %%steal   %%idle\n")
				for _, line := range strings.Split(string(data), "\n") {
					if strings.HasPrefix(line, "cpu ") {
						parts := strings.Fields(line)
						if len(parts) >= 8 {
							user, _ := strconv.ParseFloat(parts[1], 64)
							nice, _ := strconv.ParseFloat(parts[2], 64)
							sys, _ := strconv.ParseFloat(parts[3], 64)
							idle, _ := strconv.ParseFloat(parts[4], 64)
							total := user + nice + sys + idle
							fmt.Printf("          %.1f    %.1f    %.1f    %.1f    %.1f    %.1f\n",
								user/total*100, nice/total*100, sys/total*100, 0.0, 0.0, idle/total*100)
						}
					}
				}
			}
			data, err = os.ReadFile("/proc/diskstats")
			if err == nil {
				fmt.Printf("\nDevice             tps    kB_read/s    kB_wrtn/s    kB_read    kB_wrtn\n")
				for _, line := range strings.Split(string(data), "\n") {
					parts := strings.Fields(line)
					if len(parts) >= 14 {
						dev := parts[2]
						reads, _ := strconv.ParseInt(parts[5], 10, 64)
						writes, _ := strconv.ParseInt(parts[9], 10, 64)
						if reads > 0 || writes > 0 {
							fmt.Printf("%-18s %4d    %10d    %10d %10d %10d\n", dev, reads+writes, reads, writes, reads*512/1024, writes*512/1024)
						}
					}
				}
			}
		} else {
			fmt.Printf("iostat: not supported on this platform\n")
		}
		if i < count-1 {
			time.Sleep(time.Second)
		}
	}
	return 0
}

// --- lsof ---
func init() {
	applet.Register(&applet.Applet{Name: "lsof", Short: "List open files", Func: runLsof})
}

func runLsof(args []string) int {
	if runtime.GOOS == "linux" {
		entries, err := os.ReadDir("/proc")
		if err != nil {
			fmt.Fprintf(os.Stderr, "lsof: %v\n", err)
			return 1
		}
		fmt.Printf("%-8s %5s %-10s %s\n", "COMMAND", "PID", "TYPE", "NAME")
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
				link, err := os.Readlink(fmt.Sprintf("%s/%s", fdDir, fd.Name()))
				if err != nil {
					continue
				}
				fmt.Printf("%-8s %5d %-10s %s\n", cmdName, pid, "REG", link)
			}
		}
		return 0
	}
	fmt.Fprintf(os.Stderr, "lsof: not supported on this platform\n")
	return 1
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
	if runtime.GOOS == "linux" {
		data, err := os.ReadFile("/proc/stat")
		if err != nil {
			return 1
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
					total := user + nice + sys + idle
					if total == 0 {
						total = 1
					}
					fmt.Printf("%-8s %5.1f  %5.1f  %5.1f    %5.1f  %5.1f  %5.1f    %5.1f    %5.1f    %5.1f    %5.1f\n",
						parts[0], user/total*100, nice/total*100, sys/total*100, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, idle/total*100)
				}
			}
		}
		return 0
	}
	fmt.Fprintf(os.Stderr, "mpstat: not supported\n")
	return 1
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
	fmt.Fprintf(os.Stderr, "powertop: not yet implemented\n")
	return 1
}

// --- smemcap ---
func init() {
	applet.Register(&applet.Applet{Name: "smemcap", Short: "Collect memory usage data", Func: runSmemcap})
}

func runSmemcap(args []string) int {
	fmt.Fprintf(os.Stderr, "smemcap: not yet implemented\n")
	return 1
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
