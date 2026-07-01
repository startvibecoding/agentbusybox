package shell

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/agentbusybox/pkg/applet"
)

// Shell state
var (
	lastExitCode     int = 0
	positionalParams []string
	shellName        string            = "sh"
	shellVars        map[string]string = make(map[string]string)
	commandHistory   []string
	shellFunctions   map[string][]string = make(map[string][]string)
	localVars        map[string]string   = make(map[string]string)
	trapHandlers     map[string]string   = make(map[string]string)
	aliases          map[string]string   = make(map[string]string)
	breakLevel       int                 = 0
	continueLevel    int                 = 0
	returnFlag       bool
	returnCode       int
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

	lines := readScriptLines(f)
	return executeScriptLines(lines)
}

func readScriptLines(f *os.File) []string {
	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}

func executeScriptLines(lines []string) int {
	exitCode := 0
	i := 0
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.HasPrefix(line, "#") {
			i++
			continue
		}

		// Handle multi-line constructs
		if strings.HasPrefix(line, "if ") || strings.HasPrefix(line, "if\t") ||
			strings.HasPrefix(line, "for ") || strings.HasPrefix(line, "for\t") ||
			strings.HasPrefix(line, "while ") || strings.HasPrefix(line, "while\t") ||
			strings.HasPrefix(line, "until ") || strings.HasPrefix(line, "until\t") ||
			strings.HasPrefix(line, "case ") || strings.HasPrefix(line, "case\t") ||
			strings.HasPrefix(line, "function ") || (strings.Contains(line, "()") && !strings.HasPrefix(line, "(")) {
			block, consumed := collectBlock(lines, i)
			exitCode = executeBlock(block)
			i += consumed
		} else {
			exitCode = executeLine(line)
			i++
		}

		if returnFlag {
			return returnCode
		}
		// Don't decrement break/continue here - let the caller handle it
		if breakLevel > 0 {
			break
		}
		if continueLevel > 0 {
			break
		}
	}
	return exitCode
}

func collectBlock(lines []string, start int) ([]string, int) {
	block := []string{lines[start]}
	depth := 1
	i := start + 1

	for i < len(lines) && depth > 0 {
		line := strings.TrimSpace(lines[i])
		block = append(block, lines[i])

		// Count nesting
		if isBlockStart(line) {
			depth++
		}
		if isBlockEnd(line) {
			depth--
		}
		i++
	}

	return block, i - start
}

func isBlockStart(line string) bool {
	starters := []string{"if ", "if\t", "for ", "for\t", "while ", "while\t", "until ", "until\t", "case ", "case\t", "function "}
	for _, s := range starters {
		if strings.HasPrefix(line, s) {
			return true
		}
	}
	return false
}

func isBlockEnd(line string) bool {
	ends := []string{"fi", "fi;", "done", "done;", "esac", "esac;", "}", "};"}
	for _, e := range ends {
		if line == e || strings.HasPrefix(line, e+" ") || strings.HasPrefix(line, e+"\t") {
			return true
		}
	}
	return false
}

func executeBlock(block []string) int {
	if len(block) == 0 {
		return 0
	}
	first := strings.TrimSpace(block[0])

	if strings.HasPrefix(first, "if ") || strings.HasPrefix(first, "if\t") {
		return executeIfBlock(block)
	}
	if strings.HasPrefix(first, "for ") || strings.HasPrefix(first, "for\t") {
		return executeForBlock(block)
	}
	if strings.HasPrefix(first, "while ") || strings.HasPrefix(first, "while\t") {
		return executeWhileBlock(block, false)
	}
	if strings.HasPrefix(first, "until ") || strings.HasPrefix(first, "until\t") {
		return executeWhileBlock(block, true)
	}
	if strings.HasPrefix(first, "case ") || strings.HasPrefix(first, "case\t") {
		return executeCaseBlock(block)
	}
	if strings.HasPrefix(first, "function ") || (strings.Contains(first, "()") && !strings.HasPrefix(first, "(")) {
		return executeFunctionDef(block)
	}

	return 0
}

