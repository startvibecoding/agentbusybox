package fileutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode"

	"github.com/agentbusybox/pkg/applet"
)

func init() {
	// Re-register find with enhanced implementation
	applet.Register(&applet.Applet{Name: "find", Short: "Search for files in a directory hierarchy", Func: runFind})
}

// findExpr represents a parsed find expression
type findExpr struct {
	op       string // predicate name or logical operator
	arg      string // argument for predicate
	children []*findExpr
	negate   bool
}

func runFind(args []string) int {
	// Parse arguments: find [PATH...] [EXPRESSION]
	paths := []string{}
	exprArgs := []string{}

	i := 1
	// Collect paths (non-flag args before first flag-like arg)
	for ; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			i++
			break
		}
		if a == "(" || a == "!" || a == "-not" || strings.HasPrefix(a, "-") {
			break
		}
		paths = append(paths, a)
	}
	exprArgs = append(exprArgs, args[i:]...)

	if len(paths) == 0 {
		paths = []string{"."}
	}

	// Parse expression
	expr := parseFindExpr(exprArgs)

	exitCode := 0
	for _, root := range paths {
		// Handle -H and -L (follow symlinks on command line)
		info, err := os.Lstat(root)
		if err != nil {
			fmt.Fprintf(os.Stderr, "find: '%s': %v\n", root, err)
			exitCode = 1
			continue
		}
		_ = info

		err = walkFind(root, expr, &exitCode)
		if err != nil {
			fmt.Fprintf(os.Stderr, "find: '%s': %v\n", root, err)
			exitCode = 1
		}
	}
	return exitCode
}

// walkFind walks the filesystem evaluating expressions
func walkFind(root string, expr *findExpr, exitCode *int) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if evalFindExpr(expr, path, info) {
			// Default action is -print if no explicit action
			if !hasAction(expr) {
				fmt.Println(path)
			}
		}
		return nil
	})
}

// hasAction checks if the expression tree contains any action predicates
func hasAction(expr *findExpr) bool {
	if expr == nil {
		return false
	}
	switch expr.op {
	case "print", "print0", "exec", "execdir", "delete", "quit", "ls", "fls":
		return true
	}
	for _, c := range expr.children {
		if hasAction(c) {
			return true
		}
	}
	return false
}

// parseFindExpr parses find command-line arguments into an expression tree
// Grammar:
//
//	expr     = andExpr ( -o andExpr )*
//	andExpr  = unaryExpr ( (-a)? unaryExpr )*
//	unaryExpr = ( -not | ! ) unaryExpr | atom | ( expr )
func parseFindExpr(args []string) *findExpr {
	pos := 0
	expr := parseOrExpr(args, &pos)
	return expr
}

func parseOrExpr(args []string, pos *int) *findExpr {
	left := parseAndExpr(args, pos)
	for *pos < len(args) && (args[*pos] == "-o" || args[*pos] == "-or") {
		*pos++
		right := parseAndExpr(args, pos)
		left = &findExpr{op: "or", children: []*findExpr{left, right}}
	}
	return left
}

func parseAndExpr(args []string, pos *int) *findExpr {
	left := parseUnaryExpr(args, pos)
	for *pos < len(args) {
		a := args[*pos]
		// Implicit AND (no -a) or explicit -a/-and
		if a == "-a" || a == "-and" {
			*pos++
			right := parseUnaryExpr(args, pos)
			left = &findExpr{op: "and", children: []*findExpr{left, right}}
		} else if a == "-o" || a == "-or" || a == ")" {
			break
		} else {
			// Implicit AND
			right := parseUnaryExpr(args, pos)
			left = &findExpr{op: "and", children: []*findExpr{left, right}}
		}
	}
	return left
}

func parseUnaryExpr(args []string, pos *int) *findExpr {
	if *pos >= len(args) {
		return &findExpr{op: "true"}
	}
	a := args[*pos]
	if a == "!" || a == "-not" {
		*pos++
		child := parseUnaryExpr(args, pos)
		return &findExpr{op: "not", children: []*findExpr{child}}
	}
	if a == "(" {
		*pos++
		expr := parseOrExpr(args, pos)
		if *pos < len(args) && args[*pos] == ")" {
			*pos++
		}
		return expr
	}
	return parseAtom(args, pos)
}

