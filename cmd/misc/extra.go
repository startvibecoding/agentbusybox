package misc

import (
	"bufio"
	"compress/gzip"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
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

// --- bc ---
func init() {
	applet.Register(&applet.Applet{Name: "ascii", Short: "Print ASCII table", Func: runAscii})
	applet.Register(&applet.Applet{Name: "bc", Short: "Arbitrary precision calculator", Func: runBc})
	applet.Register(&applet.Applet{Name: "dc", Short: "Desk calculator", Func: runDc})
}

func runAscii(args []string) int {
	ctrl := []string{
		"NUL", "SOH", "STX", "ETX", "EOT", "ENQ", "ACK", "BEL",
		"BS ", "HT ", "NL ", "VT ", "FF ", "CR ", "SO ", "SI ",
		"DLE", "DC1", "DC2", "DC3", "DC4", "NAK", "SYN", "ETB",
		"CAN", "EM ", "SUB", "ESC", "FS ", "GS ", "RS ", "US ",
	}
	fmt.Println("Dec Hex    Dec Hex    Dec Hex  Dec Hex  Dec Hex  Dec Hex   Dec Hex   Dec Hex")
	for i := 0; i < 16; i++ {
		c1 := string(rune(i + 0x20))
		c2 := string(rune(i + 0x30))
		c3 := string(rune(i + 0x40))
		c4 := string(rune(i + 0x50))
		c5 := string(rune(i + 0x60))
		c6 := string(rune(i + 0x70))
		if i+0x70 == 0x7f {
			c6 = "DEL"
		}
		fmt.Printf("%3d %02x %.3s%4d %02x %.3s%4d %02x %s%4d %02x %s%4d %02x %s%4d %02x %s%5d %02x %s%5d %02x %s\n",
			i, i, ctrl[i],
			i+0x10, i+0x10, ctrl[i+0x10],
			i+0x20, i+0x20, c1,
			i+0x30, i+0x30, c2,
			i+0x40, i+0x40, c3,
			i+0x50, i+0x50, c4,
			i+0x60, i+0x60, c5,
			i+0x70, i+0x70, c6)
	}
	return 0
}

func runBc(args []string) int {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "quit" || line == "q" {
			break
		}
		if line == "" {
			continue
		}
		// Simple arithmetic evaluation
		result := evalBcExpr(line)
		fmt.Println(result)
	}
	return 0
}

func evalBcExpr(expr string) float64 {
	expr = strings.TrimSpace(expr)
	// Handle basic arithmetic: +, -, *, /
	var result float64
	var op byte = '+'
	num := ""
	for i := 0; i < len(expr); i++ {
		ch := expr[i]
		if ch == '+' || ch == '-' || ch == '*' || ch == '/' {
			var n float64
			fmt.Sscanf(strings.TrimSpace(num), "%f", &n)
			switch op {
			case '+':
				result += n
			case '-':
				result -= n
			case '*':
				result *= n
			case '/':
				if n != 0 {
					result /= n
				}
			}
			op = ch
			num = ""
		} else {
			num += string(ch)
		}
	}
	var n float64
	fmt.Sscanf(strings.TrimSpace(num), "%f", &n)
	switch op {
	case '+':
		result += n
	case '-':
		result -= n
	case '*':
		result *= n
	case '/':
		if n != 0 {
			result /= n
		}
	}
	return result
}

func runDc(args []string) int {
	return runBc(args)
}

// --- crond / crontab ---
func init() {
	applet.Register(&applet.Applet{Name: "conspy", Short: "Display and control a virtual console", Func: runConspy})
	applet.Register(&applet.Applet{Name: "crond", Short: "Daemon to execute scheduled commands", Func: runCrond})
	applet.Register(&applet.Applet{Name: "crontab", Short: "Manage cron jobs", Func: runCrontab})
	applet.Register(&applet.Applet{Name: "devfsd", Short: "Device filesystem daemon", Func: runDevfsd})
}

func runCrond(args []string) int {
	foreground := false
	cronDir := "/var/spool/cron/crontabs"
	for _, a := range args[1:] {
		if a == "-f" || a == "--foreground" {
			foreground = true
		}
	}
	for i := 1; i < len(args); i++ {
		if (args[i] == "-c" || args[i] == "--cron-dir") && i+1 < len(args) {
			i++
			cronDir = args[i]
		}
	}
	if !foreground {
		fmt.Fprintf(os.Stderr, "crond: daemon mode is not implemented; running in foreground\n")
	}
	lastMinute := ""
	for {
		now := time.Now()
		minute := now.Format("200601021504")
		if minute != lastMinute {
			runDueCronJobs(cronDir, now)
			lastMinute = minute
		}
		time.Sleep(time.Second)
	}
}

func runCrontab(args []string) int {
	edit := false
	list := false
	remove := false
	cronDir := "/var/spool/cron/crontabs"
	file := ""
	for i := 1; i < len(args); i++ {
		a := args[i]
		if (a == "-c" || a == "--cron-dir") && i+1 < len(args) {
			i++
			cronDir = args[i]
			continue
		}
		if a == "-e" || a == "--edit" {
			edit = true
			continue
		}
		if a == "-l" || a == "--list" {
			list = true
			continue
		}
		if a == "-r" || a == "--remove" {
			remove = true
			continue
		}
		if !strings.HasPrefix(a, "-") {
			file = a
		}
	}
	user := os.Getenv("USER")
	if user == "" {
		user = "root"
	}
	spool := filepath.Join(cronDir, user)
	if list {
		data, err := os.ReadFile(spool)
		if err != nil {
			fmt.Fprintf(os.Stderr, "crontab: no crontab for %s\n", user)
			return 1
		}
		os.Stdout.Write(data)
		return 0
	}
	if remove {
		if err := os.Remove(spool); err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "crontab: %v\n", err)
			return 1
		}
		return 0
	}
	if edit {
		return editCrontab(spool, cronDir)
	}
	if file != "" {
		data, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "crontab: %s: %v\n", file, err)
			return 1
		}
		if err := os.MkdirAll(cronDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "crontab: %v\n", err)
			return 1
		}
		if err := os.WriteFile(spool, data, 0600); err != nil {
			fmt.Fprintf(os.Stderr, "crontab: %v\n", err)
			return 1
		}
		return 0
	}
	fmt.Fprintf(os.Stderr, "crontab: missing option\n")
	return 1
}

func editCrontab(spool, cronDir string) int {
	if runtime.GOOS == "windows" {
		fmt.Fprintf(os.Stderr, "crontab: edit mode is not supported on Windows\n")
		return 1
	}

	tmp, err := os.CreateTemp("", "agentbusybox-crontab-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "crontab: %v\n", err)
		return 1
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if data, err := os.ReadFile(spool); err == nil {
		if _, err := tmp.Write(data); err != nil {
			tmp.Close()
			fmt.Fprintf(os.Stderr, "crontab: %v\n", err)
			return 1
		}
	}
	if err := tmp.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "crontab: %v\n", err)
		return 1
	}

	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}
	editorArgs := []string{editor, tmpPath}
	proc, err := startProcess(editorArgs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "crontab: %v\n", err)
		return 1
	}
	if code := waitProcess(proc); code != 0 {
		return code
	}

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "crontab: %v\n", err)
		return 1
	}
	if err := os.MkdirAll(cronDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "crontab: %v\n", err)
		return 1
	}
	if err := os.WriteFile(spool, data, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "crontab: %v\n", err)
		return 1
	}
	return 0
}

func runDueCronJobs(dir string, now time.Time) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) < 6 {
				continue
			}
			if cronFieldMatches(fields[0], now.Minute()) &&
				cronFieldMatches(fields[1], now.Hour()) &&
				cronFieldMatches(fields[2], now.Day()) &&
				cronFieldMatches(fields[3], int(now.Month())) &&
				cronFieldMatches(fields[4], int(now.Weekday())) {
				startShellCommand(strings.Join(fields[5:], " "))
			}
		}
	}
}

