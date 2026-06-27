package textproc

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/agentbusybox/pkg/applet"
)

func init() {
	applet.Register(&applet.Applet{Name: "grep", Short: "Print lines matching a pattern", Func: runGrep})
}

func RunGrep(args []string) int {
	return runGrep(args)
}

func runGrep(args []string) int {
	// Flags matching BusyBox grep
	ignoreCase := false    // -i
	invert := false        // -v
	count := false         // -c
	list := false          // -l
	lineNum := false       // -n
	quiet := false         // -q
	fixed := false         // -F
	patterns := []string{} // -e PAT -f FILE
	matchOnly := false     // -o
	word := false          // -w
	line := false          // -x
	maxCount := 0          // -m N
	addFilename := 0       // -H (1=always, -1=never, 0=auto)
	recursive := false     // -r/-R
	suppressErr := false   // -s
	contextAfter := 0      // -A N
	contextBefore := 0     // -B N
	contextBoth := 0       // -C N
	inclPat := ""          // --include
	exclPat := ""          // --exclude
	files := []string{}

	i := 1
	for ; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			i++
			break
		}
		if !strings.HasPrefix(a, "-") {
			if len(patterns) == 0 {
				patterns = append(patterns, a)
			} else {
				files = append(files, a)
			}
			continue
		}
		// Long options
		if strings.HasPrefix(a, "--") {
			switch a {
			case "--fixed-strings":
				fixed = true
			case "--extended-regexp": // always supported in Go
			case "--ignore-case":
				ignoreCase = true
			case "--invert-match":
				invert = true
			case "--count":
				count = true
			case "--files-with-matches":
				list = true
			case "--line-number":
				lineNum = true
			case "--quiet", "--silent":
				quiet = true
			case "--only-matching":
				matchOnly = true
			case "--word-regexp":
				word = true
			case "--line-regexp":
				line = true
			case "--no-filename":
				addFilename = -1
			case "--with-filename":
				addFilename = 1
			case "--recursive":
				recursive = true
			case "--no-messages":
				suppressErr = true
			default:
				if strings.HasPrefix(a, "--include=") {
					inclPat = a[10:]
					continue
				}
				if strings.HasPrefix(a, "--exclude=") {
					exclPat = a[10:]
					continue
				}
				if strings.HasPrefix(a, "--max-count=") {
					fmt.Sscanf(a[12:], "%d", &maxCount)
					continue
				}
				if strings.HasPrefix(a, "--context=") {
					fmt.Sscanf(a[10:], "%d", &contextBoth)
					continue
				}
			}
			continue
		}
		// Short flags
		for _, ch := range a[1:] {
			switch ch {
			case 'i':
				ignoreCase = true
			case 'v':
				invert = true
			case 'c':
				count = true
			case 'l':
				list = true
			case 'n':
				lineNum = true
			case 'q':
				quiet = true
			case 'F':
				fixed = true
			case 'E': // always supported in Go
			case 'o':
				matchOnly = true
			case 'w':
				word = true
			case 'x':
				line = true
			case 'H':
				addFilename = 1
			case 'h':
				addFilename = -1
			case 'r', 'R':
				recursive = true
			case 's':
				suppressErr = true
			case 'a': // ignored (assume text)
			case 'I': // ignored (assume binary)
			case 'e':
				if i+1 < len(args) {
					i++
					patterns = append(patterns, args[i])
				}
			case 'f':
				if i+1 < len(args) {
					i++
					data, err := os.ReadFile(args[i])
					if err != nil {
						if !suppressErr {
							fmt.Fprintf(os.Stderr, "grep: %s: %v\n", args[i], err)
						}
						return 2
					}
					for _, l := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
						patterns = append(patterns, l)
					}
				}
			case 'm':
				if i+1 < len(args) {
					i++
					fmt.Sscanf(args[i], "%d", &maxCount)
				}
			case 'A':
				if i+1 < len(args) {
					i++
					fmt.Sscanf(args[i], "%d", &contextAfter)
				}
			case 'B':
				if i+1 < len(args) {
					i++
					fmt.Sscanf(args[i], "%d", &contextBefore)
				}
			case 'C':
				if i+1 < len(args) {
					i++
					fmt.Sscanf(args[i], "%d", &contextBoth)
				}
			default:
				// ignore unknown
			}
		}
	}
	files = append(files, args[i:]...)

	if len(patterns) == 0 {
		fmt.Fprintf(os.Stderr, "grep: no pattern specified\n")
		return 2
	}

	if len(files) == 0 {
		if recursive {
			files = []string{"."}
		} else {
			files = []string{"-"}
		}
	}

	// Build compiled patterns
	type compiledPat struct {
		pat string
		re  *regexp.Regexp
	}
	cpats := []compiledPat{}
	for _, pat := range patterns {
		if fixed {
			cpats = append(cpats, compiledPat{pat: pat})
		} else {
			p := pat
			if word {
				p = "\\b(?:" + pat + ")\\b"
			}
			if line {
				p = "^(?:" + pat + ")$"
			}
			flags := ""
			if ignoreCase {
				flags = "(?i)"
			}
			re, err := regexp.Compile(flags + p)
			if err != nil {
				if !suppressErr {
					fmt.Fprintf(os.Stderr, "grep: invalid pattern: %v\n", err)
				}
				return 2
			}
			cpats = append(cpats, compiledPat{pat: pat, re: re})
		}
	}

	showFilename := addFilename
	if showFilename == 0 && (len(files) > 1 || recursive) {
		showFilename = 1
	}

	exitCode := 1

	processReader := func(r io.Reader, fname string) {
		scanner := bufio.NewScanner(r)
		n := 0
		matchFile := false
		fileMatchCount := 0

		for scanner.Scan() {
			ln := scanner.Text()
			n++
			matched := false
			for _, cp := range cpats {
				if fixed {
					if ignoreCase {
						if strings.Contains(strings.ToLower(ln), strings.ToLower(cp.pat)) {
							matched = true
							break
						}
					} else {
						if strings.Contains(ln, cp.pat) {
							matched = true
							break
						}
					}
				} else {
					if cp.re.MatchString(ln) {
						matched = true
						break
					}
				}
			}
			if invert {
				matched = !matched
			}

			if matched {
				matchFile = true
				fileMatchCount++
				exitCode = 0
				if maxCount > 0 && fileMatchCount > maxCount {
					break
				}
				if quiet || count || list {
					continue
				}

				prefix := ""
				if showFilename > 0 {
					prefix = fname + ":"
				}
				if matchOnly {
					for _, cp := range cpats {
						if fixed {
							if strings.Contains(ln, cp.pat) {
								fmt.Printf("%s%s\n", prefix, cp.pat)
							}
						} else {
							for _, m := range cp.re.FindAllString(ln, -1) {
								fmt.Printf("%s%s\n", prefix, m)
							}
						}
					}
				} else {
					if lineNum {
						fmt.Printf("%s%d:%s\n", prefix, n, ln)
					} else {
						fmt.Printf("%s%s\n", prefix, ln)
					}
				}
			}
		}

		if count && !quiet {
			if showFilename > 0 {
				fmt.Printf("%s:%d\n", fname, fileMatchCount)
			} else {
				fmt.Println(fileMatchCount)
			}
		}
		if list && matchFile {
			fmt.Println(fname)
		}
	}

	for _, fname := range files {
		if recursive && fname != "-" {
			info, err := os.Stat(fname)
			if err != nil {
				if !suppressErr {
					fmt.Fprintf(os.Stderr, "grep: %s: %v\n", fname, err)
				}
				continue
			}
			if info.IsDir() {
				filepath.Walk(fname, func(path string, fi os.FileInfo, err error) error {
					if err != nil || fi.IsDir() {
						return nil
					}
					if inclPat != "" {
						if m, _ := filepath.Match(inclPat, fi.Name()); !m {
							return nil
						}
					}
					if exclPat != "" {
						if m, _ := filepath.Match(exclPat, fi.Name()); m {
							return nil
						}
					}
					f, err := os.Open(path)
					if err != nil {
						return nil
					}
					defer f.Close()
					processReader(f, path)
					return nil
				})
				continue
			}
		}
		var r io.Reader
		if fname == "-" {
			r = os.Stdin
		} else {
			f, err := os.Open(fname)
			if err != nil {
				if !suppressErr {
					fmt.Fprintf(os.Stderr, "grep: %s: %v\n", fname, err)
				}
				continue
			}
			defer f.Close()
			r = f
		}
		processReader(r, fname)
	}
	return exitCode
}

