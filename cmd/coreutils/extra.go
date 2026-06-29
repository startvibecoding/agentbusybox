package coreutils

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base32"
	"encoding/base64"
	"fmt"
	"hash/crc32"
	"io"
	"math"
	"os"
	"os/user"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/agentbusybox/pkg/applet"
)

// --- sleep ---
func init() {
	applet.Register(&applet.Applet{Name: "sleep", Short: "Delay for a specified time", Func: runSleep})
}

func runSleep(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "sleep: missing operand\n")
		return 1
	}
	total := 0.0
	for _, a := range args[1:] {
		mult := 1.0
		s := a
		if strings.HasSuffix(s, "s") {
			s = s[:len(s)-1]
		} else if strings.HasSuffix(s, "m") {
			mult = 60
			s = s[:len(s)-1]
		} else if strings.HasSuffix(s, "h") {
			mult = 3600
			s = s[:len(s)-1]
		} else if strings.HasSuffix(s, "d") {
			mult = 86400
			s = s[:len(s)-1]
		}
		var v float64
		fmt.Sscanf(s, "%f", &v)
		total += v * mult
	}
	time.Sleep(time.Duration(total * float64(time.Second)))
	return 0
}

// --- usleep ---
func init() {
	applet.Register(&applet.Applet{Name: "usleep", Short: "Sleep some number of microseconds", Func: runUsleep})
}

func runUsleep(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "usleep: missing operand\n")
		return 1
	}
	us, _ := strconv.ParseInt(args[1], 10, 64)
	time.Sleep(time.Duration(us) * time.Microsecond)
	return 0
}

// --- tac ---
func init() {
	applet.Register(&applet.Applet{Name: "tac", Short: "Concatenate and print files in reverse", Func: runTac})
}

func runTac(args []string) int {
	files := args[1:]
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
			fmt.Fprintf(os.Stderr, "tac: %s: %v\n", fname, err)
			return 1
		}
		lines := strings.Split(string(data), "\n")
		// Remove trailing empty line if present
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		for i := len(lines) - 1; i >= 0; i-- {
			fmt.Println(lines[i])
		}
	}
	return 0
}

// --- base64 ---
func init() {
	applet.Register(&applet.Applet{Name: "base64", Short: "Base64 encode/decode", Func: runBase64})
}

func runBase64(args []string) int {
	decode := false
	wrap := 76
	files := []string{}
	for i := 1; i < len(args); i++ {
		if args[i] == "-d" || args[i] == "--decode" {
			decode = true
			continue
		}
		if args[i] == "-w" && i+1 < len(args) {
			i++
			fmt.Sscanf(args[i], "%d", &wrap)
			continue
		}
		if !strings.HasPrefix(args[i], "-") {
			files = append(files, args[i])
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
			fmt.Fprintf(os.Stderr, "base64: %s: %v\n", fname, err)
			return 1
		}
		if decode {
			decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(data)))
			if err != nil {
				fmt.Fprintf(os.Stderr, "base64: invalid input: %v\n", err)
				return 1
			}
			os.Stdout.Write(decoded)
		} else {
			encoded := base64.StdEncoding.EncodeToString(data)
			if wrap > 0 {
				for i := 0; i < len(encoded); i += wrap {
					end := i + wrap
					if end > len(encoded) {
						end = len(encoded)
					}
					fmt.Println(encoded[i:end])
				}
			} else {
				fmt.Println(encoded)
			}
		}
	}
	return 0
}

// --- base32 ---
func init() {
	applet.Register(&applet.Applet{Name: "base32", Short: "Base32 encode/decode", Func: runBase32})
}

