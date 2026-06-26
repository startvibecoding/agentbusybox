package misc

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/agentbusybox/pkg/applet"
)

func init() {
	applet.Register(&applet.Applet{Name: "less", Short: "Pager", Func: runLess})
}

func runLess(args []string) int {
	files := args[1:]
	if len(files) == 0 {
		files = []string{"-"}
	}
	// Delegate to system less/more/cat
	for _, fname := range files {
		var data []byte
		var err error
		if fname == "-" {
			data, err = io.ReadAll(os.Stdin)
		} else {
			data, err = os.ReadFile(fname)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "less: %s: %v\n", fname, err)
			return 1
		}
		os.Stdout.Write(data)
	}
	return 0
}

func runMore(args []string) int {
	return runLess(args)
}

// --- clear --- already in console/console.go ---

func init() {
	applet.Register(&applet.Applet{Name: "xxd", Short: "Make a hexdump", Func: runXxd})
}

func runXxd(args []string) int {
	files := []string{}
	for _, a := range args[1:] {
		if !strings.HasPrefix(a, "-") {
			files = append(files, a)
		}
	}
	if len(files) == 0 {
		files = []string{"-"}
	}

	for _, fname := range files {
		var data []byte
		var err error
		if fname == "-" {
			data, err = io.ReadAll(os.Stdin)
		} else {
			data, err = os.ReadFile(fname)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "xxd: %s: %v\n", fname, err)
			return 1
		}

		for i := 0; i < len(data); i += 16 {
			end := i + 16
			if end > len(data) {
				end = len(data)
			}
			fmt.Printf("%08x: ", i)
			for j := i; j < end; j++ {
				fmt.Printf("%02x", data[j])
				if j%2 == 1 {
					fmt.Print(" ")
				}
			}
			for j := end; j < i+16; j++ {
				fmt.Print("   ")
			}
			fmt.Print(" ")
			for j := i; j < end; j++ {
				if data[j] >= 32 && data[j] < 127 {
					fmt.Printf("%c", data[j])
				} else {
					fmt.Print(".")
				}
			}
			fmt.Println()
		}
	}
	return 0
}

func init() {
	applet.Register(&applet.Applet{Name: "time", Short: "Time a command", Func: runTime})
}

func runTime(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "time: missing command\n")
		return 1
	}

	start := time.Now()
	proc, err := startProcess(args[1:])
	code := 1
	if err == nil {
		code = waitProcess(proc)
	}
	elapsed := time.Since(start)

	fmt.Fprintf(os.Stderr, "\nreal\t%v\n", elapsed)
	if err != nil {
		fmt.Fprintf(os.Stderr, "time: %v\n", err)
		return 1
	}
	return code
}

func init() {
	applet.Register(&applet.Applet{Name: "timeout", Short: "Run a command with a time limit", Func: runTimeout})
}

func runTimeout(args []string) int {
	if len(args) < 3 {
		fmt.Fprintf(os.Stderr, "timeout: missing duration or command\n")
		return 1
	}

	duration, err := time.ParseDuration(args[1])
	if err != nil {
		// Try parsing as seconds
		var secs int
		fmt.Sscanf(args[1], "%d", &secs)
		duration = time.Duration(secs) * time.Second
	}

	proc, err := startProcess(args[2:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "timeout: %v\n", err)
		return 1
	}

	done := make(chan int, 1)
	go func() { done <- waitProcess(proc) }()

	select {
	case code := <-done:
		return code
	case <-time.After(duration):
		proc.Kill()
		return 124 // timeout exit code
	}
}

func init() {
	applet.Register(&applet.Applet{Name: "watch", Short: "Execute a program periodically", Func: runWatch})
}