func init() {
	applet.Register(&applet.Applet{Name: "sed", Short: "Stream editor", Func: runSed})
}

func runSed(args []string) int {
	quiet := false     // -n
	inplace := false   // -i
	backupSuffix := "" // -iSFX
	files := []string{}
	expressions := []string{}

	for i := 1; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			i++
			files = append(files, args[i:]...)
			break
		}
		if strings.HasPrefix(a, "--") {
			switch a {
			case "--quiet":
				quiet = true
			case "--in-place":
				inplace = true
			default:
				if strings.HasPrefix(a, "--in-place=") {
					inplace = true
					backupSuffix = a[11:]
				}
			}
			continue
		}
		if strings.HasPrefix(a, "-") && len(a) > 1 {
			for j, ch := range a[1:] {
				switch ch {
				case 'n':
					quiet = true
				case 'i':
					inplace = true
					// Check for suffix: -iSFX or -i SFX
					rest := a[j+2:]
					if rest != "" {
						backupSuffix = rest
					} else if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
						// Only consume next arg if it looks like a suffix (not a flag)
						// BusyBox: -i without suffix = no backup
					}
				case 'r', 'E': // extended regex (always supported in Go)
				case 'e':
					rest := a[j+2:]
					if rest != "" {
						expressions = append(expressions, rest)
					} else if i+1 < len(args) {
						i++
						expressions = append(expressions, args[i])
					}
				case 'f':
					rest := a[j+2:]
					if rest == "" && i+1 < len(args) {
						i++
						rest = args[i]
					}
					if rest != "" {
						data, err := os.ReadFile(rest)
						if err != nil {
							fmt.Fprintf(os.Stderr, "sed: %s: %v\n", rest, err)
							return 1
						}
						for _, l := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
							expressions = append(expressions, l)
						}
					}
				default:
					// unknown flag, ignore
				}
			}
			continue
		}
		// Non-flag argument: first is expression, rest are files
		if len(expressions) == 0 {
			expressions = append(expressions, a)
		} else {
			files = append(files, a)
		}
	}
	if len(files) == 0 {
		files = []string{"-"}
	}

	for _, fname := range files {
		var lines []string
		if fname == "-" {
			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				lines = append(lines, scanner.Text())
			}
		} else {
			data, err := os.ReadFile(fname)
			if err != nil {
				fmt.Fprintf(os.Stderr, "sed: %s: %v\n", fname, err)
				return 1
			}
			lines = strings.Split(string(data), "\n")
			// Remove trailing empty line if file ended with newline
			if len(lines) > 0 && lines[len(lines)-1] == "" {
				lines = lines[:len(lines)-1]
			}
		}

		for i, line := range lines {
			output := line
			for _, expr := range expressions {
				output = applySedExprN(output, expr, i+1, len(lines))
			}
			lines[i] = output
		}

		if !quiet {
			for _, l := range lines {
				fmt.Println(l)
			}
		}

		if inplace && fname != "-" {
			backupName := fname + backupSuffix
			if backupSuffix != "" {
				os.Rename(fname, backupName)
			}
			os.WriteFile(fname, []byte(strings.Join(lines, "\n")+"\n"), 0644)
		}
	}
	return 0
}

