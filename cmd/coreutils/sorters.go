package coreutils

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agentbusybox/pkg/applet"
)

func init() {
	applet.Register(&applet.Applet{Name: "sort", Short: "Sort lines of text files", Func: runSort})
}

func runSort(args []string) int {
	numeric := false           // -n
	reverse := false           // -r
	unique := false            // -u
	foldCase := false          // -f
	humanNum := false          // -h
	random := false            // -R
	versionSort := false       // -V
	generalNum := false        // -g
	monthSort := false         // -M
	check := false             // -c
	stable := false            // -s
	nulTerm := false           // -z
	ignoreBlanks := false      // -b
	dictOrder := false         // -d
	ignoreUnprintable := false // -i
	outputFile := ""           // -o FILE
	key := ""
	delimiter := "\t"
	files := []string{}

	i := 1
	for ; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			i++
			break
		}
		if !strings.HasPrefix(a, "-") {
			files = append(files, a)
			continue
		}
		// Long options
		if strings.HasPrefix(a, "--") {
			switch a {
			case "--numeric-sort":
				numeric = true
			case "--reverse":
				reverse = true
			case "--unique":
				unique = true
			case "--ignore-case":
				foldCase = true
			case "--human-numeric-sort":
				humanNum = true
			case "--random-sort":
				random = true
			case "--version-sort":
				versionSort = true
			case "--general-numeric-sort":
				generalNum = true
			case "--month-sort":
				monthSort = true
			case "--check":
				check = true
			case "--stable":
				stable = true
			case "--zero-terminated":
				nulTerm = true
			case "--ignore-leading-blanks":
				ignoreBlanks = true
			case "--dictionary-order":
				dictOrder = true
			case "--ignore-nonprinting":
				ignoreUnprintable = true
			default:
				if strings.HasPrefix(a, "--key=") {
					key = a[6:]
				}
				if strings.HasPrefix(a, "--field-separator=") {
					delimiter = a[18:]
				}
				if strings.HasPrefix(a, "--output=") {
					outputFile = a[9:]
				}
			}
			continue
		}
		for _, ch := range a[1:] {
			switch ch {
			case 'n':
				numeric = true
			case 'r':
				reverse = true
			case 'u':
				unique = true
			case 'f':
				foldCase = true
			case 'h':
				humanNum = true
			case 'R':
				random = true
			case 'V':
				versionSort = true
			case 'g':
				generalNum = true
			case 'M':
				monthSort = true
			case 'c':
				check = true
			case 's':
				stable = true
			case 'z':
				nulTerm = true
			case 'b':
				ignoreBlanks = true
			case 'd':
				dictOrder = true
			case 'i':
				ignoreUnprintable = true
			case 'o':
				if i+1 < len(args) {
					i++
					outputFile = args[i]
				}
			case 'k':
				if a[2:] != "" {
					key = a[2:]
				} else if i+1 < len(args) {
					i++
					key = args[i]
				}
			case 't':
				if a[2:] != "" {
					delimiter = a[2:]
				} else if i+1 < len(args) {
					i++
					delimiter = args[i]
				}
			case 'T': // ignored (tmpdir)
			case 'S': // ignored (buffer size)
			case 'm': // ignored (merge)
			default:
				// ignore unknown
			}
		}
	}
	files = append(files, args[i:]...)

	// Suppress unused warnings for flags that will be implemented
	_ = generalNum
	_ = monthSort
	_ = check
	_ = stable
	_ = nulTerm
	_ = ignoreBlanks
	_ = dictOrder
	_ = ignoreUnprintable
	_ = outputFile

	if len(files) == 0 {
		files = []string{"-"}
	}

	var allLines []string
	for _, fname := range files {
		var data []byte
		var err error
		if fname == "-" {
			data, err = io.ReadAll(os.Stdin)
		} else {
			data, err = os.ReadFile(fname)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "sort: %s: %v\n", fname, err)
			return 1
		}
		lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
		allLines = append(allLines, lines...)
	}

	// Build comparison function
	less := func(i, j int) bool {
		a, b := allLines[i], allLines[j]

		// Extract key fields
		if key != "" {
			a = extractKey(a, key, delimiter)
			b = extractKey(b, key, delimiter)
		}

		if foldCase {
			a = strings.ToLower(a)
			b = strings.ToLower(b)
		}

		if numeric {
			var na, nb float64
			fmt.Sscanf(a, "%f", &na)
			fmt.Sscanf(b, "%f", &nb)
			if reverse {
				return na > nb
			}
			return na < nb
		}
		if humanNum {
			na := parseHumanNum(a)
			nb := parseHumanNum(b)
			if reverse {
				return na > nb
			}
			return na < nb
		}
		if versionSort {
			if reverse {
				return compareVersion(a, b) > 0
			}
			return compareVersion(a, b) < 0
		}
		if random {
			// simple hash-based random sort
			ha := hashStr(a)
			hb := hashStr(b)
			return ha < hb
		}

		if reverse {
			return a > b
		}
		return a < b
	}

	sort.SliceStable(allLines, less)

	if unique {
		seen := make(map[string]bool)
		for _, line := range allLines {
			k := line
			if foldCase {
				k = strings.ToLower(k)
			}
			if !seen[k] {
				seen[k] = true
				fmt.Println(line)
			}
		}
	} else {
		for _, line := range allLines {
			fmt.Println(line)
		}
	}
	return 0
}

