//go:build !windows

package shell

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/agentbusybox/pkg/applet"
	"golang.org/x/sys/unix"
)

func runInteractivePlatform() int {
	loadHistory()

	reader := bufio.NewReader(os.Stdin)

	// Check if stdin is a terminal
	fd := int(os.Stdin.Fd())
	_, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		// Not a terminal (e.g. pipe / redirect). Run in simple cooked mode.
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				break
			}
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			executeLine(line)
		}
		return 0
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	go func() {
		for range sigChan {
			// Consume signal so the shell itself is not killed
		}
	}()

	for {
		dir, _ := os.Getwd()
		if len(dir) > 30 {
			dir = "..." + dir[len(dir)-27:]
		}
		prompt := dir + "$ "

		line, err := readRawLine(prompt, reader, commandHistory)
		if err != nil {
			if err == io.EOF {
				break
			}
			fmt.Fprintf(os.Stderr, "sh: read error: %v\n", err)
			continue
		}

		// Print newline in cooked mode so that cursor returns to column 0 properly
		fmt.Println()

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		commandHistory = append(commandHistory, line)
		saveHistoryLine(line)
		executeLine(line)
	}
	return 0
}

func readRawLine(prompt string, in *bufio.Reader, history []string) (string, error) {
	fd := int(os.Stdin.Fd())
	oldState, err := makeRaw(fd)
	if err != nil {
		return "", err
	}
	defer restore(fd, oldState)

	var lineRunes []rune
	cursorPos := 0
	historyIdx := len(history)
	var savedLine []rune

	redraw(prompt, lineRunes, cursorPos)

	for {
		b, err := in.ReadByte()
		if err != nil {
			return "", err
		}

		if b == 0x1b {
			peek, _ := in.Peek(2)
			if len(peek) >= 2 && peek[0] == '[' {
				_, _ = in.ReadByte() // '['
				_, _ = in.ReadByte() // arrow key char
				switch peek[1] {
				case 'A': // UP Arrow
					if historyIdx > 0 {
						if historyIdx == len(history) {
							savedLine = lineRunes
						}
						historyIdx--
						lineRunes = []rune(history[historyIdx])
						cursorPos = len(lineRunes)
						redraw(prompt, lineRunes, cursorPos)
					}
				case 'B': // DOWN Arrow
					if historyIdx < len(history) {
						historyIdx++
						if historyIdx == len(history) {
							lineRunes = savedLine
						} else {
							lineRunes = []rune(history[historyIdx])
						}
						cursorPos = len(lineRunes)
						redraw(prompt, lineRunes, cursorPos)
					}
				case 'C': // RIGHT Arrow
					if cursorPos < len(lineRunes) {
						cursorPos++
						redraw(prompt, lineRunes, cursorPos)
					}
				case 'D': // LEFT Arrow
					if cursorPos > 0 {
						cursorPos--
						redraw(prompt, lineRunes, cursorPos)
					}
				}
				continue
			}
		}

		if b == 3 { // Ctrl-C
			fmt.Print("^C\r\n")
			return "", nil
		}

		if b == 4 { // Ctrl-D
			if len(lineRunes) == 0 {
				return "", io.EOF
			}
			continue
		}

		if b == 127 || b == 8 { // Backspace
			if cursorPos > 0 {
				lineRunes = append(lineRunes[:cursorPos-1], lineRunes[cursorPos:]...)
				cursorPos--
				redraw(prompt, lineRunes, cursorPos)
			}
			continue
		}

		if b == '\r' || b == '\n' { // Enter
			return string(lineRunes), nil
		}

		if b == '\t' { // Tab
			wordStart := cursorPos
			for wordStart > 0 && lineRunes[wordStart-1] != ' ' {
				wordStart--
			}
			prefix := string(lineRunes[wordStart:cursorPos])

			isCmd := true
			for i := 0; i < wordStart; i++ {
				if lineRunes[i] != ' ' {
					isCmd = false
					break
				}
			}

			var matches []string
			if isCmd {
				matches = getCommandMatches(prefix)
			} else {
				matches = getPathMatches(prefix)
			}

			if len(matches) == 1 {
				match := matches[0]
				lineRunes = append(lineRunes[:wordStart], append([]rune(match), lineRunes[cursorPos:]...)...)
				cursorPos = wordStart + len(match)
				redraw(prompt, lineRunes, cursorPos)
			} else if len(matches) > 1 {
				lcp := longestCommonPrefix(matches)
				if len(lcp) > len(prefix) {
					lineRunes = append(lineRunes[:wordStart], append([]rune(lcp), lineRunes[cursorPos:]...)...)
					cursorPos = wordStart + len(lcp)
					redraw(prompt, lineRunes, cursorPos)
				} else {
					fmt.Print("\r\n")
					fmt.Print(strings.Join(matches, "  "))
					fmt.Print("\r\n")
					redraw(prompt, lineRunes, cursorPos)
				}
			}
			continue
		}

		if b >= 32 && b < 127 {
			lineRunes = append(lineRunes[:cursorPos], append([]rune{rune(b)}, lineRunes[cursorPos:]...)...)
			cursorPos++
			redraw(prompt, lineRunes, cursorPos)
		}
	}
}