func applySedExpr(line, expr string) string {
	return applySedExprN(line, expr, 0, 0)
}

func applySedExprN(line, expr string, lineNum, totalLines int) string {
	// Parse address + command
	cmd := expr
	addr := ""

	// Extract address prefix
	if len(expr) > 0 && (expr[0] >= '0' && expr[0] <= '9') {
		// Numeric address: 2p, 2d, etc.
		i := 0
		for i < len(expr) && expr[i] >= '0' && expr[i] <= '9' {
			i++
		}
		addr = expr[:i]
		cmd = expr[i:]
	} else if len(expr) > 0 && expr[0] == '/' {
		// Pattern address: /pattern/cmd
		end := strings.IndexByte(expr[1:], '/')
		if end >= 0 {
			addr = expr[1 : end+1]
			cmd = expr[end+2:]
		}
	} else if len(expr) > 1 && expr[0] == '$' {
		// Last line
		addr = "$"
		cmd = expr[1:]
	}

	// Check address
	if addr != "" {
		matched := false
		if addr == "$" {
			matched = lineNum == totalLines
		} else if n := 0; len(addr) > 0 {
			fmt.Sscanf(addr, "%d", &n)
			matched = lineNum == n
		} else {
			// Pattern match
			matched = strings.Contains(line, addr)
		}
		if !matched {
			// Address doesn't match, return line unchanged
			return line
		}
	}

	// Execute command
	if strings.HasPrefix(cmd, "s") && len(cmd) > 4 {
		delim := cmd[1]
		parts := strings.Split(cmd[1:], string(delim))
		if len(parts) >= 3 {
			old := parts[1]
			new_ := parts[2]
			global := len(parts) > 3 && strings.Contains(parts[3], "g")
			if global {
				return strings.ReplaceAll(line, old, new_)
			}
			return strings.Replace(line, old, new_, 1)
		}
	}
	if cmd == "d" {
		return ""
	}
	if cmd == "p" {
		fmt.Println(line)
	}
	return line
}

