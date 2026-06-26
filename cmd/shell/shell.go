package shell

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/agentbusybox/pkg/applet"
)

func init() {
	applet.Register(&applet.Applet{Name: "ash", Short: "Almquist shell", Func: runSh})
	applet.Register(&applet.Applet{Name: "bash", Short: "Bourne Again shell compatibility alias", Func: runSh})
	applet.Register(&applet.Applet{Name: "lash", Short: "Legacy shell compatibility alias", Func: runSh})
	applet.Register(&applet.Applet{Name: "sh", Short: "Bourne shell", Func: runSh})
	applet.Register(&applet.Applet{Name: "hush", Short: "Hush shell", Func: runSh})
}

func runSh(args []string) int {
	scriptFile := ""
	command := ""
	interactive := true

	for _, a := range args[1:] {
		if a == "-c" {
			interactive = false
			continue
		}
		if !strings.HasPrefix(a, "-") {
			if !interactive {
				command = a
			} else {
				scriptFile = a
			}
		}
	}

	if scriptFile != "" {
		return runScript(scriptFile)
	}
	if command != "" {
		return runCommand(command)
	}
	return runInteractive()
}

func runScript(path string) int {
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
	reader := bufio.NewReader(os.Stdin)
	for {
		dir, _ := os.Getwd()
		if len(dir) > 30 {
			dir = "..." + dir[len(dir)-27:]
		}
		fmt.Printf("%s$ ", dir)

		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if line == "exit" {
			break
		}
		executeLine(line)
	}
	return 0
}

func executeLine(line string) int {
	// Handle variable expansion
	line = expandVars(line)

	// Handle pipes
	if strings.Contains(line, "|") {
		return executePipe(line)
	}

	// Handle redirection
	if strings.Contains(line, ">") || strings.Contains(line, "<") {
		return executeRedirect(line)
	}

	// Parse command and arguments
	args := parseArgs(line)
	if len(args) == 0 {
		return 0
	}

	// Handle built-in commands
	switch args[0] {
	case "cd":
		return builtinCd(args)
	case "export":
		return builtinExport(args)
	case "set":
		return builtinSet(args)
	case "unset":
		return builtinUnset(args)
	case "echo":
		return builtinEcho(args)
	case "read":
		return builtinRead(args)
	case "source", ".":
		if len(args) > 1 {
			return runScript(args[1])
		}
		return 0
	case "type":
		return builtinType(args)
	case "alias":
		return builtinAlias(args)
	case "unalias":
		return builtinUnalias(args)
	case "trap":
		return builtinTrap(args)
	case "shift":
		return builtinShift(args)
	case "exit":
		code := 0
		if len(args) > 1 {
			fmt.Sscanf(args[1], "%d", &code)
		}
		os.Exit(code)
	case "exec":
		if len(args) > 1 {
			cmd := exec.Command(args[1], args[2:]...)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					return exitErr.ExitCode()
				}
				return 1
			}
		}
		return 0
	}

	// Execute external command
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
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
					result += os.Getenv(varName)
					i += end + 1
					continue
				}
			}
			// Simple variable
			end := i
			for end < len(line) && (isAlphaNum(line[end]) || line[end] == '_') {
				end++
			}
			varName := line[i:end]
			result += os.Getenv(varName)
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
		return executeLine(strings.TrimSpace(commands[0]))
	}

	var prevOutput []byte
	exitCode := 0

	for i, cmdStr := range commands {
		cmdStr = strings.TrimSpace(cmdStr)
		args := parseArgs(cmdStr)
		if len(args) == 0 {
			continue
		}

		cmd := exec.Command(args[0], args[1:]...)
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
	// Simple redirect handling
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

		cmd := exec.Command(args[0], args[1:]...)
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

		cmd := exec.Command(args[0], args[1:]...)
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

		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdin = f
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return 1
		}
		return 0
	}
	return executeLine(strings.TrimSpace(line))
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
	for _, a := range args[1:] {
		parts := strings.SplitN(a, "=", 2)
		if len(parts) == 2 {
			os.Setenv(parts[0], parts[1])
		}
	}
	return 0
}

func builtinSet(args []string) int {
	for _, e := range os.Environ() {
		fmt.Println(e)
	}
	return 0
}

func builtinUnset(args []string) int {
	for _, a := range args[1:] {
		os.Unsetenv(a)
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
	os.Setenv(varName, line)
	return 0
}

func builtinType(args []string) int {
	for _, name := range args[1:] {
		// Check builtins
		builtins := []string{"cd", "export", "set", "unset", "echo", "read", "source", ".", "type", "alias", "unalias", "trap", "shift", "exit", "exec"}
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
	// Register some more applets that don't fit elsewhere
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
		// Fisher-Yates shuffle
		for i := len(input) - 1; i > 0; i-- {
			j := i // simplified - not truly random
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

	// Parse flags
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