func parseAtom(args []string, pos *int) *findExpr {
	if *pos >= len(args) {
		return &findExpr{op: "true"}
	}
	a := args[*pos]
	*pos++

	switch a {
	case "-name":
		if *pos < len(args) {
			pat := args[*pos]
			*pos++
			return &findExpr{op: "name", arg: pat}
		}
	case "-iname":
		if *pos < len(args) {
			pat := args[*pos]
			*pos++
			return &findExpr{op: "iname", arg: pat}
		}
	case "-path", "-wholename":
		if *pos < len(args) {
			pat := args[*pos]
			*pos++
			return &findExpr{op: "path", arg: pat}
		}
	case "-ipath", "-iwholename":
		if *pos < len(args) {
			pat := args[*pos]
			*pos++
			return &findExpr{op: "ipath", arg: pat}
		}
	case "-regex":
		if *pos < len(args) {
			pat := args[*pos]
			*pos++
			return &findExpr{op: "regex", arg: pat}
		}
	case "-type":
		if *pos < len(args) {
			t := args[*pos]
			*pos++
			return &findExpr{op: "type", arg: t}
		}
	case "-size":
		if *pos < len(args) {
			s := args[*pos]
			*pos++
			return &findExpr{op: "size", arg: s}
		}
	case "-mtime":
		if *pos < len(args) {
			n := args[*pos]
			*pos++
			return &findExpr{op: "mtime", arg: n}
		}
	case "-atime":
		if *pos < len(args) {
			n := args[*pos]
			*pos++
			return &findExpr{op: "atime", arg: n}
		}
	case "-ctime":
		if *pos < len(args) {
			n := args[*pos]
			*pos++
			return &findExpr{op: "ctime", arg: n}
		}
	case "-mmin":
		if *pos < len(args) {
			n := args[*pos]
			*pos++
			return &findExpr{op: "mmin", arg: n}
		}
	case "-amin":
		if *pos < len(args) {
			n := args[*pos]
			*pos++
			return &findExpr{op: "amin", arg: n}
		}
	case "-cmin":
		if *pos < len(args) {
			n := args[*pos]
			*pos++
			return &findExpr{op: "cmin", arg: n}
		}
	case "-newer":
		if *pos < len(args) {
			f := args[*pos]
			*pos++
			return &findExpr{op: "newer", arg: f}
		}
	case "-perm":
		if *pos < len(args) {
			m := args[*pos]
			*pos++
			return &findExpr{op: "perm", arg: m}
		}
	case "-user":
		if *pos < len(args) {
			u := args[*pos]
			*pos++
			return &findExpr{op: "user", arg: u}
		}
	case "-group":
		if *pos < len(args) {
			g := args[*pos]
			*pos++
			return &findExpr{op: "group", arg: g}
		}
	case "-links":
		if *pos < len(args) {
			n := args[*pos]
			*pos++
			return &findExpr{op: "links", arg: n}
		}
	case "-inum":
		if *pos < len(args) {
			n := args[*pos]
			*pos++
			return &findExpr{op: "inum", arg: n}
		}
	case "-maxdepth":
		if *pos < len(args) {
			n := args[*pos]
			*pos++
			return &findExpr{op: "maxdepth", arg: n}
		}
	case "-mindepth":
		if *pos < len(args) {
			n := args[*pos]
			*pos++
			return &findExpr{op: "mindepth", arg: n}
		}
	case "-empty":
		return &findExpr{op: "empty"}
	case "-executable":
		return &findExpr{op: "executable"}
	case "-readable":
		return &findExpr{op: "readable"}
	case "-writable":
		return &findExpr{op: "writable"}
	case "-print":
		return &findExpr{op: "print"}
	case "-print0":
		return &findExpr{op: "print0"}
	case "-delete":
		return &findExpr{op: "delete"}
	case "-prune":
		return &findExpr{op: "prune"}
	case "-quit":
		return &findExpr{op: "quit"}
	case "-ls":
		return &findExpr{op: "ls"}
	case "-exec":
		// Collect args until ; or +
		cmdArgs := []string{}
		for *pos < len(args) && args[*pos] != ";" && args[*pos] != "+" {
			cmdArgs = append(cmdArgs, args[*pos])
			*pos++
		}
		batch := false
		if *pos < len(args) && args[*pos] == "+" {
			batch = true
			*pos++
		} else if *pos < len(args) {
			*pos++ // skip ;
		}
		if batch {
			return &findExpr{op: "exec_batch", children: nil, arg: strings.Join(cmdArgs, "\x00")}
		}
		return &findExpr{op: "exec", arg: strings.Join(cmdArgs, "\x00")}
	case "-ok":
		// Like -exec but with confirmation (simplified: just exec)
		cmdArgs := []string{}
		for *pos < len(args) && args[*pos] != ";" {
			cmdArgs = append(cmdArgs, args[*pos])
			*pos++
		}
		if *pos < len(args) {
			*pos++ // skip ;
		}
		return &findExpr{op: "exec", arg: strings.Join(cmdArgs, "\x00")}
	case "-execdir":
		cmdArgs := []string{}
		for *pos < len(args) && args[*pos] != ";" && args[*pos] != "+" {
			cmdArgs = append(cmdArgs, args[*pos])
			*pos++
		}
		if *pos < len(args) {
			*pos++ // skip ;
		}
		return &findExpr{op: "execdir", arg: strings.Join(cmdArgs, "\x00")}
	case "-depth":
		return &findExpr{op: "depth"}
	case "-xdev":
		return &findExpr{op: "xdev"}
	case "-true":
		return &findExpr{op: "true"}
	case "-false":
		return &findExpr{op: "false"}
	}

	// Unknown predicate - treat as true (pass-through)
	return &findExpr{op: "true"}
}