func runBase32(args []string) int {
	decode := false
	files := []string{}
	for _, a := range args[1:] {
		if a == "-d" || a == "--decode" {
			decode = true
			continue
		}
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
			fmt.Fprintf(os.Stderr, "base32: %s: %v\n", fname, err)
			return 1
		}
		if decode {
			decoded, err := base32.StdEncoding.DecodeString(strings.TrimSpace(string(data)))
			if err != nil {
				fmt.Fprintf(os.Stderr, "base32: invalid input: %v\n", err)
				return 1
			}
			os.Stdout.Write(decoded)
		} else {
			encoded := base32.StdEncoding.EncodeToString(data)
			for i := 0; i < len(encoded); i += 76 {
				end := i + 76
				if end > len(encoded) {
					end = len(encoded)
				}
				fmt.Println(encoded[i:end])
			}
		}
	}
	return 0
}

// --- md5sum / sha1sum / sha256sum / sha384sum / sha512sum ---
func init() {
	applet.Register(&applet.Applet{Name: "md5sum", Short: "Compute and check MD5 message digest", Func: runMd5sum})
	applet.Register(&applet.Applet{Name: "sha1sum", Short: "Compute and check SHA1 message digest", Func: runSha1sum})
	applet.Register(&applet.Applet{Name: "sha256sum", Short: "Compute and check SHA256 message digest", Func: runSha256sum})
	applet.Register(&applet.Applet{Name: "sha384sum", Short: "Compute and check SHA384 message digest", Func: runSha384sum})
	applet.Register(&applet.Applet{Name: "sha512sum", Short: "Compute and check SHA512 message digest", Func: runSha512sum})
	applet.Register(&applet.Applet{Name: "sha3sum", Short: "Compute and check SHA3 message digest", Func: runSha3sum})
}

func runMd5sum(args []string) int    { return runHash(args, "md5") }
func runSha1sum(args []string) int   { return runHash(args, "sha1") }
func runSha256sum(args []string) int { return runHash(args, "sha256") }
func runSha384sum(args []string) int { return runHash(args, "sha384") }
func runSha512sum(args []string) int { return runHash(args, "sha512") }
func runSha3sum(args []string) int   { return runHash(args, "sha3") }

func runHash(args []string, name string) int {
	check := false
	files := []string{}
	for _, a := range args[1:] {
		if a == "-c" || a == "--check" {
			check = true
			continue
		}
		if !strings.HasPrefix(a, "-") {
			files = append(files, a)
		}
	}
	if len(files) == 0 {
		files = []string{"-"}
	}

	if check {
		return verifyChecksums(files, name)
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
			fmt.Fprintf(os.Stderr, "%s: %s: %v\n", name, fname, err)
			return 1
		}
		fmt.Printf("%x  %s\n", computeHash(data, name), fname)
	}
	return 0
}

func computeHash(data []byte, name string) []byte {
	switch name {
	case "md5":
		h := md5.New()
		h.Write(data)
		return h.Sum(nil)
	case "sha1":
		h := sha1.New()
		h.Write(data)
		return h.Sum(nil)
	case "sha256":
		h := sha256.New()
		h.Write(data)
		return h.Sum(nil)
	case "sha384":
		h := sha512.New384()
		h.Write(data)
		return h.Sum(nil)
	case "sha512":
		h := sha512.New()
		h.Write(data)
		return h.Sum(nil)
	default:
		h := sha256.New()
		h.Write(data)
		return h.Sum(nil)
	}
}

func verifyChecksums(files []string, name string) int {
	exitCode := 0
	for _, fname := range files {
		var data []byte
		var err error
		if fname == "-" {
			data, err = io.ReadAll(os.Stdin)
		} else {
			data, err = os.ReadFile(fname)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s: %v\n", name, fname, err)
			return 1
		}

		for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
			parts := strings.Fields(line)
			if len(parts) < 2 {
				continue
			}
			expected := parts[0]
			file := parts[1]

			fdata, err := os.ReadFile(file)
			if err != nil {
				fmt.Printf("%s: FAILED open or read\n", file)
				exitCode = 1
				continue
			}
			actual := fmt.Sprintf("%x", computeHash(fdata, name))
			if actual == expected {
				fmt.Printf("%s: OK\n", file)
			} else {
				fmt.Printf("%s: FAILED\n", file)
				exitCode = 1
			}
		}
	}
	return exitCode
}

