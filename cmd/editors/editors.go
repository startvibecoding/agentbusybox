package editors

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/agentbusybox/pkg/applet"
)

// --- ed ---
func init() {
	applet.Register(&applet.Applet{Name: "ed", Short: "Line-oriented text editor", Func: runEd})
}

func runEd(args []string) int {
	file := ""
	if len(args) > 1 && !strings.HasPrefix(args[1], "-") {
		file = args[1]
	}

	lines := []string{}
	if file != "" {
		data, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ed: %s: %v\n", file, err)
			return 1
		}
		lines = strings.Split(string(data), "\n")
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		input, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		input = strings.TrimSpace(input)
		switch {
		case input == "q":
			return 0
		case input == "p":
			for _, l := range lines {
				fmt.Println(l)
			}
		case input == "w":
			if file != "" {
				os.WriteFile(file, []byte(strings.Join(lines, "\n")+"\n"), 0644)
			}
		case input == "a":
			// append mode
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					break
				}
				line = strings.TrimSpace(line)
				if line == "." {
					break
				}
				lines = append(lines, line)
			}
		case input == "i":
			// insert mode
			newLines := []string{}
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					break
				}
				line = strings.TrimSpace(line)
				if line == "." {
					break
				}
				newLines = append(newLines, line)
			}
			lines = append(newLines, lines...)
		case input == "d":
			if len(lines) > 0 {
				lines = lines[:len(lines)-1]
			}
		case input == "1,$p":
			for _, l := range lines {
				fmt.Println(l)
			}
		case input == "h":
			fmt.Println("?")
		default:
			fmt.Println("?")
		}
	}
	return 0
}

// --- vi (simplified line editor) ---
func init() {
	applet.Register(&applet.Applet{Name: "vi", Short: "Text editor", Func: runVi})
}

func runVi(args []string) int {
	file := ""
	if len(args) > 1 && !strings.HasPrefix(args[1], "-") {
		file = args[1]
	}

	lines := []string{}
	if file != "" {
		data, err := os.ReadFile(file)
		if err == nil {
			lines = strings.Split(string(data), "\n")
			if len(lines) > 0 && lines[len(lines)-1] == "" {
				lines = lines[:len(lines)-1]
			}
		}
	}

	reader := bufio.NewReader(os.Stdin)
	modified := false

	for {
		fmt.Print(":")
		input, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		input = strings.TrimSpace(input)

		switch {
		case input == "q":
			if modified {
				fmt.Println("No write since last change (add ! to override)")
				continue
			}
			return 0
		case input == "q!":
			return 0
		case input == "w":
			if file != "" {
				os.WriteFile(file, []byte(strings.Join(lines, "\n")+"\n"), 0644)
				fmt.Printf("\"%s\" %d lines written\n", file, len(lines))
				modified = false
			} else {
				fmt.Println("No file name")
			}
		case input == "wq":
			if file != "" {
				os.WriteFile(file, []byte(strings.Join(lines, "\n")+"\n"), 0644)
			}
			return 0
		case input == "e":
			if file != "" {
				data, err := os.ReadFile(file)
				if err == nil {
					lines = strings.Split(string(data), "\n")
					if len(lines) > 0 && lines[len(lines)-1] == "" {
						lines = lines[:len(lines)-1]
					}
				}
			}
		case input == "p" || input == "%p":
			for i, l := range lines {
				fmt.Printf("%d\t%s\n", i+1, l)
			}
		case input == "a":
			// append mode
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					break
				}
				line = strings.TrimRight(line, "\n")
				if line == "." {
					break
				}
				lines = append(lines, line)
				modified = true
			}
		case input == "i":
			// insert mode
			newLines := []string{}
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					break
				}
				line = strings.TrimRight(line, "\n")
				if line == "." {
					break
				}
				newLines = append(newLines, line)
				modified = true
			}
			lines = append(newLines, lines...)
		case input == "d":
			if len(lines) > 0 {
				lines = lines[:len(lines)-1]
				modified = true
			}
		case input == "set number" || input == "set nu":
			for i, l := range lines {
				fmt.Printf("%d\t%s\n", i+1, l)
			}
		case strings.HasPrefix(input, "s/"):
			// substitute
			parts := strings.SplitN(input, "/", 3)
			if len(parts) >= 3 {
				old := parts[1]
				new := strings.TrimSuffix(parts[2], "/")
				for i, l := range lines {
					lines[i] = strings.Replace(l, old, new, 1)
				}
				modified = true
			}
		case strings.HasPrefix(input, "/"):
			// search
			pattern := strings.TrimPrefix(input, "/")
			pattern = strings.TrimSuffix(pattern, "/")
			for i, l := range lines {
				if strings.Contains(l, pattern) {
					fmt.Printf("%d\t%s\n", i+1, l)
				}
			}
		case input == "n":
			for i, l := range lines {
				fmt.Printf("%d\t%s\n", i+1, l)
			}
		case input == "help":
			fmt.Println("Commands: w(rite) q(uit) q! wq e(dit) a(ppend) i(nsert) d(elete) p(rint) /search s/old/new/ n(umber)")
		default:
			if len(input) > 0 {
				fmt.Println("?")
			}
		}
	}
	return 0
}