func cronFieldMatches(field string, value int) bool {
	for _, part := range strings.Split(field, ",") {
		if part == "*" {
			return true
		}
		if strings.HasPrefix(part, "*/") {
			n, err := strconv.Atoi(part[2:])
			return err == nil && n > 0 && value%n == 0
		}
		n, err := strconv.Atoi(part)
		if err == nil && n == value {
			return true
		}
	}
	return false
}

func startShellCommand(command string) {
	devNull, _ := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if devNull == nil {
		devNull = os.Stdin
	}
	attr := &os.ProcAttr{
		Env:   os.Environ(),
		Files: []*os.File{devNull, os.Stdout, os.Stderr},
	}
	proc, err := os.StartProcess("/bin/sh", []string{"sh", "-c", command}, attr)
	if err == nil {
		go proc.Wait()
	}
	if devNull != os.Stdin {
		_ = devNull.Close()
	}
}

func runConspy(args []string) int {
	vt := 0
	for _, a := range args[1:] {
		if !strings.HasPrefix(a, "-") {
			fmt.Sscanf(a, "%d", &vt)
		}
	}
	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "conspy: not supported\n")
		return 1
	}
	dev := fmt.Sprintf("/dev/tty%d", vt)
	f, err := os.OpenFile(dev, os.O_RDWR, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "conspy: %s: %v\n", dev, err)
		return 1
	}
	defer f.Close()
	buf := make([]byte, 4096)
	for {
		n, err := f.Read(buf)
		if n > 0 {
			os.Stdout.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}
	return 0
}

func runDevfsd(args []string) int {
	fmt.Fprintf(os.Stderr, "devfsd: not yet implemented in pure Go\n")
	return 1
}

// --- devmem ---
func init() {
	applet.Register(&applet.Applet{Name: "devmem", Short: "Read/write from physical address", Func: runDevmem})
}

func runDevmem(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "devmem: missing address\n")
		return 1
	}
	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "devmem: not supported\n")
		return 1
	}
	var addr uint64
	fmt.Sscanf(args[1], "%x", &addr)

	width := 8
	value := uint64(0)
	write := false

	if len(args) >= 3 {
		write = true
		fmt.Sscanf(args[2], "%x", &value)
	}
	if len(args) >= 4 {
		fmt.Sscanf(args[3], "%d", &width)
	}

	f, err := os.OpenFile("/dev/mem", os.O_RDWR, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "devmem: cannot open /dev/mem: %v\n", err)
		return 1
	}
	defer f.Close()

	f.Seek(int64(addr), 0)
	if write {
		var buf []byte
		switch width {
		case 1:
			buf = []byte{byte(value)}
		case 2:
			buf = []byte{byte(value), byte(value >> 8)}
		case 4:
			buf = make([]byte, 4)
			binary.LittleEndian.PutUint32(buf, uint32(value))
		case 8:
			buf = make([]byte, 8)
			binary.LittleEndian.PutUint64(buf, value)
		}
		_, err = f.Write(buf)
		if err != nil {
			fmt.Fprintf(os.Stderr, "devmem: write error: %v\n", err)
			return 1
		}
	} else {
		var buf []byte
		switch width {
		case 1:
			buf = make([]byte, 1)
		case 2:
			buf = make([]byte, 2)
		case 4:
			buf = make([]byte, 4)
		case 8:
			buf = make([]byte, 8)
		}
		n, err := f.Read(buf)
		if err != nil || n < width {
			fmt.Fprintf(os.Stderr, "devmem: read error: %v\n", err)
			return 1
		}
		switch width {
		case 1:
			fmt.Printf("0x%02x\n", buf[0])
		case 2:
			fmt.Printf("0x%04x\n", binary.LittleEndian.Uint16(buf))
		case 4:
			fmt.Printf("0x%08x\n", binary.LittleEndian.Uint32(buf))
		case 8:
			fmt.Printf("0x%016x\n", binary.LittleEndian.Uint64(buf))
		}
	}
	return 0
}

// --- hdparm ---
func init() {
	applet.Register(&applet.Applet{Name: "hdparm", Short: "Get/set hard disk parameters", Func: runHdparm})
}

func runHdparm(args []string) int {
	devices := []string{}
	for _, a := range args[1:] {
		if !strings.HasPrefix(a, "-") {
			devices = append(devices, a)
		}
	}
	if len(devices) == 0 {
		fmt.Fprintf(os.Stderr, "hdparm: missing device\n")
		return 1
	}
	for _, dev := range devices {
		name := filepath.Base(dev)
		sys := filepath.Join("/sys/class/block", name)
		fmt.Printf("\n%s:\n", dev)
		printSysfsValue(" readonly", filepath.Join(sys, "ro"))
		printSysfsValue(" readahead", filepath.Join(sys, "queue/read_ahead_kb"))
		printSysfsValue(" logical sector size", filepath.Join(sys, "queue/logical_block_size"))
		printSysfsValue(" physical sector size", filepath.Join(sys, "queue/physical_block_size"))
		if size, err := readBlockSizeBytes(name); err == nil {
			fmt.Printf(" size = %d bytes\n", size)
		}
	}
	return 0
}

// --- iconv ---
func init() {
	applet.Register(&applet.Applet{Name: "iconv", Short: "Convert character encoding", Func: runIconv})
}

func runIconv(args []string) int {
	from := "UTF-8"
	to := "UTF-8"
	files := []string{}
	for i := 1; i < len(args); i++ {
		if args[i] == "-f" && i+1 < len(args) {
			i++
			from = args[i]
			continue
		}
		if args[i] == "-t" && i+1 < len(args) {
			i++
			to = args[i]
			continue
		}
		if args[i] == "-l" {
			fmt.Println("UTF-8\nASCII\nISO-8859-1")
			return 0
		}
		if !strings.HasPrefix(args[i], "-") {
			files = append(files, args[i])
		}
	}
	_ = from
	_ = to
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
			fmt.Fprintf(os.Stderr, "iconv: %s: %v\n", fname, err)
			return 1
		}
		os.Stdout.Write(data)
	}
	return 0
}

// --- man ---
func init() {
	applet.Register(&applet.Applet{Name: "man", Short: "Format and display manual pages", Func: runMan})
}

func runMan(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "What manual page do you want?\n")
		return 1
	}
	page := args[len(args)-1]
	path, err := findManPage(page)
	if err != nil {
		fmt.Fprintf(os.Stderr, "man: no entry for %s in the manual\n", page)
		return 1
	}
	return printManPage(path)
}

// --- ts (timestamp) ---
func init() {
	applet.Register(&applet.Applet{Name: "ts", Short: "Timestamp input", Func: runTs})
}

func runTs(args []string) int {
	format := "2006-01-02 15:04:05"
	for _, a := range args[1:] {
		if strings.HasPrefix(a, "-") {
			continue
		}
		format = a
	}
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		fmt.Printf("%s %s\n", time.Now().Format(format), scanner.Text())
	}
	return 0
}

// --- volname ---
func init() {
	applet.Register(&applet.Applet{Name: "volname", Short: "Show volume name of CD/DVD", Func: runVolname})
}

func runVolname(args []string) int {
	device := "/dev/cdrom"
	if len(args) > 1 {
		device = args[1]
	}
	f, err := os.Open(device)
	if err != nil {
		fmt.Fprintf(os.Stderr, "volname: %v\n", err)
		return 1
	}
	defer f.Close()
	buf := make([]byte, 2048)
	if _, err := f.ReadAt(buf, 16*2048); err != nil {
		fmt.Fprintf(os.Stderr, "volname: %v\n", err)
		return 1
	}
	if buf[0] != 1 || string(buf[1:6]) != "CD001" {
		fmt.Fprintf(os.Stderr, "volname: %s: not an ISO9660 primary volume\n", device)
		return 1
	}
	fmt.Println(strings.TrimSpace(string(buf[40:72])))
	return 0
}