// --- cksum ---
func init() {
	applet.Register(&applet.Applet{Name: "cksum", Short: "CRC checksum and byte count", Func: runCksum})
}

func runCksum(args []string) int {
	files := args[1:]
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
			fmt.Fprintf(os.Stderr, "cksum: %s: %v\n", fname, err)
			return 1
		}
		crc := crc32.ChecksumIEEE(data)
		if fname == "-" {
			fmt.Printf("%d %d\n", crc, len(data))
		} else {
			fmt.Printf("%d %d %s\n", crc, len(data), fname)
		}
	}
	return 0
}

// --- crc32 ---
func init() {
	applet.Register(&applet.Applet{Name: "crc32", Short: "CRC32 checksum", Func: runCrc32})
}

func runCrc32(args []string) int {
	files := args[1:]
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
			fmt.Fprintf(os.Stderr, "crc32: %s: %v\n", fname, err)
			return 1
		}
		fmt.Printf("%08x\n", crc32.ChecksumIEEE(data))
	}
	return 0
}

// --- nproc ---
func init() {
	applet.Register(&applet.Applet{Name: "nproc", Short: "Print the number of processing units", Func: runNproc})
}

func runNproc(args []string) int {
	fmt.Println(runtime.NumCPU())
	return 0
}

// --- printenv ---
func init() {
	applet.Register(&applet.Applet{Name: "printenv", Short: "Print environment variables", Func: runPrintenv})
}

func runPrintenv(args []string) int {
	if len(args) < 2 {
		for _, e := range os.Environ() {
			fmt.Println(e)
		}
		return 0
	}
	exitCode := 0
	for _, name := range args[1:] {
		val := os.Getenv(name)
		if val != "" {
			fmt.Println(val)
		} else {
			exitCode = 1
		}
	}
	return exitCode
}

// --- hostid ---
func init() {
	applet.Register(&applet.Applet{Name: "hostid", Short: "Print numeric host identifier", Func: runHostid})
}

func runHostid(args []string) int {
	name, _ := os.Hostname()
	h := uint32(0)
	for _, c := range name {
		h = h*31 + uint32(c)
	}
	fmt.Printf("%08x\n", h)
	return 0
}

// --- link ---
func init() {
	applet.Register(&applet.Applet{Name: "link", Short: "Create a hard link", Func: runLinkCmd})
}

func runLinkCmd(args []string) int {
	if len(args) < 3 {
		fmt.Fprintf(os.Stderr, "link: missing operand\n")
		return 1
	}
	if err := os.Link(args[1], args[2]); err != nil {
		fmt.Fprintf(os.Stderr, "link: %v\n", err)
		return 1
	}
	return 0
}

// --- join ---
func init() {
	applet.Register(&applet.Applet{Name: "join", Short: "Join lines on a common field", Func: runJoin})
}

func runJoin(args []string) int {
	sep := ""
	field1, field2 := 1, 1
	files := []string{}
	for i := 1; i < len(args); i++ {
		if args[i] == "-t" && i+1 < len(args) {
			i++
			sep = args[i]
			continue
		}
		if strings.HasPrefix(args[i], "-1") && len(args[i]) > 2 {
			fmt.Sscanf(args[i][2:], "%d", &field1)
			continue
		}
		if strings.HasPrefix(args[i], "-2") && len(args[i]) > 2 {
			fmt.Sscanf(args[i][2:], "%d", &field2)
			continue
		}
		if !strings.HasPrefix(args[i], "-") {
			files = append(files, args[i])
		}
	}
	if len(files) < 2 {
		fmt.Fprintf(os.Stderr, "join: missing operand\n")
		return 1
	}
	if sep == "" {
		sep = " "
	}

	lines1 := readLinesJoin(files[0])
	lines2 := readLinesJoin(files[1])

	map2 := make(map[string]string)
	for _, l := range lines2 {
		parts := strings.SplitN(l, sep, field2+1)
		if len(parts) >= field2 {
			key := parts[field2-1]
			map2[key] = l
		}
	}

	for _, l := range lines1 {
		parts := strings.SplitN(l, sep, field1+1)
		if len(parts) >= field1 {
			key := parts[field1-1]
			if match, ok := map2[key]; ok {
				fmt.Printf("%s%s%s\n", l, sep, match)
			}
		}
	}
	return 0
}