// evalFindExpr evaluates a find expression against a file
func evalFindExpr(expr *findExpr, path string, info os.FileInfo) bool {
	if expr == nil {
		return true
	}

	switch expr.op {
	case "and":
		return evalFindExpr(expr.children[0], path, info) && evalFindExpr(expr.children[1], path, info)
	case "or":
		return evalFindExpr(expr.children[0], path, info) || evalFindExpr(expr.children[1], path, info)
	case "not":
		return !evalFindExpr(expr.children[0], path, info)
	case "true":
		return true
	case "false":
		return false

	case "name":
		matched, _ := filepath.Match(expr.arg, info.Name())
		return matched
	case "iname":
		matched, _ := filepath.Match(strings.ToLower(expr.arg), strings.ToLower(info.Name()))
		return matched
	case "path":
		return matchPathGlob(expr.arg, path)
	case "ipath":
		return matchPathGlob(strings.ToLower(expr.arg), strings.ToLower(path))
	case "regex":
		re, err := regexp.Compile(expr.arg)
		if err != nil {
			return false
		}
		return re.MatchString(path)

	case "type":
		return matchFileType(info, expr.arg)

	case "size":
		return matchFileSize(info.Size(), expr.arg)

	case "mtime":
		return matchTime(info.ModTime(), expr.arg, "mtime")
	case "atime":
		stat := getSysStat(info)
		if stat != nil {
			return matchTime(time.Unix(stat.Atim.Sec, 0), expr.arg, "atime")
		}
		return matchTime(info.ModTime(), expr.arg, "atime")
	case "ctime":
		stat := getSysStat(info)
		if stat != nil {
			return matchTime(time.Unix(stat.Ctim.Sec, 0), expr.arg, "ctime")
		}
		return matchTime(info.ModTime(), expr.arg, "ctime")
	case "mmin":
		return matchMinutes(info.ModTime(), expr.arg)
	case "amin":
		stat := getSysStat(info)
		if stat != nil {
			return matchMinutes(time.Unix(stat.Atim.Sec, 0), expr.arg)
		}
		return matchMinutes(info.ModTime(), expr.arg)
	case "cmin":
		stat := getSysStat(info)
		if stat != nil {
			return matchMinutes(time.Unix(stat.Ctim.Sec, 0), expr.arg)
		}
		return matchMinutes(info.ModTime(), expr.arg)

	case "newer":
		refInfo, err := os.Stat(expr.arg)
		if err != nil {
			return false
		}
		return info.ModTime().After(refInfo.ModTime())

	case "perm":
		return matchPerm(info.Mode(), expr.arg)

	case "user":
		return matchUser(info, expr.arg)
	case "group":
		return matchGroup(info, expr.arg)

	case "links":
		n, _ := strconv.ParseInt(expr.arg, 10, 64)
		stat := getSysStat(info)
		if stat != nil {
			return int64(stat.Nlink) == n
		}
		return false

	case "inum":
		n, _ := strconv.ParseUint(expr.arg, 10, 64)
		stat := getSysStat(info)
		if stat != nil {
			return stat.Ino == n
		}
		return false

	case "empty":
		if info.IsDir() {
			entries, err := os.ReadDir(path)
			return err == nil && len(entries) == 0
		}
		return info.Size() == 0

	case "executable":
		return info.Mode()&0111 != 0
	case "readable":
		return info.Mode()&0444 != 0
	case "writable":
		return info.Mode()&0222 != 0

	case "maxdepth":
		// Handled at walk level, always true in predicate
		return true
	case "mindepth":
		return true

	// Actions (return true, side effects)
	case "print":
		fmt.Println(path)
		return true
	case "print0":
		fmt.Print(path + "\x00")
		return true
	case "exec":
		return doExec(expr.arg, path, false)
	case "exec_batch":
		// Batch exec is handled at a higher level; for now, exec per file
		return doExec(expr.arg, path, false)
	case "execdir":
		return doExec(expr.arg, path, true)
	case "delete":
		if info.IsDir() {
			os.Remove(path) // remove empty dir after children processed
		} else {
			os.Remove(path)
		}
		return true
	case "prune":
		// Return false to prevent descending
		return false
	case "quit":
		fmt.Println(path)
		os.Exit(0)
		return true
	case "ls":
		printLs(path, info)
		return true
	case "depth":
		return true
	case "xdev":
		return true

	default:
		return true
	}
}