func init() {
	applet.Register(&applet.Applet{Name: "strings", Short: "Print printable strings from files", Func: runStrings})
}

func runStrings(args []string) int {
	minLen := 4
	printName := false    // -f
	printOffset := false  // -o
	radix := "o"          // -t o|d|x
	files := []string{}

	i := 1
	for ; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			i++
			break
		}
		if !strings.HasPrefix(a, "-") || len(a) == 1 {
			files = append(files, a)
			continue
		}
		// Handle combined flags like -fo, -n4, -tx
		j := 1
		for j < len(a) {
			switch a[j] {
			case 'a': // scan whole file (default)
				j++
			case 'f':
				printName = true
				j++
			case 'o':
				printOffset = true
				j++
			case 'n':
				j++
				if j < len(a) {
					// -nLEN attached
					fmt.Sscanf(a[j:], "%d", &minLen)
					j = len(a)
				} else if i+1 < len(args) {
					// -n LEN separate
					i++
					fmt.Sscanf(args[i], "%d", &minLen)
				}
			case 't':
				printOffset = true
				j++
				if j < len(a) {
					// -tFORMAT attached
					radix = a[j:]
					j = len(a)
				} else if i+1 < len(args) {
					// -t FORMAT separate
					i++
					radix = args[i]
				}
			default:
				j++
			}
		}
	}
	files = append(files, args[i:]...)

	if len(files) == 0 {
		files = []string{"-"}
	}

	if radix != "o" && radix != "d" && radix != "x" {
		fmt.Fprintf(os.Stderr, "strings: invalid radix '%s'\n", radix)
		return 1
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
			fmt.Fprintf(os.Stderr, "strings: %s: %v\n", fname, err)
			return 1
		}

		displayName := fname
		if fname == "-" {
			displayName = "{standard input}"
		}

		current := ""
		offset := int64(0)
		startOffset := int64(0)
		for _, b := range data {
			if b >= 32 && b < 127 {
				if len(current) == 0 {
					startOffset = offset
				}
				current += string(rune(b))
			} else {
				if len(current) >= minLen {
					if printName {
						fmt.Printf("%s: ", displayName)
					}
					if printOffset {
						switch radix {
						case "o":
							fmt.Printf("%7o ", startOffset)
						case "x":
							fmt.Printf("%7x ", startOffset)
						case "d":
							fmt.Printf("%7d ", startOffset)
						}
					}
					fmt.Println(current)
				}
				current = ""
			}
			offset++
		}
		if len(current) >= minLen {
			if printName {
				fmt.Printf("%s: ", displayName)
			}
			if printOffset {
				switch radix {
				case "o":
					fmt.Printf("%7o ", startOffset)
				case "x":
					fmt.Printf("%7x ", startOffset)
				case "d":
					fmt.Printf("%7d ", startOffset)
				}
			}
			fmt.Println(current)
		}
	}
	return 0
}

func init() {
	applet.Register(&applet.Applet{Name: "diff", Short: "Compare files line by line", Func: runDiff})
}

func runDiff(args []string) int {
	// Simplified diff - unified format
	files := []string{}
	for _, a := range args[1:] {
		if a == "-u" {
			continue
		}
		if !strings.HasPrefix(a, "-") {
			files = append(files, a)
		}
	}

	if len(files) != 2 {
		fmt.Fprintf(os.Stderr, "diff: missing operand\n")
		return 1
	}

	data1, err := os.ReadFile(files[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "diff: %s: %v\n", files[0], err)
		return 1
	}
	data2, err := os.ReadFile(files[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "diff: %s: %v\n", files[1], err)
		return 1
	}

	lines1 := strings.Split(string(data1), "\n")
	lines2 := strings.Split(string(data2), "\n")

	// Simple LCS-based diff
	lcs := lcsLines(lines1, lines2)
	i, j, k := 0, 0, 0
	exitCode := 0

	for k < len(lcs) {
		for i < len(lines1) && lines1[i] != lcs[k] {
			fmt.Printf("- %s\n", lines1[i])
			i++
			exitCode = 1
		}
		for j < len(lines2) && lines2[j] != lcs[k] {
			fmt.Printf("+ %s\n", lines2[j])
			j++
			exitCode = 1
		}
		i++
		j++
		k++
	}
	for ; i < len(lines1); i++ {
		fmt.Printf("- %s\n", lines1[i])
		exitCode = 1
	}
	for ; j < len(lines2); j++ {
		fmt.Printf("+ %s\n", lines2[j])
		exitCode = 1
	}
	return exitCode
}

