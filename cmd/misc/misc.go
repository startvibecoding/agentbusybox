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
	"unsafe"

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
	priority := 0
	pids := []int{}
	isAbsolute := false

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
		} else if !strings.HasPrefix(a, "-") {
			pid, err := strconv.Atoi(a)
			if err == nil {
				pids = append(pids, pid)
			}
		} else {
			// +N or -N
			if strings.HasPrefix(a, "+") {
				fmt.Sscanf(a[1:], "%d", &priority)
			} else {
				fmt.Sscanf(a, "%d", &priority)
				isAbsolute = true
			}
		}
		i++
	}

	if len(pids) == 0 {
		fmt.Fprintf(os.Stderr, "renice: missing pid\n")
		return 1
	}

	exitCode := 0
	for _, pid := range pids {
		if isAbsolute {
			if err := syscall.Setpriority(syscall.PRIO_PROCESS, pid, priority); err != nil {
				fmt.Fprintf(os.Stderr, "renice: %d: %v\n", pid, err)
				exitCode = 1
			} else {
				fmt.Printf("%d: set priority %d\n", pid, priority)
			}
		} else {
			current, err := syscall.Getpriority(syscall.PRIO_PROCESS, pid)
			if err != nil {
				fmt.Fprintf(os.Stderr, "renice: %d: %v\n", pid, err)
				exitCode = 1
				continue
			}
			newPrio := current + priority
			if err := syscall.Setpriority(syscall.PRIO_PROCESS, pid, newPrio); err != nil {
				fmt.Fprintf(os.Stderr, "renice: %d: %v\n", pid, err)
				exitCode = 1
			} else {
				fmt.Printf("%d: set priority %d\n", pid, newPrio)
			}
		}
	}
	return exitCode
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
	clear := false
	raw := false
	level := -1
	size := 0

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-c":
			clear = true
		case "-r":
			raw = true
		case "-n":
			if i+1 < len(args) {
				i++
				fmt.Sscanf(args[i], "%d", &level)
			}
		case "-s":
			if i+1 < len(args) {
				i++
				fmt.Sscanf(args[i], "%d", &size)
			}
		}
	}

	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "dmesg: not supported on this platform\n")
		return 1
	}

	// Set console log level if requested
	if level >= 0 {
		// Write level to /dev/kmsg
		if f, err := os.OpenFile("/dev/kmsg", os.O_WRONLY, 0); err == nil {
			defer f.Close()
			fmt.Fprintf(f, "<%s> ", fmt.Sprintf("%d", level))
		} else {
			fmt.Fprintf(os.Stderr, "dmesg: %v", err)
			return 1
		}
		return 0
	}

	// Try klogctl first via /dev/kmsg, then /proc/kmsg, then /var/log/dmesg
	var data []byte
	var err error

	if size == 0 {
		size = 16 * 1024
	}
	if size > 16*1024*1024 {
		size = 16 * 1024 * 1024
	}

	// Read from /dev/kmsg (preferred)
	f, err := os.Open("/dev/kmsg")
	if err == nil {
		defer f.Close()
		buf := make([]byte, size)
		n, readErr := f.Read(buf)
		if n > 0 {
			data = buf[:n]
		}
		if clear {
			// Seek to end to clear
			f.Seek(0, 2)
		}
		_ = readErr
	}

	// Fallback to /proc/kmsg
	if len(data) == 0 {
		data, err = os.ReadFile("/proc/kmsg")
		if err != nil {
			// Fallback to /var/log/dmesg
			data, err = os.ReadFile("/var/log/dmesg")
			if err != nil {
				fmt.Fprintf(os.Stderr, "dmesg: %v\n", err)
				return 1
			}
		}
	}

	if raw {
		os.Stdout.Write(data)
	} else {
		// Pretty print: strip syslog level prefix <N>
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if len(line) > 0 && line[0] == '<' {
				idx := strings.Index(line, ">")
				if idx >= 0 && idx < 4 {
					line = line[idx+1:]
				}
			}
			fmt.Println(line)
		}
	}

	return 0
}

func runEject(args []string) int {
	device := ""
	trayClose := false
	for _, a := range args[1:] {
		if a == "-t" {
			trayClose = true
		} else if !strings.HasPrefix(a, "-") {
			device = a
		}
	}
	if device == "" {
		device = "/dev/cdrom"
	}
	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "eject: not supported\n")
		return 1
	}
	f, err := os.Open(device)
	if err != nil {
		fmt.Fprintf(os.Stderr, "eject: %s: %v\n", device, err)
		return 1
	}
	defer f.Close()
	// CDROMEJECT = 0x5309, CDROMCLOSETRAY = 0x5319
	const cdromEject = 0x5309
	const cdromCloseTray = 0x5319
	if trayClose {
		_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), cdromCloseTray, 0)
		if errno != 0 {
			fmt.Fprintf(os.Stderr, "eject: %v\n", errno)
			return 1
		}
	} else {
		_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), cdromEject, 0)
		if errno != 0 {
			fmt.Fprintf(os.Stderr, "eject: %v\n", errno)
			return 1
		}
	}
	return 0
}