func readLinesJoin(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	return lines
}

// --- fsync ---
func init() {
	applet.Register(&applet.Applet{Name: "fsync", Short: "Synchronize file data to disk", Func: runFsync})
}

func runFsync(args []string) int {
	files := args[1:]
	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "fsync: missing operand\n")
		return 1
	}
	exitCode := 0
	for _, fname := range files {
		f, err := os.Open(fname)
		if err != nil {
			fmt.Fprintf(os.Stderr, "fsync: %s: %v\n", fname, err)
			exitCode = 1
			continue
		}
		if err := f.Sync(); err != nil {
			fmt.Fprintf(os.Stderr, "fsync: %s: %v\n", fname, err)
			exitCode = 1
		}
		f.Close()
	}
	return exitCode
}

// --- dos2unix / unix2dos ---
func init() {
	applet.Register(&applet.Applet{Name: "dos2unix", Short: "Convert text file from DOS to Unix format", Func: runDos2unix})
	applet.Register(&applet.Applet{Name: "unix2dos", Short: "Convert text file from Unix to DOS format", Func: runUnix2dos})
}

func runDos2unix(args []string) int { return convertLineEndings(args, false) }
func runUnix2dos(args []string) int { return convertLineEndings(args, true) }

func convertLineEndings(args []string, toDos bool) int {
	files := args[1:]
	if len(files) == 0 {
		data, _ := io.ReadAll(os.Stdin)
		s := string(data)
		if toDos {
			s = strings.ReplaceAll(s, "\r\n", "\n")
			s = strings.ReplaceAll(s, "\n", "\r\n")
		} else {
			s = strings.ReplaceAll(s, "\r\n", "\n")
		}
		fmt.Print(s)
		return 0
	}
	for _, fname := range files {
		data, err := os.ReadFile(fname)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s: %v\n", args[0], fname, err)
			return 1
		}
		s := string(data)
		if toDos {
			s = strings.ReplaceAll(s, "\r\n", "\n")
			s = strings.ReplaceAll(s, "\n", "\r\n")
		} else {
			s = strings.ReplaceAll(s, "\r\n", "\n")
		}
		info, _ := os.Stat(fname)
		mode := os.FileMode(0666)
		if info != nil {
			mode = info.Mode()
		}
		if err := os.WriteFile(fname, []byte(s), mode); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s: %v\n", args[0], fname, err)
			return 1
		}
	}
	return 0
}

// --- expr ---
func init() {
	applet.Register(&applet.Applet{Name: "expr", Short: "Evaluate expressions", Func: runExpr})
}

