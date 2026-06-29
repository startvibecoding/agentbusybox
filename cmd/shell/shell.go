package shell

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/agentbusybox/pkg/applet"
)

var (
	lastExitCode     int = 0
	positionalParams []string
	shellName        string            = "sh"
	shellVars        map[string]string = make(map[string]string)
	commandHistory   []string
)

func init() {
	applet.Register(&applet.Applet{Name: "ash", Short: "Almquist shell", Func: runSh})
	applet.Register(&applet.Applet{Name: "bash", Short: "Bourne Again shell compatibility alias", Func: runSh})
	applet.Register(&applet.Applet{Name: "lash", Short: "Legacy shell compatibility alias", Func: runSh})
	applet.Register(&applet.Applet{Name: "sh", Short: "Bourne shell", Func: runSh})
	applet.Register(&applet.Applet{Name: "hush", Short: "Hush shell", Func: runSh})
}

func runSh(args []string) int {
	if len(args) > 0 {
		shellName = args[0]
	}
	scriptFile := ""
	command := ""
	interactive := true
	var scriptArgs []string

	for i := 1; i < len(args); i++ {
		a := args[i]
		if a == "-c" {
			interactive = false
			continue
		}
		if !strings.HasPrefix(a, "-") {
			if !interactive {
				command = a
			} else {
				if scriptFile == "" {
					scriptFile = a
					scriptArgs = args[i+1:]
					break
				}
			}
		}
	}

	if scriptFile != "" {
		return runScriptWithArgs(scriptFile, scriptArgs)
	}
	if command != "" {
		return runCommand(command)
	}
	return runInteractive()
}

func runScript(path string) int {
	return runScriptWithArgs(path, nil)
}

func runScriptWithArgs(path string, args []string) int {
	oldParams := positionalParams
	positionalParams = args
	defer func() {
		positionalParams = oldParams
	}()

	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sh: %s: %v\n", path, err)
		return 1
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	exitCode := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		exitCode = executeLine(line)
	}
	return exitCode
}

func runCommand(cmd string) int {
	return executeLine(cmd)
}

func runInteractive() int {
	return runInteractivePlatform()
}

type commandPart struct {
	cmd string
	op  string // "", ";", "&&", "||"
}

func parseAndOr(line string) []commandPart {
	var parts []commandPart
	current := ""
	inSingle, inDouble := false, false
	i := 0
	for i < len(line) {
		ch := line[i]
		switch {
		case ch == '\'' && !inDouble:
			inSingle = !inSingle
			current += string(ch)
			i++
		case ch == '"' && !inSingle:
			inDouble = !inDouble
			current += string(ch)
			i++
		case !inSingle && !inDouble && i+1 < len(line) && line[i:i+2] == "&&":
			parts = append(parts, commandPart{cmd: strings.TrimSpace(current), op: "&&"})
			current = ""
			i += 2
		case !inSingle && !inDouble && i+1 < len(line) && line[i:i+2] == "||":
			parts = append(parts, commandPart{cmd: strings.TrimSpace(current), op: "||"})
			current = ""
			i += 2
		case !inSingle && !inDouble && ch == ';':
			parts = append(parts, commandPart{cmd: strings.TrimSpace(current), op: ";"})
			current = ""
			i++
		default:
			current += string(ch)
			i++
		}
	}
	if current != "" {
		parts = append(parts, commandPart{cmd: strings.TrimSpace(current), op: ""})
	}
	return parts
}

func executeLine(line string) int {
	parts := parseAndOr(line)
	if len(parts) == 0 {
		return 0
	}

	exitCode := 0
	skip := false

	for i, part := range parts {
		if skip {
			prevOp := parts[i-1].op
			if prevOp == ";" {
				skip = false
			} else if prevOp == "&&" && exitCode == 0 {
				skip = false
			} else if prevOp == "||" && exitCode != 0 {
				skip = false
			}
		}

		if !skip {
			if part.cmd != "" {
				exitCode = executeSinglePipeline(part.cmd)
				lastExitCode = exitCode
			}
		}

		if part.op == "&&" && exitCode != 0 {
			skip = true
		} else if part.op == "||" && exitCode == 0 {
			skip = true
		} else {
			skip = false
		}
	}

	return exitCode
}

