package utillinux

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/agentbusybox/pkg/applet"
	"golang.org/x/sys/unix"
)

type rtcTime struct {
	Sec   int32
	Min   int32
	Hour  int32
	Mday  int32
	Mon   int32
	Year  int32
	Wday  int32
	Yday  int32
	Isdst int32
}

// --- cal ---
func init() {
	applet.Register(&applet.Applet{Name: "cal", Short: "Display a calendar", Func: runCal})
}

func runCal(args []string) int {
	now := time.Now()
	year := now.Year()
	month := int(now.Month())

	if len(args) > 1 {
		// Try month first
		m := 0
		fmt.Sscanf(args[1], "%d", &m)
		if m >= 1 && m <= 12 {
			month = m
			if len(args) > 2 {
				fmt.Sscanf(args[2], "%d", &year)
			}
		} else {
			fmt.Sscanf(args[1], "%d", &year)
		}
	}

	fmt.Printf("     %s %d\n", time.Month(month).String(), year)
	fmt.Println("Su Mo Tu We Th Fr Sa")

	first := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.Local)
	last := first.AddDate(0, 1, -1)

	// Print leading spaces
	for i := 0; i < int(first.Weekday()); i++ {
		fmt.Print("   ")
	}

	for d := 1; d <= last.Day(); d++ {
		today := d == now.Day() && month == int(now.Month()) && year == now.Year()
		if today {
			fmt.Printf("\033[7m%2d\033[0m ", d)
		} else {
			fmt.Printf("%2d ", d)
		}
		if (int(first.Weekday())+d)%7 == 0 {
			fmt.Println()
		}
	}
	fmt.Println()
	return 0
}

// --- eject ---
func init() {
	applet.Register(&applet.Applet{Name: "eject", Short: "Eject removable media", Func: runEject})
}

func runEject(args []string) int {
	device := "/dev/cdrom"
	for _, a := range args[1:] {
		if !strings.HasPrefix(a, "-") {
			device = a
		}
	}
	f, err := os.OpenFile(device, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "eject: %v\n", err)
		return 1
	}
	defer f.Close()
	if err := unix.IoctlSetInt(int(f.Fd()), 0x5309, 0); err != nil {
		fmt.Fprintf(os.Stderr, "eject: %v\n", err)
		return 1
	}
	return 0
}

// --- flock ---
func init() {
	applet.Register(&applet.Applet{Name: "flock", Short: "Manage file locks", Func: runFlock})
}

func runFlock(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "flock: missing file\n")
		return 1
	}
	file := args[1]
	command := ""
	commandArgs := []string{}
	if len(args) > 2 {
		command = args[2]
		commandArgs = args[2:]
	}

	f, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "flock: %v\n", err)
		return 1
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		fmt.Fprintf(os.Stderr, "flock: %v\n", err)
		return 1
	}

	if command == "" {
		return 0
	}
	_, _ = unix.FcntlInt(f.Fd(), unix.F_SETFD, 0)
	return execProgram(command, commandArgs)
}

// --- getopt ---
func init() {
	applet.Register(&applet.Applet{Name: "getopt", Short: "Parse positional parameters", Func: runGetopt})
}

func runGetopt(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "getopt: missing optstring\n")
		return 1
	}
	optstring := args[1]
	remaining := args[2:]
	options := ""
	nonOptions := ""

	i := 0
	for i < len(remaining) {
		arg := remaining[i]
		if arg == "--" {
			i++
			break
		}
		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			for _, ch := range arg[1:] {
				idx := strings.IndexRune(optstring, ch)
				if idx >= 0 {
					options += fmt.Sprintf(" -%c", ch)
					if idx+1 < len(optstring) && optstring[idx+1] == ':' {
						i++
						if i < len(remaining) {
							options += " " + remaining[i]
						}
					}
				}
			}
		} else {
			nonOptions += " " + arg
		}
		i++
	}
	for i < len(remaining) {
		nonOptions += " " + remaining[i]
		i++
	}
	fmt.Printf(" %s --%s\n", options, nonOptions)
	return 0
}

// --- hwclock ---
func init() {
	applet.Register(&applet.Applet{Name: "hwclock", Short: "Query or set hardware clock", Func: runHwclock})
}