// --- watchdog ---
func init() {
	applet.Register(&applet.Applet{Name: "watchdog", Short: "Software watchdog daemon", Func: runWatchdog})
}

func runWatchdog(args []string) int {
	device := "/dev/watchdog"
	if len(args) > 1 {
		device = args[1]
	}
	f, err := os.OpenFile(device, os.O_RDWR, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "watchdog: cannot open %s: %v\n", device, err)
		return 1
	}
	defer f.Close()
	for {
		f.Write([]byte{1})
		time.Sleep(10 * time.Second)
	}
}

// --- chat ---
func init() {
	applet.Register(&applet.Applet{Name: "chat", Short: "Automated conversational script", Func: runChat})
}

func runChat(args []string) int {
	timeout := 10
	script := []string{}

	for i := 1; i < len(args); i++ {
		if args[i] == "-t" && i+1 < len(args) {
			i++
			fmt.Sscanf(args[i], "%d", &timeout)
		} else if args[i] == "-v" {
			// verbose
		} else if !strings.HasPrefix(args[i], "-") {
			script = append(script, args[i])
		}
	}

	if len(script) == 0 {
		fmt.Fprintf(os.Stderr, "chat: missing script\n")
		return 1
	}

	_ = timeout
	for i := 0; i < len(script); i += 2 {
		expect := script[i]
		send := ""
		if i+1 < len(script) {
			send = script[i+1]
		}
		if expect == "" || expect == "ABORT" {
			continue
		}
		if send != "" {
			fmt.Printf("%s", send)
		}
	}
	return 0
}

// --- microcom ---
func init() {
	applet.Register(&applet.Applet{Name: "microcom", Short: "Minimal terminal program", Func: runMicrocom})
}

func runMicrocom(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "microcom: missing port\n")
		return 1
	}
	port := ""
	for _, a := range args[1:] {
		if !strings.HasPrefix(a, "-") {
			port = a
		}
	}
	f, err := os.OpenFile(port, os.O_RDWR, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "microcom: %v\n", err)
		return 1
	}
	defer f.Close()
	done := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(f, os.Stdin)
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(os.Stdout, f)
		done <- struct{}{}
	}()
	<-done
	return 0
}

// --- less (improved) ---
// Already in misc/misc.go, no duplicate here ---

// --- adjtimex ---
func init() {
	applet.Register(&applet.Applet{Name: "adjtimex", Short: "Set kernel clock parameters", Func: runAdjtimex})
}

func runAdjtimex(args []string) int {
	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "adjtimex: not supported\n")
		return 1
	}
	quiet := false
	var tx unix.Timex
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-q":
			quiet = true
		case "-o":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "adjtimex: -o requires an argument\n")
				return 1
			}
			i++
			v, err := strconv.ParseInt(args[i], 10, 64)
			if err != nil {
				fmt.Fprintf(os.Stderr, "adjtimex: invalid offset '%s'\n", args[i])
				return 1
			}
			tx.Offset = v
			tx.Modes |= unix.ADJ_OFFSET_SINGLESHOT
		case "-f":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "adjtimex: -f requires an argument\n")
				return 1
			}
			i++
			v, err := strconv.ParseInt(args[i], 10, 64)
			if err != nil {
				fmt.Fprintf(os.Stderr, "adjtimex: invalid frequency '%s'\n", args[i])
				return 1
			}
			tx.Freq = v
			tx.Modes |= unix.ADJ_FREQUENCY
		case "-p":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "adjtimex: -p requires an argument\n")
				return 1
			}
			i++
			v, err := strconv.ParseInt(args[i], 10, 64)
			if err != nil {
				fmt.Fprintf(os.Stderr, "adjtimex: invalid timeconstant '%s'\n", args[i])
				return 1
			}
			tx.Constant = v
			tx.Modes |= unix.ADJ_TIMECONST
		case "-t":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "adjtimex: -t requires an argument\n")
				return 1
			}
			i++
			v, err := strconv.ParseInt(args[i], 10, 64)
			if err != nil {
				fmt.Fprintf(os.Stderr, "adjtimex: invalid tick '%s'\n", args[i])
				return 1
			}
			tx.Tick = v
			tx.Modes |= unix.ADJ_TICK
		default:
			fmt.Fprintf(os.Stderr, "adjtimex: unknown option %s\n", args[i])
			return 1
		}
	}
	state, err := unix.Adjtimex(&tx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "adjtimex: %v\n", err)
		return 1
	}
	if quiet {
		return 0
	}
	fmt.Printf("    mode:         %d\n", tx.Modes)
	fmt.Printf("-o  offset:       %d us\n", tx.Offset)
	fmt.Printf("-f  freq.adjust:  %d (65536 = 1ppm)\n", tx.Freq)
	fmt.Printf("    maxerror:     %d\n", tx.Maxerror)
	fmt.Printf("    esterror:     %d\n", tx.Esterror)
	fmt.Printf("    status:       %d (%s)\n", tx.Status, timexStatusNames(tx.Status))
	fmt.Printf("-p  timeconstant: %d\n", tx.Constant)
	fmt.Printf("    precision:    %d us\n", tx.Precision)
	fmt.Printf("    tolerance:    %d\n", tx.Tolerance)
	fmt.Printf("-t  tick:         %d us\n", tx.Tick)
	fmt.Printf("    time.tv_sec:  %d\n", tx.Time.Sec)
	fmt.Printf("    time.tv_usec: %d\n", tx.Time.Usec)
	fmt.Printf("    return value: %d (%s)\n", state, timexStateDescription(state))
	return 0
}

// --- bbconfig ---
func init() {
	applet.Register(&applet.Applet{Name: "bbconfig", Short: "Show BusyBox build config", Func: runBbconfig})
}

func runBbconfig(args []string) int {
	fmt.Println("# AgentBusyBox build configuration")
	fmt.Println("# Built with Go", runtime.Version())
	return 0
}

// --- i2cdetect / i2cdump / i2cget / i2cset / i2ctransfer ---
func init() {
	applet.Register(&applet.Applet{Name: "i2cdetect", Short: "Detect I2C chips", Func: runI2cdetect})
	applet.Register(&applet.Applet{Name: "i2cdump", Short: "Examine I2C registers", Func: runI2cdump})
	applet.Register(&applet.Applet{Name: "i2cget", Short: "Read I2C register", Func: runI2cget})
	applet.Register(&applet.Applet{Name: "i2cset", Short: "Write I2C register", Func: runI2cset})
	applet.Register(&applet.Applet{Name: "i2ctransfer", Short: "I2C transfer tool", Func: runI2ctransfer})
}

func runI2cdetect(args []string) int {
	if len(args) == 1 || args[1] == "-l" {
		entries, err := os.ReadDir("/sys/class/i2c-dev")
		if err != nil {
			fmt.Fprintf(os.Stderr, "i2cdetect: %v\n", err)
			return 1
		}
		for _, entry := range entries {
			name := entry.Name()
			devName, _ := os.ReadFile(filepath.Join("/sys/class/i2c-dev", name, "name"))
			fmt.Printf("%s\t%s\n", name, strings.TrimSpace(string(devName)))
		}
		return 0
	}
	fmt.Fprintf(os.Stderr, "i2cdetect: bus scan is not yet implemented in pure Go\n")
	return 1
}

func parseI2CByte(s string) (int, error) {
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		v, err := strconv.ParseInt(s[2:], 16, 0)
		return int(v), err
	}
	return strconv.Atoi(s)
}