// --- patch ---
func init() {
	applet.Register(&applet.Applet{Name: "patch", Short: "Apply a diff file to an original", Func: runPatch})
}

func runPatch(args []string) int {
	input := ""
	for _, a := range args[1:] {
		if a == "-i" {
			continue
		}
		if !strings.HasPrefix(a, "-") {
			input = a
		}
	}

	var r io.Reader
	if input == "" || input == "-" {
		r = os.Stdin
	} else {
		f, err := os.Open(input)
		if err != nil {
			fmt.Fprintf(os.Stderr, "patch: %s: %v\n", input, err)
			return 1
		}
		defer f.Close()
		r = f
	}

	scanner := bufio.NewScanner(r)
	file := ""
	lines := []string{}
	exitCode := 0

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ ") {
			continue
		}
		if strings.HasPrefix(line, "@@") {
			continue
		}
		if strings.HasPrefix(line, "diff ") {
			// Process previous file
			if file != "" && len(lines) > 0 {
				if err := applyPatchLines(file, lines); err != nil {
					fmt.Fprintf(os.Stderr, "patch: %s: %v\n", file, err)
					exitCode = 1
				}
			}
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				file = strings.TrimPrefix(parts[2], "a/")
			}
			lines = []string{}
			continue
		}
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "+") {
			lines = append(lines, line[1:])
		}
	}

	if file != "" && len(lines) > 0 {
		if err := applyPatchLines(file, lines); err != nil {
			fmt.Fprintf(os.Stderr, "patch: %s: %v\n", file, err)
			exitCode = 1
		}
	}
	return exitCode
}

func applyPatchLines(file string, lines []string) error {
	return os.WriteFile(file, []byte(strings.Join(lines, "\n")+"\n"), 0644)
}

// --- awk (improved) ---
func init() {
	applet.Register(&applet.Applet{Name: "awk", Short: "Pattern scanning and processing language", Func: runAwk})
}

func runAwk(args []string) int {
	program := ""
	files := []string{}
	fieldSep := ""

	for i := 1; i < len(args); i++ {
		if args[i] == "-F" && i+1 < len(args) {
			i++
			fieldSep = args[i]
			continue
		}
		if strings.HasPrefix(args[i], "-F") && len(args[i]) > 2 {
			fieldSep = args[i][2:]
			continue
		}
		if args[i] == "-f" && i+1 < len(args) {
			i++ /* read from file */
			continue
		}
		if !strings.HasPrefix(args[i], "-") {
			if program == "" {
				program = args[i]
			} else {
				files = append(files, args[i])
			}
		}
	}
	if program == "" {
		fmt.Fprintf(os.Stderr, "awk: missing program\n")
		return 1
	}
	if fieldSep == "" {
		fieldSep = " "
	}

	if len(files) == 0 {
		files = []string{"-"}
	}

	for _, fname := range files {
		var r io.Reader
		if fname == "-" {
			r = os.Stdin
		} else {
			f, err := os.Open(fname)
			if err != nil {
				fmt.Fprintf(os.Stderr, "awk: %s: %v\n", fname, err)
				return 1
			}
			defer f.Close()
			r = f
		}
		scanner := bufio.NewScanner(r)
		nr := 0
		for scanner.Scan() {
			nr++
			line := scanner.Text()
			fields := strings.Split(line, fieldSep)
			nf := len(fields)

			// Simple pattern matching
			matched := false
			action := ""

			if program == "{print}" || program == "{ print }" {
				matched = true
				action = "print"
			} else if strings.HasPrefix(program, "/") && strings.HasSuffix(program, "/") {
				pattern := program[1 : len(program)-1]
				if strings.Contains(line, pattern) {
					matched = true
					action = "print"
				}
			} else if program == "{print $0}" {
				matched = true
				action = "print"
			} else if strings.Contains(program, "print") {
				matched = true
				action = program
			}

			if matched {
				if action == "print" || action == "{print}" || action == "{ print }" {
					fmt.Println(line)
				} else if action == "{print $0}" {
					fmt.Println(line)
				} else {
					// Try to evaluate simple print expressions
					result := evalAwkPrint(action, fields, nr, nf)
					if result != "" {
						fmt.Println(result)
					}
				}
			}
		}
	}
	return 0
}

func evalAwkPrint(expr string, fields []string, nr, nf int) string {
	result := ""
	// Handle {print $N} patterns
	expr = strings.TrimSpace(expr)
	expr = strings.Trim(expr, "{}")
	expr = strings.TrimSpace(expr)

	if !strings.HasPrefix(expr, "print ") && expr != "print" {
		return ""
	}
	expr = strings.TrimSpace(strings.TrimPrefix(expr, "print"))

	if expr == "" || expr == "$0" {
		return strings.Join(fields, " ")
	}

	parts := strings.Split(expr, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "$0" {
			result += strings.Join(fields, " ")
		} else if strings.HasPrefix(p, "$") {
			idx := 0
			fmt.Sscanf(p[1:], "%d", &idx)
			if idx >= 1 && idx <= len(fields) {
				result += fields[idx-1]
			}
		} else if p == "NR" {
			result += fmt.Sprintf("%d", nr)
		} else if p == "NF" {
			result += fmt.Sprintf("%d", nf)
		} else {
			// strip quotes
			p = strings.Trim(p, "\"")
			result += p
		}
	}
	return result
}