func extractKey(line, key, delim string) string {
	parts := strings.Split(line, delim)
	var fieldNum int
	fmt.Sscanf(key, "%d", &fieldNum)
	if fieldNum >= 1 && fieldNum <= len(parts) {
		return parts[fieldNum-1]
	}
	return line
}

func parseHumanNum(s string) float64 {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return 0
	}
	multiplier := 1.0
	last := s[len(s)-1]
	switch last {
	case 'K', 'k':
		multiplier = 1024
		s = s[:len(s)-1]
	case 'M', 'm':
		multiplier = 1024 * 1024
		s = s[:len(s)-1]
	case 'G', 'g':
		multiplier = 1024 * 1024 * 1024
		s = s[:len(s)-1]
	case 'T', 't':
		multiplier = 1024 * 1024 * 1024 * 1024
		s = s[:len(s)-1]
	}
	var n float64
	fmt.Sscanf(s, "%f", &n)
	return n * multiplier
}

func compareVersion(a, b string) int {
	// Simplified version comparison
	pa := strings.FieldsFunc(a, func(r rune) bool { return r == '.' || r == '-' })
	pb := strings.FieldsFunc(b, func(r rune) bool { return r == '.' || r == '-' })
	for i := 0; i < len(pa) && i < len(pb); i++ {
		var na, nb int
		fmt.Sscanf(pa[i], "%d", &na)
		fmt.Sscanf(pb[i], "%d", &nb)
		if na != nb {
			return na - nb
		}
	}
	return len(pa) - len(pb)
}

func hashStr(s string) uint32 {
	var h uint32
	for _, c := range s {
		h = h*31 + uint32(c)
	}
	return h
}

func init() {
	applet.Register(&applet.Applet{Name: "uniq", Short: "Filter adjacent duplicate lines", Func: runUniq})
}

func runUniq(args []string) int {
	count, repeated, unique, ignoreCase := false, false, false, false
	skipFields := 0
	files := []string{}

	for _, a := range args[1:] {
		if a == "-c" || a == "--count" {
			count = true
			continue
		}
		if a == "-d" || a == "--repeated" {
			repeated = true
			continue
		}
		if a == "-u" || a == "--unique" {
			unique = true
			continue
		}
		if a == "-i" || a == "--ignore-case" {
			ignoreCase = true
			continue
		}
		if strings.HasPrefix(a, "-f") && len(a) > 2 {
			fmt.Sscanf(a[2:], "%d", &skipFields)
			continue
		}
		if a == "--" {
			continue
		}
		if !strings.HasPrefix(a, "-") {
			files = append(files, a)
		}
	}
	if len(files) == 0 {
		files = []string{"-"}
	}

	var data []byte
	var err error
	if files[0] == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(files[0])
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "uniq: %v\n", err)
		return 1
	}

	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) == 0 {
		return 0
	}

	prevKey := skipFieldsKey(lines[0], skipFields, ignoreCase)
	prevLine := lines[0]
	groupCount := 1

	for _, line := range lines[1:] {
		key := skipFieldsKey(line, skipFields, ignoreCase)
		if key == prevKey {
			groupCount++
		} else {
			printUniqLine(prevLine, groupCount, count, repeated, unique)
			prevKey = key
			prevLine = line
			groupCount = 1
		}
	}
	printUniqLine(prevLine, groupCount, count, repeated, unique)
	return 0
}