func openI2C(args []string) (*os.File, int, int, error) {
	if len(args) < 4 {
		return nil, 0, 0, fmt.Errorf("missing bus chip register")
	}
	bus, err := strconv.Atoi(args[1])
	if err != nil {
		return nil, 0, 0, err
	}
	addr, err := parseI2CByte(args[2])
	if err != nil {
		return nil, 0, 0, err
	}
	reg, err := parseI2CByte(args[3])
	if err != nil {
		return nil, 0, 0, err
	}
	f, err := os.OpenFile(fmt.Sprintf("/dev/i2c-%d", bus), os.O_RDWR, 0)
	if err != nil {
		return nil, 0, 0, err
	}
	if err := unix.IoctlSetInt(int(f.Fd()), i2cSlaveAddr, addr); err != nil {
		f.Close()
		return nil, 0, 0, err
	}
	return f, addr, reg, nil
}

func runI2cdump(args []string) int {
	if len(args) < 3 {
		fmt.Fprintf(os.Stderr, "i2cdump: missing bus chip\n")
		return 1
	}
	for reg := 0; reg < 256; reg++ {
		f, _, _, err := openI2C([]string{args[0], args[1], args[2], fmt.Sprintf("0x%02x", reg)})
		if err != nil {
			fmt.Fprintf(os.Stderr, "i2cdump: %v\n", err)
			return 1
		}
		_, _ = f.Write([]byte{byte(reg)})
		buf := []byte{0}
		_, err = f.Read(buf)
		f.Close()
		if reg%16 == 0 {
			fmt.Printf("\n%02x:", reg)
		}
		if err != nil {
			fmt.Print(" XX")
		} else {
			fmt.Printf(" %02x", buf[0])
		}
	}
	fmt.Println()
	return 0
}
func runI2cget(args []string) int {
	f, _, reg, err := openI2C(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "i2cget: %v\n", err)
		return 1
	}
	defer f.Close()
	if _, err := f.Write([]byte{byte(reg)}); err != nil {
		fmt.Fprintf(os.Stderr, "i2cget: %v\n", err)
		return 1
	}
	buf := []byte{0}
	if _, err := f.Read(buf); err != nil {
		fmt.Fprintf(os.Stderr, "i2cget: %v\n", err)
		return 1
	}
	fmt.Printf("0x%02x\n", buf[0])
	return 0
}
func runI2cset(args []string) int {
	if len(args) < 5 {
		fmt.Fprintf(os.Stderr, "i2cset: missing bus chip register value\n")
		return 1
	}
	f, _, reg, err := openI2C(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "i2cset: %v\n", err)
		return 1
	}
	defer f.Close()
	value, err := parseI2CByte(args[4])
	if err != nil {
		fmt.Fprintf(os.Stderr, "i2cset: %v\n", err)
		return 1
	}
	if _, err := f.Write([]byte{byte(reg), byte(value)}); err != nil {
		fmt.Fprintf(os.Stderr, "i2cset: %v\n", err)
		return 1
	}
	return 0
}
func runI2ctransfer(args []string) int {
	allAddresses := false
	i := 1
	for i < len(args) && strings.HasPrefix(args[i], "-") {
		switch args[i] {
		case "-a":
			allAddresses = true
		case "-f", "-y":
		default:
			fmt.Fprintf(os.Stderr, "i2ctransfer: unknown option %s\n", args[i])
			return 1
		}
		i++
	}
	if len(args)-i < 2 {
		fmt.Fprintf(os.Stderr, "i2ctransfer: usage: i2ctransfer [-fay] I2CBUS {rLEN[@ADDR]|wLEN[@ADDR] DATA...}...\n")
		return 1
	}
	devPath, err := i2cDevicePath(args[i])
	if err != nil {
		fmt.Fprintf(os.Stderr, "i2ctransfer: %v\n", err)
		return 1
	}
	i++
	f, err := os.OpenFile(devPath, os.O_RDWR, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "i2ctransfer: %s: %v\n", devPath, err)
		return 1
	}
	defer f.Close()

	lastAddr := -1
	messages := []i2cMsg{}
	buffers := [][]byte{}
	for i < len(args) {
		if len(messages) >= i2cRdwrMaxMsgs {
			fmt.Fprintf(os.Stderr, "i2ctransfer: too many messages, max: %d\n", i2cRdwrMaxMsgs)
			return 1
		}
		spec := args[i]
		i++
		if len(spec) < 2 || (spec[0] != 'r' && spec[0] != 'w') {
			fmt.Fprintf(os.Stderr, "i2ctransfer: invalid message '%s'\n", spec)
			return 1
		}
		lengthPart := spec[1:]
		if at := strings.IndexByte(lengthPart, '@'); at >= 0 {
			addr, err := parseI2CByte(lengthPart[at+1:])
			if err != nil {
				fmt.Fprintf(os.Stderr, "i2ctransfer: invalid address in '%s'\n", spec)
				return 1
			}
			if !allAddresses && (addr < 0x03 || addr > 0x77) {
				fmt.Fprintf(os.Stderr, "i2ctransfer: address 0x%02x out of range\n", addr)
				return 1
			}
			lastAddr = addr
			lengthPart = lengthPart[:at]
		}
		if lastAddr < 0 {
			fmt.Fprintf(os.Stderr, "i2ctransfer: no address given in '%s'\n", spec)
			return 1
		}
		length, err := strconv.ParseUint(lengthPart, 0, 16)
		if err != nil {
			fmt.Fprintf(os.Stderr, "i2ctransfer: invalid length in '%s'\n", spec)
			return 1
		}
		buf := make([]byte, int(length))
		if spec[0] == 'w' {
			if err := fillI2CWriteBuffer(buf, args, &i); err != nil {
				fmt.Fprintf(os.Stderr, "i2ctransfer: %v\n", err)
				return 1
			}
		}
		msg := i2cMsg{Addr: uint16(lastAddr), Len: uint16(length)}
		if spec[0] == 'r' {
			msg.Flags = i2cMsgRead
		}
		if len(buf) > 0 {
			msg.Buf = uintptr(unsafe.Pointer(&buf[0]))
		}
		messages = append(messages, msg)
		buffers = append(buffers, buf)
	}
	if len(messages) == 0 {
		fmt.Fprintf(os.Stderr, "i2ctransfer: no messages\n")
		return 1
	}
	rdwr := i2cRdwrIoctlData{
		Msgs:  uintptr(unsafe.Pointer(&messages[0])),
		Nmsgs: uint32(len(messages)),
	}
	n, _, errno := unix.Syscall(unix.SYS_IOCTL, f.Fd(), i2cRdwrIoctl, uintptr(unsafe.Pointer(&rdwr)))
	if errno != 0 {
		fmt.Fprintf(os.Stderr, "i2ctransfer: I2C_RDWR: %v\n", errno)
		return 1
	}
	for idx := 0; idx < int(n) && idx < len(messages); idx++ {
		if messages[idx].Flags&i2cMsgRead == 0 || len(buffers[idx]) == 0 {
			continue
		}
		for j, b := range buffers[idx] {
			if j > 0 {
				fmt.Print(" ")
			}
			fmt.Printf("0x%02x", b)
		}
		fmt.Println()
	}
	return 0
}

// --- flash_eraseall / flash_lock / flash_unlock / flashcp ---
func init() {
	applet.Register(&applet.Applet{Name: "fbsplash", Short: "Framebuffer splash utility", Func: runFbsplash})
	applet.Register(&applet.Applet{Name: "flash_eraseall", Short: "Erase MTD device", Func: runFlashEraseall})
	applet.Register(&applet.Applet{Name: "flash_lock", Short: "Lock MTD device", Func: runFlashLock})
	applet.Register(&applet.Applet{Name: "flash_unlock", Short: "Unlock MTD device", Func: runFlashUnlock})
	applet.Register(&applet.Applet{Name: "flashcp", Short: "Copy to MTD device", Func: runFlashcp})
}

func runFbsplash(args []string) int {
	fmt.Fprintf(os.Stderr, "fbsplash: not yet implemented in pure Go\n")
	return 1
}

