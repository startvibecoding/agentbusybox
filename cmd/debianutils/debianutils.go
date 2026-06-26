package debianutils

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/agentbusybox/pkg/applet"
)

func init() {
	applet.Register(&applet.Applet{Name: "run-parts", Short: "Run scripts in a directory", Func: runRunParts})
	applet.Register(&applet.Applet{Name: "start-stop-daemon", Short: "Start and stop system daemon programs", Func: runStartStopDaemon})
	applet.Register(&applet.Applet{Name: "pipe_progress", Short: "Display pipe progress", Func: runPipeProgress})
}

func runRunParts(args []string) int {
	dir := ""
	test := false
	for _, a := range args[1:] {
		if a == "--test" {
			test = true
			continue
		}
		if !strings.HasPrefix(a, "-") {
			dir = a
		}
	}
	if dir == "" {
		fmt.Fprintf(os.Stderr, "run-parts: missing directory\n")
		return 1
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "run-parts: %s: %v\n", dir, err)
		return 1
	}

	scripts := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Skip files with non-alphanumeric chars (except - and _)
		for _, c := range name {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.') {
				goto skip
			}
		}
		scripts = append(scripts, name)
	skip:
	}

	sort.Strings(scripts)
	for _, script := range scripts {
		fullPath := filepath.Join(dir, script)
		if test {
			fmt.Println(fullPath)
			continue
		}
		info, _ := os.Stat(fullPath)
		if info != nil && info.Mode()&0111 != 0 {
			proc, err := os.StartProcess(fullPath, []string{fullPath}, &os.ProcAttr{
				Env:   os.Environ(),
				Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
			})
			if err == nil {
				_, err = proc.Wait()
			}
			if err != nil {
				fmt.Fprintf(os.Stderr, "run-parts: %s: %v\n", script, err)
			}
		}
	}
	return 0
}

func runStartStopDaemon(args []string) int {
	start := false
	stop := false
	background := false
	name := ""
	executable := ""
	pidFile := ""

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-S", "--start":
			start = true
		case "-K", "--stop":
			stop = true
		case "-b", "--background":
			background = true
		case "-n", "--name":
			if i+1 < len(args) {
				i++
				name = args[i]
			}
		case "-x", "--exec":
			if i+1 < len(args) {
				i++
				executable = args[i]
			}
		case "-p", "--pidfile":
			if i+1 < len(args) {
				i++
				pidFile = args[i]
			}
		}
	}

	if stop {
		if pidFile != "" {
			data, err := os.ReadFile(pidFile)
			if err == nil {
				var pid int
				fmt.Sscanf(strings.TrimSpace(string(data)), "%d", &pid)
				if p, err := os.FindProcess(pid); err == nil {
					p.Kill()
				}
			}
		} else if name != "" {
			killProcessesByName(name)
		}
		return 0
	}

	if start {
		if executable != "" {
			command := []string{executable}
			if background {
				proc, err := os.StartProcess(executable, command, &os.ProcAttr{
					Env:   os.Environ(),
					Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
				})
				if err != nil {
					fmt.Fprintf(os.Stderr, "start-stop-daemon: %v\n", err)
					return 1
				}
				if pidFile != "" {
					os.WriteFile(pidFile, []byte(fmt.Sprintf("%d\n", proc.Pid)), 0644)
				}
				go proc.Wait()
				time.Sleep(100 * time.Millisecond)
			} else {
				proc, err := os.StartProcess(executable, command, &os.ProcAttr{
					Env:   os.Environ(),
					Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
				})
				if err != nil {
					return 1
				}
				state, err := proc.Wait()
				if err != nil {
					return 1
				}
				if !state.Success() {
					if status, ok := state.Sys().(syscall.WaitStatus); ok {
						return status.ExitStatus()
					}
					return 1
				}
			}
		}
		return 0
	}

	fmt.Fprintf(os.Stderr, "start-stop-daemon: missing --start or --stop\n")
	return 1
}

func killProcessesByName(name string) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return
	}
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		comm, _ := os.ReadFile(filepath.Join("/proc", entry.Name(), "comm"))
		cmdline, _ := os.ReadFile(filepath.Join("/proc", entry.Name(), "cmdline"))
		commName := strings.TrimSpace(string(comm))
		cmdName := strings.TrimRight(strings.ReplaceAll(string(cmdline), "\x00", " "), " ")
		if commName == name || strings.Contains(cmdName, name) {
			if p, err := os.FindProcess(pid); err == nil {
				_ = p.Kill()
			}
		}
	}
}

func runPipeProgress(args []string) int {
	buf := make([]byte, 4096)
	total := 0
	for {
		n, err := os.Stdin.Read(buf)
		if n > 0 {
			os.Stdout.Write(buf[:n])
			total += n
			fmt.Fprintf(os.Stderr, "\r%d bytes", total)
		}
		if err != nil {
			break
		}
	}
	fmt.Fprintln(os.Stderr)
	return 0
}