func runWatch(args []string) int {
	interval := 2 * time.Second
	command := ""

	for i := 1; i < len(args); i++ {
		if args[i] == "-n" && i+1 < len(args) {
			i++
			var secs int
			fmt.Sscanf(args[i], "%d", &secs)
			interval = time.Duration(secs) * time.Second
			continue
		}
		command = strings.Join(args[i:], " ")
		break
	}

	if command == "" {
		fmt.Fprintf(os.Stderr, "watch: missing command\n")
		return 1
	}

	for {
		if runtime.GOOS != "windows" {
			fmt.Print("\033[2J\033[H")
		}
		fmt.Printf("Every %v: %s\n\n", interval, command)
		if proc, err := startProcess([]string{"sh", "-c", command}); err == nil {
			_ = waitProcess(proc)
		}
		time.Sleep(interval)
	}
}

func init() {
	applet.Register(&applet.Applet{Name: "nohup", Short: "Run a command immune to hangups", Func: runNohup})
	applet.Register(&applet.Applet{Name: "nice", Short: "Run a program with modified scheduling priority", Func: runNice})
	applet.Register(&applet.Applet{Name: "killall", Short: "Kill processes by name", Func: runKillall})
	applet.Register(&applet.Applet{Name: "tty", Short: "Print terminal name", Func: runTty})
	applet.Register(&applet.Applet{Name: "logname", Short: "Print current login name", Func: runLogname})
	applet.Register(&applet.Applet{Name: "users", Short: "Print current user names", Func: runUsers})
	applet.Register(&applet.Applet{Name: "groups", Short: "Print group names a user is in", Func: runGroups})

	// --- getopt --- already in util-linux/extra.go ---

}

func runWhich(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "which: missing operand\n")
		return 1
	}
	for _, name := range args[1:] {
		path, err := lookPath(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "which: no %s in (PATH)\n", name)
		} else {
			fmt.Println(path)
		}
	}
	return 0
}

func runTrue(args []string) int  { return 0 }
func runFalse(args []string) int { return 1 }

func runYes(args []string) int {
	s := "y"
	if len(args) > 1 {
		s = strings.Join(args[1:], " ")
	}
	for {
		fmt.Println(s)
	}
}

func runNohup(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "nohup: missing command\n")
		return 1
	}
	devNull, _ := os.Open(os.DevNull)
	if devNull == nil {
		devNull = os.Stdin
	}
	proc, err := startProcessWithFiles(args[1:], []*os.File{devNull, os.Stdout, os.Stderr})
	if err != nil {
		fmt.Fprintf(os.Stderr, "nohup: %v\n", err)
		return 1
	}
	_ = proc.Release()
	return 0
}

func runNice(args []string) int {
	if len(args) < 2 {
		fmt.Println("0")
		return 0
	}
	proc, err := startProcess(args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "nice: %v\n", err)
		return 1
	}
	return waitProcess(proc)
}

func runRenice(args []string) int {
	fmt.Fprintf(os.Stderr, "renice: not yet implemented\n")
	return 1
}

func runKillall(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "killall: missing process name\n")
		return 1
	}
	name := args[1]
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 1
	}
	killed := false
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		comm, _ := os.ReadFile("/proc/" + entry.Name() + "/comm")
		cmdline, _ := os.ReadFile("/proc/" + entry.Name() + "/cmdline")
		commName := strings.TrimSpace(string(comm))
		cmdName := strings.TrimRight(strings.ReplaceAll(string(cmdline), "\x00", " "), " ")
		if commName == name || strings.Contains(cmdName, name) {
			p, _ := os.FindProcess(pid)
			if p != nil && p.Kill() == nil {
				killed = true
			}
		}
	}
	if !killed {
		return 1
	}
	return 0
}

func runDmesg(args []string) int {
	if runtime.GOOS == "linux" {
		data, err := os.ReadFile("/var/log/dmesg")
		if err != nil {
			data, err = os.ReadFile("/proc/kmsg")
			if err != nil {
				fmt.Fprintf(os.Stderr, "dmesg: %v\n", err)
				return 1
			}
		}
		os.Stdout.Write(data)
		return 0
	}
	fmt.Fprintf(os.Stderr, "dmesg: not supported on this platform\n")
	return 1
}