func runFlashEraseall(args []string) int {
	fmt.Fprintf(os.Stderr, "flash_eraseall: not implemented\n")
	return 1
}
func runFlashLock(args []string) int {
	fmt.Fprintf(os.Stderr, "flash_lock: not implemented\n")
	return 1
}
func runFlashUnlock(args []string) int {
	fmt.Fprintf(os.Stderr, "flash_unlock: not implemented\n")
	return 1
}
func runFlashcp(args []string) int { fmt.Fprintf(os.Stderr, "flashcp: not implemented\n"); return 1 }

// --- hexedit ---
func init() {
	applet.Register(&applet.Applet{Name: "hexedit", Short: "View a file as hex", Func: runHexedit})
}

func runHexedit(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "hexedit: missing file\n")
		return 1
	}
	data, err := os.ReadFile(args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "hexedit: %s: %v\n", args[1], err)
		return 1
	}
	for off := 0; off < len(data); off += 16 {
		end := off + 16
		if end > len(data) {
			end = len(data)
		}
		fmt.Printf("%08x  %-47s  |", off, spacedHex(data[off:end]))
		for _, b := range data[off:end] {
			if b >= 32 && b < 127 {
				fmt.Printf("%c", b)
			} else {
				fmt.Print(".")
			}
		}
		fmt.Println("|")
	}
	return 0
}

func spacedHex(data []byte) string {
	dst := make([]byte, hex.EncodedLen(len(data)))
	hex.Encode(dst, data)
	var b strings.Builder
	for i := 0; i < len(dst); i += 2 {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.Write(dst[i : i+2])
	}
	return b.String()
}

// --- Windows busybox-w32 helpers ---
func init() {
	applet.Register(&applet.Applet{Name: "cdrop", Short: "Run command without elevated privileges using cmd.exe", Func: runDrop})
	applet.Register(&applet.Applet{Name: "drop", Short: "Run command without elevated privileges", Func: runDrop})
	applet.Register(&applet.Applet{Name: "jn", Short: "Create directory junction", Func: runJn})
	applet.Register(&applet.Applet{Name: "pdrop", Short: "Run command without elevated privileges using PowerShell", Func: runDrop})
}

func runDrop(args []string) int {
	return runDropPlatform(args)
}

func runJn(args []string) int {
	if len(args) != 3 {
		fmt.Fprintf(os.Stderr, "jn: usage: jn DIR JUNC\n")
		return 1
	}
	if runtime.GOOS == "windows" {
		if err := os.Symlink(args[1], args[2]); err != nil {
			fmt.Fprintf(os.Stderr, "jn: %v\n", err)
			return 1
		}
		return 0
	}
	fmt.Fprintf(os.Stderr, "jn: directory junctions are only supported on Windows\n")
	return 1
}

// --- makedevs / mt / nand / raid / readahead / rx ---
func init() {
	applet.Register(&applet.Applet{Name: "makedevs", Short: "Create device files from a table", Func: runMakedevs})
	applet.Register(&applet.Applet{Name: "mim", Short: "Minimal make-like script runner", Func: runMim})
	applet.Register(&applet.Applet{Name: "mt", Short: "Control magnetic tape drive operation", Func: runMt})
	applet.Register(&applet.Applet{Name: "nanddump", Short: "Dump NAND flash", Func: runNanddump})
	applet.Register(&applet.Applet{Name: "nandwrite", Short: "Write NAND flash", Func: runNandwrite})
	applet.Register(&applet.Applet{Name: "raidautorun", Short: "Tell kernel to autorun RAID arrays", Func: runRaidautorun})
	applet.Register(&applet.Applet{Name: "readahead", Short: "Preload files into memory", Func: runReadahead})
	applet.Register(&applet.Applet{Name: "rx", Short: "Receive files using XMODEM", Func: runRx})
}

func runMakedevs(args []string) int {
	tablePath := ""
	rootDir := "."
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-d", "--device-table":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "makedevs: option %s requires an argument\n", args[i])
				return 1
			}
			i++
			tablePath = args[i]
		default:
			if !strings.HasPrefix(args[i], "-") {
				if tablePath == "" {
					tablePath = args[i]
				} else {
					rootDir = args[i]
				}
			}
		}
	}
	if tablePath == "" {
		fmt.Fprintf(os.Stderr, "makedevs: missing device table\n")
		return 1
	}
	data, err := os.ReadFile(tablePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "makedevs: %s: %v\n", tablePath, err)
		return 1
	}
	exitCode := 0
	for lineno, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 2 {
			fmt.Fprintf(os.Stderr, "makedevs: %s:%d: malformed entry\n", tablePath, lineno+1)
			exitCode = 1
			continue
		}
		if err := applyDeviceTableEntry(rootDir, fields); err != nil {
			fmt.Fprintf(os.Stderr, "makedevs: %s:%d: %v\n", tablePath, lineno+1, err)
			exitCode = 1
		}
	}
	return exitCode
}

func applyDeviceTableEntry(rootDir string, fields []string) error {
	name := fields[0]
	kind := fields[1]
	path := filepath.Join(rootDir, filepath.Clean(name))

	switch kind {
	case "d":
		mode := os.FileMode(0755)
		if len(fields) > 2 {
			if m, err := parseFileMode(fields[2]); err == nil {
				mode = m
			}
		}
		if err := os.MkdirAll(path, mode); err != nil {
			return err
		}
		if len(fields) > 4 {
			uid, gid, err := parseUIDGID(fields[3], fields[4])
			if err != nil {
				return err
			}
			if err := os.Chown(path, uid, gid); err != nil {
				return err
			}
		}
		return os.Chmod(path, mode)
	case "p":
		mode := os.FileMode(0666)
		if err := unix.Mkfifo(path, uint32(mode)); err != nil {
			return err
		}
		if len(fields) > 4 {
			uid, gid, err := parseUIDGID(fields[3], fields[4])
			if err != nil {
				return err
			}
			if err := os.Chown(path, uid, gid); err != nil {
				return err
			}
		}
		return nil
	case "c", "b", "u":
		if len(fields) < 5 {
			return fmt.Errorf("missing major/minor")
		}
		major, err := strconv.Atoi(fields[2])
		if err != nil {
			return fmt.Errorf("invalid major %q", fields[2])
		}
		minor, err := strconv.Atoi(fields[3])
		if err != nil {
			return fmt.Errorf("invalid minor %q", fields[3])
		}
		mode := uint32(0666)
		switch kind {
		case "b":
			mode |= unix.S_IFBLK
		default:
			mode |= unix.S_IFCHR
		}
		if len(fields) > 4 {
			if m, err := parseFileMode(fields[4]); err == nil {
				mode = uint32(m) | (mode &^ 0777)
			}
		}
		if err := unix.Mknod(path, mode, int(unix.Mkdev(uint32(major), uint32(minor)))); err != nil {
			return err
		}
		if len(fields) > 6 {
			uid, gid, err := parseUIDGID(fields[5], fields[6])
			if err != nil {
				return err
			}
			if err := os.Chown(path, uid, gid); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown node type %q", kind)
	}
}

func parseFileMode(s string) (os.FileMode, error) {
	v, err := strconv.ParseUint(s, 0, 32)
	if err != nil {
		return 0, err
	}
	return os.FileMode(v), nil
}

func parseUIDGID(uidStr, gidStr string) (int, int, error) {
	uid, err := strconv.Atoi(uidStr)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid uid %q", uidStr)
	}
	gid, err := strconv.Atoi(gidStr)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid gid %q", gidStr)
	}
	return uid, gid, nil
}