// executeIfBlock handles: if ... then ... elif ... then ... else ... fi
func executeIfBlock(block []string) int {
	i := 0
	conditionMet := false
	exitCode := 0

	for i < len(block) {
		line := strings.TrimSpace(block[i])

		if strings.HasPrefix(line, "if ") || strings.HasPrefix(line, "if\t") {
			cond := strings.TrimPrefix(line, "if ")
			if strings.HasPrefix(line, "if\t") {
				cond = strings.TrimPrefix(line, "if\t")
			}
			// Remove trailing "then" if on same line
			hasThenOnSameLine := strings.HasSuffix(strings.TrimSpace(cond), "; then") ||
				strings.HasSuffix(strings.TrimSpace(cond), ";then") ||
				strings.HasSuffix(strings.TrimSpace(cond), "then")
			cond = strings.TrimSuffix(strings.TrimSpace(cond), "; then")
			cond = strings.TrimSuffix(strings.TrimSpace(cond), ";then")
			cond = strings.TrimSuffix(strings.TrimSpace(cond), "then")

			condCode := executeLine(strings.TrimSpace(cond))
			i++

			// Skip "then" line if not already on same line
			if !hasThenOnSameLine {
				for i < len(block) {
					l := strings.TrimSpace(block[i])
					if l == "then" || strings.HasPrefix(l, "then ") || strings.HasPrefix(l, "then\t") {
						i++
						break
					}
					i++
				}
			}

			// Collect lines until elif/else/fi
			var thenBlock []string
			depth := 1
			for i < len(block) {
				l := strings.TrimSpace(block[i])
				if l == "fi" || l == "fi;" {
					depth--
					if depth == 0 {
						i++
						break
					}
				}
				if (l == "elif" || strings.HasPrefix(l, "elif ") || strings.HasPrefix(l, "elif\t") ||
					l == "else" || strings.HasPrefix(l, "else ") || strings.HasPrefix(l, "else\t")) && depth == 1 {
					break
				}
				if isBlockStart(l) {
					depth++
				}
				if isBlockEnd(l) {
					depth--
				}
				thenBlock = append(thenBlock, block[i])
				i++
			}

			if condCode == 0 {
				conditionMet = true
				exitCode = executeScriptLines(thenBlock)
			}

		} else if (line == "elif" || strings.HasPrefix(line, "elif ") || strings.HasPrefix(line, "elif\t")) && !conditionMet {
			cond := strings.TrimPrefix(line, "elif ")
			if strings.HasPrefix(line, "elif\t") {
				cond = strings.TrimPrefix(line, "elif\t")
			}
			// Remove trailing "then" if on same line
			hasThenOnSameLine := strings.HasSuffix(strings.TrimSpace(cond), "; then") ||
				strings.HasSuffix(strings.TrimSpace(cond), ";then") ||
				strings.HasSuffix(strings.TrimSpace(cond), "then")
			cond = strings.TrimSuffix(strings.TrimSpace(cond), "; then")
			cond = strings.TrimSuffix(strings.TrimSpace(cond), ";then")
			cond = strings.TrimSuffix(strings.TrimSpace(cond), "then")

			condCode := executeLine(strings.TrimSpace(cond))
			i++

			// Skip "then" line if not already on same line
			if !hasThenOnSameLine {
				for i < len(block) {
					l := strings.TrimSpace(block[i])
					if l == "then" || strings.HasPrefix(l, "then ") || strings.HasPrefix(l, "then\t") {
						i++
						break
					}
					i++
				}
			}

			// Collect elif block
			var elifBlock []string
			depth := 1
			for i < len(block) {
				l := strings.TrimSpace(block[i])
				if l == "fi" || l == "fi;" {
					depth--
					if depth == 0 {
						i++
						break
					}
				}
				if (l == "elif" || strings.HasPrefix(l, "elif ") || strings.HasPrefix(l, "elif\t") ||
					l == "else" || strings.HasPrefix(l, "else ") || strings.HasPrefix(l, "else\t")) && depth == 1 {
					break
				}
				if isBlockStart(l) {
					depth++
				}
				if isBlockEnd(l) {
					depth--
				}
				elifBlock = append(elifBlock, block[i])
				i++
			}

			if condCode == 0 {
				conditionMet = true
				exitCode = executeScriptLines(elifBlock)
			}

		} else if (line == "else" || strings.HasPrefix(line, "else ") || strings.HasPrefix(line, "else\t")) && !conditionMet {
			i++
			// Skip "else" keyword
			var elseBlock []string
			depth := 1
			for i < len(block) {
				l := strings.TrimSpace(block[i])
				if l == "fi" || l == "fi;" {
					depth--
					if depth == 0 {
						i++
						break
					}
				}
				if isBlockStart(l) {
					depth++
				}
				if isBlockEnd(l) {
					depth--
				}
				elseBlock = append(elseBlock, block[i])
				i++
			}
			exitCode = executeScriptLines(elseBlock)
		} else if line == "fi" || line == "fi;" {
			i++
		} else {
			i++
		}
	}

	return exitCode
}