func doExec(cmdArgStr string, path string, execDir bool) bool {
	parts := strings.Split(cmdArgStr, "\x00")
	if len(parts) == 0 {
		return false
	}

	// Replace {} with path in all args
	args := []string{}
	for _, p := range parts {
		if p == "{}" {
			args = append(args, path)
		} else {
			args = append(args, strings.ReplaceAll(p, "{}", path))
		}
	}

	var cmd *exec.Cmd
	if len(args) == 1 {
		cmd = exec.Command(args[0])
	} else {
		cmd = exec.Command(args[0], args[1:]...)
	}

	if execDir {
		cmd.Dir = filepath.Dir(path)
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	return err == nil
}

func printLs(path string, info os.FileInfo) {
	stat := getSysStat(info)
	var ino uint64
	var nlink uint64
	if stat != nil {
		ino = stat.Ino
		nlink = uint64(stat.Nlink)
	}
	fmt.Printf("%6d %4d %s %8d %s %s\n",
		ino, nlink, info.Mode().String(), info.Size(),
		info.ModTime().Format("Jan _2 15:04"), path)
}

func getSysStat(info os.FileInfo) *syscall.Stat_t {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return stat
	}
	return nil
}

func matchFileType(info os.FileInfo, typeStr string) bool {
	switch typeStr {
	case "f":
		return !info.IsDir() && info.Mode()&os.ModeSymlink == 0
	case "d":
		return info.IsDir()
	case "l":
		return info.Mode()&os.ModeSymlink != 0
	case "b":
		return info.Mode()&os.ModeDevice != 0 && info.Mode()&os.ModeCharDevice == 0
	case "c":
		return info.Mode()&os.ModeCharDevice != 0
	case "p":
		return info.Mode()&os.ModeNamedPipe != 0
	case "s":
		return info.Mode()&os.ModeSocket != 0
	}
	return false
}

func matchFileSize(size int64, expr string) bool {
	if len(expr) == 0 {
		return true
	}
	suffix := ""
	s := expr
	if !unicode.IsDigit(rune(expr[len(expr)-1])) {
		suffix = string(expr[len(expr)-1])
		s = expr[:len(expr)-1]
	}

	unit := int64(1)
	switch suffix {
	case "c":
		unit = 1
	case "w":
		unit = 2
	case "b":
		unit = 512
	case "k":
		unit = 1024
	case "M":
		unit = 1024 * 1024
	case "G":
		unit = 1024 * 1024 * 1024
	}

	n, _ := strconv.ParseInt(s, 10, 64)

	// Check prefix: +N (greater), -N (less), N (exact)
	if strings.HasPrefix(expr, "+") {
		return size > n*unit
	} else if strings.HasPrefix(expr, "-") {
		return size < n*unit
	}
	return size == n*unit || (n == 0 && size == 0)
}