func runMim(args []string) int {
	mimfile := "Mimfile"
	target := ""
	var targetArgs []string

	for i := 1; i < len(args); i++ {
		if args[i] == "-f" && i+1 < len(args) {
			mimfile = args[i+1]
			i++
			continue
		}
		if target == "" {
			target = args[i]
		} else {
			targetArgs = append(targetArgs, args[i])
		}
	}

	data, err := os.ReadFile(mimfile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mim: %s: %v\n", mimfile, err)
		return 1
	}

	lines := strings.Split(string(data), "\n")
	var targets []string
	targetMap := make(map[string][]string)
	currentTarget := ""

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasSuffix(trimmed, ":") {
			currentTarget = strings.TrimSuffix(trimmed, ":")
			targets = append(targets, currentTarget)
			targetMap[currentTarget] = []string{}
		} else if currentTarget != "" {
			targetMap[currentTarget] = append(targetMap[currentTarget], line)
		}
	}

	if len(targets) == 0 {
		fmt.Fprintf(os.Stderr, "mim: no targets found in %s\n", mimfile)
		return 1
	}

	if target == "" {
		target = targets[0]
	}

	targetLines, exists := targetMap[target]
	if !exists {
		fmt.Fprintf(os.Stderr, "mim: target '%s' not found\n", target)
		return 1
	}

	var sb strings.Builder
	for _, l := range targetLines {
		sb.WriteString(l)
		sb.WriteString("\n")
	}
	scriptContent := sb.String()

	shellExe := "sh"
	if runtime.GOOS == "windows" {
		if _, err := exec.LookPath("sh"); err != nil {
			for _, l := range targetLines {
				trimmedCmd := strings.TrimSpace(l)
				if trimmedCmd == "" {
					continue
				}
				cmd := exec.Command("cmd", "/c", trimmedCmd)
				cmd.Stdin = os.Stdin
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				if err := cmd.Run(); err != nil {
					if exitErr, ok := err.(*exec.ExitError); ok {
						if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
							return status.ExitStatus()
						}
					}
					return 1
				}
			}
			return 0
		}
	}

	shellArgs := []string{"-s"}
	shellArgs = append(shellArgs, targetArgs...)
	cmd := exec.Command(shellExe, shellArgs...)
	cmd.Stdin = strings.NewReader(scriptContent)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				return status.ExitStatus()
			}
		}
		fmt.Fprintf(os.Stderr, "mim: failed to execute script: %v\n", err)
		return 1
	}

	return 0
}

func runMt(args []string) int {
	fmt.Fprintf(os.Stderr, "mt: not yet implemented in pure Go\n")
	return 1
}

func runNanddump(args []string) int {
	fmt.Fprintf(os.Stderr, "nanddump: not yet implemented in pure Go\n")
	return 1
}

func runNandwrite(args []string) int {
	fmt.Fprintf(os.Stderr, "nandwrite: not yet implemented in pure Go\n")
	return 1
}

func runRaidautorun(args []string) int {
	fmt.Fprintf(os.Stderr, "raidautorun: not yet implemented in pure Go\n")
	return 1
}

func runReadahead(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "readahead: missing file\n")
		return 1
	}
	exitCode := 0
	buf := make([]byte, 1024*1024)
	for _, name := range args[1:] {
		f, err := os.Open(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "readahead: %s: %v\n", name, err)
			exitCode = 1
			continue
		}
		for {
			_, err := f.Read(buf)
			if err != nil {
				break
			}
		}
		f.Close()
	}
	return exitCode
}

func runRx(args []string) int {
	fmt.Fprintf(os.Stderr, "rx: not yet implemented in pure Go\n")
	return 1
}

// --- UBI tools ---
func init() {
	applet.Register(&applet.Applet{Name: "ubiattach", Short: "Attach UBI device", Func: runUbiTool})
	applet.Register(&applet.Applet{Name: "ubidetach", Short: "Detach UBI device", Func: runUbiTool})
	applet.Register(&applet.Applet{Name: "ubimkvol", Short: "Create UBI volume", Func: runUbiTool})
	applet.Register(&applet.Applet{Name: "ubirename", Short: "Rename UBI volume", Func: runUbiTool})
	applet.Register(&applet.Applet{Name: "ubirmvol", Short: "Remove UBI volume", Func: runUbiTool})
	applet.Register(&applet.Applet{Name: "ubirsvol", Short: "Resize UBI volume", Func: runUbiTool})
	applet.Register(&applet.Applet{Name: "ubiupdatevol", Short: "Update UBI volume", Func: runUbiTool})
}

func runUbiTool(args []string) int {
	fmt.Fprintf(os.Stderr, "%s: not yet implemented in pure Go\n", args[0])
	return 1
}

// --- development helpers from busybox-w32 ---
func init() {
	applet.Register(&applet.Applet{Name: "parse", Short: "Parse simple config input", Func: runParse})
	applet.Register(&applet.Applet{Name: "unit", Short: "Run built-in unit tests", Func: runUnit})
}

func runParse(args []string) int {
	files := []string{}
	for _, a := range args[1:] {
		if !strings.HasPrefix(a, "-") {
			files = append(files, a)
		}
	}
	if len(files) == 0 {
		files = []string{"-"}
	}
	for _, name := range files {
		var r io.Reader
		if name == "-" {
			r = os.Stdin
		} else {
			f, err := os.Open(name)
			if err != nil {
				fmt.Fprintf(os.Stderr, "parse: %s: %v\n", name, err)
				return 1
			}
			defer f.Close()
			r = f
		}
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			fmt.Println(line)
		}
	}
	return 0
}

func runUnit(args []string) int {
	fmt.Println("unit: no built-in unit tests are registered")
	return 0
}

// --- rfkill ---
func init() {
	applet.Register(&applet.Applet{Name: "rfkill", Short: "Enable/disable wireless devices", Func: runRfkill})
}

func runRfkill(args []string) int {
	if len(args) == 1 || args[1] == "list" {
		entries, err := os.ReadDir("/sys/class/rfkill")
		if err != nil {
			fmt.Fprintf(os.Stderr, "rfkill: %v\n", err)
			return 1
		}
		for _, entry := range entries {
			base := filepath.Join("/sys/class/rfkill", entry.Name())
			name, _ := os.ReadFile(filepath.Join(base, "name"))
			typ, _ := os.ReadFile(filepath.Join(base, "type"))
			soft, _ := os.ReadFile(filepath.Join(base, "soft"))
			hard, _ := os.ReadFile(filepath.Join(base, "hard"))
			fmt.Printf("%s: %s: %s\n", entry.Name(), strings.TrimSpace(string(typ)), strings.TrimSpace(string(name)))
			fmt.Printf("\tSoft blocked: %s\n", yesNo(strings.TrimSpace(string(soft)) == "1"))
			fmt.Printf("\tHard blocked: %s\n", yesNo(strings.TrimSpace(string(hard)) == "1"))
		}
		return 0
	}
	if len(args) >= 3 && (args[1] == "block" || args[1] == "unblock") {
		value := "1"
		if args[1] == "unblock" {
			value = "0"
		}
		return setRfkill(args[2], value)
	}
	fmt.Fprintf(os.Stderr, "rfkill: usage: rfkill [list|block ID|unblock ID]\n")
	return 1
}

// --- inotifyd ---
func init() {
	applet.Register(&applet.Applet{Name: "inotifyd", Short: "Wait for filesystem events", Func: runInotifyd})
}