// executeForBlock handles: for VAR in ... do ... done
func executeForBlock(block []string) int {
	// Parse "for VAR in WORDS"
	first := strings.TrimSpace(block[0])
	first = strings.TrimPrefix(first, "for ")
	first = strings.TrimPrefix(first, "for\t")

	parts := strings.SplitN(first, " in ", 2)
	if len(parts) < 2 {
		parts = strings.SplitN(first, "\tin\t", 2)
	}
	if len(parts) < 2 {
		// "for VAR" without "in" - iterate over positional params
		varName := strings.TrimSpace(parts[0])
		varName = strings.TrimSuffix(varName, "; do")
		varName = strings.TrimSuffix(varName, ";do")
		varName = strings.TrimSpace(varName)
		return executeForLoop(block[1:], varName, positionalParams)
	}

	varName := strings.TrimSpace(parts[0])
	wordsStr := strings.TrimSpace(parts[1])
	wordsStr = strings.TrimSuffix(wordsStr, "; do")
	wordsStr = strings.TrimSuffix(wordsStr, ";do")
	wordsStr = strings.TrimSpace(wordsStr)

	// Parse word list and expand globs
	words := parseWordList(wordsStr)
	words = expandGlobs(words)

	// If "do" is on the same line, the rest of the block starts from the next line
	restBlock := block[1:]
	if strings.Contains(strings.TrimSpace(parts[1]), "; do") || strings.Contains(strings.TrimSpace(parts[1]), ";do") {
		// "do" is on the same line, rest of block starts from next line
	} else {
		// Check if the next line is "do"
		if len(block) > 1 {
			nextLine := strings.TrimSpace(block[1])
			if nextLine == "do" || nextLine == "do;" || strings.HasPrefix(nextLine, "do ") || strings.HasPrefix(nextLine, "do\t") {
				restBlock = block[2:]
			}
		}
	}

	return executeForLoop(restBlock, varName, words)
}

func parseWordList(s string) []string {
	var words []string
	current := ""
	inSingle, inDouble := false, false

	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch == '\'' && !inDouble:
			inSingle = !inSingle
		case ch == '"' && !inSingle:
			inDouble = !inDouble
		case ch == ' ' && !inSingle && !inDouble:
			if current != "" {
				words = append(words, current)
				current = ""
			}
		default:
			current += string(ch)
		}
	}
	if current != "" {
		words = append(words, current)
	}
	return words
}

func expandGlobs(words []string) []string {
	var result []string
	for _, word := range words {
		if strings.ContainsAny(word, "*?") {
			matches, err := filepath.Glob(word)
			if err == nil && len(matches) > 0 {
				result = append(result, matches...)
			} else {
				result = append(result, word)
			}
		} else {
			result = append(result, word)
		}
	}
	return result
}

func executeForLoop(block []string, varName string, words []string) int {
	// Find do/done
	var body []string
	inDo := false
	for _, line := range block {
		l := strings.TrimSpace(line)
		if l == "do" || l == "do;" || strings.HasPrefix(l, "do ") || strings.HasPrefix(l, "do\t") {
			inDo = true
			continue
		}
		if l == "done" || l == "done;" {
			break
		}
		if inDo {
			body = append(body, line)
		}
	}

	// If no "do" keyword was found, treat entire block as body (minus "done")
	if !inDo {
		for _, line := range block {
			l := strings.TrimSpace(line)
			if l == "done" || l == "done;" {
				break
			}
			body = append(body, line)
		}
	}

	exitCode := 0
	for _, word := range words {
		shellVars[varName] = word
		exitCode = executeScriptLines(body)

		if returnFlag {
			return returnCode
		}
		if breakLevel > 0 {
			breakLevel--
			break
		}
		if continueLevel > 0 {
			continueLevel--
			continue
		}
	}
	return exitCode
}

// executeWhileBlock handles: while/until ... do ... done
func executeWhileBlock(block []string, isUntil bool) int {
	// Parse condition
	first := strings.TrimSpace(block[0])
	if isUntil {
		first = strings.TrimPrefix(first, "until ")
		first = strings.TrimPrefix(first, "until\t")
	} else {
		first = strings.TrimPrefix(first, "while ")
		first = strings.TrimPrefix(first, "while\t")
	}

	// Check if "do" is on the same line
	hasDoOnSameLine := strings.HasSuffix(first, "; do") || strings.HasSuffix(first, ";do") ||
		strings.HasSuffix(first, "do")
	first = strings.TrimSuffix(first, "; do")
	first = strings.TrimSuffix(first, ";do")
	first = strings.TrimSuffix(first, "do")
	first = strings.TrimSuffix(first, ";")
	condition := strings.TrimSpace(first)

	// Find do/done
	var body []string
	inDo := hasDoOnSameLine // If do is on same line, we're already in the body
	for i, line := range block {
		if i == 0 {
			continue // Skip the while/until line
		}
		l := strings.TrimSpace(line)
		if l == "do" || l == "do;" || strings.HasPrefix(l, "do ") || strings.HasPrefix(l, "do\t") {
			inDo = true
			continue
		}
		if l == "done" || l == "done;" {
			break
		}
		if inDo {
			body = append(body, line)
		}
	}

	exitCode := 0
	for {
		condCode := executeLine(condition)
		shouldRun := (condCode == 0) != isUntil

		if !shouldRun {
			break
		}

		exitCode = executeScriptLines(body)

		if returnFlag {
			return returnCode
		}
		if breakLevel > 0 {
			breakLevel--
			break
		}
		if continueLevel > 0 {
			continueLevel--
			continue
		}
	}
	return exitCode
}

