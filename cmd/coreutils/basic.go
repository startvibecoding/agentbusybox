package coreutils

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/agentbusybox/pkg/applet"
)

func init() {
	applet.Register(&applet.Applet{Name: "ls", Short: "List directory contents", Func: runLs})
}

type lsOpts struct {
	all       bool // -a
	almostAll bool // -A
	long      bool // -l
	human     bool // -h
	recursive bool // -R
	dirs      bool // -d
	one       bool // -1
	columns   bool // -C
	byLines   bool // -x
	sortSize  bool // -S
	sortExt   bool // -X
	sortTime  bool // -t
	sortCTime bool // -c
	sortATime bool // -u
	reverse   bool // -r
	class     bool // -F
	dirSlash  bool // -p
	inode     bool // -i
	numeric   bool // -n
	noOwner   bool // -g
	blocks    bool // -s
	quote     bool // -Q
	followL   bool // -L
	fullTime  bool // --full-time
	dirFirst  bool // --group-directories-first
}

func runLs(args []string) int {
	opts := lsOpts{}
	paths := []string{}

	i := 1
	for ; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") {
			break
		}
		if a == "--" {
			i++
			break
		}
		// Long options
		if strings.HasPrefix(a, "--") {
			switch a {
			case "--full-time":
				opts.fullTime = true
			case "--group-directories-first":
				opts.dirFirst = true
			default:
				fmt.Fprintf(os.Stderr, "ls: unrecognized option '%s'\n", a)
			}
			continue
		}
		for _, ch := range a[1:] {
			switch ch {
			case 'a':
				opts.all = true
			case 'A':
				opts.almostAll = true
			case 'l':
				opts.long = true
			case 'h':
				opts.human = true
			case 'R':
				opts.recursive = true
			case 'd':
				opts.dirs = true
			case '1':
				opts.one = true
			case 'C':
				opts.columns = true
			case 'x':
				opts.byLines = true
			case 'S':
				opts.sortSize = true
			case 'X':
				opts.sortExt = true
			case 't':
				opts.sortTime = true
			case 'c':
				opts.sortCTime = true
			case 'u':
				opts.sortATime = true
			case 'r':
				opts.reverse = true
			case 'F':
				opts.class = true
			case 'p':
				opts.dirSlash = true
			case 'i':
				opts.inode = true
			case 'n':
				opts.numeric = true
			case 'g':
				opts.noOwner = true
				opts.long = true
			case 's':
				opts.blocks = true
			case 'Q':
				opts.quote = true
			case 'L':
				opts.followL = true
			case 'q', 'k', 'T':
				// ignored (compatibility)
			default:
				fmt.Fprintf(os.Stderr, "ls: invalid option -- '%c'\n", ch)
				return 1
			}
		}
	}
	paths = args[i:]
	if len(paths) == 0 {
		paths = []string{"."}
	}

	exitCode := 0
	if len(paths) == 1 {
		if err := lsDir(paths[0], opts, false); err != nil {
			fmt.Fprintf(os.Stderr, "ls: %s: %v\n", paths[0], err)
			exitCode = 1
		}
	} else {
		sort.Strings(paths)
		first := true
		for _, p := range paths {
			info, err := os.Stat(p)
			if err != nil {
				fmt.Fprintf(os.Stderr, "ls: %s: %v\n", p, err)
				exitCode = 1
				continue
			}
			if info.IsDir() {
				if !first {
					fmt.Println()
				}
				fmt.Printf("%s:\n", p)
				if err := lsDir(p, opts, false); err != nil {
					fmt.Fprintf(os.Stderr, "ls: %s: %v\n", p, err)
					exitCode = 1
				}
				first = false
			} else {
				lsFile(info, p, opts)
			}
		}
	}
	return exitCode
}

