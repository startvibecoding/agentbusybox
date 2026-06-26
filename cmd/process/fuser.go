package process

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/agentbusybox/pkg/applet"
)

func init() {
	applet.Register(&applet.Applet{Name: "fuser", Short: "Identify processes using files or sockets", Func: runFuser})
}

func runFuser(args []string) int {
	kill := false    // -k
	mount := false   // -m
	silent := false  // -s
	verbose := false // -v
	files := []string{}

	for _, a := range args[1:] {
		switch a {
		case "-k", "--kill":
			kill = true
		case "-m":
			mount = true
		case "-s":
			silent = true
		case "-v", "--verbose":
			verbose = true
		default:
			if !strings.HasPrefix(a, "-") {
				files = append(files, a)
			}
		}
	}

	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "fuser: no file specified\n")
		return 1
	}

	_ = mount

	exitCode := 1
	for _, fname := range files {
		pids := findPidsUsingFile(fname)
		if len(pids) > 0 {
			exitCode = 0
			if !silent {
				if verbose {
					for _, pid := range pids {
						comm := getComm(pid)
						fmt.Printf("%s: %8d  %s\n", fname, pid, comm)
					}
				} else {
					pidStrs := []string{}
					for _, p := range pids {
						pidStrs = append(pidStrs, fmt.Sprintf("%d", p))
					}
					fmt.Printf("%s: %s\n", fname, strings.Join(pidStrs, " "))
				}
			}
			if kill {
				for _, pid := range pids {
					p, err := os.FindProcess(pid)
					if err == nil {
						p.Kill()
					}
				}
			}
		}
	}
	return exitCode
}

func findPidsUsingFile(fname string) []int {
	pids := []int{}
	if runtime.GOOS != "linux" {
		return pids
	}

	entries, err := os.ReadDir("/proc")
	if err != nil {
		return pids
	}

	absPath, _ := resolvePath(fname)

	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}

		// Check /proc/PID/fd/*
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
			if link == absPath || link == fname {
				pids = append(pids, pid)
				break
			}
		}

		// Check /proc/PID/maps
		mapsFile := fmt.Sprintf("/proc/%d/maps", pid)
		if f, err := os.Open(mapsFile); err == nil {
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				if strings.Contains(scanner.Text(), fname) {
					pids = append(pids, pid)
					break
				}
			}
			f.Close()
		}
	}
	return pids
}

func resolvePath(p string) (string, error) {
	if abs, err := os.Readlink(p); err == nil {
		return abs, nil
	}
	return p, nil
}

func getComm(pid int) string {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
	if err != nil {
		return "?"
	}
	return strings.TrimSpace(string(data))
}