func skipFieldsKey(line string, skip int, ignoreCase bool) string {
	fields := strings.Fields(line)
	if skip >= len(fields) {
		return ""
	}
	key := strings.Join(fields[skip:], " ")
	if ignoreCase {
		key = strings.ToLower(key)
	}
	return key
}

func printUniqLine(line string, groupCount int, count, repeated, unique bool) {
	show := false
	if repeated && groupCount > 1 {
		show = true
	}
	if unique && groupCount == 1 {
		show = true
	}
	if !repeated && !unique {
		show = true
	}
	if show {
		if count {
			fmt.Printf("%7d %s\n", groupCount, line)
		} else {
			fmt.Println(line)
		}
	}
}

func init() {
	applet.Register(&applet.Applet{Name: "tr", Short: "Translate/squeeze/delete characters", Func: runTr})
}

func runTr(args []string) int {
	delete, squeeze, complement := false, false, false
	a := args[1:]

	// Parse flags
	for len(a) > 0 && strings.HasPrefix(a[0], "-") && a[0] != "--" {
		for _, ch := range a[0][1:] {
			switch ch {
			case 'd':
				delete = true
			case 's':
				squeeze = true
			case 'c':
				complement = true
			}
		}
		a = a[1:]
	}
	if len(a) > 0 && a[0] == "--" {
		a = a[1:]
	}

	if delete && len(a) < 1 {
		fmt.Fprintf(os.Stderr, "tr: missing operand\n")
		return 1
	}
	if !delete && len(a) < 2 {
		fmt.Fprintf(os.Stderr, "tr: missing operand\n")
		return 1
	}

	set1 := expandSet(a[0])
	var set2 []rune
	if !delete {
		set2 = expandSet(a[1])
	}

	// Build translation map
	trans := make(map[rune]rune)
	if delete {
		delSet := make(map[rune]bool)
		for _, r := range set1 {
			delSet[r] = true
		}
		if complement {
			// delete all chars NOT in set1
			allRunes := []rune{}
			for _, line := range readStdinLines() {
				for _, r := range line {
					if !delSet[r] {
						allRunes = append(allRunes, r)
					}
				}
			}
			fmt.Print(string(allRunes))
		} else {
			for _, line := range readStdinLines() {
				for _, r := range line {
					if !delSet[r] {
						fmt.Printf("%c", r)
					}
				}
			}
		}
		return 0
	}

	for i, r := range set1 {
		if i < len(set2) {
			trans[r] = set2[i]
		} else {
			trans[r] = set2[len(set2)-1]
		}
	}

	for _, line := range readStdinLines() {
		var out []rune
		for _, r := range line {
			if t, ok := trans[r]; ok {
				r = t
			}
			if squeeze && len(out) > 0 && out[len(out)-1] == r {
				continue
			}
			out = append(out, r)
		}
		fmt.Println(string(out))
	}
	return 0
}

func expandSet(s string) []rune {
	var result []rune
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		if i+2 < len(runes) && runes[i+1] == '-' {
			for r := runes[i]; r <= runes[i+2]; r++ {
				result = append(result, r)
			}
			i += 2
		} else if s[i] == '\\' && i+1 < len(runes) {
			i++
			switch runes[i] {
			case 'n':
				result = append(result, '\n')
			case 't':
				result = append(result, '\t')
			case 'r':
				result = append(result, '\r')
			default:
				result = append(result, runes[i])
			}
		} else {
			result = append(result, runes[i])
		}
	}
	return result
}

func readStdinLines() []string {
	data, _ := io.ReadAll(os.Stdin)
	return strings.Split(string(data), "\n")
}

func init() {
	applet.Register(&applet.Applet{Name: "split", Short: "Split a file into pieces", Func: runSplit})
}

