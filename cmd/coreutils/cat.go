package coreutils

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/agentbusybox/pkg/applet"
)

func init() {
	applet.Register(&applet.Applet{Name: "cat", Short: "Concatenate and print files", Func: runCat})
}

func runCat(args []string) int {
	number := false
	numberNonBlank := false
	squeeze := false
	showEnds := false
	showTabs := false
	showNonPrint := false

	files := []string{}
	i := 1
	for ; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") || a == "-" {
			if a == "-" {
				i++
			}
			break
		}
		if a == "--" {
			i++
			break
		}
		for _, ch := range a[1:] {
			switch ch {
			case 'n':
				number = true
			case 'b':
				numberNonBlank = true
			case 's':
				squeeze = true
			case 'E':
				showEnds = true
			case 'T':
				showTabs = true
			case 'v':
				showNonPrint = true
			case 'A':
				showNonPrint = true
				showEnds = true
				showTabs = true
			case 'e':
				showNonPrint = true
				showEnds = true
			case 't':
				showNonPrint = true
				showTabs = true
			default:
				fmt.Fprintf(os.Stderr, "cat: invalid option -- '%c'\n", ch)
				return 1
			}
		}
	}
	files = args[i:]

	if len(files) == 0 {
		files = []string{"-"}
	}

	lineNum := 1
	prevBlank := false

	for _, fname := range files {
		var r io.Reader
		if fname == "-" {
			r = os.Stdin
		} else {
			f, err := os.Open(fname)
			if err != nil {
				fmt.Fprintf(os.Stderr, "cat: %s: %v\n", fname, err)
				return 1
			}
			defer f.Close()
			r = f
		}

		buf := make([]byte, 32*1024)
		line := ""
		for {
			n, err := r.Read(buf)
			if n > 0 {
				line += string(buf[:n])
				for {
					idx := strings.IndexByte(line, '\n')
					if idx < 0 {
						break
					}
					l := line[:idx+1]
					line = line[idx+1:]

					isBlank := strings.TrimSpace(l[:len(l)-1]) == ""
					if squeeze && isBlank && prevBlank {
						continue
					}
					prevBlank = isBlank

					printLine(l[:len(l)-1], lineNum, number, numberNonBlank, showEnds, showTabs, showNonPrint, isBlank)
					if number || (numberNonBlank && !isBlank) {
						lineNum++
					}
				}
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				fmt.Fprintf(os.Stderr, "cat: %v\n", err)
				return 1
			}
		}
		// handle remaining data without newline
		if line != "" {
			isBlank := strings.TrimSpace(line) == ""
			printLine(line, lineNum, number, numberNonBlank, showEnds, showTabs, showNonPrint, isBlank)
		}
	}
	return 0
}

func printLine(s string, num int, number, numberNonBlank, showEnds, showTabs, showNonPrint, isBlank bool) {
	if number || (numberNonBlank && !isBlank) {
		fmt.Printf("%6d\t", num)
	}
	if showNonPrint {
		s = showNonPrinting(s, showTabs)
	} else if showTabs {
		s = strings.ReplaceAll(s, "\t", "^I")
	}
	fmt.Print(s)
	if showEnds {
		fmt.Print("$")
	}
	fmt.Println()
}

func showNonPrinting(s string, showTabs bool) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\t' && showTabs:
			b.WriteString("^I")
		case r == '\n':
			b.WriteByte('\n')
		case r < 32:
			b.WriteRune('^')
			b.WriteRune(r + 64)
		case r == 127:
			b.WriteString("^?")
		case r > 127:
			b.WriteString("M-")
			if r < 128+32 {
				b.WriteRune('^')
				b.WriteRune(r - 128 + 64)
			} else {
				b.WriteRune(r - 128)
			}
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