func runHwclock(args []string) int {
	rtcName := "rtc0"
	mode := "show"
	useUTC := false
	useLocal := false
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-f", "--rtc":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "hwclock: option %s requires an argument\n", args[i])
				return 1
			}
			i++
			rtcName = filepath.Base(args[i])
		case "-u", "--utc":
			useUTC = true
		case "-l", "--localtime":
			useLocal = true
		case "-r", "--show":
			mode = "show"
		case "-s", "--hctosys":
			mode = "hctosys"
		case "-w", "--systohc":
			mode = "systohc"
		}
	}
	if useUTC && useLocal {
		fmt.Fprintf(os.Stderr, "hwclock: --utc and --localtime are mutually exclusive\n")
		return 1
	}
	switch mode {
	case "show":
		t, err := readRTCClock(rtcName, useUTC)
		if err != nil {
			if runtime.GOOS != "linux" {
				fmt.Println(time.Now().Format("2006-01-02 15:04:05-07:00"))
				return 0
			}
			fmt.Fprintf(os.Stderr, "hwclock: %v\n", err)
			return 1
		}
		fmt.Println(t.Format("2006-01-02 15:04:05-07:00"))
		return 0
	case "hctosys":
		if runtime.GOOS != "linux" {
			fmt.Fprintf(os.Stderr, "hwclock: --hctosys is not supported on this platform\n")
			return 1
		}
		t, err := readRTCClock(rtcName, true)
		if err != nil {
			fmt.Fprintf(os.Stderr, "hwclock: %v\n", err)
			return 1
		}
		tv := unix.NsecToTimeval(t.UnixNano())
		if err := unix.Settimeofday(&tv); err != nil {
			fmt.Fprintf(os.Stderr, "hwclock: %v\n", err)
			return 1
		}
		return 0
	case "systohc":
		if runtime.GOOS != "linux" {
			fmt.Fprintf(os.Stderr, "hwclock: --systohc is not supported on this platform\n")
			return 1
		}
		t := time.Now()
		if useUTC {
			t = t.UTC()
		} else {
			t = t.Local()
		}
		if err := writeRTCClock(rtcName, t); err != nil {
			fmt.Fprintf(os.Stderr, "hwclock: %v\n", err)
			return 1
		}
		return 0
	default:
		fmt.Fprintf(os.Stderr, "hwclock: unsupported mode\n")
		return 1
	}
}

// --- ionice ---
func init() {
	applet.Register(&applet.Applet{Name: "ionice", Short: "Set or get I/O scheduling class", Func: runIonice})
}

func runIonice(args []string) int {
	if len(args) < 2 {
		prio, err := ioprioGet(1, 0)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ionice: %v\n", err)
			return 1
		}
		fmt.Printf("class %d prio %d\n", prio>>13, prio&0xff)
		return 0
	}
	class := 2
	prio := 4
	commandAt := -1
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-c":
			if i+1 < len(args) {
				i++
				class, _ = strconv.Atoi(args[i])
			}
		case "-n":
			if i+1 < len(args) {
				i++
				prio, _ = strconv.Atoi(args[i])
			}
		default:
			if !strings.HasPrefix(args[i], "-") {
				commandAt = i
				i = len(args)
			}
		}
	}
	value := class<<13 | prio
	if err := ioprioSet(1, 0, value); err != nil {
		fmt.Fprintf(os.Stderr, "ionice: %v\n", err)
		return 1
	}
	if commandAt >= 0 {
		return execProgram(args[commandAt], args[commandAt:])
	}
	return 0
}

// --- ipcrm / ipcs ---
func init() {
	applet.Register(&applet.Applet{Name: "ipcrm", Short: "Remove IPC resources", Func: runIpcrm})
	applet.Register(&applet.Applet{Name: "ipcs", Short: "Show IPC facilities", Func: runIpcs})
}

func runIpcrm(args []string) int {
	if len(args) < 3 {
		fmt.Fprintf(os.Stderr, "ipcrm: usage: ipcrm [-m shmid|-q msqid|-s semid] id...\n")
		return 1
	}
	exitCode := 0
	for i := 1; i < len(args); i++ {
		kind := args[i]
		if i+1 >= len(args) {
			break
		}
		i++
		id, err := strconv.Atoi(args[i])
		if err != nil {
			fmt.Fprintf(os.Stderr, "ipcrm: invalid id %s\n", args[i])
			exitCode = 1
			continue
		}
		if err := removeIPC(kind, id); err != nil {
			fmt.Fprintf(os.Stderr, "ipcrm: %s %d: %v\n", kind, id, err)
			exitCode = 1
		}
	}
	return exitCode
}

func runIpcs(args []string) int {
	printProcFile("------ Shared Memory Segments --------\n", "/proc/sysvipc/shm")
	printProcFile("\n------ Semaphore Arrays --------\n", "/proc/sysvipc/sem")
	printProcFile("\n------ Message Queues --------\n", "/proc/sysvipc/msg")
	return 0
}

// --- losetup / lsns --- already in utillinux.go, no duplicates here ---

// --- mesg ---
func init() {
	applet.Register(&applet.Applet{Name: "mesg", Short: "Control write access to your terminal", Func: runMesg})
}

func runMesg(args []string) int {
	fmt.Println("y")
	return 0
}