// executeCaseBlock handles: case WORD in PATTERN) ... ;; esac
func executeCaseBlock(block []string) int {
	// Parse case word
	first := strings.TrimSpace(block[0])
	first = strings.TrimPrefix(first, "case ")
	first = strings.TrimPrefix(first, "case\t")
	first = strings.TrimSuffix(first, " in")
	first = strings.TrimSuffix(first, "in")
	word := expandVars(strings.TrimSpace(first))
	// Strip surrounding quotes from expanded word
	word = strings.Trim(word, "\"")
	word = strings.Trim(word, "'")

	// Parse patterns and execute matching block
	i := 1
	exitCode := 0
	for i < len(block) {
		line := strings.TrimSpace(block[i])

		if line == "esac" || line == "esac;" {
			break
		}

		// Check for pattern: PATTERN) or PATTERN | PATTERN)
		if strings.HasSuffix(line, ")") {
			pattern := strings.TrimSuffix(line, ")")
			pattern = strings.TrimSpace(pattern)

			if matchCasePattern(word, pattern) {
				// Collect commands until ;;
				i++
				var cmdLines []string
				for i < len(block) {
					l := strings.TrimSpace(block[i])
					if l == ";;" || l == "esac" || l == "esac;" {
						break
					}
					// Check for next pattern
					if strings.HasSuffix(l, ")") && !strings.Contains(l, " ") {
						break
					}
					cmdLines = append(cmdLines, block[i])
					i++
				}
				exitCode = executeScriptLines(cmdLines)
				return exitCode
			}

			// Skip until ;;
			i++
			for i < len(block) {
				l := strings.TrimSpace(block[i])
				if l == ";;" {
					i++
					break
				}
				if l == "esac" || l == "esac;" {
					break
				}
				i++
			}
		} else {
			i++
		}
	}
	return exitCode
}

func matchCasePattern(word, pattern string) bool {
	// Simple glob matching
	if pattern == "*" {
		return true
	}
	if pattern == word {
		return true
	}

	// Handle ? (single char) and * (any chars)
	return matchGlob(word, pattern)
}

func matchGlob(word, pattern string) bool {
	if pattern == "*" {
		return true
	}
	if pattern == "" {
		return word == ""
	}
	if word == "" {
		return pattern == "*" || pattern == ""
	}

	if pattern[0] == '*' {
		// Try matching * with any prefix
		for i := 0; i <= len(word); i++ {
			if matchGlob(word[i:], pattern[1:]) {
				return true
			}
		}
		return false
	}

	if pattern[0] == '?' || pattern[0] == word[0] {
		return matchGlob(word[1:], pattern[1:])
	}

	return false
}