func runSplit(args []string) int {
	lines := 1000
	bytes := 0
	numericSuffix := false
	suffixLen := 2
	prefix := "x"
	file := ""

	i := 1
	for ; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			i++
			break
		}
		if strings.HasPrefix(a, "-l") {
			s := a[2:]
			if s == "" && i+1 < len(args) {
				i++
				s = args[i]
			}
			fmt.Sscanf(s, "%d", &lines)
			continue
		}
		if strings.HasPrefix(a, "-b") {
			s := a[2:]
			if s == "" && i+1 < len(args) {
				i++
				s = args[i]
			}
			bytes = parseSize(s)
			continue
		}
		if a == "-d" {
			numericSuffix = true
			continue
		}
		if strings.HasPrefix(a, "-a") {
			s := a[2:]
			if s == "" && i+1 < len(args) {
				i++
				s = args[i]
			}
			fmt.Sscanf(s, "%d", &suffixLen)
			continue
		}
		if !strings.HasPrefix(a, "-") {
			if file == "" {
				file = a
			} else {
				prefix = a
			}
			continue
		}
	}
	if file == "" {
		file = "-"
	}
	_ = numericSuffix
	_ = suffixLen

	var data []byte
	var err error
	if file == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(file)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "split: %v\n", err)
		return 1
	}

	if bytes > 0 {
		// Split by bytes
		idx := 0
		partNum := 0
		for idx < len(data) {
			end := idx + bytes
			if end > len(data) {
				end = len(data)
			}
			suffix := fmt.Sprintf("%0*d", suffixLen, partNum)
			os.WriteFile(prefix+suffix, data[idx:end], 0666)
			idx = end
			partNum++
		}
	} else {
		// Split by lines
		allLines := strings.Split(string(data), "\n")
		idx := 0
		partNum := 0
		for idx < len(allLines) {
			end := idx + lines
			if end > len(allLines) {
				end = len(allLines)
			}
			suffix := fmt.Sprintf("%0*d", suffixLen, partNum)
			content := strings.Join(allLines[idx:end], "\n") + "\n"
			os.WriteFile(prefix+suffix, []byte(content), 0666)
			idx = end
			partNum++
		}
	}
	return 0
}

func parseSize(s string) int {
	multiplier := 1
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return 0
	}
	last := s[len(s)-1]
	switch last {
	case 'K', 'k':
		multiplier = 1024
		s = s[:len(s)-1]
	case 'M', 'm':
		multiplier = 1024 * 1024
		s = s[:len(s)-1]
	case 'G', 'g':
		multiplier = 1024 * 1024 * 1024
		s = s[:len(s)-1]
	}
	var n int
	fmt.Sscanf(s, "%d", &n)
	return n * multiplier
}

func init() {
	applet.Register(&applet.Applet{Name: "sum", Short: "Checksum and block count", Func: runSum})
	applet.Register(&applet.Applet{Name: "factor", Short: "Factor numbers", Func: runFactor})
	applet.Register(&applet.Applet{Name: "tsort", Short: "Topological sort", Func: runTsort})
	applet.Register(&applet.Applet{Name: "od", Short: "Dump files in octal/hex", Func: runOd})
}

func runSum(args []string) int {
	bsd := true
	files := []string{}
	for _, a := range args[1:] {
		if a == "-r" {
			bsd = true
			continue
		}
		if a == "-s" {
			bsd = false
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
			fmt.Fprintf(os.Stderr, "sum: %s: %v\n", fname, err)
			return 1
		}

		var checksum uint16
		blocks := (len(data) + 511) / 512
		if bsd {
			for _, b := range data {
				checksum = (checksum >> 1) + ((checksum & 1) << 15)
				checksum += uint16(b)
				checksum &= 0xffff
			}
		} else {
			for _, b := range data {
				checksum += uint16(b)
			}
		}
		fmt.Printf("%d %d", checksum, blocks)
		if fname != "-" {
			fmt.Printf(" %s", fname)
		}
		fmt.Println()
	}
	return 0
}

func runFactor(args []string) int {
	if len(args) < 2 {
		// read from stdin
		for {
			var n int64
			_, err := fmt.Scan(&n)
			if err != nil {
				break
			}
			factorNum(n)
		}
		return 0
	}
	for _, a := range args[1:] {
		var n int64
		fmt.Sscanf(a, "%d", &n)
		factorNum(n)
	}
	return 0
}

func factorNum(n int64) {
	if n < 2 {
		fmt.Printf("%d:\n", n)
		return
	}
	fmt.Printf("%d:", n)
	for i := int64(2); i*i <= n; i++ {
		for n%i == 0 {
			fmt.Printf(" %d", i)
			n /= i
		}
	}
	if n > 1 {
		fmt.Printf(" %d", n)
	}
	fmt.Println()
}

