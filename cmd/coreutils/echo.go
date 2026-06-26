package coreutils

import (
	"fmt"
	"strings"

	"github.com/agentbusybox/pkg/applet"
)

func init() {
	applet.Register(&applet.Applet{Name: "echo", Short: "Display text", Func: runEcho})
}

func runEcho(args []string) int {
	noNewline := false
	enableEscape := true
	escapeFlag := false
	rawFlag := false

	i := 1
	for ; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") {
			break
		}
		for _, ch := range a[1:] {
			switch ch {
			case 'n':
				noNewline = true
			case 'e':
				escapeFlag = true
			case 'E':
				rawFlag = true
			default:
				// unknown flag, treat as argument
				goto done
			}
		}
	}
done:

	if escapeFlag {
		enableEscape = true
	}
	if rawFlag {
		enableEscape = false
	}

	out := strings.Join(args[i:], " ")
	if enableEscape {
		out = expandEscapes(out)
	}
	fmt.Print(out)
	if !noNewline {
		fmt.Println()
	}
	return 0
}

func expandEscapes(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			switch s[i] {
			case 'a':
				b.WriteByte('\a')
			case 'b':
				b.WriteByte('\b')
			case 'c':
				return b.String() // stop output
			case 'e', 'E':
				b.WriteByte(0x1b)
			case 'f':
				b.WriteByte('\f')
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			case 'v':
				b.WriteByte('\v')
			case '\\':
				b.WriteByte('\\')
			case '0':
				// octal
				if i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '7' {
					i++
				}
				b.WriteByte(0)
			default:
				b.WriteByte('\\')
				b.WriteByte(s[i])
			}
		} else {
			b.WriteByte(s[i])
		}
	}
	return b.String()
}