func runInotifyd(args []string) int {
	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "inotifyd: not supported\n")
		return 1
	}
	if len(args) < 3 {
		fmt.Fprintf(os.Stderr, "inotifyd: usage: inotifyd PROG FILE[:MASK]...\n")
		return 1
	}

	prog := args[1]
	files := args[2:]

	// inotify_init
	fd, _, errno := syscall.RawSyscall(290, 0, 0, 0) // __NR_inotify_init
	if errno != 0 {
		fmt.Fprintf(os.Stderr, "inotifyd: %v\n", errno)
		return 1
	}
	defer syscall.Close(int(fd))

	for _, f := range files {
		name := f
		mask := uint32(0xfff) // IN_ALL_EVENTS
		if idx := strings.Index(f, ":"); idx >= 0 {
			name = f[:idx]
			// Parse mask
			maskStr := f[idx+1:]
			if strings.Contains(maskStr, "r") {
				mask |= 0x1 // IN_ACCESS
			}
			if strings.Contains(maskStr, "w") {
				mask |= 0x2 // IN_MODIFY
			}
			if strings.Contains(maskStr, "c") {
				mask |= 0x100 // IN_CREATE
			}
		}
		// inotify_add_watch
		nameBytes := append([]byte(name), 0)
		syscall.RawSyscall(292, fd, uintptr(unsafe.Pointer(&nameBytes[0])), uintptr(mask))
	}

	buf := make([]byte, 4096)
	for {
		n, _, _ := syscall.RawSyscall(syscall.SYS_READ, fd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		if n == 0 {
			break
		}
		// Parse inotify_event struct
		for offset := 0; offset < int(n); {
			wd := int32(binary.LittleEndian.Uint32(buf[offset:]))
			eventMask := binary.LittleEndian.Uint32(buf[offset+4:])
			_ = wd
			// Execute program with event info
			eventName := "?"
			switch eventMask {
			case 1:
				eventName = "access"
			case 2:
				eventName = "modify"
			case 4:
				eventName = "attrib"
			case 8:
				eventName = "close_write"
			case 0x10:
				eventName = "close_nowrite"
			case 0x20:
				eventName = "open"
			case 0x100:
				eventName = "create"
			case 0x200:
				eventName = "delete"
			case 0x400:
				eventName = "delete_self"
			case 0x800:
				eventName = "move"
			}
			cmd := exec.Command(prog, eventName)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Run()
			offset += 16
			if offset < len(buf) {
				nameLen := binary.LittleEndian.Uint32(buf[offset-4:])
				offset += int(nameLen)
			}
		}
	}
	return 0
}

// --- setserial ---
func init() {
	applet.Register(&applet.Applet{Name: "setserial", Short: "Get/set serial port parameters", Func: runSetserial})
}

func runSetserial(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "setserial: missing device\n")
		return 1
	}
	for _, dev := range args[1:] {
		if strings.HasPrefix(dev, "-") {
			continue
		}
		name := filepath.Base(dev)
		base := filepath.Join("/sys/class/tty", name)
		driver := "unknown"
		if link, err := os.Readlink(filepath.Join(base, "device/driver")); err == nil {
			driver = filepath.Base(link)
		}
		fmt.Printf("%s, UART: %s\n", dev, driver)
	}
	return 0
}

// --- make (improved) ---
// Already in misc/misc.go - no duplicate

// --- getfattr / setfattr ---
func init() {
	applet.Register(&applet.Applet{Name: "getfattr", Short: "Get file extended attributes", Func: runGetfattr})
	applet.Register(&applet.Applet{Name: "setfattr", Short: "Set file extended attributes", Func: runSetfattr})
}

func runGetfattr(args []string) int {
	name := ""
	files := []string{}
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-n", "--name":
			if i+1 < len(args) {
				i++
				name = args[i]
			}
		default:
			if !strings.HasPrefix(args[i], "-") {
				files = append(files, args[i])
			}
		}
	}
	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "getfattr: missing file\n")
		return 1
	}
	exitCode := 0
	for _, file := range files {
		if name != "" {
			value, err := getXattr(file, name)
			if err != nil {
				fmt.Fprintf(os.Stderr, "getfattr: %s: %v\n", file, err)
				exitCode = 1
				continue
			}
			fmt.Printf("# file: %s\n%s=\"%s\"\n", file, name, escapeAttr(value))
			continue
		}
		names, err := listXattrs(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "getfattr: %s: %v\n", file, err)
			exitCode = 1
			continue
		}
		if len(names) > 0 {
			fmt.Printf("# file: %s\n", file)
		}
		for _, n := range names {
			value, err := getXattr(file, n)
			if err == nil {
				fmt.Printf("%s=\"%s\"\n", n, escapeAttr(value))
			}
		}
	}
	return exitCode
}

func runSetfattr(args []string) int {
	name := ""
	value := ""
	remove := ""
	files := []string{}
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-n", "--name":
			if i+1 < len(args) {
				i++
				name = args[i]
			}
		case "-v", "--value":
			if i+1 < len(args) {
				i++
				value = args[i]
			}
		case "-x", "--remove":
			if i+1 < len(args) {
				i++
				remove = args[i]
			}
		default:
			if !strings.HasPrefix(args[i], "-") {
				files = append(files, args[i])
			}
		}
	}
	if len(files) == 0 || (name == "" && remove == "") {
		fmt.Fprintf(os.Stderr, "setfattr: missing attribute or file\n")
		return 1
	}
	exitCode := 0
	for _, file := range files {
		var err error
		if remove != "" {
			err = unix.Removexattr(file, remove)
		} else {
			err = unix.Setxattr(file, name, []byte(value), 0)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "setfattr: %s: %v\n", file, err)
			exitCode = 1
		}
	}
	return exitCode
}

// --- lsscsi ---
func init() {
	applet.Register(&applet.Applet{Name: "lsscsi", Short: "List SCSI devices", Func: runLsscsi})
}

func runLsscsi(args []string) int {
	entries, err := os.ReadDir("/sys/class/scsi_device")
	if err != nil {
		fmt.Fprintf(os.Stderr, "lsscsi: %v\n", err)
		return 1
	}
	for _, entry := range entries {
		base := filepath.Join("/sys/class/scsi_device", entry.Name(), "device")
		vendor, _ := os.ReadFile(filepath.Join(base, "vendor"))
		model, _ := os.ReadFile(filepath.Join(base, "model"))
		typ, _ := os.ReadFile(filepath.Join(base, "type"))
		fmt.Printf("[%s] %-8s %-16s type=%s\n", entry.Name(), strings.TrimSpace(string(vendor)), strings.TrimSpace(string(model)), strings.TrimSpace(string(typ)))
	}
	return 0
}

// --- pdpmake ---
func init() {
	applet.Register(&applet.Applet{Name: "pdpmake", Short: "PDP Make utility", Func: runPdpmake})
}

func runPdpmake(args []string) int {
	return runMake(args)
}

// --- seedrng ---
func init() {
	applet.Register(&applet.Applet{Name: "seedrng", Short: "Seed random number generator", Func: runSeedrng})
}

func runSeedrng(args []string) int {
	seedDir := "/var/lib/seedrng"
	skipCredit := false
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-n", "--skip-credit":
			skipCredit = true
		case "-d", "--seed-dir":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "seedrng: option %s requires an argument\n", args[i])
				return 1
			}
			i++
			seedDir = args[i]
		}
	}
	if err := os.MkdirAll(seedDir, 0700); err != nil {
		fmt.Fprintf(os.Stderr, "seedrng: %v\n", err)
		return 1
	}
	newSeed, err := buildSeedMaterial(seedDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "seedrng: %v\n", err)
		return 1
	}
	credited := false
	if runtime.GOOS == "linux" && !skipCredit {
		credited = writeEntropy("/dev/random", newSeed) == nil
	}
	if !credited {
		_ = writeEntropy("/dev/urandom", newSeed)
	}
	target := filepath.Join(seedDir, "seed")
	if credited {
		target = filepath.Join(seedDir, "creditable.seed")
	}
	if err := os.WriteFile(target, newSeed, 0400); err != nil {
		fmt.Fprintf(os.Stderr, "seedrng: %v\n", err)
		return 1
	}
	if credited {
		_ = os.Remove(filepath.Join(seedDir, "seed"))
	}
	fmt.Printf("Saving %d bits of %screditable seed for next boot\n", len(newSeed)*8, map[bool]string{true: "", false: "non-"}[credited])
	return 0
}