func runTsort(args []string) int {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return 1
	}

	graph := make(map[string][]string)
	inDegree := make(map[string]int)
	nodes := make(map[string]bool)

	for range strings.Fields(string(data)) {
		// pairs of words
	}
	_ = graph
	_ = inDegree
	_ = nodes

	fmt.Fprintf(os.Stderr, "tsort: not fully implemented\n")
	return 1
}

func runOd(args []string) int {
	radix := "o" // o=octal, x=hex, d=decimal
	bytes := 0
	files := []string{}

	for _, a := range args[1:] {
		if strings.HasPrefix(a, "-t") {
			radix = a[2:]
			continue
		}
		if a == "-A" {
			continue
		}
		if !strings.HasPrefix(a, "-") {
			files = append(files, a)
		}
	}
	if len(files) == 0 {
		files = []string{"-"}
	}

	var allData []byte
	for _, fname := range files {
		var data []byte
		var err error
		if fname == "-" {
			data, err = io.ReadAll(os.Stdin)
		} else {
			data, err = os.ReadFile(fname)
		}
		if err != nil {
			return 1
		}
		allData = append(allData, data...)
	}
	_ = bytes

	addr := 0
	for i := 0; i < len(allData); i += 16 {
		end := i + 16
		if end > len(allData) {
			end = len(allData)
		}
		fmt.Printf("%07o", addr)
		for j := i; j < end; j += 2 {
			if j+1 < end {
				switch radix {
				case "x":
					fmt.Printf(" %04x", uint16(allData[j])|uint16(allData[j+1])<<8)
				case "d":
					fmt.Printf(" %5d", uint16(allData[j])|uint16(allData[j+1])<<8)
				default:
					fmt.Printf(" %06o", uint16(allData[j])|uint16(allData[j+1])<<8)
				}
			} else {
				switch radix {
				case "x":
					fmt.Printf(" %04x", allData[j])
				case "d":
					fmt.Printf(" %5d", allData[j])
				default:
					fmt.Printf(" %06o", allData[j])
				}
			}
		}
		fmt.Println()
		addr += 16
	}
	return 0
}

func init() {
	applet.Register(&applet.Applet{Name: "uname", Short: "Print system information", Func: runUname})
}

func runUname(args []string) int {
	showAll, showSys, showNode, showRel, showVer, showMachine, showOS := false, false, false, false, false, false, false

	if len(args) == 1 {
		showSys = true
	}

	for _, a := range args[1:] {
		if a == "-a" {
			showAll = true
		}
		if a == "-s" || a == "--kernel-name" {
			showSys = true
		}
		if a == "-n" || a == "--nodename" {
			showNode = true
		}
		if a == "-r" || a == "--kernel-release" {
			showRel = true
		}
		if a == "-v" || a == "--kernel-version" {
			showVer = true
		}
		if a == "-m" || a == "--machine" {
			showMachine = true
		}
		if a == "-o" || a == "--operating-system" {
			showOS = true
		}
	}

	if showAll {
		showSys, showNode, showRel, showVer, showMachine, showOS = true, true, true, true, true, true
	}

	host, _ := os.Hostname()
	parts := []string{}
	if showSys {
		parts = append(parts, getKernelName())
	}
	if showNode {
		parts = append(parts, host)
	}
	if showRel {
		parts = append(parts, getKernelRelease())
	}
	if showVer {
		parts = append(parts, getKernelVersion())
	}
	if showMachine {
		parts = append(parts, getMachine())
	}
	if showOS {
		parts = append(parts, getOSName())
	}

	fmt.Println(strings.Join(parts, " "))
	return 0
}

func getKernelName() string    { return "Linux" }
func getKernelRelease() string { return "6.1.0" }
func getKernelVersion() string { return "#1 SMP" }
func getMachine() string       { return "x86_64" }
func getOSName() string        { return "GNU/Linux" }

func init() {
	applet.Register(&applet.Applet{Name: "id", Short: "Print user/group IDs", Func: runId})
}