func executeSinglePipeline(line string) int {
	line = expandVars(line)

	if strings.Contains(line, "|") {
		return executePipe(line)
	}

	if strings.Contains(line, ">") || strings.Contains(line, "<") {
		return executeRedirect(line)
	}

	args := parseArgs(line)
	if len(args) == 0 {
		return 0
	}

	return executeSingleCommand(args)
}

func prepareCmd(name string, args []string) *exec.Cmd {
	if applet.Get(name) != nil {
		self, err := os.Executable()
		if err != nil {
			self = "/proc/self/exe"
		}
		fullArgs := append([]string{name}, args...)
		cmd := exec.Command(self, fullArgs...)
		return cmd
	}
	return exec.Command(name, args...)
}

func isValidVarName(s string) bool {
	if len(s) == 0 {
		return false
	}
	first := s[0]
	if !((first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z') || first == '_') {
		return false
	}
	for i := 1; i < len(s); i++ {
		ch := s[i]
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_') {
			return false
		}
	}
	return true
}

func executeSingleCommand(args []string) int {
	assignments := []string{}
	cmdIdx := 0
	for i, arg := range args {
		if idx := strings.IndexByte(arg, '='); idx > 0 {
			name := arg[:idx]
			if isValidVarName(name) {
				assignments = append(assignments, arg)
				cmdIdx = i + 1
				continue
			}
		}
		break
	}

	if cmdIdx == len(args) {
		for _, assoc := range assignments {
			parts := strings.SplitN(assoc, "=", 2)
			name, val := parts[0], parts[1]
			shellVars[name] = val
			if _, exists := os.LookupEnv(name); exists {
				os.Setenv(name, val)
			}
		}
		return 0
	}

	cmdArgs := args[cmdIdx:]

	switch cmdArgs[0] {
	case "cd":
		return builtinCd(cmdArgs)
	case "export":
		return builtinExport(cmdArgs)
	case "set":
		return builtinSet(cmdArgs)
	case "unset":
		return builtinUnset(cmdArgs)
	case "echo":
		return builtinEcho(cmdArgs)
	case "read":
		return builtinRead(cmdArgs)
	case "pwd":
		return builtinPwd(cmdArgs)
	case "history":
		return builtinHistory(cmdArgs)
	case "source", ".":
		if len(cmdArgs) > 1 {
			return runScript(cmdArgs[1])
		}
		return 0
	case "type":
		return builtinType(cmdArgs)
	case "alias":
		return builtinAlias(cmdArgs)
	case "unalias":
		return builtinUnalias(cmdArgs)
	case "trap":
		return builtinTrap(cmdArgs)
	case "shift":
		return builtinShift(cmdArgs)
	case "exit":
		code := 0
		if len(cmdArgs) > 1 {
			fmt.Sscanf(cmdArgs[1], "%d", &code)
		}
		os.Exit(code)
	case "exec":
		if len(cmdArgs) > 1 {
			cmd := prepareCmd(cmdArgs[1], cmdArgs[2:])
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if len(assignments) > 0 {
				cmd.Env = append(os.Environ(), assignments...)
			}
			if err := cmd.Run(); err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					return exitErr.ExitCode()
				}
				return 1
			}
		}
		return 0
	}

	cmd := prepareCmd(cmdArgs[0], cmdArgs[1:])
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if len(assignments) > 0 {
		cmd.Env = append(os.Environ(), assignments...)
	}

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		return 127
	}
	return 0
}