func lcsLines(a, b []string) []string {
	m, n := len(a), len(b)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else {
				if dp[i-1][j] > dp[i][j-1] {
					dp[i][j] = dp[i-1][j]
				} else {
					dp[i][j] = dp[i][j-1]
				}
			}
		}
	}
	result := []string{}
	i, j := m, n
	for i > 0 && j > 0 {
		if a[i-1] == b[j-1] {
			result = append([]string{a[i-1]}, result...)
			i--
			j--
		} else if dp[i-1][j] > dp[i][j-1] {
			i--
		} else {
			j--
		}
	}
	return result
}

func init() {
	applet.Register(&applet.Applet{Name: "cmp", Short: "Compare two files byte by byte", Func: runCmp})
}

func runCmp(args []string) int {
	verbose := false   // -l
	silent := false    // -s
	maxBytes := int64(0) // -n COUNT
	files := []string{}

	i := 1
	for ; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			i++
			break
		}
		if a == "-l" || a == "--verbose" {
			verbose = true
			continue
		}
		if a == "-s" || a == "--quiet" || a == "--silent" {
			silent = true
			continue
		}
		if strings.HasPrefix(a, "-n") {
			if len(a) > 2 {
				fmt.Sscanf(a[2:], "%d", &maxBytes)
			} else if i+1 < len(args) {
				i++
				fmt.Sscanf(args[i], "%d", &maxBytes)
			}
			continue
		}
		if !strings.HasPrefix(a, "-") {
			files = append(files, a)
		}
	}
	files = append(files, args[i:]...)

	if len(files) != 2 {
		if !silent {
			fmt.Fprintf(os.Stderr, "cmp: missing operand\n")
		}
		return 1
	}

	f1, err := os.Open(files[0])
	if err != nil {
		if !silent {
			fmt.Fprintf(os.Stderr, "cmp: %s: %v\n", files[0], err)
		}
		return 1
	}
	defer f1.Close()
	f2, err := os.Open(files[1])
	if err != nil {
		if !silent {
			fmt.Fprintf(os.Stderr, "cmp: %s: %v\n", files[1], err)
		}
		return 1
	}
	defer f2.Close()

	buf1 := make([]byte, 8192)
	buf2 := make([]byte, 8192)
	offset := int64(0)
	lineNum := 1

	for {
		n1, err1 := f1.Read(buf1)
		n2, err2 := f2.Read(buf2)

		minN := n1
		if n2 < minN {
			minN = n2
		}
		if maxBytes > 0 && offset+int64(minN) > maxBytes {
			minN = int(maxBytes - offset)
			if minN < 0 {
				minN = 0
			}
		}

		for i := 0; i < minN; i++ {
			if buf1[i] == '\n' {
				lineNum++
			}
			if buf1[i] != buf2[i] {
				if silent {
					return 1
				}
				if verbose {
					fmt.Printf("%d %o %o\n", offset+int64(i)+1, buf1[i], buf2[i])
				} else {
					fmt.Printf("%s %s differ: byte %d, line %d\n", files[0], files[1], offset+int64(i)+1, lineNum)
				}
				return 1
			}
		}

		if maxBytes > 0 && offset+int64(minN) >= maxBytes {
			return 0
		}

		offset += int64(minN)

		if n1 != n2 {
			// Different number of bytes read - files differ in length
			if !silent {
				fmt.Fprintf(os.Stderr, "cmp: EOF on %s\n", files[0])
			}
			return 1
		}

		if err1 == io.EOF && err2 == io.EOF {
			break
		}
	}
	return 0
}

func init() {
	applet.Register(&applet.Applet{Name: "xargs", Short: "Build and execute command lines from stdin", Func: runXargs})
}