func runEject(args []string) int {
	fmt.Fprintf(os.Stderr, "eject: not supported\n")
	return 1
}

func runMknod(args []string) int {
	fmt.Fprintf(os.Stderr, "mknod: not supported\n")
	return 1
}

func runMkfifo(args []string) int {
	fmt.Fprintf(os.Stderr, "mkfifo: not supported on this platform\n")
	return 1
}

func runStty(args []string) int {
	fmt.Fprintf(os.Stderr, "stty: not yet implemented\n")
	return 0
}

func runTty(args []string) int {
	if f, err := os.Open("/proc/self/fd/0"); err == nil {
		defer f.Close()
		name, _ := os.Readlink(f.Name())
		fmt.Println(name)
		return 0
	}
	fmt.Println("/dev/tty")
	return 0
}

func runLogname(args []string) int {
	u := os.Getenv("LOGNAME")
	if u == "" {
		u = os.Getenv("USER")
	}
	if u == "" {
		u = os.Getenv("USERNAME")
	}
	if u == "" {
		fmt.Fprintf(os.Stderr, "logname: no login name\n")
		return 1
	}
	fmt.Println(u)
	return 0
}

func runUsers(args []string) int {
	u := os.Getenv("USER")
	if u == "" {
		u = os.Getenv("USERNAME")
	}
	fmt.Println(u)
	return 0
}

func runGroups(args []string) int {
	u := os.Getenv("USER")
	if u == "" {
		u = os.Getenv("USERNAME")
	}
	fmt.Printf("%s\n", u)
	return 0
}

// --- getopt --- already in util-linux/extra.go ---

func runRealpath(args []string) int {
	if len(args) < 2 {
		return 1
	}
	for _, a := range args[1:] {
		if path, err := os.Readlink(a); err == nil {
			fmt.Println(path)
		} else {
			fmt.Println(a)
		}
	}
	return 0
}

func runReadlink(args []string) int {
	if len(args) < 2 {
		return 1
	}
	for _, a := range args[1:] {
		if path, err := os.Readlink(a); err == nil {
			fmt.Println(path)
		} else {
			return 1
		}
	}
	return 0
}

func runDirname(args []string) int {
	if len(args) < 2 {
		return 1
	}
	for _, a := range args[1:] {
		idx := strings.LastIndex(a, "/")
		if idx < 0 {
			fmt.Println(".")
			continue
		}
		if idx == 0 {
			fmt.Println("/")
			continue
		}
		fmt.Println(a[:idx])
	}
	return 0
}

func runBasename(args []string) int {
	if len(args) < 2 {
		return 1
	}
	suffix := ""
	name := args[1]
	if len(args) > 2 {
		suffix = args[2]
	}
	idx := strings.LastIndex(name, "/")
	if idx >= 0 {
		name = name[idx+1:]
	}
	if suffix != "" && strings.HasSuffix(name, suffix) {
		name = name[:len(name)-len(suffix)]
	}
	fmt.Println(name)
	return 0
}

// --- awk --- already in editors/editors.go ---
func init() {
	applet.Register(&applet.Applet{Name: "make", Short: "GNU make utility", Func: runMake})
}

func runMake(args []string) int {
	makefile := "Makefile"
	targets := []string{}
	for i := 1; i < len(args); i++ {
		if args[i] == "-f" && i+1 < len(args) {
			i++
			makefile = args[i]
			continue
		}
		if !strings.HasPrefix(args[i], "-") {
			targets = append(targets, args[i])
		}
	}
	mf, err := parseMakefile(makefile)
	if err != nil {
		if makefile == "Makefile" {
			mf, err = parseMakefile("makefile")
		}
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "make: %v\n", err)
		return 1
	}
	if len(targets) == 0 {
		if mf.first == "" {
			fmt.Fprintf(os.Stderr, "make: no targets\n")
			return 1
		}
		targets = []string{mf.first}
	}
	seen := map[string]bool{}
	for _, target := range targets {
		if code := mf.run(target, seen); code != 0 {
			return code
		}
	}
	return 0
}

