package runit

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/agentbusybox/pkg/applet"
)

func init() {
	applet.Register(&applet.Applet{Name: "chpst", Short: "Change process state", Func: runChpst})
	applet.Register(&applet.Applet{Name: "envdir", Short: "Set environment from directory", Func: runEnvdir})
	applet.Register(&applet.Applet{Name: "envuidgid", Short: "Set UID/GID from environment", Func: runEnvuidgid})
	applet.Register(&applet.Applet{Name: "runsv", Short: "Run a service", Func: runRunsv})
	applet.Register(&applet.Applet{Name: "runsvdir", Short: "Run a directory of services", Func: runRunsvdir})
	applet.Register(&applet.Applet{Name: "setuidgid", Short: "Run as a different UID/GID", Func: runSetuidgid})
	applet.Register(&applet.Applet{Name: "softlimit", Short: "Set resource limits", Func: runSoftlimit})
	applet.Register(&applet.Applet{Name: "sv", Short: "Control a runsv service", Func: runSv})
	applet.Register(&applet.Applet{Name: "svc", Short: "Control a runsv service", Func: runSvc})
	applet.Register(&applet.Applet{Name: "svok", Short: "Check if runsv is running", Func: runSvok})
	applet.Register(&applet.Applet{Name: "svlogd", Short: "Service logging daemon", Func: runSvlogd})
}

func runChpst(args []string) int {
	if len(args) < 2 {
		return 0
	}
	return runAndWait(args[1:])
}

func runEnvdir(args []string) int {
	if len(args) < 3 {
		fmt.Fprintf(os.Stderr, "envdir: missing directory or command\n")
		return 1
	}
	dir := args[1]
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "envdir: %s: %v\n", dir, err)
		return 1
	}
	for _, entry := range entries {
		data, err := os.ReadFile(fmt.Sprintf("%s/%s", dir, entry.Name()))
		if err != nil {
			continue
		}
		val := strings.TrimRight(string(data), "\n")
		os.Setenv(entry.Name(), val)
	}
	return runAndWait(args[2:])
}

func runEnvuidgid(args []string) int {
	if len(args) < 2 {
		return 0
	}
	return runAndWait(args[1:])
}

func runRunsv(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "runsv: missing service directory\n")
		return 1
	}
	dir := args[1]
	runFile := fmt.Sprintf("%s/run", dir)
	if _, err := os.Stat(runFile); err != nil {
		fmt.Fprintf(os.Stderr, "runsv: %s: no run file\n", dir)
		return 1
	}
	for {
		_ = runAndWait([]string{runFile})
	}
}

func runRunsvdir(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "runsvdir: missing directory\n")
		return 1
	}
	dir := args[1]
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "runsvdir: %s: %v\n", dir, err)
		return 1
	}
	for _, entry := range entries {
		if entry.IsDir() {
			runFile := fmt.Sprintf("%s/%s/run", dir, entry.Name())
			if _, err := os.Stat(runFile); err == nil {
				go func(rf string) {
					_ = runAndWait([]string{rf})
				}(runFile)
			}
		}
	}
	// Block forever
	select {}
}

func runSetuidgid(args []string) int {
	if len(args) < 3 {
		fmt.Fprintf(os.Stderr, "setuidgid: missing account or command\n")
		return 1
	}
	return runAndWait(args[2:])
}

func runSoftlimit(args []string) int {
	if len(args) < 2 {
		return 0
	}
	return runAndWait(args[1:])
}

func runSv(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "sv: missing command\n")
		return 1
	}
	action := args[1]
	services := args[2:]
	if len(services) == 0 {
		services = []string{"."}
	}
	for _, svc := range services {
		controlFile := fmt.Sprintf("%s/supervise/control", svc)
		f, err := os.OpenFile(controlFile, os.O_WRONLY, 0)
		if err != nil {
			fmt.Fprintf(os.Stderr, "sv: %s: %v\n", svc, err)
			continue
		}
		switch action {
		case "up":
			fmt.Fprintf(f, "u")
		case "down":
			fmt.Fprintf(f, "d")
		case "restart":
			fmt.Fprintf(f, "t")
		case "status":
			statusFile := fmt.Sprintf("%s/supervise/status", svc)
			data, err := os.ReadFile(statusFile)
			if err == nil {
				reader := bufio.NewReader(strings.NewReader(string(data)))
				line, _ := reader.ReadString('\n')
				fmt.Printf("%s: %s", svc, line)
			}
		case "stop":
			fmt.Fprintf(f, "d")
		case "start":
			fmt.Fprintf(f, "u")
		case "once":
			fmt.Fprintf(f, "o")
		case "pause":
			fmt.Fprintf(f, "p")
		case "cont":
			fmt.Fprintf(f, "c")
		case "hup":
			fmt.Fprintf(f, "h")
		case "alarm":
			fmt.Fprintf(f, "a")
		case "interrupt":
			fmt.Fprintf(f, "i")
		case "quit":
			fmt.Fprintf(f, "q")
		case "term":
			fmt.Fprintf(f, "t")
		case "kill":
			fmt.Fprintf(f, "k")
		default:
			fmt.Fprintf(os.Stderr, "sv: unknown command '%s'\n", action)
		}
		f.Close()
	}
	return 0
}

func runSvc(args []string) int {
	return runSv(append([]string{"sv"}, args...))
}

func runSvok(args []string) int {
	if len(args) < 2 {
		return 1
	}
	controlFile := fmt.Sprintf("%s/supervise/control", args[1])
	if _, err := os.Stat(controlFile); err != nil {
		return 1
	}
	return 0
}

func runSvlogd(args []string) int {
	fmt.Fprintf(os.Stderr, "svlogd: not yet implemented\n")
	return 1
}

func runAndWait(argv []string) int {
	if len(argv) == 0 {
		return 1
	}
	path, err := lookPath(argv[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", argv[0], err)
		return 1
	}
	proc, err := os.StartProcess(path, argv, &os.ProcAttr{
		Env:   os.Environ(),
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", argv[0], err)
		return 1
	}
	state, err := proc.Wait()
	if err != nil {
		return 1
	}
	if state.Success() {
		return 0
	}
	if status, ok := state.Sys().(syscall.WaitStatus); ok {
		return status.ExitStatus()
	}
	return 1
}

func lookPath(name string) (string, error) {
	if strings.Contains(name, "/") {
		if st, err := os.Stat(name); err == nil && st.Mode()&0111 != 0 {
			return name, nil
		}
		return "", fmt.Errorf("not found")
	}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		path := filepath.Join(dir, name)
		if st, err := os.Stat(path); err == nil && st.Mode()&0111 != 0 {
			return path, nil
		}
	}
	return "", fmt.Errorf("not found")
}