func runExpr(args []string) int {
	a := args[1:]
	if len(a) == 0 {
		fmt.Fprintf(os.Stderr, "expr: missing operand\n")
		return 1
	}
	// Simple expression evaluator
	if len(a) == 1 {
		fmt.Println(a[0])
		if a[0] == "" || a[0] == "0" {
			return 1
		}
		return 0
	}
	if len(a) == 3 {
		left := a[0]
		op := a[1]
		right := a[2]
		switch op {
		case "+", "-", "*", "/", "%":
			var l, r float64
			fmt.Sscanf(left, "%f", &l)
			fmt.Sscanf(right, "%f", &r)
			var result float64
			switch op {
			case "+":
				result = l + r
			case "-":
				result = l - r
			case "*":
				result = l * r
			case "/":
				if r == 0 {
					fmt.Fprintf(os.Stderr, "expr: division by zero\n")
					return 2
				}
				result = math.Floor(l / r)
			case "%":
				if r == 0 {
					fmt.Fprintf(os.Stderr, "expr: division by zero\n")
					return 2
				}
				result = math.Mod(l, r)
			}
			if result == math.Floor(result) {
				fmt.Printf("%d\n", int64(result))
			} else {
				fmt.Println(result)
			}
			return 0
		case "=":
			if left == right {
				fmt.Println("1")
				return 0
			}
			fmt.Println("0")
			return 1
		case "!=", "<>":
			if left != right {
				fmt.Println("1")
				return 0
			}
			fmt.Println("0")
			return 1
		case "<":
			if left < right {
				fmt.Println("1")
				return 0
			}
			fmt.Println("0")
			return 1
		case "<=":
			if left <= right {
				fmt.Println("1")
				return 0
			}
			fmt.Println("0")
			return 1
		case ">":
			if left > right {
				fmt.Println("1")
				return 0
			}
			fmt.Println("0")
			return 1
		case ">=":
			if left >= right {
				fmt.Println("1")
				return 0
			}
			fmt.Println("0")
			return 1
		case "&":
			if left != "" && right != "" {
				fmt.Println(left)
				return 0
			}
			fmt.Println("0")
			return 1
		case "|":
			if left != "" {
				fmt.Println(left)
				return 0
			}
			if right != "" {
				fmt.Println(right)
				return 0
			}
			fmt.Println("0")
			return 1
		case ":":
			// regex match using POSIX expr semantics
			// Convert POSIX \( \) grouping to Go regex ( ) grouping
			pattern := right
			pattern = strings.ReplaceAll(pattern, "\\(", "(")
			pattern = strings.ReplaceAll(pattern, "\\)", ")")
			if !strings.HasPrefix(pattern, "^") {
				pattern = "^" + pattern
			}
			re, err := regexp.Compile(pattern)
			if err != nil {
				fmt.Fprintf(os.Stderr, "expr: invalid regex: %v\n", err)
				return 2
			}
			match := re.FindString(left)
			if len(match) > 0 {
				submatch := re.FindStringSubmatch(left)
				if len(submatch) > 1 {
					fmt.Println(submatch[1])
				} else {
					fmt.Println(len(match))
				}
				return 0
			}
			fmt.Println("")
			return 1
		}
	}
	// length, match, substr, index, etc.
	if len(a) >= 2 {
		switch a[0] {
		case "length":
			fmt.Println(len(a[1]))
			return 0
		case "match":
			if strings.Contains(a[1], a[2]) {
				fmt.Println("1")
				return 0
			}
			fmt.Println("0")
			return 1
		case "substr":
			if len(a) >= 4 {
				var start, length int
				fmt.Sscanf(a[2], "%d", &start)
				fmt.Sscanf(a[3], "%d", &length)
				if start > 0 {
					start--
				}
				end := start + length
				if end > len(a[1]) {
					end = len(a[1])
				}
				if start < len(a[1]) {
					fmt.Println(a[1][start:end])
				}
			}
			return 0
		case "index":
			if len(a) >= 3 {
				idx := strings.Index(a[1], a[2])
				if idx >= 0 {
					fmt.Println(idx + 1)
				} else {
					fmt.Println("0")
				}
			}
			return 0
		}
	}
	fmt.Fprintf(os.Stderr, "expr: syntax error\n")
	return 2
}

// --- arch ---
func init() {
	applet.Register(&applet.Applet{Name: "arch", Short: "Print machine architecture", Func: runArch})
}

func runArch(args []string) int {
	fmt.Println(runtime.GOARCH)
	return 0
}

// --- who ---
func init() {
	applet.Register(&applet.Applet{Name: "who", Short: "Show who is logged on", Func: runWho})
}