func runMknod(args []string) int {
	mode := uint32(0)
	major := 0
	minor := 0
	name := ""

	i := 1
	for i < len(args) {
		a := args[i]
		if a == "-m" && i+1 < len(args) {
			i++
			// Parse mode as octal
			fmt.Sscanf(args[i], "%o", &mode)
		} else if !strings.HasPrefix(a, "-") {
			if name == "" {
				name = a
			} else if mode == 0 {
				// First positional arg after name is the type
				switch a {
				case "b":
					mode = syscall.S_IFBLK | 0660
				case "c", "u":
					mode = syscall.S_IFCHR | 0660
				case "p":
					mode = syscall.S_IFIFO | 0660
				}
			} else {
				// Major and minor
				if major == 0 {
					fmt.Sscanf(a, "%d", &major)
				} else {
					fmt.Sscanf(a, "%d", &minor)
				}
			}
		}
		i++
	}

	if name == "" {
		fmt.Fprintf(os.Stderr, "mknod: missing operand\n")
		return 1
	}
	if mode == 0 {
		fmt.Fprintf(os.Stderr, "mknod: missing type\n")
		return 1
	}

	dev := int(((major & 0xfff) << 8) | (minor & 0xff) | ((minor & 0xffffff00) << 12))
	if err := syscall.Mknod(name, mode, dev); err != nil {
		fmt.Fprintf(os.Stderr, "mknod: %s: %v\n", name, err)
		return 1
	}
	return 0
}

func runMkfifo(args []string) int {
	mode := uint32(0666)
	names := []string{}

	for i := 1; i < len(args); i++ {
		if args[i] == "-m" && i+1 < len(args) {
			i++
			fmt.Sscanf(args[i], "%o", &mode)
		} else if !strings.HasPrefix(args[i], "-") {
			names = append(names, args[i])
		}
	}

	if len(names) == 0 {
		fmt.Fprintf(os.Stderr, "mkfifo: missing operand\n")
		return 1
	}

	exitCode := 0
	for _, name := range names {
		if err := syscall.Mkfifo(name, mode); err != nil {
			fmt.Fprintf(os.Stderr, "mkfifo: %s: %v\n", name, err)
			exitCode = 1
		}
	}
	return exitCode
}

func runStty(args []string) int {
	showAll := false
	showReadable := false

	for _, a := range args[1:] {
		if a == "-a" || a == "--all" {
			showAll = true
		} else if a == "-g" || a == "--save" {
			showReadable = true
		}
	}

	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "stty: not supported on this platform\n")
		return 1
	}

	f, err := os.Open("/dev/tty")
	if err != nil {
		f = os.Stdin
	}
	defer func() {
		if f != os.Stdin {
			f.Close()
		}
	}()

	// Get terminal attributes via ioctl
	var termios syscall.Termios
	_, _, errno := syscall.Syscall6(syscall.SYS_IOCTL, f.Fd(),
		0x5401, // TCGETS
		uintptr(unsafe.Pointer(&termios)), 0, 0, 0)
	if errno != 0 {
		fmt.Fprintf(os.Stderr, "stty: %v\n", errno)
		return 1
	}

	if showReadable {
		// Output in stty-readable format
		fmt.Printf("%x:%x:%x:%x\n",
			termios.Iflag, termios.Oflag, termios.Cflag, termios.Lflag)
		return 0
	}

	if showAll {
		fmt.Printf("speed %d baud; line = 0;\n", getBaudRate(termios))
		fmt.Printf("min = %d; time = %d;\n", termios.Cc[syscall.VMIN], termios.Cc[syscall.VTIME])
		fmt.Printf("-brkint -imaxbel\n")
		fmt.Printf("-icanon -echo -echoe -echok\n")
		return 0
	}

	// Default output: show changed settings
	fmt.Printf("speed %d baud; line = 0;\n", getBaudRate(termios))
	return 0
}

func getBaudRate(t syscall.Termios) int {
	baud := t.Cflag & 0xf
	switch baud {
	case 0:
		return 0
	case 1:
		return 50
	case 2:
		return 75
	case 3:
		return 110
	case 4:
		return 134
	case 5:
		return 150
	case 6:
		return 200
	case 7:
		return 300
	case 8:
		return 600
	case 9:
		return 1200
	case 10:
		return 1800
	case 11:
		return 2400
	case 12:
		return 4800
	case 13:
		return 9600
	case 14:
		return 19200
	case 15:
		return 38400
	default:
		return 9600
	}
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