func redraw(prompt string, runes []rune, cursorPos int) {
	fmt.Print("\r")
	fmt.Print(prompt)
	fmt.Print(string(runes))
	fmt.Print("\x1b[K")
	backspaces := len(runes) - cursorPos
	if backspaces > 0 {
		fmt.Print(strings.Repeat("\b", backspaces))
	}
}

func getCommandMatches(prefix string) []string {
	var matches []string

	builtins := []string{"cd", "export", "set", "unset", "echo", "read", "pwd", "history", "source", "type", "alias", "unalias", "trap", "shift", "exit", "exec"}
	for _, b := range builtins {
		if strings.HasPrefix(b, prefix) {
			matches = append(matches, b)
		}
	}

	for _, name := range applet.Names() {
		if strings.HasPrefix(name, prefix) {
			dup := false
			for _, m := range matches {
				if m == name {
					dup = true
					break
				}
			}
			if !dup {
				matches = append(matches, name)
			}
		}
	}

	pathEnv := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(pathEnv) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			name := entry.Name()
			if strings.HasPrefix(name, prefix) {
				dup := false
				for _, m := range matches {
					if m == name {
						dup = true
						break
					}
				}
				if !dup {
					matches = append(matches, name)
				}
			}
		}
	}

	return matches
}

func getPathMatches(prefix string) []string {
	var matches []string
	dir, filePrefix := filepath.Split(prefix)

	searchDir := dir
	if searchDir == "" {
		searchDir = "."
	}

	entries, err := os.ReadDir(searchDir)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, filePrefix) {
			suffix := ""
			if entry.IsDir() {
				suffix = "/"
			}

			var match string
			if dir == "" {
				match = name + suffix
			} else {
				match = dir + name + suffix
			}
			matches = append(matches, match)
		}
	}

	return matches
}

func longestCommonPrefix(strs []string) string {
	if len(strs) == 0 {
		return ""
	}
	prefix := strs[0]
	for _, s := range strs[1:] {
		for !strings.HasPrefix(s, prefix) {
			prefix = prefix[:len(prefix)-1]
			if len(prefix) == 0 {
				return ""
			}
		}
	}
	return prefix
}

func makeRaw(fd int) (*unix.Termios, error) {
	old, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return nil, err
	}
	raw := *old
	raw.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP | unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON
	raw.Oflag &^= unix.OPOST
	raw.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN
	raw.Cflag &^= unix.CSIZE | unix.PARENB
	raw.Cflag |= unix.CS8
	raw.Cc[unix.VMIN] = 1
	raw.Cc[unix.VTIME] = 0

	if err := unix.IoctlSetTermios(fd, unix.TCSETS, &raw); err != nil {
		return nil, err
	}
	return old, nil
}

func restore(fd int, old *unix.Termios) {
	_ = unix.IoctlSetTermios(fd, unix.TCSETS, old)
}