// utmpRecord represents a Linux utmp entry (384 bytes)
type utmpRecord struct {
	Type int16
	Pid  int32
	Line [32]byte
	Id   [4]byte
	User [32]byte
	Host [256]byte
	Exit struct {
		Termination int16
		Exit        int16
	}
	Session int32
	Time    struct {
		Sec  int32
		Usec int32
	}
	AddrV6 [16]byte
	Unused [20]byte
}

const (
	utmpType_USER_PROCESS = 7
)

func runWho(args []string) int {
	showAll := false
	showHeader := false
	for _, a := range args[1:] {
		if a == "-a" {
			showAll = true
		}
		if a == "-H" {
			showHeader = true
		}
	}

	if showHeader || showAll {
		fmt.Println("USER		TTY		LOGIN@\t\t IDLE  PID  COMMENT")
	}

	// Try to read utmp
	records, err := readUtmp()
	if err != nil || len(records) == 0 {
		// Fallback: show current user
		u, err2 := user.Current()
		if err2 != nil {
			fmt.Fprintf(os.Stderr, "who: %v\n", err2)
			return 1
		}
		tty := "tty1"
		if t := os.Getenv("TTY"); t != "" {
			tty = t
		}
		fmt.Printf("%-10s %-12s %s\n", u.Username, tty, time.Now().Format("2006-01-02 15:04"))
		return 0
	}

	for _, r := range records {
		if r.Type == utmpType_USER_PROCESS {
			userStr := cstrToString(r.User[:])
			lineStr := cstrToString(r.Line[:])
			loginTime := time.Unix(int64(r.Time.Sec), 0)
			fmt.Printf("%-10s %-12s %s\n", userStr, lineStr, loginTime.Format("Jan _2 15:04"))
		}
	}
	return 0
}

func readUtmp() ([]utmpRecord, error) {
	data, err := os.ReadFile("/var/run/utmp")
	if err != nil {
		return nil, err
	}
	var records []utmpRecord
	size := 384 // sizeof(struct utmp) on Linux
	for i := 0; i+size <= len(data); i += size {
		var r utmpRecord
		// Parse the binary record manually
		r.Type = int16(data[i]) | int16(data[i+1])<<8
		r.Pid = int32(data[i+4]) | int32(data[i+5])<<8 | int32(data[i+6])<<16 | int32(data[i+7])<<24
		copy(r.Line[:], data[i+8:i+40])
		copy(r.User[:], data[i+44:i+76])
		copy(r.Host[:], data[i+76:i+332])
		r.Time.Sec = int32(data[i+340]) | int32(data[i+341])<<8 | int32(data[i+342])<<16 | int32(data[i+343])<<24
		records = append(records, r)
	}
	return records, nil
}