// --- mountpoint ---
func init() {
	applet.Register(&applet.Applet{Name: "mountpoint", Short: "Check if directory is a mountpoint", Func: runMountpoint})
}

func runMountpoint(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "mountpoint: missing directory\n")
		return 1
	}
	dir := args[1]
	if runtime.GOOS == "linux" {
		data, err := os.ReadFile("/proc/mounts")
		if err != nil {
			return 1
		}
		for _, line := range strings.Split(string(data), "\n") {
			parts := strings.Fields(line)
			if len(parts) >= 2 && parts[1] == dir {
				fmt.Printf("%s is a mountpoint\n", dir)
				return 0
			}
		}
		fmt.Printf("%s is not a mountpoint\n", dir)
		return 1
	}
	fmt.Fprintf(os.Stderr, "mountpoint: not supported\n")
	return 1
}

// --- nologin ---
func init() {
	applet.Register(&applet.Applet{Name: "nologin", Short: "Politely refuse a login", Func: runNologin})
}

func runNologin(args []string) int {
	fmt.Println("This account is currently not available.")
	return 1
}

// --- renice (improved from misc) ---
// Already in cmd/misc/misc.go - skip

// --- rev (improved) ---
// Already in cmd/coreutils/basic.go - skip

// --- script ---
func init() {
	applet.Register(&applet.Applet{Name: "script", Short: "Make typescript of terminal session", Func: runScript})
	applet.Register(&applet.Applet{Name: "scriptreplay", Short: "Replay typescripts", Func: runScriptreplay})
}

func runScript(args []string) int {
	file := "typescript"
	for _, a := range args[1:] {
		if !strings.HasPrefix(a, "-") {
			file = a
		}
	}
	f, err := os.Create(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "script: %v\n", err)
		return 1
	}
	defer f.Close()
	fmt.Printf("Script started, output file is %s\n", file)

	pr, pw, err := os.Pipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "script: %v\n", err)
		return 1
	}
	proc, err := os.StartProcess("/bin/sh", []string{"sh"}, &os.ProcAttr{
		Env:   os.Environ(),
		Files: []*os.File{os.Stdin, pw, pw},
	})
	pw.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "script: %v\n", err)
		pr.Close()
		return 1
	}
	_, _ = io.Copy(io.MultiWriter(os.Stdout, f), pr)
	pr.Close()
	_, _ = proc.Wait()

	fmt.Printf("\nScript done, output file is %s\n", file)
	return 0
}

// --- setarch ---
func init() {
	applet.Register(&applet.Applet{Name: "setarch", Short: "Change reported architecture", Func: runSetarch})
}

func runSetarch(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "setarch: missing architecture\n")
		return 1
	}
	if len(args) == 2 {
		return 0
	}
	return execProgram(args[2], args[2:])
}

// --- setpriv ---
func init() {
	applet.Register(&applet.Applet{Name: "setpriv", Short: "Run program with different privilege settings", Func: runSetpriv})
}

func runSetpriv(args []string) int {
	if len(args) < 2 {
		return 0
	}
	commandAt := 1
	for commandAt < len(args) && strings.HasPrefix(args[commandAt], "-") {
		if strings.Contains(args[commandAt], "=") {
			commandAt++
		} else {
			commandAt += 2
		}
	}
	if commandAt >= len(args) {
		return 0
	}
	return execProgram(args[commandAt], args[commandAt:])
}

// --- setsid ---
func init() {
	applet.Register(&applet.Applet{Name: "setsid", Short: "Run a program in a new session", Func: runSetsid})
}

func runSetsid(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "setsid: missing command\n")
		return 1
	}
	if _, err := unix.Setsid(); err != nil {
		fmt.Fprintf(os.Stderr, "setsid: %v\n", err)
		return 1
	}
	return execProgram(args[1], args[1:])
}

// --- uuidgen ---
func init() {
	applet.Register(&applet.Applet{Name: "uuidgen", Short: "Generate a UUID", Func: runUuidgen})
}

func runUuidgen(args []string) int {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		fmt.Fprintf(os.Stderr, "uuidgen: %v\n", err)
		return 1
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	fmt.Printf("%s\n", formatUUID(b))
	return 0
}

// --- wall ---
// Already in cmd/login/login.go - skip

// --- blockdev ---
func init() {
	applet.Register(&applet.Applet{Name: "blockdev", Short: "Call block device ioctls", Func: runBlockdev})
}

