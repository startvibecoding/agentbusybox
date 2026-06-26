package utillinux

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

func init() {
	// --- scriptreplay --- already in extra.go ---
}

func runScriptreplay(args []string) int {
	timingFile := ""
	scriptFile := ""
	divisor := 1.0

	for i := 1; i < len(args); i++ {
		a := args[i]
		switch a {
		case "-t", "--timing":
			if i+1 < len(args) {
				i++
				timingFile = args[i]
			}
		case "-s", "--divisor":
			if i+1 < len(args) {
				i++
				fmt.Sscanf(args[i], "%f", &divisor)
			}
		default:
			if !strings.HasPrefix(a, "-") {
				if timingFile == "" {
					timingFile = a
				} else {
					scriptFile = a
				}
			}
		}
	}

	if timingFile == "" {
		fmt.Fprintf(os.Stderr, "scriptreplay: missing timing file\n")
		return 1
	}

	// Read timing file
	type timing struct {
		delay  float64
		nbytes int
	}
	timings := []timing{}

	f, err := os.Open(timingFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scriptreplay: %v\n", err)
		return 1
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		parts := strings.Fields(scanner.Text())
		if len(parts) >= 2 {
			var d float64
			var n int
			fmt.Sscanf(parts[0], "%f", &d)
			fmt.Sscanf(parts[1], "%d", &n)
			timings = append(timings, timing{d, n})
		}
	}

	// Read script file
	var scriptData []byte
	if scriptFile != "" {
		scriptData, err = os.ReadFile(scriptFile)
	} else {
		scriptData, err = os.ReadFile("typescript")
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "scriptreplay: %v\n", err)
		return 1
	}

	offset := 0
	for _, t := range timings {
		if offset+t.nbytes > len(scriptData) {
			break
		}
		time.Sleep(time.Duration(t.delay / divisor * float64(time.Second)))
		os.Stdout.Write(scriptData[offset : offset+t.nbytes])
		offset += t.nbytes
	}
	return 0
}