func lsDir(dir string, opts lsOpts, header bool) error {
	if opts.dirs {
		info, err := os.Stat(dir)
		if err != nil {
			return err
		}
		lsFile(info, dir, opts)
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	type entry struct {
		name  string
		info  os.FileInfo
		isDir bool
	}

	items := []entry{}
	for _, e := range entries {
		if !opts.all && strings.HasPrefix(e.Name(), ".") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		items = append(items, entry{e.Name(), info, e.IsDir()})
	}

	// Sort
	if opts.sortSize {
		sort.Slice(items, func(i, j int) bool {
			if opts.reverse {
				return items[i].info.Size() > items[j].info.Size()
			}
			return items[i].info.Size() < items[j].info.Size()
		})
	} else if opts.sortTime {
		sort.Slice(items, func(i, j int) bool {
			if opts.reverse {
				return items[i].info.ModTime().Before(items[j].info.ModTime())
			}
			return items[j].info.ModTime().Before(items[i].info.ModTime())
		})
	} else {
		sort.Slice(items, func(i, j int) bool {
			if opts.reverse {
				return items[i].name > items[j].name
			}
			return items[i].name < items[j].name
		})
	}

	if opts.long || opts.one {
		for _, it := range items {
			lsFile(it.info, it.name, opts)
		}
	} else {
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		for _, it := range items {
			name := it.name
			if opts.class {
				if it.isDir {
					name += "/"
				} else if it.info.Mode()&0111 != 0 {
					name += "*"
				}
			}
			if opts.inode {
				fmt.Fprintf(w, "%d\t", inodeOf(it.info))
			}
			fmt.Fprintf(w, "%s  ", name)
		}
		fmt.Fprintln(w)
		w.Flush()
	}

	if opts.recursive {
		for _, it := range items {
			if it.isDir {
				subpath := filepath.Join(dir, it.name)
				fmt.Printf("\n%s:\n", subpath)
				lsDir(subpath, opts, true)
			}
		}
	}

	return nil
}

func lsFile(info os.FileInfo, name string, opts lsOpts) {
	if opts.inode && !opts.long {
		fmt.Printf("%d\t", inodeOf(info))
	}
	if opts.long {
		fmt.Printf("%s %4d %s %s %8s %s ",
			info.Mode().String(),
			1, // link count
			ownerName(info),
			groupName(info),
			formatSize(info.Size(), opts.human),
			info.ModTime().Format("Jan _2 15:04"),
		)
	}
	dispName := name
	if opts.class {
		if info.IsDir() {
			dispName += "/"
		} else if info.Mode()&0111 != 0 {
			dispName += "*"
		}
	}
	if opts.inode && opts.long {
		fmt.Printf("%d ", inodeOf(info))
	}
	fmt.Println(dispName)
}

func formatSize(size int64, human bool) string {
	if !human {
		return fmt.Sprintf("%d", size)
	}
	units := []string{"B", "K", "M", "G", "T", "P"}
	f := float64(size)
	for _, u := range units {
		if f < 1024 {
			if f >= 10 {
				return fmt.Sprintf("%.0f%s", f, u)
			}
			return fmt.Sprintf("%.1f%s", f, u)
		}
		f /= 1024
	}
	return fmt.Sprintf("%.0fE", f)
}

func ownerName(info os.FileInfo) string {
	_ = info
	return fmt.Sprintf("%d", os.Getuid())
}

func groupName(info os.FileInfo) string {
	_ = info
	return fmt.Sprintf("%d", os.Getgid())
}

func inodeOf(info os.FileInfo) uint64 {
	// cross-platform inode (0 on non-unix)
	return 0
}

func init() {
	// Additional date registration
	applet.Register(&applet.Applet{Name: "date", Short: "Display or set date and time", Func: runDate})
}

func runDate(args []string) int {
	utc := false
	rfc2822 := false
	iso8601 := false
	format := ""

	i := 1
	for ; i < len(args); i++ {
		a := args[i]
		if a == "-u" {
			utc = true
		} else if a == "-R" {
			rfc2822 = true
		} else if strings.HasPrefix(a, "-I") {
			iso8601 = true
		} else if strings.HasPrefix(a, "+") {
			format = a[1:]
		} else if a == "--" {
			i++
			break
		} else if !strings.HasPrefix(a, "-") {
			break
		}
	}

	now := time.Now()
	if utc {
		now = now.UTC()
	}

	if rfc2822 {
		fmt.Println(now.Format(time.RFC1123Z))
		return 0
	}
	if iso8601 {
		fmt.Println(now.Format("2006-01-02"))
		return 0
	}
	if format != "" {
		fmt.Println(formatDate(now, format))
		return 0
	}
	fmt.Println(now.Format("Mon Jan _2 15:04:05 MST 2006"))
	return 0
}

func formatDate(t time.Time, format string) string {
	r := strings.NewReplacer(
		"%Y", t.Format("2006"),
		"%m", t.Format("01"),
		"%d", t.Format("02"),
		"%H", t.Format("15"),
		"%M", t.Format("04"),
		"%S", t.Format("05"),
		"%Z", t.Format("MST"),
		"%A", t.Format("Monday"),
		"%a", t.Format("Mon"),
		"%B", t.Format("January"),
		"%b", t.Format("Jan"),
		"%e", t.Format("_2"),
		"%T", t.Format("15:04:05"),
		"%F", t.Format("2006-01-02"),
		"%s", fmt.Sprintf("%d", t.Unix()),
	)
	return r.Replace(format)
}

func init() {
	applet.Register(&applet.Applet{Name: "seq", Short: "Print sequences of numbers", Func: runSeq})
}

func runSeq(args []string) int {
	args = args[1:]
	sep := "\n"
	width := false
	format := ""

	// Parse flags
	cleaned := []string{}
	for i := 0; i < len(args); i++ {
		if args[i] == "-s" && i+1 < len(args) {
			sep = args[i+1]
			i++
		} else if args[i] == "-w" {
			width = true
		} else if args[i] == "-f" && i+1 < len(args) {
			format = args[i+1]
			i++
		} else {
			cleaned = append(cleaned, args[i])
		}
	}

	var start, end, step float64
	step = 1
	switch len(cleaned) {
	case 1:
		start = 1
		end = parseFloat(cleaned[0])
	case 2:
		start = parseFloat(cleaned[0])
		end = parseFloat(cleaned[1])
	case 3:
		start = parseFloat(cleaned[0])
		step = parseFloat(cleaned[1])
		end = parseFloat(cleaned[2])
	default:
		fmt.Fprintf(os.Stderr, "seq: missing operand\n")
		return 1
	}

	if step == 0 {
		fmt.Fprintf(os.Stderr, "seq: step must not be zero\n")
		return 1
	}

	maxW := 0
	if width {
		maxW = max(len(fmt.Sprintf("%g", start)), len(fmt.Sprintf("%g", end)))
	}

	first := true
	for {
		if step > 0 && start > end {
			break
		}
		if step < 0 && start < end {
			break
		}
		if !first {
			fmt.Print(sep)
		}
		first = false
		if format != "" {
			fmt.Printf(format, start)
		} else if width {
			fmt.Printf("%*g", maxW, start)
		} else {
			fmt.Printf("%g", start)
		}
		start += step
	}
	if !first {
		fmt.Println()
	}
	return 0
}

func parseFloat(s string) float64 {
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func init() {
	applet.Register(&applet.Applet{Name: "yes", Short: "Output a string repeatedly", Func: runYes})
}

func runYes(args []string) int {
	s := "y"
	if len(args) > 1 {
		s = strings.Join(args[1:], " ")
	}
	for {
		fmt.Println(s)
	}
}

func init() {
	applet.Register(&applet.Applet{Name: "true", Short: "Exit with success status", Func: runTrue})
	applet.Register(&applet.Applet{Name: "false", Short: "Exit with failure status", Func: runFalse})
}

func runTrue(args []string) int  { return 0 }
func runFalse(args []string) int { return 1 }

func init() {
	applet.Register(&applet.Applet{Name: "env", Short: "Display or set environment", Func: runEnv})
}

func runEnv(args []string) int {
	clearEnv := false
	nullDelim := false

	i := 1
	for ; i < len(args); i++ {
		a := args[i]
		if a == "-i" {
			clearEnv = true
		} else if a == "-0" {
			nullDelim = true
		} else if a == "--" {
			i++
			break
		} else if !strings.HasPrefix(a, "-") {
			break
		} else {
			break
		}
	}

	if clearEnv {
		os.Clearenv()
	}

	// Set any VAR=VALUE pairs before command
	j := i
	for ; j < len(args); j++ {
		if idx := strings.IndexByte(args[j], '='); idx >= 0 {
			os.Setenv(args[j][:idx], args[j][idx+1:])
		} else {
			break
		}
	}

	if j >= len(args) {
		// No command, print env
		delim := "\n"
		if nullDelim {
			delim = "\x00"
		}
		for _, e := range os.Environ() {
			fmt.Print(e + delim)
		}
		return 0
	}

	// Run command - simplified, just print args
	fmt.Fprintf(os.Stderr, "env: command execution not yet supported\n")
	return 1
}

func init() {
	applet.Register(&applet.Applet{Name: "hostname", Short: "Print or set hostname", Func: runHostname})
}

func runHostname(args []string) int {
	if len(args) > 1 && !strings.HasPrefix(args[1], "-") {
		fmt.Fprintf(os.Stderr, "hostname: setting hostname not supported\n")
		return 1
	}
	name, err := os.Hostname()
	if err != nil {
		fmt.Fprintf(os.Stderr, "hostname: %v\n", err)
		return 1
	}
	for _, a := range args[1:] {
		if a == "-f" || a == "--fqdn" {
			fmt.Println(name)
			return 0
		}
	}
	fmt.Println(name)
	return 0
}

func init() {
	applet.Register(&applet.Applet{Name: "whoami", Short: "Print effective user name", Func: runWhoami})
}

func runWhoami(args []string) int {
	u := os.Getenv("USER")
	if u == "" {
		u = os.Getenv("LOGNAME")
	}
	if u == "" {
		u = os.Getenv("USERNAME")
	}
	if u == "" {
		fmt.Fprintf(os.Stderr, "whoami: cannot find name for current user\n")
		return 1
	}
	fmt.Println(u)
	return 0
}

func init() {
	applet.Register(&applet.Applet{Name: "tee", Short: "Read stdin and write to files and stdout", Func: runTee})
}

func runTee(args []string) int {
	appendMode := false
	files := []string{}

	for _, a := range args[1:] {
		if a == "-a" || a == "--append" {
			appendMode = true
		} else if a == "--" {
			continue
		} else if !strings.HasPrefix(a, "-") {
			files = append(files, a)
		}
	}

	writers := []io.Writer{os.Stdout}
	for _, fname := range files {
		flag := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
		if appendMode {
			flag = os.O_WRONLY | os.O_CREATE | os.O_APPEND
		}
		f, err := os.OpenFile(fname, flag, 0666)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tee: %s: %v\n", fname, err)
			return 1
		}
		defer f.Close()
		writers = append(writers, f)
	}

	mw := io.MultiWriter(writers...)
	if _, err := io.Copy(mw, os.Stdin); err != nil {
		return 1
	}
	return 0
}

func init() {
	applet.Register(&applet.Applet{Name: "rev", Short: "Reverse lines of a file", Func: runRev})
}

func runRev(args []string) int {
	files := args[1:]
	if len(files) == 0 {
		files = []string{"-"}
	}

	for _, fname := range files {
		var f *os.File
		if fname == "-" {
			f = os.Stdin
		} else {
			var err error
			f, err = os.Open(fname)
			if err != nil {
				fmt.Fprintf(os.Stderr, "rev: %s: %v\n", fname, err)
				return 1
			}
			defer f.Close()
		}

		data, err := io.ReadAll(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "rev: %s: %v\n", fname, err)
			return 1
		}

		for _, line := range strings.Split(string(data), "\n") {
			runes := []rune(line)
			for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
				runes[i], runes[j] = runes[j], runes[i]
			}
			fmt.Println(string(runes))
		}
	}
	return 0
}