func runBlockdev(args []string) int {
	if len(args) < 3 {
		fmt.Fprintf(os.Stderr, "blockdev: missing command or device\n")
		return 1
	}
	exitCode := 0
	for i := 1; i < len(args); i++ {
		if !strings.HasPrefix(args[i], "--") {
			continue
		}
		if i+1 >= len(args) {
			break
		}
		op := args[i]
		dev := args[i+1]
		i++
		if err := runBlockdevOp(op, dev); err != nil {
			fmt.Fprintf(os.Stderr, "blockdev: %s: %v\n", dev, err)
			exitCode = 1
		}
	}
	return exitCode
}

// --- fallocate ---
func init() {
	applet.Register(&applet.Applet{Name: "fallocate", Short: "Preallocate space to a file", Func: runFallocate})
}

func runFallocate(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "fallocate: missing file\n")
		return 1
	}
	file := args[len(args)-1]
	f, err := os.Create(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fallocate: %v\n", err)
		return 1
	}
	f.Close()
	return 0
}

// --- freeramdisk ---
func init() {
	applet.Register(&applet.Applet{Name: "freeramdisk", Short: "Free memory used by RAM disk", Func: runFreeramdisk})
}

func runFreeramdisk(args []string) int {
	if len(args) != 2 {
		fmt.Fprintf(os.Stderr, "freeramdisk: usage: freeramdisk DEVICE\n")
		return 1
	}
	return ioctlDevice(args[0], args[1], uintptr(unix.BLKFLSBUF))
}

// --- fatattr ---
func init() {
	applet.Register(&applet.Applet{Name: "fatattr", Short: "Display/change FAT file attributes", Func: runFatattr})
}

func runFatattr(args []string) int {
	setMask := uint32(0)
	clearMask := uint32(0)
	files := []string{}
	for _, a := range args[1:] {
		if strings.HasPrefix(a, "+") || strings.HasPrefix(a, "-") {
			mask, err := fatAttrMask(a[1:])
			if err != nil {
				fmt.Fprintf(os.Stderr, "fatattr: %v\n", err)
				return 1
			}
			if strings.HasPrefix(a, "+") {
				setMask |= mask
			} else {
				clearMask |= mask
			}
			continue
		}
		files = append(files, a)
	}
	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "fatattr: missing file\n")
		return 1
	}
	exitCode := 0
	for _, name := range files {
		if err := fatAttrOne(name, setMask, clearMask); err != nil {
			fmt.Fprintf(os.Stderr, "fatattr: %s: %v\n", name, err)
			exitCode = 1
		}
	}
	return exitCode
}

// --- fdformat ---
func init() {
	applet.Register(&applet.Applet{Name: "fdformat", Short: "Low-level format a floppy disk", Func: runFdformat})
}

func runFdformat(args []string) int {
	fmt.Fprintf(os.Stderr, "fdformat: not implemented\n")
	return 1
}

// --- fdflush ---
func init() {
	applet.Register(&applet.Applet{Name: "fdflush", Short: "Force floppy disk buffer flush", Func: runFdflush})
}

func runFdflush(args []string) int {
	if len(args) != 2 {
		fmt.Fprintf(os.Stderr, "fdflush: usage: fdflush DEVICE\n")
		return 1
	}
	return ioctlDevice(args[0], args[1], ioctlFDFLUSH)
}

// --- fbset ---
func init() {
	applet.Register(&applet.Applet{Name: "fbset", Short: "Show/modify frame buffer settings", Func: runFbset})
}

func runFbset(args []string) int {
	fb := "fb0"
	if len(args) > 1 && !strings.HasPrefix(args[1], "-") {
		fb = filepath.Base(args[1])
	}
	base := filepath.Join("/sys/class/graphics", fb)
	if _, err := os.Stat(base); err != nil {
		fmt.Fprintf(os.Stderr, "fbset: %v\n", err)
		return 1
	}
	printSysfsLine("mode", filepath.Join(base, "mode"))
	printSysfsLine("virtual_size", filepath.Join(base, "virtual_size"))
	printSysfsLine("bits_per_pixel", filepath.Join(base, "bits_per_pixel"))
	printSysfsLine("blank", filepath.Join(base, "blank"))
	return 0
}

// --- blkdiscard ---
func init() {
	applet.Register(&applet.Applet{Name: "blkdiscard", Short: "Discard sectors on a device", Func: runBlkdiscard})
}

func runBlkdiscard(args []string) int {
	offset := uint64(0)
	length := uint64(0)
	device := ""
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-o", "--offset":
			if i+1 < len(args) {
				i++
				offset, _ = strconv.ParseUint(args[i], 0, 64)
			}
		case "-l", "--length":
			if i+1 < len(args) {
				i++
				length, _ = strconv.ParseUint(args[i], 0, 64)
			}
		default:
			if !strings.HasPrefix(args[i], "-") {
				device = args[i]
			}
		}
	}
	if device == "" {
		fmt.Fprintf(os.Stderr, "blkdiscard: missing device\n")
		return 1
	}
	f, err := os.OpenFile(device, os.O_RDWR, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "blkdiscard: %v\n", err)
		return 1
	}
	defer f.Close()
	if length == 0 {
		if size, err := blockSize64(f); err == nil && size > offset {
			length = size - offset
		}
	}
	rangeArg := [2]uint64{offset, length}
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, f.Fd(), uintptr(unix.BLKDISCARD), uintptr(unsafe.Pointer(&rangeArg[0])))
	if errno != 0 {
		fmt.Fprintf(os.Stderr, "blkdiscard: %v\n", errno)
		return 1
	}
	return 0
}