func matchTime(fileTime time.Time, expr string, which string) bool {
	// Parse: +N (more than N days ago), -N (less than N days ago), N (exactly N days ago)
	s := expr
	prefix := ""
	if len(s) > 0 && (s[0] == '+' || s[0] == '-') {
		prefix = string(s[0])
		s = s[1:]
	}
	n, _ := strconv.Atoi(s)
	days := time.Duration(n) * 24 * time.Hour
	now := time.Now()
	age := now.Sub(fileTime)

	switch prefix {
	case "+":
		return age > days
	case "-":
		return age < days
	default:
		// Exactly N days ago (within 24h window)
		return age >= days && age < days+24*time.Hour
	}
}

func matchMinutes(fileTime time.Time, expr string) bool {
	s := expr
	prefix := ""
	if len(s) > 0 && (s[0] == '+' || s[0] == '-') {
		prefix = string(s[0])
		s = s[1:]
	}
	n, _ := strconv.Atoi(s)
	mins := time.Duration(n) * time.Minute
	now := time.Now()
	age := now.Sub(fileTime)

	switch prefix {
	case "+":
		return age > mins
	case "-":
		return age < mins
	default:
		return age >= mins && age < mins+time.Minute
	}
}

func matchPerm(mode os.FileMode, expr string) bool {
	// -perm MODE: exact match
	// -perm -MODE: all bits set
	// -perm /MODE: any bit set
	prefix := ""
	s := expr
	if len(s) > 0 && (s[0] == '-' || s[0] == '/') {
		prefix = string(s[0])
		s = s[1:]
	}

	var perm uint32
	if _, err := fmt.Sscanf(s, "%o", &perm); err != nil {
		return false
	}

	permMode := uint32(mode & os.ModePerm)

	switch prefix {
	case "-":
		// All specified bits must be set
		return permMode&perm == perm
	case "/":
		// Any specified bit must be set
		return permMode&perm != 0
	default:
		// Exact match
		return permMode == perm
	}
}

func matchUser(info os.FileInfo, name string) bool {
	stat := getSysStat(info)
	if stat == nil {
		return false
	}

	// Try numeric UID
	if uid, err := strconv.ParseUint(name, 10, 32); err == nil {
		return stat.Uid == uint32(uid)
	}

	// Try username lookup
	u, err := userLookup(name)
	if err != nil {
		return false
	}
	uid, _ := strconv.ParseUint(u, 10, 32)
	return stat.Uid == uint32(uid)
}

func matchGroup(info os.FileInfo, name string) bool {
	stat := getSysStat(info)
	if stat == nil {
		return false
	}

	// Try numeric GID
	if gid, err := strconv.ParseUint(name, 10, 32); err == nil {
		return stat.Gid == uint32(gid)
	}

	// Try group lookup from /etc/group
	g, err := groupLookup(name)
	if err != nil {
		return false
	}
	gid, _ := strconv.ParseUint(g, 10, 32)
	return stat.Gid == uint32(gid)
}

// userLookup looks up a username in /etc/passwd and returns UID
func userLookup(name string) (string, error) {
	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Split(line, ":")
		if len(fields) >= 3 && fields[0] == name {
			return fields[2], nil
		}
	}
	return "", fmt.Errorf("user not found")
}

// groupLookup looks up a group name in /etc/group and returns GID
func groupLookup(name string) (string, error) {
	data, err := os.ReadFile("/etc/group")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Split(line, ":")
		if len(fields) >= 3 && fields[0] == name {
			return fields[2], nil
		}
	}
	return "", fmt.Errorf("group not found")
}

// matchSize is already defined elsewhere but we need the enhanced version
func matchSizeEnhanced(size int64, expr string) bool {
	return matchFileSize(size, expr)
}

// matchPathGlob matches a path against a glob pattern where * matches anything including /
func matchPathGlob(pattern, path string) bool {
	// Convert glob to regex
	re := "^"
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*':
			re += ".*"
		case '?':
			re += "."
		case '.':
			re += "\\."
		case '[':
			re += "["
		case ']':
			re += "]"
		default:
			re += string(pattern[i])
		}
	}
	re += "$"
	matched, _ := regexp.MatchString(re, path)
	return matched
}

// sort helpers for tree
var _ = sort.Strings