func expandVars(line string) string {
	result := ""
	i := 0
	for i < len(line) {
		if line[i] == '$' && i+1 < len(line) {
			i++
			if line[i] == '{' {
				end := strings.IndexByte(line[i:], '}')
				if end >= 0 {
					varName := line[i+1 : i+end]
					result += expandSingleVar(varName)
					i += end + 1
					continue
				}
			}

			ch := line[i]
			if ch == '?' {
				result += strconv.Itoa(lastExitCode)
				i++
				continue
			}
			if ch == '$' {
				result += strconv.Itoa(os.Getpid())
				i++
				continue
			}
			if ch == '#' {
				result += strconv.Itoa(len(positionalParams))
				i++
				continue
			}
			if ch == '*' || ch == '@' {
				result += strings.Join(positionalParams, " ")
				i++
				continue
			}
			if ch >= '0' && ch <= '9' {
				idx := int(ch - '0')
				if idx == 0 {
					result += shellName
				} else if idx-1 < len(positionalParams) {
					result += positionalParams[idx-1]
				}
				i++
				continue
			}

			end := i
			for end < len(line) && (isAlphaNum(line[end]) || line[end] == '_') {
				end++
			}
			varName := line[i:end]
			result += expandSingleVar(varName)
			i = end
		} else if line[i] == '~' && (i == 0 || line[i-1] == ' ') {
			result += os.Getenv("HOME")
			i++
		} else {
			result += string(line[i])
			i++
		}
	}
	return result
}

func expandSingleVar(name string) string {
	if val, exists := os.LookupEnv(name); exists {
		return val
	}
	if val, exists := shellVars[name]; exists {
		return val
	}
	return ""
}

func isAlphaNum(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

func parseArgs(line string) []string {
	args := []string{}
	current := ""
	inSingle, inDouble := false, false

	for i := 0; i < len(line); i++ {
		ch := line[i]
		switch {
		case ch == '\'' && !inDouble:
			inSingle = !inSingle
		case ch == '"' && !inSingle:
			inDouble = !inDouble
		case ch == ' ' && !inSingle && !inDouble:
			if current != "" {
				args = append(args, current)
				current = ""
			}
		default:
			current += string(ch)
		}
	}
	if current != "" {
		args = append(args, current)
	}
	return args
}

func executePipe(line string) int {
	commands := strings.Split(line, "|")
	if len(commands) < 2 {
		return executeSinglePipeline(strings.TrimSpace(commands[0]))
	}

	var prevOutput []byte
	exitCode := 0

	for i, cmdStr := range commands {
		cmdStr = strings.TrimSpace(cmdStr)
		args := parseArgs(cmdStr)
		if len(args) == 0 {
			continue
		}

		cmd := prepareCmd(args[0], args[1:])
		if i > 0 {
			cmd.Stdin = strings.NewReader(string(prevOutput))
		} else {
			cmd.Stdin = os.Stdin
		}
		if i < len(commands)-1 {
			out, err := cmd.Output()
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					exitCode = exitErr.ExitCode()
				} else {
					exitCode = 1
				}
			}
			prevOutput = out
		} else {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					exitCode = exitErr.ExitCode()
				} else {
					exitCode = 1
				}
			}
		}
	}
	return exitCode
}

func executeRedirect(line string) int {
	if strings.Contains(line, ">>") {
		parts := strings.SplitN(line, ">>", 2)
		cmdStr := strings.TrimSpace(parts[0])
		fileName := strings.TrimSpace(parts[1])
		args := parseArgs(cmdStr)
		if len(args) == 0 {
			return 0
		}

		f, err := os.OpenFile(fileName, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			fmt.Fprintf(os.Stderr, "sh: %v\n", err)
			return 1
		}
		defer f.Close()

		cmd := prepareCmd(args[0], args[1:])
		cmd.Stdin = os.Stdin
		cmd.Stdout = f
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return 1
		}
		return 0
	}
	if strings.Contains(line, ">") {
		parts := strings.SplitN(line, ">", 2)
		cmdStr := strings.TrimSpace(parts[0])
		fileName := strings.TrimSpace(parts[1])
		args := parseArgs(cmdStr)
		if len(args) == 0 {
			return 0
		}

		f, err := os.Create(fileName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "sh: %v\n", err)
			return 1
		}
		defer f.Close()

		cmd := prepareCmd(args[0], args[1:])
		cmd.Stdin = os.Stdin
		cmd.Stdout = f
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return 1
		}
		return 0
	}
	if strings.Contains(line, "<") {
		parts := strings.SplitN(line, "<", 2)
		cmdStr := strings.TrimSpace(parts[0])
		fileName := strings.TrimSpace(parts[1])
		args := parseArgs(cmdStr)
		if len(args) == 0 {
			return 0
		}

		f, err := os.Open(fileName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "sh: %v\n", err)
			return 1
		}
		defer f.Close()

		cmd := prepareCmd(args[0], args[1:])
		cmd.Stdin = f
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return 1
		}
		return 0
	}
	return executeSinglePipeline(strings.TrimSpace(line))
}