// executeFunctionDef handles: function NAME() { ... } or NAME() { ... }
func executeFunctionDef(block []string) int {
	first := strings.TrimSpace(block[0])

	// Parse function name
	name := ""
	if strings.HasPrefix(first, "function ") {
		name = strings.TrimPrefix(first, "function ")
		name = strings.TrimSpace(name)
		name = strings.TrimSuffix(name, "()")
		name = strings.TrimSpace(name)
	} else if strings.Contains(first, "()") {
		name = strings.Split(first, "()")[0]
		name = strings.TrimSpace(name)
	}

	if name == "" {
		fmt.Fprintf(os.Stderr, "sh: invalid function definition\n")
		return 1
	}

	// Collect function body (skip { and })
	var body []string
	inBody := false
	// Check if { is on the same line as the function name
	if strings.Contains(first, "{") {
		inBody = true
	}
	for _, line := range block[1:] {
		l := strings.TrimSpace(line)
		if l == "{" || strings.HasPrefix(l, "{ ") {
			inBody = true
			continue
		}
		if l == "}" || l == "};" {
			break
		}
		if inBody {
			body = append(body, line)
		}
	}

	shellFunctions[name] = body
	return 0
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

// stripComments removes inline comments (unquoted # preceded by space)
func stripComments(line string) string {
	inSingle, inDouble := false, false
	for i := 0; i < len(line); i++ {
		switch {
		case line[i] == '\'' && !inDouble:
			inSingle = !inSingle
		case line[i] == '"' && !inSingle:
			inDouble = !inDouble
		case line[i] == '#' && !inSingle && !inDouble:
			if i == 0 || line[i-1] == ' ' || line[i-1] == '\t' {
				return strings.TrimRight(line[:i], " \t")
			}
		}
	}
	return line
}

func executeSinglePipeline(line string) int {
	line = stripComments(line)
	line = expandVars(line)

	// Handle here documents
	if strings.Contains(line, "<<") {
		return executeHereDoc(line)
	}

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

	// Check for function call
	if _, exists := shellFunctions[cmdArgs[0]]; exists {
		return executeFunction(cmdArgs[0], cmdArgs[1:])
	}

	switch cmdArgs[0] {
	case "cd":
		return builtinCd(cmdArgs)
	case "export":
		return builtinExport(cmdArgs)
	case "local":
		return builtinLocal(cmdArgs)
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
	case "return":
		return builtinReturn(cmdArgs)
	case "break":
		return builtinBreak(cmdArgs)
	case "continue":
		return builtinContinue(cmdArgs)
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
	case "eval":
		if len(cmdArgs) > 1 {
			return executeLine(strings.Join(cmdArgs[1:], " "))
		}
		return 0
	case "true":
		return 0
	case "false":
		return 1
	case "test", "[":
		return builtinTest(cmdArgs)
	case "getopts":
		return builtinGetopts(cmdArgs)
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

func executeFunction(name string, args []string) int {
	body, exists := shellFunctions[name]
	if !exists {
		fmt.Fprintf(os.Stderr, "sh: %s: function not found\n", name)
		return 1
	}

	// Save and restore positional params and local vars
	oldParams := positionalParams
	oldReturnFlag := returnFlag
	returnFlag = false
	oldLocalVars := localVars
	localVars = make(map[string]string)

	positionalParams = args
	exitCode := executeScriptLines(body)

	// Restore local variables
	for k, v := range localVars {
		if v == "__UNSET__" {
			delete(shellVars, k)
		} else {
			shellVars[k] = v
		}
	}
	localVars = oldLocalVars
	returnFlag = oldReturnFlag
	positionalParams = oldParams

	return exitCode
}

func expandVars(line string) string {
	result := ""
	i := 0
	for i < len(line) {
		if line[i] == '$' && i+1 < len(line) {
			i++
			if line[i] == '(' && i+1 < len(line) && line[i+1] == '(' {
				// Arithmetic expansion $(())
				end := strings.Index(line[i:], "))")
				if end >= 0 {
					expr := line[i+2 : i+end]
					val := evalArithmetic(expr)
					result += strconv.Itoa(val)
					i += end + 2
					continue
				}
			}
			if line[i] == '(' {
				// Command substitution $()
				end := findMatchingParen(line, i)
				if end >= 0 {
					cmd := line[i+1 : end]
					out := captureCommandOutput(cmd)
					result += strings.TrimRight(out, "\n")
					i = end + 1
					continue
				}
			}
			if line[i] == '{' {
				end := strings.IndexByte(line[i:], '}')
				if end >= 0 {
					varName := line[i+1 : i+end]
					// Handle ${VAR:-default}, ${VAR:=default}, ${VAR:+alt}, ${VAR:?err}
					if idx := strings.Index(varName, ":-"); idx >= 0 {
						name := varName[:idx]
						def := varName[idx+2:]
						val := expandSingleVar(name)
						if val == "" {
							val = expandVars(def)
						}
						result += val
					} else if idx := strings.Index(varName, ":="); idx >= 0 {
						name := varName[:idx]
						def := varName[idx+2:]
						val := expandSingleVar(name)
						if val == "" {
							val = expandVars(def)
							shellVars[name] = val
						}
						result += val
					} else if idx := strings.Index(varName, ":+"); idx >= 0 {
						name := varName[:idx]
						alt := varName[idx+2:]
						val := expandSingleVar(name)
						if val != "" {
							result += expandVars(alt)
						}
					} else if idx := strings.Index(varName, ":?"); idx >= 0 {
						name := varName[:idx]
						err := varName[idx+2:]
						val := expandSingleVar(name)
						if val == "" {
							fmt.Fprintf(os.Stderr, "sh: %s: %s\n", name, expandVars(err))
							return ""
						}
						result += val
					} else if strings.HasSuffix(varName, "#") {
						// ${VAR#pattern} - remove shortest prefix
						name := varName[:len(varName)-1]
						val := expandSingleVar(name)
						result += val
					} else if strings.HasSuffix(varName, "##") {
						// ${VAR##pattern} - remove longest prefix
						name := varName[:len(varName)-2]
						val := expandSingleVar(name)
						result += val
					} else if strings.HasSuffix(varName, "%") {
						// ${VAR%pattern} - remove shortest suffix
						name := varName[:len(varName)-1]
						val := expandSingleVar(name)
						result += val
					} else if strings.HasSuffix(varName, "%%") {
						// ${VAR%%pattern} - remove longest suffix
						name := varName[:len(varName)-2]
						val := expandSingleVar(name)
						result += val
					} else {
						result += expandSingleVar(varName)
					}
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
		} else if line[i] == '\\' && i+1 < len(line) {
			// Escape sequence
			result += string(line[i+1])
			i += 2
		} else {
			result += string(line[i])
			i++
		}
	}
	return result
}

func findMatchingParen(s string, start int) int {
	if s[start] != '(' {
		return -1
	}
	depth := 1
	for i := start + 1; i < len(s); i++ {
		if s[i] == '(' {
			depth++
		} else if s[i] == ')' {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func captureCommandOutput(cmd string) string {
	// Create temp script
	tmpFile, err := os.CreateTemp("", "sh-subshell-*.sh")
	if err != nil {
		return ""
	}
	defer os.Remove(tmpFile.Name())

	tmpFile.WriteString(cmd)
	tmpFile.Close()

	self, err := os.Executable()
	if err != nil {
		self = "/proc/self/exe"
	}

	out, _ := exec.Command(self, tmpFile.Name()).Output()
	return string(out)
}

func evalArithmetic(expr string) int {
	// Simple arithmetic evaluation
	expr = strings.TrimSpace(expr)

	// Handle $(()) nesting
	for strings.Contains(expr, "$(") {
		start := strings.Index(expr, "$(")
		if start >= 0 {
			end := findMatchingParen(expr[start:], 0)
			if end >= 0 {
				inner := expr[start+2 : start+end]
				val := evalArithmetic(inner)
				expr = expr[:start] + strconv.Itoa(val) + expr[start+end+1:]
			}
		}
	}

	// Try to evaluate as simple expression
	var result int
	_, err := fmt.Sscanf(expr, "%d", &result)
	if err != nil {
		// Not a number - try to resolve as variable
		expr = strings.TrimSpace(expr)
		if isValidVarName(expr) {
			val := expandSingleVar(expr)
			if val != "" {
				fmt.Sscanf(val, "%d", &result)
			}
		}
	}

	// Handle basic operations
	if strings.Contains(expr, "+") {
		parts := strings.SplitN(expr, "+", 2)
		left := evalArithmetic(parts[0])
		right := evalArithmetic(parts[1])
		return left + right
	}
	if strings.Contains(expr, "-") {
		parts := strings.SplitN(expr, "-", 2)
		left := evalArithmetic(parts[0])
		right := evalArithmetic(parts[1])
		return left - right
	}
	if strings.Contains(expr, "*") {
		parts := strings.SplitN(expr, "*", 2)
		left := evalArithmetic(parts[0])
		right := evalArithmetic(parts[1])
		return left * right
	}
	if strings.Contains(expr, "/") {
		parts := strings.SplitN(expr, "/", 2)
		left := evalArithmetic(parts[0])
		right := evalArithmetic(parts[1])
		if right != 0 {
			return left / right
		}
		return 0
	}
	if strings.Contains(expr, "%") {
		parts := strings.SplitN(expr, "%", 2)
		left := evalArithmetic(parts[0])
		right := evalArithmetic(parts[1])
		if right != 0 {
			return left % right
		}
		return 0
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
		case ch == '\\' && i+1 < len(line) && !inSingle:
			// Escape next char in double quotes or outside quotes
			if inDouble || !inDouble {
				current += string(line[i+1])
				i++
			}
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
	// Handle stderr redirect 2>
	if strings.Contains(line, "2>>") {
		parts := strings.SplitN(line, "2>>", 2)
		cmdStr := strings.TrimSpace(parts[0])
		fileName := expandVars(strings.TrimSpace(parts[1]))
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
		cmd.Stdout = os.Stdout
		cmd.Stderr = f
		if err := cmd.Run(); err != nil {
			return 1
		}
		return 0
	}
	if strings.Contains(line, "2>") {
		parts := strings.SplitN(line, "2>", 2)
		cmdStr := strings.TrimSpace(parts[0])
		fileName := expandVars(strings.TrimSpace(parts[1]))
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
		cmd.Stdout = os.Stdout
		cmd.Stderr = f
		if err := cmd.Run(); err != nil {
			return 1
		}
		return 0
	}

	// Handle stdout redirect
	if strings.Contains(line, ">>") {
		parts := strings.SplitN(line, ">>", 2)
		cmdStr := strings.TrimSpace(parts[0])
		fileName := expandVars(strings.TrimSpace(parts[1]))
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
		fileName := expandVars(strings.TrimSpace(parts[1]))
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
		fileName := expandVars(strings.TrimSpace(parts[1]))
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

func executeHereDoc(line string) int {
	// Simplified here doc support
	parts := strings.SplitN(line, "<<", 2)
	if len(parts) < 2 {
		return executeSinglePipeline(strings.TrimSpace(parts[0]))
	}
	cmdStr := strings.TrimSpace(parts[0])
	delimiter := strings.TrimSpace(parts[1])
	delimiter = strings.TrimPrefix(delimiter, "-")
	delimiter = strings.Trim(delimiter, "'\"")

	args := parseArgs(cmdStr)
	if len(args) == 0 {
		return 0
	}

	// Read here doc content from stdin (simplified)
	var content strings.Builder
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		l := scanner.Text()
		if l == delimiter {
			break
		}
		content.WriteString(l + "\n")
	}

	cmd := prepareCmd(args[0], args[1:])
	cmd.Stdin = strings.NewReader(content.String())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return 1
	}
	return 0
}

// Built-in commands

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

func builtinLocal(args []string) int {
	for _, a := range args[1:] {
		parts := strings.SplitN(a, "=", 2)
		if len(parts) == 2 {
			// Save old value if not already saved
			if _, exists := localVars[parts[0]]; !exists {
				if oldVal, ok := shellVars[parts[0]]; ok {
					localVars[parts[0]] = oldVal
				} else {
					localVars[parts[0]] = "__UNSET__"
				}
			}
			shellVars[parts[0]] = parts[1]
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
		// Check builtins
		builtins := []string{"cd", "export", "local", "set", "unset", "echo", "read", "pwd", "history", "source", ".", "type", "alias", "unalias", "trap", "shift", "return", "break", "continue", "exit", "exec", "eval", "true", "false", "test", "[", "getopts"}
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

		// Check functions
		if _, exists := shellFunctions[name]; exists {
			fmt.Printf("%s is a function\n", name)
			continue
		}

		// Check PATH
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
	if len(args) == 1 {
		for k, v := range aliases {
			fmt.Printf("alias %s='%s'\n", k, v)
		}
		return 0
	}
	for _, a := range args[1:] {
		if idx := strings.IndexByte(a, '='); idx >= 0 {
			name := a[:idx]
			val := a[idx+1:]
			aliases[name] = val
		} else {
			if val, exists := aliases[a]; exists {
				fmt.Printf("alias %s='%s'\n", a, val)
			}
		}
	}
	return 0
}

func builtinUnalias(args []string) int {
	for _, a := range args[1:] {
		delete(aliases, a)
	}
	return 0
}

func builtinTrap(args []string) int {
	if len(args) == 1 {
		for sig, cmd := range trapHandlers {
			fmt.Printf("trap -- '%s' %s\n", cmd, sig)
		}
		return 0
	}
	if len(args) >= 3 {
		cmd := args[1]
		for _, sig := range args[2:] {
			trapHandlers[sig] = cmd
		}
	}
	return 0
}

func builtinShift(args []string) int {
	n := 1
	if len(args) > 1 {
		fmt.Sscanf(args[1], "%d", &n)
	}
	if n > len(positionalParams) {
		fmt.Fprintf(os.Stderr, "shift: shift count out of range\n")
		return 1
	}
	positionalParams = positionalParams[n:]
	return 0
}

func builtinReturn(args []string) int {
	code := 0
	if len(args) > 1 {
		fmt.Sscanf(args[1], "%d", &code)
	}
	returnFlag = true
	returnCode = code
	return code
}

func builtinBreak(args []string) int {
	n := 1
	if len(args) > 1 {
		fmt.Sscanf(args[1], "%d", &n)
	}
	breakLevel = n
	return 0
}

func builtinContinue(args []string) int {
	n := 1
	if len(args) > 1 {
		fmt.Sscanf(args[1], "%d", &n)
	}
	continueLevel = n
	return 0
}

func builtinTest(args []string) int {
	// Handle [ ... ] syntax
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

	return evalTestExpr(a)
}

func evalTestExpr(args []string) int {
	if len(args) == 0 {
		return 1
	}

	// Unary operators
	if len(args) == 2 {
		op, arg := args[0], args[1]
		switch op {
		case "-f":
			info, err := os.Stat(arg)
			if err != nil {
				return 1
			}
			if info.Mode().IsRegular() {
				return 0
			}
			return 1
		case "-d":
			info, err := os.Stat(arg)
			if err != nil {
				return 1
			}
			if info.IsDir() {
				return 0
			}
			return 1
		case "-e":
			_, err := os.Stat(arg)
			if err != nil {
				return 1
			}
			return 0
		case "-r":
			f, err := os.Open(arg)
			if err != nil {
				return 1
			}
			f.Close()
			return 0
		case "-w":
			info, err := os.Stat(arg)
			if err != nil {
				return 1
			}
			if info.Mode()&0200 != 0 {
				return 0
			}
			return 1
		case "-x":
			info, err := os.Stat(arg)
			if err != nil {
				return 1
			}
			if info.Mode()&0100 != 0 {
				return 0
			}
			return 1
		case "-s":
			info, err := os.Stat(arg)
			if err != nil {
				return 1
			}
			if info.Size() > 0 {
				return 0
			}
			return 1
		case "-z":
			if arg == "" {
				return 0
			}
			return 1
		case "-n":
			if arg != "" {
				return 0
			}
			return 1
		case "-h", "-L":
			info, err := os.Lstat(arg)
			if err != nil {
				return 1
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return 0
			}
			return 1
		}
	}

	// String operators
	if len(args) == 1 {
		if args[0] != "" {
			return 0
		}
		return 1
	}

	// Binary operators
	if len(args) == 3 {
		left, op, right := args[0], args[1], args[2]
		switch op {
		case "=":
			if left == right {
				return 0
			}
			return 1
		case "!=":
			if left != right {
				return 0
			}
			return 1
		case "-eq":
			a, _ := strconv.Atoi(left)
			b, _ := strconv.Atoi(right)
			if a == b {
				return 0
			}
			return 1
		case "-ne":
			a, _ := strconv.Atoi(left)
			b, _ := strconv.Atoi(right)
			if a != b {
				return 0
			}
			return 1
		case "-lt":
			a, _ := strconv.Atoi(left)
			b, _ := strconv.Atoi(right)
			if a < b {
				return 0
			}
			return 1
		case "-le":
			a, _ := strconv.Atoi(left)
			b, _ := strconv.Atoi(right)
			if a <= b {
				return 0
			}
			return 1
		case "-gt":
			a, _ := strconv.Atoi(left)
			b, _ := strconv.Atoi(right)
			if a > b {
				return 0
			}
			return 1
		case "-ge":
			a, _ := strconv.Atoi(left)
			b, _ := strconv.Atoi(right)
			if a >= b {
				return 0
			}
			return 1
		case "-ef":
			i1, err1 := os.Stat(left)
			i2, err2 := os.Stat(right)
			if err1 == nil && err2 == nil {
				s1, ok1 := i1.Sys().(*syscall.Stat_t)
				s2, ok2 := i2.Sys().(*syscall.Stat_t)
				if ok1 && ok2 && s1.Ino == s2.Ino && s1.Dev == s2.Dev {
					return 0
				}
			}
			return 1
		case "-nt":
			i1, err1 := os.Stat(left)
			i2, err2 := os.Stat(right)
			if err1 == nil && err2 == nil {
				if i1.ModTime().After(i2.ModTime()) {
					return 0
				}
			}
			return 1
		case "-ot":
			i1, err1 := os.Stat(left)
			i2, err2 := os.Stat(right)
			if err1 == nil && err2 == nil {
				if i1.ModTime().Before(i2.ModTime()) {
					return 0
				}
			}
			return 1
		}
	}

	// Negation
	if args[0] == "!" {
		if evalTestExpr(args[1:]) == 0 {
			return 1
		}
		return 0
	}

	// AND
	for i, arg := range args {
		if arg == "-a" {
			left := args[:i]
			right := args[i+1:]
			if evalTestExpr(left) == 0 && evalTestExpr(right) == 0 {
				return 0
			}
			return 1
		}
	}

	// OR
	for i, arg := range args {
		if arg == "-o" {
			left := args[:i]
			right := args[i+1:]
			if evalTestExpr(left) == 0 || evalTestExpr(right) == 0 {
				return 0
			}
			return 1
		}
	}

	return 1
}

func builtinGetopts(args []string) int {
	if len(args) < 3 {
		fmt.Fprintf(os.Stderr, "getopts: usage: getopts optstring name [args]\n")
		return 1
	}

	optstring := args[1]
	varName := args[2]

	// Simple getopts implementation
	optind := 1
	if val, exists := shellVars["OPTIND"]; exists {
		fmt.Sscanf(val, "%d", &optind)
	}

	if optind >= len(args) {
		shellVars[varName] = "?"
		return 1
	}

	opt := args[optind]
	if len(opt) < 2 || opt[0] != '-' {
		shellVars[varName] = "?"
		return 1
	}

	optChar := opt[1]
	found := false
	for i := 0; i < len(optstring); i++ {
		if optstring[i] == optChar {
			found = true
			shellVars[varName] = string(optChar)
			if i+1 < len(optstring) && optstring[i+1] == ':' {
				// Option requires argument
				if len(opt) > 2 {
					shellVars["OPTARG"] = opt[2:]
				} else if optind+1 < len(args) {
					optind++
					shellVars["OPTARG"] = args[optind]
				} else {
					shellVars[varName] = ":"
					shellVars["OPTARG"] = string(optChar)
				}
			}
			break
		}
	}

	if !found {
		shellVars[varName] = "?"
		shellVars["OPTARG"] = string(optChar)
	}

	optind++
	shellVars["OPTIND"] = strconv.Itoa(optind)
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
		} else if format[i] == '\\' && i+1 < len(format) {
			i++
			switch format[i] {
			case 'n':
				result += "\n"
			case 't':
				result += "\t"
			case 'r':
				result += "\r"
			case '\\':
				result += "\\"
			default:
				result += "\\" + string(format[i])
			}
		} else {
			result += string(format[i])
		}
	}
	fmt.Print(result)
	return 0
}