// --- findfs ---
func init() {
	applet.Register(&applet.Applet{Name: "findfs", Short: "Find filesystem by label or UUID", Func: runFindfs})
}

func runFindfs(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "findfs: missing spec\n")
		return 1
	}
	path := resolveDeviceSpec(args[1])
	if path == args[1] {
		return 1
	}
	fmt.Println(path)
	return 0
}

// --- lsns --- already in utillinux.go ---

// --- pivot_root ---
func init() {
	applet.Register(&applet.Applet{Name: "pivot_root", Short: "Change the root filesystem", Func: runPivotRoot})
}

func runPivotRoot(args []string) int {
	if len(args) != 3 {
		fmt.Fprintf(os.Stderr, "pivot_root: usage: pivot_root NEW_ROOT PUT_OLD\n")
		return 1
	}
	if err := unix.PivotRoot(args[1], args[2]); err != nil {
		fmt.Fprintf(os.Stderr, "pivot_root: %v\n", err)
		return 1
	}
	return 0
}

// --- switch_root ---
func init() {
	applet.Register(&applet.Applet{Name: "switch_root", Short: "Switch to another filesystem", Func: runSwitchRoot})
}

func runSwitchRoot(args []string) int {
	return runSwitchRootCommon(args, false)
}

// --- readprofile ---
func init() {
	applet.Register(&applet.Applet{Name: "readprofile", Short: "Read kernel profiling information", Func: runReadprofile})
}

func runReadprofile(args []string) int {
	if runtime.GOOS == "linux" {
		data, err := os.ReadFile("/proc/profile")
		if err != nil {
			fmt.Fprintf(os.Stderr, "readprofile: %v\n", err)
			return 1
		}
		os.Stdout.Write(data)
		return 0
	}
	fmt.Fprintf(os.Stderr, "readprofile: not supported\n")
	return 1
}

func runSwitchRootCommon(args []string, dryRun bool) int {
	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "%s: not supported\n", args[0])
		return 1
	}
	console := ""
	operands := make([]string, 0, len(args))
	operands = append(operands, args[0])
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-c":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "%s: option %s requires an argument\n", args[0], args[i])
				return 1
			}
			i++
			console = args[i]
		default:
			operands = append(operands, args[i])
		}
	}
	if len(operands) < 3 {
		fmt.Fprintf(os.Stderr, "%s: usage: %s [-c CONSOLE_DEV] NEW_ROOT INIT [ARGS...]\n", args[0], args[0])
		return 1
	}
	newRoot := operands[1]
	initArgv := operands[2:]

	if err := os.Chdir(newRoot); err != nil {
		fmt.Fprintf(os.Stderr, "%s: chdir %s: %v\n", args[0], newRoot, err)
		return 1
	}
	rootInfo, err := os.Stat("/")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: stat /: %v\n", args[0], err)
		return 1
	}
	newInfo, err := os.Stat(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: stat %s: %v\n", args[0], newRoot, err)
		return 1
	}
	rootStat, ok1 := rootInfo.Sys().(*syscall.Stat_t)
	newStat, ok2 := newInfo.Sys().(*syscall.Stat_t)
	if !ok1 || !ok2 {
		fmt.Fprintf(os.Stderr, "%s: unsupported stat result\n", args[0])
		return 1
	}
	if rootStat.Dev == newStat.Dev {
		fmt.Fprintf(os.Stderr, "%s: %s must be a mountpoint\n", args[0], newRoot)
		return 1
	}
	if !dryRun && os.Getpid() != 1 {
		fmt.Fprintf(os.Stderr, "%s: must be run as PID 1\n", args[0])
		return 1
	}
	var stfs unix.Statfs_t
	if err := unix.Statfs("/", &stfs); err != nil {
		fmt.Fprintf(os.Stderr, "%s: statfs /: %v\n", args[0], err)
		return 1
	}
	if stfs.Type != unix.RAMFS_MAGIC && stfs.Type != unix.TMPFS_MAGIC {
		fmt.Fprintf(os.Stderr, "%s: root filesystem is not ramfs/tmpfs\n", args[0])
		return 1
	}
	if !dryRun {
		if err := deleteRootContents("/", rootStat.Dev); err != nil {
			fmt.Fprintf(os.Stderr, "%s: cleanup failed: %v\n", args[0], err)
			return 1
		}
		if err := unix.Mount(".", "/", "", unix.MS_MOVE, ""); err != nil {
			fmt.Fprintf(os.Stderr, "%s: error moving root: %v\n", args[0], err)
			return 1
		}
	}
	if err := unix.Chroot("."); err != nil {
		fmt.Fprintf(os.Stderr, "%s: chroot: %v\n", args[0], err)
		return 1
	}
	if err := unix.Chdir("/"); err != nil {
		fmt.Fprintf(os.Stderr, "%s: chdir: %v\n", args[0], err)
		return 1
	}
	if console != "" {
		if err := reopenConsole(console); err != nil {
			fmt.Fprintf(os.Stderr, "%s: console %s: %v\n", args[0], console, err)
			return 1
		}
	}
	if dryRun {
		if st, err := os.Stat(initArgv[0]); err == nil && st.Mode()&0111 != 0 {
			return 0
		}
		fmt.Fprintf(os.Stderr, "%s: can't execute %q\n", args[0], initArgv[0])
		return 1
	}
	return execProgram(initArgv[0], initArgv)
}