func runId(args []string) int {
	showUID, showGID, showGroups, showName := false, false, false, false

	for _, a := range args[1:] {
		if a == "-u" {
			showUID = true
		}
		if a == "-g" {
			showGID = true
		}
		if a == "-G" {
			showGroups = true
		}
		if a == "-n" {
			showName = true
		}
	}

	uid := os.Getuid()
	gid := os.Getgid()
	user := os.Getenv("USER")
	if user == "" {
		user = os.Getenv("USERNAME")
	}
	group := "users"

	if !showUID && !showGID && !showGroups {
		fmt.Printf("uid=%d(%s) gid=%d(%s)\n", uid, user, gid, group)
		return 0
	}

	if showName {
		if showUID {
			fmt.Println(user)
			return 0
		}
		if showGID {
			fmt.Println(group)
			return 0
		}
	} else {
		if showUID {
			fmt.Println(uid)
			return 0
		}
		if showGID {
			fmt.Println(gid)
			return 0
		}
	}
	if showGroups {
		if showName {
			fmt.Println(group)
		} else {
			fmt.Println(gid)
		}
	}
	return 0
}

func init() {
	applet.Register(&applet.Applet{Name: "test", Short: "Evaluate conditional expression", Func: runTest})
	applet.Register(&applet.Applet{Name: "[", Short: "Evaluate conditional expression", Func: runTest})
	applet.Register(&applet.Applet{Name: "[[", Short: "Evaluate conditional expression", Func: runTest})
}

func runTest(args []string) int {
	// Strip [ ... ]
	a := args[1:]
	if len(a) > 0 && a[0] == "[" {
		a = a[1:]
	}
	if len(a) > 0 && a[len(a)-1] == "]" {
		a = a[:len(a)-1]
	}
	if len(a) == 0 {
		return 1
	}

	result := evalExpr(a)
	if result {
		return 0
	}
	return 1
}

func evalExpr(args []string) bool {
	if len(args) == 0 {
		return false
	}
	if len(args) == 1 {
		return args[0] != ""
	}

	// Unary file tests
	if len(args) == 2 {
		op, path := args[0], args[1]
		info, err := os.Lstat(path)
		switch op {
		case "-e":
			return err == nil
		case "-f":
			return err == nil && !info.IsDir()
		case "-d":
			return err == nil && info.IsDir()
		case "-L", "-h":
			return err == nil && info.Mode()&os.ModeSymlink != 0
		case "-r":
			return err == nil && info.Mode()&0400 != 0
		case "-w":
			return err == nil && info.Mode()&0200 != 0
		case "-x":
			return err == nil && info.Mode()&0100 != 0
		case "-s":
			return err == nil && info.Size() > 0
		case "-z":
			return len(path) == 0
		case "-n":
			return len(path) > 0
		}
		return path != ""
	}

	// Binary operations
	if len(args) == 3 {
		left, op, right := args[0], args[1], args[2]
		switch op {
		case "=":
			return left == right
		case "!=":
			return left != right
		case "-eq":
			var a, b int
			fmt.Sscanf(left, "%d", &a)
			fmt.Sscanf(right, "%d", &b)
			return a == b
		case "-ne":
			var a, b int
			fmt.Sscanf(left, "%d", &a)
			fmt.Sscanf(right, "%d", &b)
			return a != b
		case "-lt":
			var a, b int
			fmt.Sscanf(left, "%d", &a)
			fmt.Sscanf(right, "%d", &b)
			return a < b
		case "-le":
			var a, b int
			fmt.Sscanf(left, "%d", &a)
			fmt.Sscanf(right, "%d", &b)
			return a <= b
		case "-gt":
			var a, b int
			fmt.Sscanf(left, "%d", &a)
			fmt.Sscanf(right, "%d", &b)
			return a > b
		case "-ge":
			var a, b int
			fmt.Sscanf(left, "%d", &a)
			fmt.Sscanf(right, "%d", &b)
			return a >= b
		}
	}

	// negation
	if args[0] == "!" {
		return !evalExpr(args[1:])
	}

	return false
}

func init() {
	applet.Register(&applet.Applet{Name: "install", Short: "Copy files and set attributes", Func: runInstall})
	applet.Register(&applet.Applet{Name: "chroot", Short: "Change root directory", Func: runChroot})
}