func runXargs(args []string) int {
	maxArgs := 0       // -n NUM
	maxChars := 0       // -s NUM
	verbose := false    // -t
	nullTerm := false   // -0
	replStr := ""       // -I STR
	eofStr := ""        // -E STR
	delimiter := ""     // -d DELIM
	command := ""
	commandArgs := []string{}

	i := 1
	for ; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			i++
			break
		}
		if !strings.HasPrefix(a, "-") {
			break
		}
		switch a {
		case "-t":
			verbose = true
		case "-0":
			nullTerm = true
		case "-x":
			// exit if size exceeded (handled implicitly)
		case "-p":
			// interactive confirmation (simplified: just print)
			verbose = true
		default:
			if strings.HasPrefix(a, "-n") {
				if len(a) > 2 {
					fmt.Sscanf(a[2:], "%d", &maxArgs)
				} else if i+1 < len(args) {
					i++
					fmt.Sscanf(args[i], "%d", &maxArgs)
				}
			} else if strings.HasPrefix(a, "-s") {
				if len(a) > 2 {
					fmt.Sscanf(a[2:], "%d", &maxChars)
				} else if i+1 < len(args) {
					i++
					fmt.Sscanf(args[i], "%d", &maxChars)
				}
			} else if strings.HasPrefix(a, "-I") {
				if len(a) > 2 {
					replStr = a[2:]
				} else if i+1 < len(args) {
					i++
					replStr = args[i]
				}
			} else if strings.HasPrefix(a, "-E") {
				if len(a) > 2 {
					eofStr = a[2:]
				} else if i+1 < len(args) {
					i++
					eofStr = args[i]
				}
			} else if strings.HasPrefix(a, "-d") {
				if len(a) > 2 {
					delimiter = a[2:]
				} else if i+1 < len(args) {
					i++
					delimiter = args[i]
				}
			}
		}
	}

	if i >= len(args) {
		fmt.Fprintf(os.Stderr, "xargs: no command specified\n")
		return 1
	}
	command = args[i]
	commandArgs = args[i+1:]

	if maxChars == 0 {
		maxChars = 131072
	}

	// Read items from stdin
	var items []string
	if nullTerm {
		// Read NUL-delimited input
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return 1
		}
		items = strings.Split(string(data), "\x00")
		// Remove trailing empty item
		if len(items) > 0 && items[len(items)-1] == "" {
			items = items[:len(items)-1]
		}
	} else if delimiter != "" {
		d := rune(delimiter[0])
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			line := scanner.Text()
			for _, part := range strings.Split(line, string(d)) {
				if part != "" {
					items = append(items, part)
				}
			}
		}
	} else {
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			// Check for EOF string
			if eofStr != "" && line == eofStr {
				break
			}
			items = append(items, strings.Fields(line)...)
		}
	}

	if len(items) == 0 && replStr == "" {
		// No items, execute command once with no extra args
		if verbose {
			fmt.Fprintf(os.Stderr, "%s\n", command)
		}
		cmd := exec.Command(command)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil {
			return 123
		}
		return 0
	}

	exitCode := 0

	if replStr != "" {
		// -I replacement mode: one invocation per item
		for _, item := range items {
			fullArgs := make([]string, len(commandArgs))
			copy(fullArgs, commandArgs)
			for j, arg := range fullArgs {
				fullArgs[j] = strings.ReplaceAll(arg, replStr, item)
			}
			if verbose {
				fmt.Fprintf(os.Stderr, "%s %s\n", command, strings.Join(fullArgs, " "))
			}
			cmd := exec.Command(command, fullArgs...)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			err := cmd.Run()
			if err != nil {
				exitCode = 123
			}
		}
	} else {
		// Batch mode
		if maxArgs <= 0 {
			maxArgs = len(items)
		}
		if maxArgs <= 0 {
			maxArgs = 1
		}

		for start := 0; start < len(items); start += maxArgs {
			end := start + maxArgs
			if end > len(items) {
				end = len(items)
			}
			batch := items[start:end]

			cmdArgs := make([]string, len(commandArgs)+len(batch))
			copy(cmdArgs, commandArgs)
			copy(cmdArgs[len(commandArgs):], batch)

			// Check total size
			totalSize := len(command)
			for _, a := range cmdArgs {
				totalSize += len(a) + 1
			}
			if totalSize > maxChars {
				fmt.Fprintf(os.Stderr, "xargs: argument line too long\n")
				return 1
			}

			if verbose {
				fmt.Fprintf(os.Stderr, "%s %s\n", command, strings.Join(cmdArgs, " "))
			}
			cmd := exec.Command(command, cmdArgs...)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			err := cmd.Run()
			if err != nil {
				exitCode = 123
			}
		}
	}
	return exitCode
}