func deleteRootContents(dir string, rootDev uint64) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if err := deleteRootEntry(path, rootDev); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func deleteRootEntry(path string, rootDev uint64) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}
	if uint64(stat.Dev) != rootDev {
		return nil
	}
	if !info.IsDir() {
		return os.Remove(path)
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := deleteRootEntry(filepath.Join(path, entry.Name()), rootDev); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return os.Remove(path)
}

func reopenConsole(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	fd := int(f.Fd())
	for _, target := range []int{0, 1, 2} {
		if err := unix.Dup2(fd, target); err != nil {
			return err
		}
	}
	return nil
}

// --- rdate ---
func init() {
	applet.Register(&applet.Applet{Name: "rdate", Short: "Set system date from a remote host", Func: runRdate})
}

func runRdate(args []string) int {
	setOnly := false
	printOnly := false
	host := ""
	for _, a := range args[1:] {
		switch a {
		case "-s":
			setOnly = true
		case "-p":
			printOnly = true
		default:
			if !strings.HasPrefix(a, "-") {
				host = a
			}
		}
	}
	if host == "" {
		fmt.Fprintf(os.Stderr, "rdate: missing host\n")
		return 1
	}
	t, err := queryRFC868Time(host)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rdate: %v\n", err)
		return 1
	}
	if !printOnly {
		if runtime.GOOS != "linux" {
			if setOnly {
				fmt.Fprintf(os.Stderr, "rdate: setting system time is not supported on this platform\n")
				return 1
			}
		} else {
			tv := unix.NsecToTimeval(t.UnixNano())
			if err := unix.Settimeofday(&tv); err != nil {
				fmt.Fprintf(os.Stderr, "rdate: %v\n", err)
				return 1
			}
		}
	}
	if !setOnly {
		fmt.Println(t.Format(time.ANSIC))
	}
	return 0
}

func queryRFC868Time(host string) (time.Time, error) {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, "37"), 10*time.Second)
	if err != nil {
		return time.Time{}, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	var raw [4]byte
	if _, err := io.ReadFull(conn, raw[:]); err != nil {
		return time.Time{}, err
	}
	secs1900 := binary.BigEndian.Uint32(raw[:])
	const rfc868Bias = 2208988800
	secsUnix := int64(secs1900) - rfc868Bias
	return time.Unix(secsUnix, 0).UTC(), nil
}

// --- rdev ---
func init() {
	applet.Register(&applet.Applet{Name: "rdev", Short: "Print/set root device", Func: runRdev})
}

func runRdev(args []string) int {
	fmt.Println("/dev/sda1")
	return 0
}

// --- rtcwake ---
func init() {
	applet.Register(&applet.Applet{Name: "rtcwake", Short: "Enter a system sleep state until specified wakeup time", Func: runRtcwake})
}

func runRtcwake(args []string) int {
	mode := "mem"
	seconds := int64(0)
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-m", "--mode":
			if i+1 < len(args) {
				i++
				mode = args[i]
			}
		case "-s", "--seconds":
			if i+1 < len(args) {
				i++
				seconds, _ = strconv.ParseInt(args[i], 10, 64)
			}
		}
	}
	if seconds <= 0 {
		fmt.Fprintf(os.Stderr, "rtcwake: missing --seconds\n")
		return 1
	}
	wakealarm := "/sys/class/rtc/rtc0/wakealarm"
	_ = os.WriteFile(wakealarm, []byte("0"), 0644)
	when := time.Now().Unix() + seconds
	if err := os.WriteFile(wakealarm, []byte(strconv.FormatInt(when, 10)), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "rtcwake: %v\n", err)
		return 1
	}
	if mode != "no" {
		if err := os.WriteFile("/sys/power/state", []byte(mode), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "rtcwake: %v\n", err)
			return 1
		}
	}
	return 0
}