func runInstall(args []string) int {
	mode := os.FileMode(0755)
	dir := ""
	files := []string{}

	i := 1
	for ; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "-m") && len(a) > 2 {
			fmt.Sscanf(a[2:], "%o", &mode)
			continue
		}
		if a == "-d" {
			dir = "create"
			continue
		}
		if strings.HasPrefix(a, "-d") && len(a) > 2 {
			dir = a[2:]
			continue
		}
		if !strings.HasPrefix(a, "-") {
			files = append(files, a)
		}
	}

	if dir != "" {
		for _, d := range files {
			os.MkdirAll(d, mode)
		}
		return 0
	}

	if len(files) < 2 {
		fmt.Fprintf(os.Stderr, "install: missing file operand\n")
		return 1
	}

	dest := files[len(files)-1]
	sources := files[:len(files)-1]

	destInfo, err := os.Stat(dest)
	destIsDir := err == nil && destInfo.IsDir()

	if len(sources) > 1 && !destIsDir {
		fmt.Fprintf(os.Stderr, "install: target '%s' is not a directory\n", dest)
		return 1
	}

	for _, src := range sources {
		dst := dest
		if destIsDir {
			dst = strings.Join([]string{dest, filepath.Base(src)}, string(os.PathSeparator))
		}
		if err := copyFile(src, dst, mode, true, false, false); err != nil {
			fmt.Fprintf(os.Stderr, "install: %v\n", err)
			return 1
		}
	}
	return 0
}

func runChroot(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "chroot: missing operand\n")
		return 1
	}
	newRoot := args[1]
	command := "/bin/sh"
	commandArgs := []string{}
	if len(args) > 2 {
		command = args[2]
		commandArgs = args[2:]
	}

	if err := os.Chdir(newRoot); err != nil {
		fmt.Fprintf(os.Stderr, "chroot: cannot chdir to %s: %v\n", newRoot, err)
		return 1
	}
	_ = command
	_ = commandArgs
	return 0
}

func init() {
	applet.Register(&applet.Applet{Name: "numfmt", Short: "Reformat numbers", Func: runNumfmt})
	applet.Register(&applet.Applet{Name: "fmt", Short: "Reformat paragraph text", Func: runFmt})
	applet.Register(&applet.Applet{Name: "pr", Short: "Paginate or columnate files for printing", Func: runPr})
}

func runNumfmt(args []string) int {
	fromUnit := 1.0
	toUnit := 1.0
	files := []string{}

	for i := 1; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "--from=") {
			fromUnit = parseUnit(a[7:])
		} else if strings.HasPrefix(a, "--to=") {
			toUnit = parseUnit(a[5:])
		} else if !strings.HasPrefix(a, "-") {
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
			return 1
		}
		for _, line := range strings.Fields(string(data)) {
			var n float64
			fmt.Sscanf(line, "%f", &n)
			result := n * fromUnit / toUnit
			if result == float64(int(result)) {
				fmt.Printf("%d\n", int(result))
			} else {
				fmt.Printf("%.2f\n", result)
			}
		}
	}
	return 0
}

func parseUnit(s string) float64 {
	switch strings.ToLower(s) {
	case "k", "kib":
		return 1024
	case "m", "mib":
		return 1024 * 1024
	case "g", "gib":
		return 1024 * 1024 * 1024
	case "t", "tib":
		return 1024 * 1024 * 1024 * 1024
	case "kb":
		return 1000
	case "mb":
		return 1000 * 1000
	case "gb":
		return 1000 * 1000 * 1000
	}
	return 1
}

func runFmt(args []string) int {
	width := 75
	files := []string{}
	for _, a := range args[1:] {
		if strings.HasPrefix(a, "-w") && len(a) > 2 {
			fmt.Sscanf(a[2:], "%d", &width)
		} else if !strings.HasPrefix(a, "-") {
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
			return 1
		}
		fmt.Println(wrapText(string(data), width))
	}
	return 0
}

func wrapText(text string, width int) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}
	var lines []string
	line := words[0]
	for _, word := range words[1:] {
		if len(line)+1+len(word) > width {
			lines = append(lines, line)
			line = word
		} else {
			line += " " + word
		}
	}
	lines = append(lines, line)
	return strings.Join(lines, "\n")
}

func runPr(args []string) int {
	fmt.Fprintf(os.Stderr, "pr: not yet implemented\n")
	return 1
}