func cstrToString(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

// --- w ---
func init() {
	applet.Register(&applet.Applet{Name: "w", Short: "Show who is logged on and what they are doing", Func: runW})
}

func runW(args []string) int {
	noHeader := false
	for _, a := range args[1:] {
		if a == "-h" {
			noHeader = true
		}
	}

	// Show uptime header
	if !noHeader {
		uptimeSec, loadAvg := readUptimeInfo()
		days := uptimeSec / 86400
		hours := (uptimeSec % 86400) / 3600
		mins := (uptimeSec % 3600) / 60
		fmt.Printf(" %s up %d days, %d:%02d, load average: %.2f, %.2f, %.2f\n",
			time.Now().Format("15:04:05"), days, hours, mins,
			loadAvg[0], loadAvg[1], loadAvg[2])
		fmt.Println("USER	TTY	LOGIN@		IDLE	WHAT")
	}

	records, err := readUtmp()
	if err != nil || len(records) == 0 {
		u, err2 := user.Current()
		if err2 != nil {
			fmt.Fprintf(os.Stderr, "w: %v\n", err2)
			return 1
		}
		tty := "tty1"
		if t := os.Getenv("TTY"); t != "" {
			tty = t
		}
		fmt.Printf("%-10s %-8s %-14s %s\n", u.Username, tty, time.Now().Format("Jan _2 15:04"), "-")
		return 0
	}

	for _, r := range records {
		if r.Type == utmpType_USER_PROCESS {
			userStr := cstrToString(r.User[:])
			lineStr := cstrToString(r.Line[:])
			loginTime := time.Unix(int64(r.Time.Sec), 0)
			idle := "-"
			fmt.Printf("%-10s %-8s %-14s %s\n", userStr, lineStr, loginTime.Format("Jan _2 15:04"), idle)
		}
	}
	return 0
}

func readUptimeInfo() (int64, [3]float64) {
	var uptime int64
	var loadAvg [3]float64

	// Read uptime
	data, err := os.ReadFile("/proc/uptime")
	if err == nil {
		fmt.Sscanf(string(data), "%d", &uptime)
	}

	// Read load average
	data2, err2 := os.ReadFile("/proc/loadavg")
	if err2 == nil {
		fmt.Sscanf(string(data2), "%f %f %f", &loadAvg[0], &loadAvg[1], &loadAvg[2])
	}

	return uptime, loadAvg
}

// --- uudecode ---
func init() {
	applet.Register(&applet.Applet{Name: "uudecode", Short: "Decode a file created by uuencode", Func: runUudecode})
}

func runUudecode(args []string) int {
	files := args[1:]
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
			fmt.Fprintf(os.Stderr, "uudecode: %s: %v\n", fname, err)
			return 1
		}

		lines := strings.Split(string(data), "\n")
		inBlock := false
		for _, line := range lines {
			if strings.HasPrefix(line, "begin ") {
				inBlock = true
				continue
			}
			if line == "end" {
				break
			}
			if !inBlock {
				continue
			}
			if len(line) == 0 {
				continue
			}
			// decode
			n := int(line[0]-32) & 0x3f
			decoded := make([]byte, 0, n)
			pos := 1
			for len(decoded) < n && pos+3 <= len(line) {
				c1 := int(line[pos]-32) & 0x3f
				c2 := int(line[pos+1]-32) & 0x3f
				c3 := int(line[pos+2]-32) & 0x3f
				decoded = append(decoded, byte(c1<<2|c2>>4))
				if len(decoded) < n {
					decoded = append(decoded, byte(c2<<4|c3>>2))
				}
				if len(decoded) < n {
					decoded = append(decoded, byte(c3<<6|0))
				}
				pos += 3
			}
			os.Stdout.Write(decoded[:n])
		}
	}
	return 0
}

// --- uuencode ---
func init() {
	applet.Register(&applet.Applet{Name: "uuencode", Short: "Encode a file", Func: runUuencode})
}

func runUuencode(args []string) int {
	files := args[1:]
	name := "stdin"
	if len(files) > 0 && !strings.HasPrefix(files[0], "-") {
		name = files[0]
		files = files[1:]
	}

	var data []byte
	var err error
	if len(files) == 0 || files[0] == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(files[0])
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "uuencode: %v\n", err)
		return 1
	}

	fmt.Printf("begin 644 %s\n", name)
	for i := 0; i < len(data); i += 45 {
		end := i + 45
		if end > len(data) {
			end = len(data)
		}
		chunk := data[i:end]
		fmt.Printf("%c", byte(len(chunk)+32))
		for j := 0; j < len(chunk); j += 3 {
			b1 := chunk[j]
			var b2, b3 byte
			if j+1 < len(chunk) {
				b2 = chunk[j+1]
			}
			if j+2 < len(chunk) {
				b3 = chunk[j+2]
			}
			fmt.Printf("%c%c%c%c",
				byte((b1>>2)&0x3f)+32,
				byte(((b1<<4)|(b2>>4))&0x3f)+32,
				byte(((b2<<2)|(b3>>6))&0x3f)+32,
				byte(b3&0x3f)+32)
		}
		fmt.Println()
	}
	fmt.Printf(" \nend\n")
	return 0
}