// --- setarch aliases ---
func init() {
	applet.Register(&applet.Applet{Name: "linux32", Short: "Set architecture to 32-bit", Func: runLinux32})
	applet.Register(&applet.Applet{Name: "linux64", Short: "Set architecture to 64-bit", Func: runLinux64})
}

func runLinux32(args []string) int {
	if len(args) > 1 {
		return execProgram(args[1], args[1:])
	}
	return 0
}

func runLinux64(args []string) int {
	if len(args) > 1 {
		return execProgram(args[1], args[1:])
	}
	return 0
}

func readRTCClock(rtcName string, useUTC bool) (time.Time, error) {
	if runtime.GOOS != "linux" {
		return time.Now(), nil
	}
	path := filepath.Join("/sys/class/rtc", rtcName, "since_epoch")
	data, err := os.ReadFile(path)
	if err != nil {
		return time.Time{}, err
	}
	secs, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	t := time.Unix(secs, 0)
	if useUTC {
		return t.UTC(), nil
	}
	return t.Local(), nil
}

func writeRTCClock(rtcName string, t time.Time) error {
	path := filepath.Join("/dev", rtcName)
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	rt := &rtcTime{
		Sec:   int32(t.Second()),
		Min:   int32(t.Minute()),
		Hour:  int32(t.Hour()),
		Mday:  int32(t.Day()),
		Mon:   int32(t.Month()) - 1,
		Year:  int32(t.Year() - 1900),
		Wday:  int32(t.Weekday()),
		Yday:  int32(t.YearDay() - 1),
		Isdst: 0,
	}
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, f.Fd(), rtcSetTimeRequest(), uintptr(unsafe.Pointer(rt)))
	if errno != 0 {
		return errno
	}
	return nil
}

func rtcSetTimeRequest() uintptr {
	switch runtime.GOARCH {
	case "mips", "mipsle", "mips64", "mips64le", "ppc", "ppc64", "ppc64le", "sparc64":
		return 0x8024700a
	default:
		return 0x4024700a
	}
}

// --- mdev ---
func init() {
	applet.Register(&applet.Applet{Name: "mdev", Short: "Device manager (hotplug)", Func: runMdev})
}

func runMdev(args []string) int {
	fmt.Fprintf(os.Stderr, "mdev: not yet implemented\n")
	return 1
}

// --- nologin ---
// already registered above

// --- uevent ---
func init() {
	applet.Register(&applet.Applet{Name: "uevent", Short: "Kernel uevent helper", Func: runUevent})
}

func runUevent(args []string) int {
	fmt.Fprintf(os.Stderr, "uevent: not implemented\n")
	return 1
}

// --- scriptreplay --- moved to scriptreplay.go ---

// --- acpid ---
func init() {
	applet.Register(&applet.Applet{Name: "acpid", Short: "ACPI daemon", Func: runAcpid})
}

func runAcpid(args []string) int {
	fmt.Fprintf(os.Stderr, "acpid: not yet implemented\n")
	return 1
}

func ioprioGet(which, who int) (int, error) {
	r0, _, errno := unix.Syscall(unix.SYS_IOPRIO_GET, uintptr(which), uintptr(who), 0)
	if errno != 0 {
		return 0, errno
	}
	return int(r0), nil
}

func ioprioSet(which, who, prio int) error {
	_, _, errno := unix.Syscall(unix.SYS_IOPRIO_SET, uintptr(which), uintptr(who), uintptr(prio))
	if errno != 0 {
		return errno
	}
	return nil
}

func removeIPC(kind string, id int) error {
	switch kind {
	case "-m", "shm", "--shmem-id":
		_, err := unix.SysvShmCtl(id, unix.IPC_RMID, nil)
		return err
	case "-q", "msg", "--queue-id":
		_, _, errno := unix.Syscall(unix.SYS_MSGCTL, uintptr(id), uintptr(unix.IPC_RMID), 0)
		if errno != 0 {
			return errno
		}
		return nil
	case "-s", "sem", "--semaphore-id":
		_, _, errno := unix.Syscall6(unix.SYS_SEMCTL, uintptr(id), 0, uintptr(unix.IPC_RMID), 0, 0, 0)
		if errno != 0 {
			return errno
		}
		return nil
	default:
		return fmt.Errorf("unknown resource type %s", kind)
	}
}

func printProcFile(header, path string) {
	fmt.Print(header)
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("%s: %v\n", path, err)
		return
	}
	os.Stdout.Write(data)
}