type makeTarget struct {
	deps []string
	cmds []string
}

type makeFile struct {
	first   string
	vars    map[string]string
	targets map[string]*makeTarget
}

func parseMakefile(path string) (*makeFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	mf := &makeFile{vars: map[string]string{}, targets: map[string]*makeTarget{}}
	for _, env := range os.Environ() {
		if idx := strings.Index(env, "="); idx >= 0 {
			mf.vars[env[:idx]] = env[idx+1:]
		}
	}
	var current *makeTarget
	for _, raw := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(raw) == "" || strings.HasPrefix(strings.TrimSpace(raw), "#") {
			continue
		}
		if strings.HasPrefix(raw, "\t") || strings.HasPrefix(raw, "    ") {
			if current != nil {
				current.cmds = append(current.cmds, mf.expand(strings.TrimSpace(raw)))
			}
			continue
		}
		if idx := strings.Index(raw, "="); idx > 0 && !strings.Contains(raw[:idx], ":") {
			name := strings.TrimSpace(raw[:idx])
			value := strings.TrimSpace(raw[idx+1:])
			mf.vars[name] = mf.expand(value)
			continue
		}
		if idx := strings.Index(raw, ":"); idx >= 0 {
			name := strings.TrimSpace(raw[:idx])
			deps := strings.Fields(mf.expand(raw[idx+1:]))
			target := &makeTarget{deps: deps}
			mf.targets[name] = target
			if mf.first == "" {
				mf.first = name
			}
			current = target
		}
	}
	return mf, nil
}

func (mf *makeFile) expand(s string) string {
	for name, value := range mf.vars {
		s = strings.ReplaceAll(s, "$("+name+")", value)
		s = strings.ReplaceAll(s, "${"+name+"}", value)
	}
	return s
}

func (mf *makeFile) run(target string, seen map[string]bool) int {
	if seen[target] {
		return 0
	}
	seen[target] = true
	t, ok := mf.targets[target]
	if !ok {
		if _, err := os.Stat(target); err == nil {
			return 0
		}
		fmt.Fprintf(os.Stderr, "make: *** No rule to make target '%s'. Stop.\n", target)
		return 2
	}
	for _, dep := range t.deps {
		if _, ok := mf.targets[dep]; ok {
			if code := mf.run(dep, seen); code != 0 {
				return code
			}
		}
	}
	for _, raw := range t.cmds {
		cmdline := raw
		echo := true
		ignore := false
		for strings.HasPrefix(cmdline, "@") || strings.HasPrefix(cmdline, "-") {
			if strings.HasPrefix(cmdline, "@") {
				echo = false
			}
			if strings.HasPrefix(cmdline, "-") {
				ignore = true
			}
			cmdline = strings.TrimSpace(cmdline[1:])
		}
		if echo {
			fmt.Println(cmdline)
		}
		proc, err := startProcess([]string{"sh", "-c", cmdline})
		if err != nil && !ignore {
			return 1
		}
		if err == nil {
			code := waitProcess(proc)
			if code != 0 && !ignore {
				return code
			}
		}
	}
	return 0
}

func startProcess(argv []string) (*os.Process, error) {
	return startProcessWithFiles(argv, []*os.File{os.Stdin, os.Stdout, os.Stderr})
}

func startProcessWithFiles(argv []string, files []*os.File) (*os.Process, error) {
	if len(argv) == 0 {
		return nil, fmt.Errorf("missing command")
	}
	path, err := lookPath(argv[0])
	if err != nil {
		return nil, err
	}
	return os.StartProcess(path, argv, &os.ProcAttr{Env: os.Environ(), Files: files})
}

func waitProcess(proc *os.Process) int {
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
		return "", fmt.Errorf("%s: not found", name)
	}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		path := filepath.Join(dir, name)
		if st, err := os.Stat(path); err == nil && st.Mode()&0111 != 0 {
			return path, nil
		}
	}
	return "", fmt.Errorf("%s: not found", name)
}