func buildSeedMaterial(seedDir string) ([]byte, error) {
	h := sha256.New()
	for _, name := range []string{"seed", "creditable.seed"} {
		data, err := os.ReadFile(filepath.Join(seedDir, name))
		if err == nil {
			_, _ = h.Write(data)
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, _ = h.Write([]byte(now))
	fresh := make([]byte, 32)
	if _, err := cryptorand.Read(fresh); err != nil {
		return nil, err
	}
	_, _ = h.Write(fresh)
	sum := h.Sum(nil)
	copy(fresh, sum)
	return fresh, nil
}

func writeEntropy(path string, seed []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(seed)
	return err
}

func printSysfsValue(label, path string) {
	data, err := os.ReadFile(path)
	if err == nil {
		fmt.Printf("%s = %s\n", label, strings.TrimSpace(string(data)))
	}
}

func readBlockSizeBytes(name string) (int64, error) {
	data, err := os.ReadFile(filepath.Join("/sys/class/block", name, "size"))
	if err != nil {
		return 0, err
	}
	sectors, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, err
	}
	return sectors * 512, nil
}

func findManPage(page string) (string, error) {
	roots := []string{"/usr/share/man", "/usr/local/share/man"}
	for _, root := range roots {
		for section := 1; section <= 9; section++ {
			pattern := filepath.Join(root, fmt.Sprintf("man%d", section), page+".*")
			matches, _ := filepath.Glob(pattern)
			if len(matches) > 0 {
				return matches[0], nil
			}
		}
	}
	return "", os.ErrNotExist
}

func printManPage(path string) int {
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "man: %v\n", err)
		return 1
	}
	defer f.Close()
	var r io.Reader = f
	if strings.HasSuffix(path, ".gz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "man: %v\n", err)
			return 1
		}
		defer gz.Close()
		r = gz
	}
	_, err = io.Copy(os.Stdout, r)
	if err != nil {
		return 1
	}
	return 0
}

func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func setRfkill(target, value string) int {
	entries, err := os.ReadDir("/sys/class/rfkill")
	if err != nil {
		fmt.Fprintf(os.Stderr, "rfkill: %v\n", err)
		return 1
	}
	exitCode := 1
	for _, entry := range entries {
		name := entry.Name()
		id := strings.TrimPrefix(name, "rfkill")
		devName, _ := os.ReadFile(filepath.Join("/sys/class/rfkill", name, "name"))
		if target != "all" && target != id && target != name && target != strings.TrimSpace(string(devName)) {
			continue
		}
		if err := os.WriteFile(filepath.Join("/sys/class/rfkill", name, "soft"), []byte(value), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "rfkill: %s: %v\n", name, err)
		} else {
			exitCode = 0
		}
	}
	return exitCode
}

func listXattrs(path string) ([]string, error) {
	n, err := unix.Listxattr(path, nil)
	if err != nil || n == 0 {
		return nil, err
	}
	buf := make([]byte, n)
	n, err = unix.Listxattr(path, buf)
	if err != nil {
		return nil, err
	}
	names := []string{}
	for _, part := range strings.Split(string(buf[:n]), "\x00") {
		if part != "" {
			names = append(names, part)
		}
	}
	return names, nil
}

func getXattr(path, name string) ([]byte, error) {
	n, err := unix.Getxattr(path, name, nil)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, n)
	n, err = unix.Getxattr(path, name, buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

func escapeAttr(data []byte) string {
	s := string(data)
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}

// --- ttysize ---
func init() {
	applet.Register(&applet.Applet{Name: "ttysize", Short: "Print terminal size", Func: runTtysize})
}

func runTtysize(args []string) int {
	width, height := ttySize()
	if len(args) == 1 {
		fmt.Printf("%d %d\n", width, height)
		return 0
	}
	first := true
	for _, arg := range args[1:] {
		switch arg {
		case "w":
			if !first {
				fmt.Print(" ")
			}
			fmt.Print(width)
			first = false
		case "h":
			if !first {
				fmt.Print(" ")
			}
			fmt.Print(height)
			first = false
		}
	}
	fmt.Println()
	return 0
}

const (
	i2cSlaveAddr   = 0x0703
	i2cRdwrIoctl   = 0x0707
	i2cRdwrMaxMsgs = 42
	i2cMsgRead     = 0x0001
)

type i2cMsg struct {
	Addr  uint16
	Flags uint16
	Len   uint16
	_     uint16
	Buf   uintptr
}

type i2cRdwrIoctlData struct {
	Msgs  uintptr
	Nmsgs uint32
}

func timexStatusNames(status int32) string {
	parts := []string{}
	flags := []struct {
		mask int32
		name string
	}{
		{unix.STA_PLL, "PLL"},
		{unix.STA_PPSFREQ, "PPSFREQ"},
		{unix.STA_PPSTIME, "PPSTIME"},
		{unix.STA_FLL, "FLL"},
		{unix.STA_INS, "INS"},
		{unix.STA_DEL, "DEL"},
		{unix.STA_UNSYNC, "UNSYNC"},
		{unix.STA_FREQHOLD, "FREQHOLD"},
		{unix.STA_PPSSIGNAL, "PPSSIGNAL"},
		{unix.STA_PPSJITTER, "PPSJITTER"},
		{unix.STA_PPSWANDER, "PPSWANDER"},
		{unix.STA_PPSERROR, "PPSERROR"},
		{unix.STA_CLOCKERR, "CLOCKERR"},
	}
	for _, flag := range flags {
		if status&flag.mask != 0 {
			parts = append(parts, flag.name)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " | ")
}

func timexStateDescription(state int) string {
	switch state {
	case unix.TIME_OK:
		return "clock synchronized"
	case unix.TIME_INS:
		return "insert leap second"
	case unix.TIME_DEL:
		return "delete leap second"
	case unix.TIME_OOP:
		return "leap second in progress"
	case unix.TIME_WAIT:
		return "leap second has occurred"
	case unix.TIME_ERROR:
		return "clock not synchronized"
	default:
		return "error"
	}
}

func i2cDevicePath(bus string) (string, error) {
	if n, err := strconv.Atoi(bus); err == nil {
		return fmt.Sprintf("/dev/i2c-%d", n), nil
	}
	if strings.HasPrefix(bus, "/dev/") {
		return bus, nil
	}
	return "", fmt.Errorf("invalid bus %q", bus)
}

func fillI2CWriteBuffer(buf []byte, args []string, idx *int) error {
	pos := 0
	for pos < len(buf) {
		if *idx >= len(args) {
			return fmt.Errorf("missing data bytes")
		}
		token := args[*idx]
		*idx = *idx + 1
		value, suffix, err := parseI2CDataToken(token)
		if err != nil {
			return err
		}
		for pos < len(buf) {
			buf[pos] = value
			pos++
			if suffix == 0 || pos >= len(buf) {
				break
			}
			switch suffix {
			case 'p':
				value = ((value ^ 27) + 13)
				value = (value << 1) | (value >> 7)
			case '+':
				value++
			case '-':
				value--
			case '=':
			default:
				return fmt.Errorf("invalid data byte suffix: %q", token)
			}
		}
	}
	return nil
}

func parseI2CDataToken(token string) (byte, byte, error) {
	suffix := byte(0)
	if len(token) > 0 {
		last := token[len(token)-1]
		switch last {
		case 'p', '+', '-', '=':
			suffix = last
			token = token[:len(token)-1]
		}
	}
	n, err := strconv.ParseUint(token, 0, 8)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid data byte %q", token)
	}
	return byte(n), suffix, nil
}

func ttySize() (int, int) {
	width, height := 80, 24
	for _, fd := range []int{0, 1, 2} {
		ws, err := unix.IoctlGetWinsize(fd, unix.TIOCGWINSZ)
		if err == nil && ws.Col != 0 && ws.Row != 0 {
			return int(ws.Col), int(ws.Row)
		}
	}
	return width, height
}