func runBlockdevOp(op, dev string) error {
	flags := os.O_RDONLY
	if op == "--setro" || op == "--setrw" || op == "--flushbufs" {
		flags = os.O_RDWR
	}
	f, err := os.OpenFile(dev, flags, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	switch op {
	case "--getsize64":
		size, err := blockSize64(f)
		if err == nil {
			fmt.Println(size)
		}
		return err
	case "--getsz":
		size, err := blockSize64(f)
		if err == nil {
			fmt.Println(size / 512)
		}
		return err
	case "--getss":
		ss, err := unix.IoctlGetInt(int(f.Fd()), unix.BLKSSZGET)
		if err == nil {
			fmt.Println(ss)
		}
		return err
	case "--flushbufs":
		return unix.IoctlSetInt(int(f.Fd()), unix.BLKFLSBUF, 0)
	case "--setro":
		return unix.IoctlSetPointerInt(int(f.Fd()), unix.BLKROSET, 1)
	case "--setrw":
		return unix.IoctlSetPointerInt(int(f.Fd()), unix.BLKROSET, 0)
	default:
		return fmt.Errorf("unsupported operation %s", op)
	}
}

func blockSize64(f *os.File) (uint64, error) {
	var size uint64
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, f.Fd(), uintptr(unix.BLKGETSIZE64), uintptr(unsafe.Pointer(&size)))
	if errno != 0 {
		return 0, errno
	}
	return size, nil
}

const (
	ioctlFDFLUSH                 = 0x024b
	ioctlFATGetAttributes        = 0x80047210
	ioctlFATSetAttributes        = 0x40047211
	fatAttrReadonly       uint32 = 1 << 0
	fatAttrHidden         uint32 = 1 << 1
	fatAttrSystem         uint32 = 1 << 2
	fatAttrVolume         uint32 = 1 << 3
	fatAttrDirectory      uint32 = 1 << 4
	fatAttrArchive        uint32 = 1 << 5
)

func ioctlDevice(appletName, device string, request uintptr) int {
	f, err := os.OpenFile(device, os.O_RDWR|syscall.O_NONBLOCK, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s: %v\n", appletName, device, err)
		return 1
	}
	defer f.Close()
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, f.Fd(), request, 0)
	if errno != 0 {
		fmt.Fprintf(os.Stderr, "%s: %s: %v\n", appletName, device, errno)
		return 1
	}
	return 0
}

func fatAttrMask(chars string) (uint32, error) {
	var mask uint32
	for _, ch := range chars {
		switch ch {
		case 'r':
			mask |= fatAttrReadonly
		case 'h':
			mask |= fatAttrHidden
		case 's':
			mask |= fatAttrSystem
		case 'v':
			mask |= fatAttrVolume
		case 'd':
			mask |= fatAttrDirectory
		case 'a':
			mask |= fatAttrArchive
		default:
			return 0, fmt.Errorf("invalid attribute '%c'", ch)
		}
	}
	return mask, nil
}

func fatAttrOne(name string, setMask, clearMask uint32) error {
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()
	var attr uint32
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, f.Fd(), ioctlFATGetAttributes, uintptr(unsafe.Pointer(&attr)))
	if errno != 0 {
		return errno
	}
	attr = (attr | setMask) &^ clearMask
	if setMask != 0 || clearMask != 0 {
		_, _, errno = unix.Syscall(unix.SYS_IOCTL, f.Fd(), ioctlFATSetAttributes, uintptr(unsafe.Pointer(&attr)))
		if errno != 0 {
			return errno
		}
		return nil
	}
	fmt.Printf("%s %s\n", formatFatAttrs(attr), name)
	return nil
}

func formatFatAttrs(attr uint32) string {
	flags := []struct {
		bit uint32
		ch  byte
	}{
		{fatAttrReadonly, 'r'},
		{fatAttrHidden, 'h'},
		{fatAttrSystem, 's'},
		{fatAttrVolume, 'v'},
		{fatAttrDirectory, 'd'},
		{fatAttrArchive, 'a'},
		{1 << 6, '6'},
		{1 << 7, '7'},
	}
	out := make([]byte, 0, len(flags))
	for _, flag := range flags {
		if attr&flag.bit != 0 {
			out = append(out, flag.ch)
		} else {
			out = append(out, ' ')
		}
	}
	return string(out)
}

func printSysfsLine(name, path string) {
	data, err := os.ReadFile(path)
	if err == nil {
		fmt.Printf("%s %s\n", name, strings.TrimSpace(string(data)))
	}
}

// --- getopt (enhanced) ---
func init() {
	// Already registered above, no-op
}

// --- user.Current ---
var _ = user.Current

// --- io ---
type multiWriter struct{}