func builtinCd(args []string) int {
	dir := os.Getenv("HOME")
	if len(args) > 1 {
		dir = args[1]
	}
	if dir == "-" {
		dir = os.Getenv("OLDPWD")
	}
	if err := os.Chdir(dir); err != nil {
		fmt.Fprintf(os.Stderr, "cd: %s: %v\n", dir, err)
		return 1
	}
	os.Setenv("OLDPWD", os.Getenv("PWD"))
	newDir, _ := os.Getwd()
	os.Setenv("PWD", newDir)
	return 0
}

func builtinExport(args []string) int {
	if len(args) == 1 {
		for _, e := range os.Environ() {
			fmt.Println("export " + e)
		}
		return 0
	}
	for _, a := range args[1:] {
		parts := strings.SplitN(a, "=", 2)
		if len(parts) == 2 {
			name, val := parts[0], parts[1]
			shellVars[name] = val
			os.Setenv(name, val)
		} else {
			name := parts[0]
			if val, exists := shellVars[name]; exists {
				os.Setenv(name, val)
			} else {
				os.Setenv(name, "")
			}
		}
	}
	return 0
}

func builtinSet(args []string) int {
	for _, e := range os.Environ() {
		fmt.Println(e)
	}
	for k, v := range shellVars {
		if _, exists := os.LookupEnv(k); !exists {
			fmt.Printf("%s=%s\n", k, v)
		}
	}
	return 0
}

func builtinUnset(args []string) int {
	for _, a := range args[1:] {
		os.Unsetenv(a)
		delete(shellVars, a)
	}
	return 0
}

func builtinEcho(args []string) int {
	fmt.Println(strings.Join(args[1:], " "))
	return 0
}

func builtinRead(args []string) int {
	varName := "REPLY"
	if len(args) > 1 {
		varName = args[1]
	}

	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return 1
	}
	line = strings.TrimRight(line, "\n")
	shellVars[varName] = line
	os.Setenv(varName, line)
	return 0
}

func builtinPwd(args []string) int {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "pwd: %v\n", err)
		return 1
	}
	fmt.Println(dir)
	return 0
}

func builtinHistory(args []string) int {
	for i, cmd := range commandHistory {
		fmt.Printf("%5d  %s\n", i+1, cmd)
	}
	return 0
}

func loadHistory() {
	home := os.Getenv("HOME")
	if home == "" {
		return
	}
	histFile := filepath.Join(home, ".ash_history")
	data, err := os.ReadFile(histFile)
	if err != nil {
		return
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			commandHistory = append(commandHistory, trimmed)
		}
	}
}

func saveHistoryLine(line string) {
	home := os.Getenv("HOME")
	if home == "" {
		return
	}
	histFile := filepath.Join(home, ".ash_history")
	f, err := os.OpenFile(histFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(line + "\n")
}

func builtinType(args []string) int {
	for _, name := range args[1:] {
		builtins := []string{"cd", "export", "set", "unset", "echo", "read", "pwd", "history", "source", ".", "type", "alias", "unalias", "trap", "shift", "exit", "exec"}
		isBuiltin := false
		for _, b := range builtins {
			if b == name {
				isBuiltin = true
				break
			}
		}
		if isBuiltin {
			fmt.Printf("%s is a shell builtin\n", name)
			continue
		}
		path, err := exec.LookPath(name)
		if err != nil {
			fmt.Printf("-sh: type: %s: not found\n", name)
		} else {
			fmt.Printf("%s is %s\n", name, path)
		}
	}
	return 0
}

func builtinAlias(args []string) int {
	fmt.Fprintf(os.Stderr, "alias: not yet implemented\n")
	return 0
}

func builtinUnalias(args []string) int {
	return 0
}

func builtinTrap(args []string) int {
	return 0
}

func builtinShift(args []string) int {
	return 0
}

func init() {
	applet.Register(&applet.Applet{Name: "shuf", Short: "Generate random permutations", Func: runShuf})
}

func runShuf(args []string) int {
	input := []string{}
	count := 0
	repeat := false

	for i := 1; i < len(args); i++ {
		if args[i] == "-n" && i+1 < len(args) {
			i++
			fmt.Sscanf(args[i], "%d", &count)
			continue
		}
		if args[i] == "-r" {
			repeat = true
			continue
		}
		if args[i] == "-i" && i+1 < len(args) {
			i++
			var start, end int
			fmt.Sscanf(args[i], "%d-%d", &start, &end)
			for j := start; j <= end; j++ {
				input = append(input, fmt.Sprintf("%d", j))
			}
			continue
		}
	}

	if len(input) == 0 {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			input = append(input, scanner.Text())
		}
	}

	if count == 0 {
		count = len(input)
	}
	if repeat {
		for i := 0; i < count; i++ {
			fmt.Println(input[i%len(input)])
		}
	} else {
		for i := len(input) - 1; i > 0; i-- {
			j := i
			input[i], input[j] = input[j], input[i]
		}
		for i := 0; i < count && i < len(input); i++ {
			fmt.Println(input[i])
		}
	}
	return 0
}

func runSeq(args []string) int {
	start, end, step := 1.0, 0.0, 1.0
	separator := "\n"
	width := false

	args = args[1:]
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "seq: missing operand\n")
		return 1
	}

	cleaned := []string{}
	for i := 0; i < len(args); i++ {
		if args[i] == "-s" && i+1 < len(args) {
			i++
			separator = args[i]
			continue
		}
		if args[i] == "-w" {
			width = true
			continue
		}
		cleaned = append(cleaned, args[i])
	}

	switch len(cleaned) {
	case 1:
		end = parseFloat(cleaned[0])
	case 2:
		start = parseFloat(cleaned[0])
		end = parseFloat(cleaned[1])
	case 3:
		start = parseFloat(cleaned[0])
		step = parseFloat(cleaned[1])
		end = parseFloat(cleaned[2])
	}

	if step == 0 {
		return 1
	}
	if step > 0 && start > end {
		return 0
	}
	if step < 0 && start < end {
		return 0
	}

	_ = width
	for i := start; (step > 0 && i <= end) || (step < 0 && i >= end); i += step {
		if i != start {
			fmt.Print(separator)
		}
		if i == float64(int(i)) {
			fmt.Printf("%d", int(i))
		} else {
			fmt.Printf("%g", i)
		}
	}
	fmt.Println()
	return 0
}

func parseFloat(s string) float64 {
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}

func init() {
	applet.Register(&applet.Applet{Name: "printf", Short: "Format and print data", Func: runPrintf})
}

func runPrintf(args []string) int {
	if len(args) < 2 {
		return 0
	}
	format := args[1]
	args = args[2:]

	result := ""
	argIdx := 0
	for i := 0; i < len(format); i++ {
		if format[i] == '%' && i+1 < len(format) {
			i++
			switch format[i] {
			case 's':
				if argIdx < len(args) {
					result += args[argIdx]
					argIdx++
				}
			case 'd':
				if argIdx < len(args) {
					var n int
					fmt.Sscanf(args[argIdx], "%d", &n)
					result += fmt.Sprintf("%d", n)
					argIdx++
				}
			case 'f':
				if argIdx < len(args) {
					var n float64
					fmt.Sscanf(args[argIdx], "%f", &n)
					result += fmt.Sprintf("%f", n)
					argIdx++
				}
			case '%':
				result += "%"
			default:
				result += "%" + string(format[i])
			}
		} else {
			result += string(format[i])
		}
	}
	fmt.Print(result)
	return 0
}
