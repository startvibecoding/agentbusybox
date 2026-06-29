package editors

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/agentbusybox/pkg/applet"
	"golang.org/x/sys/unix"
)

func init() {
	applet.Register(&applet.Applet{Name: "vi", Short: "Text editor", Func: runVi})
}

type viMode int

const (
	viNormal viMode = iota
	viInsert
	viCommand
)

type viEditor struct {
	file     string
	lines    []string
	curLine  int
	curCol   int
	top      int
	showNum  bool
	modified bool

	yank     []string
	lastFind string
	lastFwd  bool
	lastCmd  string
	message  string
	status   string
	prompt   string
	mode     viMode
	cmdBuf   []rune
	count    int
	pending  rune
	undo     []viSnapshot
	undoLock bool
}

type viSnapshot struct {
	lines    []string
	curLine  int
	curCol   int
	top      int
	showNum  bool
	modified bool
	yank     []string
	lastFind string
	lastFwd  bool
	lastCmd  string
	message  string
	status   string
	prompt   string
	mode     viMode
	cmdBuf   []rune
	count    int
	pending  rune
}

func runVi(args []string) int {
	if runtime.GOOS == "windows" {
		fmt.Fprintln(os.Stderr, "vi: not supported on this platform")
		return 1
	}

	ed := &viEditor{showNum: false}
	for _, a := range args[1:] {
		if !strings.HasPrefix(a, "-") {
			ed.file = a
			break
		}
	}
	if ed.file != "" {
		if err := ed.load(ed.file); err != nil {
			fmt.Fprintf(os.Stderr, "vi: %v\n", err)
			return 1
		}
	} else {
		ed.lines = []string{""}
	}

	fd := int(os.Stdin.Fd())
	old, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		fmt.Fprintf(os.Stderr, "vi: %v\n", err)
		return 1
	}
	raw := *old
	applyViRaw(&raw)
	if err := unix.IoctlSetTermios(fd, unix.TCSETS, &raw); err != nil {
		fmt.Fprintf(os.Stderr, "vi: %v\n", err)
		return 1
	}
	defer unix.IoctlSetTermios(fd, unix.TCSETS, old)

	in := bufio.NewReader(os.Stdin)
	ed.fixCursor()
	ed.refresh()
	for {
		k := ed.readKey(in)
		if k.err != nil {
			break
		}
		if ed.handleKey(k) {
			break
		}
		ed.fixCursor()
		ed.refresh()
	}
	return 0
}

type viKey struct {
	r   rune
	err error
}

func (e *viEditor) readKey(in *bufio.Reader) viKey {
	b, err := in.ReadByte()
	if err != nil {
		return viKey{err: err}
	}
	if b != 0x1b {
		if b < utf8.RuneSelf {
			return viKey{r: rune(b)}
		}
		buf := []byte{b}
		for !utf8.FullRune(buf) {
			nb, err := in.ReadByte()
			if err != nil {
				return viKey{err: err}
			}
			buf = append(buf, nb)
		}
		r, _ := utf8.DecodeRune(buf)
		return viKey{r: r}
	}
	peek, _ := in.Peek(2)
	if len(peek) < 2 {
		return viKey{r: 0x1b}
	}
	if peek[0] != '[' {
		return viKey{r: 0x1b}
	}
	_, _ = in.ReadByte()
	if len(peek) > 1 {
		_, _ = in.ReadByte()
		switch peek[1] {
		case 'A':
			return viKey{r: keyUp}
		case 'B':
			return viKey{r: keyDown}
		case 'C':
			return viKey{r: keyRight}
		case 'D':
			return viKey{r: keyLeft}
		case 'H':
			return viKey{r: keyHome}
		case 'F':
			return viKey{r: keyEnd}
		case '3':
			if tail, _ := in.Peek(1); len(tail) == 1 && tail[0] == '~' {
				_, _ = in.ReadByte()
				return viKey{r: keyDelete}
			}
		case '5':
			if tail, _ := in.Peek(1); len(tail) == 1 && tail[0] == '~' {
				_, _ = in.ReadByte()
				return viKey{r: keyPageUp}
			}
		case '6':
			if tail, _ := in.Peek(1); len(tail) == 1 && tail[0] == '~' {
				_, _ = in.ReadByte()
				return viKey{r: keyPageDown}
			}
		}
	}
	return viKey{r: 0x1b}
}

const (
	keyUp       = -1001
	keyDown     = -1002
	keyLeft     = -1003
	keyRight    = -1004
	keyHome     = -1005
	keyEnd      = -1006
	keyDelete   = -1007
	keyPageUp   = -1008
	keyPageDown = -1009
)

func (e *viEditor) handleKey(k viKey) bool {
	if k.err != nil {
		return true
	}
	switch e.mode {
	case viInsert:
		return e.handleInsert(k.r)
	case viCommand:
		return e.handleCommandLine(k.r)
	default:
		return e.handleNormal(k.r)
	}
}

func (e *viEditor) handleNormal(r rune) bool {
	if e.pending != 0 {
		return e.handlePending(r)
	}

	if r >= '1' && r <= '9' {
		e.count = e.count*10 + int(r-'0')
		return false
	}

	switch r {
	case 'q':
		return !e.modified
	case 'Q':
		return true
	case 'i':
		e.pushUndo()
		e.mode = viInsert
		e.prompt = "-- INSERT --"
	case 'a':
		e.pushUndo()
		e.moveRight(1)
		e.mode = viInsert
		e.prompt = "-- INSERT --"
	case 'A':
		e.pushUndo()
		e.curCol = len([]rune(e.line()))
		e.mode = viInsert
		e.prompt = "-- INSERT --"
	case 'I':
		e.pushUndo()
		e.curCol = 0
		for e.curCol < len([]rune(e.line())) && isSpace(runeAt(e.line(), e.curCol)) {
			e.curCol++
		}
		e.mode = viInsert
		e.prompt = "-- INSERT --"
	case 'o':
		e.pushUndo()
		e.openLine(false)
		e.mode = viInsert
	case 'O':
		e.pushUndo()
		e.openLine(true)
		e.mode = viInsert
	case 'x':
		e.pushUndo()
		e.deleteChars(e.takeCount(1))
	case 'd':
		e.pending = 'd'
		return false
	case 'y':
		e.pending = 'y'
		return false
	case 'c':
		e.pending = 'c'
		return false
	case 'p':
		e.pushUndo()
		e.putAfter(e.takeCount(1))
	case 'P':
		e.pushUndo()
		e.putBefore(e.takeCount(1))
	case 'u':
		e.undoPop()
	case '/':
		e.mode = viCommand
		e.prompt = "/"
		e.cmdBuf = e.cmdBuf[:0]
		e.lastFwd = true
	case '?':
		e.mode = viCommand
		e.prompt = "?"
		e.cmdBuf = e.cmdBuf[:0]
		e.lastFwd = false
	case 'n':
		e.searchRepeat()
	case 'N':
		e.searchRepeatOpposite()
	case ':':
		e.mode = viCommand
		e.prompt = ":"
		e.cmdBuf = e.cmdBuf[:0]
	case 'h':
		e.moveLeft(e.takeCount(1))
	case 'l', ' ':
		e.moveRight(e.takeCount(1))
	case 'j':
		e.moveDown(e.takeCount(1))
	case 'k':
		e.moveUp(e.takeCount(1))
	case '0':
		if e.count == 0 {
			e.curCol = 0
		} else {
			e.count = e.count * 10
		}
	case '$':
		e.curCol = len([]rune(e.line()))
	case 'g':
		e.pending = 'g'
		return false
	case 'G':
		e.pushUndo()
		n := e.takeCount(len(e.lines))
		e.gotoLine(n - 1)
	case 'w':
		e.moveWord(1, e.takeCount(1))
	case 'b':
		e.moveWord(-1, e.takeCount(1))
	case 'J':
		e.pushUndo()
		e.joinLine(e.takeCount(1))
	case 'r':
		e.pending = 'r'
		return false
	case 0x1b:
		e.prompt = ""
		e.lastCmd = ""
		e.count = 0
		e.pending = 0
	case '\r', '\n':
		e.pushUndo()
		e.moveDown(e.takeCount(1))
	default:
		e.count = 0
	}
	e.count = 0
	return false
}

func (e *viEditor) handlePending(r rune) bool {
	n := e.takeCount(1)
	switch e.pending {
	case 'd':
		if r == 'd' {
			e.pushUndo()
			e.deleteLine(n)
		}
	case 'y':
		if r == 'y' {
			e.yankLine(n)
		}
	case 'c':
		if r == 'c' {
			e.pushUndo()
			e.changeLine(n)
		}
	case 'g':
		if r == 'g' {
			e.gotoLine(n - 1)
		}
	case 'r':
		if r >= 0x20 {
			e.pushUndo()
			e.replaceChars(n, r)
		}
	}
	e.pending = 0
	e.count = 0
	return false
}

func (e *viEditor) handleInsert(r rune) bool {
	switch r {
	case 0x1b:
		e.mode = viNormal
		e.prompt = ""
	case 0x7f, 0x08:
		e.backspace()
	case '\r', '\n':
		e.newline()
	default:
		if r >= 0x20 {
			e.insertRune(r)
		}
	}
	return false
}

func (e *viEditor) handleCommandLine(r rune) bool {
	switch r {
	case 0x1b:
		e.mode = viNormal
		e.prompt = ""
		e.cmdBuf = e.cmdBuf[:0]
	case '\r', '\n':
		cmd := string(e.cmdBuf)
		e.mode = viNormal
		e.prompt = ""
		e.cmdBuf = e.cmdBuf[:0]
		e.runColon(cmd)
	default:
		if r == 0x7f || r == 0x08 {
			if len(e.cmdBuf) > 0 {
				e.cmdBuf = e.cmdBuf[:len(e.cmdBuf)-1]
			}
		} else if r >= 0x20 {
			e.cmdBuf = append(e.cmdBuf, r)
		}
	}
	return false
}

func (e *viEditor) runColon(cmd string) {
	cmd = strings.TrimSpace(cmd)
	switch {
	case cmd == "q":
		if !e.modified {
			e.status = ""
			e.mode = viNormal
		} else {
			e.status = "No write since last change (add ! to override)"
		}
	case cmd == "q!":
		e.modified = false
		e.status = ""
		e.mode = viNormal
	case cmd == "w":
		if err := e.save(); err != nil {
			e.status = err.Error()
		} else {
			e.status = fmt.Sprintf("\"%s\" %d lines written", e.file, len(e.lines))
		}
	case cmd == "wq" || cmd == "x":
		_ = e.save()
		e.modified = false
	case cmd == "set nu" || cmd == "set number":
		e.showNum = true
	case cmd == "set nonu":
		e.showNum = false
	case strings.HasPrefix(cmd, "s/"):
		e.substitute(cmd)
	case strings.HasPrefix(cmd, "/"):
		e.search(cmd[1:], true)
	case strings.HasPrefix(cmd, "?"):
		e.search(cmd[1:], false)
	case cmd == "":
		return
	default:
		e.status = "?"
	}
}

func (e *viEditor) takeCount(def int) int {
	if e.count <= 0 {
		return def
	}
	n := e.count
	e.count = 0
	return n
}

func (e *viEditor) pushUndo() {
	if e.undoLock {
		return
	}
	if e.modified && len(e.undo) > 0 {
		prev := e.undo[len(e.undo)-1]
		if len(prev.lines) == len(e.lines) && strings.Join(prev.lines, "\n") == strings.Join(e.lines, "\n") {
			return
		}
	}
	snap := viSnapshot{
		lines:    append([]string(nil), e.lines...),
		curLine:  e.curLine,
		curCol:   e.curCol,
		top:      e.top,
		showNum:  e.showNum,
		modified: e.modified,
		yank:     append([]string(nil), e.yank...),
		lastFind: e.lastFind,
		lastFwd:  e.lastFwd,
		lastCmd:  e.lastCmd,
		message:  e.message,
		status:   e.status,
		prompt:   e.prompt,
		mode:     e.mode,
		cmdBuf:   append([]rune(nil), e.cmdBuf...),
		count:    e.count,
		pending:  e.pending,
	}
	e.undo = append(e.undo, snap)
}

func (e *viEditor) undoPop() {
	if len(e.undo) == 0 {
		e.status = "nothing to undo"
		return
	}
	snap := e.undo[len(e.undo)-1]
	e.undo = e.undo[:len(e.undo)-1]
	e.undoLock = true
	e.lines = append([]string(nil), snap.lines...)
	e.curLine = snap.curLine
	e.curCol = snap.curCol
	e.top = snap.top
	e.showNum = snap.showNum
	e.modified = snap.modified
	e.yank = append([]string(nil), snap.yank...)
	e.lastFind = snap.lastFind
	e.lastFwd = snap.lastFwd
	e.lastCmd = snap.lastCmd
	e.message = snap.message
	e.status = snap.status
	e.prompt = snap.prompt
	e.mode = snap.mode
	e.cmdBuf = append([]rune(nil), snap.cmdBuf...)
	e.count = snap.count
	e.pending = snap.pending
	e.undoLock = false
}

func (e *viEditor) load(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	e.lines = strings.Split(text, "\n")
	if len(e.lines) == 0 {
		e.lines = []string{""}
	}
	if len(e.lines) > 0 && e.lines[len(e.lines)-1] == "" {
		e.lines = e.lines[:len(e.lines)-1]
	}
	if len(e.lines) == 0 {
		e.lines = []string{""}
	}
	return nil
}

func (e *viEditor) save() error {
	if e.file == "" {
		return fmt.Errorf("No file name")
	}
	data := strings.Join(e.lines, "\n") + "\n"
	if err := os.WriteFile(e.file, []byte(data), 0644); err != nil {
		return err
	}
	e.modified = false
	return nil
}

func (e *viEditor) line() string {
	if e.curLine < 0 || e.curLine >= len(e.lines) {
		return ""
	}
	return e.lines[e.curLine]
}

func (e *viEditor) lineRunes() []rune {
	return []rune(e.line())
}

func (e *viEditor) fixCursor() {
	if len(e.lines) == 0 {
		e.lines = []string{""}
	}
	if e.curLine < 0 {
		e.curLine = 0
	}
	if e.curLine >= len(e.lines) {
		e.curLine = len(e.lines) - 1
	}
	if e.curCol < 0 {
		e.curCol = 0
	}
	l := len([]rune(e.line()))
	if e.curCol > l {
		e.curCol = l
	}
	if e.curLine < e.top {
		e.top = e.curLine
	}
	h := e.height()
	if e.curLine >= e.top+h-1 {
		e.top = e.curLine - h + 2
		if e.top < 0 {
			e.top = 0
		}
	}
}

func (e *viEditor) height() int {
	if ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ); err == nil && ws.Row > 0 {
		return int(ws.Row)
	}
	if v := os.Getenv("LINES"); v != "" {
		if n, _ := strconv.Atoi(v); n > 0 {
			return n
		}
	}
	return 24
}

func (e *viEditor) width() int {
	if ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ); err == nil && ws.Col > 0 {
		return int(ws.Col)
	}
	if v := os.Getenv("COLUMNS"); v != "" {
		if n, _ := strconv.Atoi(v); n > 0 {
			return n
		}
	}
	return 80
}

func (e *viEditor) refresh() {
	h := e.height()
	w := e.width()
	statusRow := h - 1
	fmt.Print("\x1b[H\x1b[2J")
	for row := 0; row < statusRow; row++ {
		lineIdx := e.top + row
		if lineIdx >= len(e.lines) {
			fmt.Print("~\r\n")
			continue
		}
		line := e.lines[lineIdx]
		if e.showNum {
			fmt.Printf("%4d ", lineIdx+1)
		}
		if lineIdx == e.curLine {
			e.drawLineWithCursor(line, w)
		} else {
			fmt.Println(trimToWidth(line, w))
		}
	}
	prompt := e.prompt
	if prompt == "" {
		prompt = "-- NORMAL --"
	}
	if e.message != "" {
		prompt = e.message
	}
	if e.status != "" {
		prompt = e.status
	}
	fmt.Printf("\x1b[%d;1H%s", statusRow+1, trimToWidth(prompt, w))
	fmt.Printf("\x1b[%d;%dH", e.cursorScreenRow()+1, e.cursorScreenCol()+1)
}

func (e *viEditor) cursorScreenRow() int {
	return e.curLine - e.top
}

func (e *viEditor) cursorScreenCol() int {
	col := e.curCol + 1
	if e.showNum {
		col += 5
	}
	return col
}

func (e *viEditor) drawLineWithCursor(line string, width int) {
	runes := []rune(line)
	if len(runes) == 0 {
		fmt.Print(" \r\n")
		return
	}
	cursor := e.curCol
	if cursor > len(runes) {
		cursor = len(runes)
	}
	for i, r := range runes {
		if i == cursor {
			fmt.Print("\x1b[7m")
			fmt.Printf("%c", r)
			fmt.Print("\x1b[m")
		} else {
			fmt.Printf("%c", r)
		}
	}
	if cursor == len(runes) {
		fmt.Print("\x1b[7m \x1b[m")
	}
	fmt.Print("\r\n")
}

func trimToWidth(s string, width int) string {
	r := []rune(s)
	if len(r) > width {
		return string(r[:width])
	}
	return s
}

func (e *viEditor) insertRune(r rune) {
	line := []rune(e.line())
	if e.curCol > len(line) {
		e.curCol = len(line)
	}
	line = append(line[:e.curCol], append([]rune{r}, line[e.curCol:]...)...)
	e.lines[e.curLine] = string(line)
	e.curCol++
	e.modified = true
}

func (e *viEditor) backspace() {
	line := []rune(e.line())
	if e.curCol > 0 {
		line = append(line[:e.curCol-1], line[e.curCol:]...)
		e.lines[e.curLine] = string(line)
		e.curCol--
		e.modified = true
		return
	}
	if e.curLine > 0 {
		prev := []rune(e.lines[e.curLine-1])
		e.curCol = len(prev)
		e.lines[e.curLine-1] += e.lines[e.curLine]
		e.lines = append(e.lines[:e.curLine], e.lines[e.curLine+1:]...)
		e.curLine--
		e.modified = true
	}
}

func (e *viEditor) newline() {
	line := []rune(e.line())
	right := append([]rune(nil), line[e.curCol:]...)
	left := string(line[:e.curCol])
	e.lines[e.curLine] = left
	e.lines = append(e.lines[:e.curLine+1], append([]string{string(right)}, e.lines[e.curLine+1:]...)...)
	e.curLine++
	e.curCol = 0
	e.modified = true
}

func (e *viEditor) moveLeft(n int) {
	for ; n > 0 && e.curCol > 0; n-- {
		e.curCol--
	}
}

func (e *viEditor) moveRight(n int) {
	for ; n > 0 && e.curCol < len([]rune(e.line())); n-- {
		e.curCol++
	}
}

func (e *viEditor) moveDown(n int) {
	e.curLine += n
	if e.curLine >= len(e.lines) {
		e.curLine = len(e.lines) - 1
	}
	if e.curLine < 0 {
		e.curLine = 0
	}
	if e.curCol > len([]rune(e.line())) {
		e.curCol = len([]rune(e.line()))
	}
}

func (e *viEditor) moveUp(n int) {
	e.moveDown(-n)
}

func (e *viEditor) gotoLine(n int) {
	if n < 0 {
		n = 0
	}
	if n >= len(e.lines) {
		n = len(e.lines) - 1
	}
	e.curLine = n
	if e.curCol > len([]rune(e.line())) {
		e.curCol = len([]rune(e.line()))
	}
}

func (e *viEditor) moveWord(dir, n int) {
	for ; n > 0; n-- {
		if dir > 0 {
			for e.curLine < len(e.lines) {
				runes := []rune(e.line())
				for e.curCol < len(runes) && isWord(runes[e.curCol]) {
					e.curCol++
				}
				for e.curCol < len(runes) && !isWord(runes[e.curCol]) {
					e.curCol++
				}
				if e.curCol < len(runes) {
					return
				}
				if e.curLine == len(e.lines)-1 {
					return
				}
				e.curLine++
				e.curCol = 0
			}
		} else if e.curCol > 0 {
			e.curCol--
			for e.curCol > 0 && !isWord(runeAt(e.line(), e.curCol)) {
				e.curCol--
			}
		}
	}
}

func (e *viEditor) openLine(above bool) {
	if above {
		e.lines = append(e.lines[:e.curLine], append([]string{""}, e.lines[e.curLine:]...)...)
	} else {
		e.lines = append(e.lines[:e.curLine+1], append([]string{""}, e.lines[e.curLine+1:]...)...)
		e.curLine++
	}
	e.curCol = 0
	e.modified = true
}

func (e *viEditor) deleteChars(n int) {
	line := []rune(e.line())
	if e.curCol >= len(line) {
		return
	}
	end := e.curCol + n
	if end > len(line) {
		end = len(line)
	}
	e.lines[e.curLine] = string(append(line[:e.curCol], line[end:]...))
	e.modified = true
}

func (e *viEditor) deleteLine(n int) {
	if n <= 0 {
		n = 1
	}
	if len(e.lines) == 0 {
		e.lines = []string{""}
		e.curLine = 0
		e.curCol = 0
		return
	}
	start := e.curLine
	end := e.curLine + n
	if end > len(e.lines) {
		end = len(e.lines)
	}
	e.yank = append([]string(nil), e.lines[start:end]...)
	e.lines = append(e.lines[:start], e.lines[end:]...)
	if len(e.lines) == 0 {
		e.lines = []string{""}
	}
	if e.curLine >= len(e.lines) {
		e.curLine = len(e.lines) - 1
	}
	e.curCol = 0
	e.modified = true
}

func (e *viEditor) yankLine(n int) {
	if n <= 0 {
		n = 1
	}
	start := e.curLine
	end := e.curLine + n
	if end > len(e.lines) {
		end = len(e.lines)
	}
	e.yank = append([]string(nil), e.lines[start:end]...)
	e.status = fmt.Sprintf("%d lines yanked", len(e.yank))
}

func (e *viEditor) changeLine(n int) {
	e.deleteLine(n)
	e.mode = viInsert
	e.prompt = "-- INSERT --"
}

func (e *viEditor) putAfter(n int) {
	if len(e.yank) == 0 {
		return
	}
	pos := e.curLine + 1
	cp := append([]string(nil), e.yank...)
	for ; n > 0; n-- {
		e.lines = append(e.lines[:pos], append(cp, e.lines[pos:]...)...)
		pos += len(cp)
	}
	e.modified = true
}

func (e *viEditor) putBefore(n int) {
	if len(e.yank) == 0 {
		return
	}
	pos := e.curLine
	cp := append([]string(nil), e.yank...)
	for ; n > 0; n-- {
		e.lines = append(e.lines[:pos], append(cp, e.lines[pos:]...)...)
		pos += len(cp)
	}
	e.modified = true
}

func (e *viEditor) joinLine(n int) {
	for ; n > 0; n-- {
		if e.curLine >= len(e.lines)-1 {
			return
		}
		e.lines[e.curLine] = strings.TrimRight(e.lines[e.curLine], " \t") + " " + strings.TrimLeft(e.lines[e.curLine+1], " \t")
		e.lines = append(e.lines[:e.curLine+1], e.lines[e.curLine+2:]...)
		e.modified = true
	}
}

func (e *viEditor) replaceChar() {
	e.status = "r not yet implemented"
}

func (e *viEditor) replaceChars(n int, r rune) {
	if n <= 0 {
		n = 1
	}
	line := []rune(e.line())
	for ; n > 0; n-- {
		if e.curCol >= len(line) {
			return
		}
		line[e.curCol] = r
		if e.curCol < len(line)-1 {
			e.curCol++
		}
	}
	e.lines[e.curLine] = string(line)
	e.modified = true
}

func (e *viEditor) substitute(cmd string) {
	parts := strings.SplitN(cmd[2:], "/", 2)
	if len(parts) != 2 {
		e.status = "bad substitute"
		return
	}
	old := parts[0]
	new := parts[1]
	line := e.line()
	if strings.Contains(line, old) {
		e.lines[e.curLine] = strings.Replace(line, old, new, 1)
		e.modified = true
	}
}

func (e *viEditor) search(pattern string, forward bool) {
	if pattern == "" {
		pattern = e.lastFind
	}
	if pattern == "" {
		e.status = "No previous search"
		return
	}
	e.lastFind = pattern
	e.lastFwd = forward
	pat := []rune(pattern)
	if forward {
		for i := e.curLine; i < len(e.lines); i++ {
			line := []rune(e.lines[i])
			start := 0
			if i == e.curLine {
				start = e.curCol + 1
			}
			if start > len(line) {
				continue
			}
			if idx := indexRunes(line[start:], pat); idx >= 0 {
				e.curLine = i
				e.curCol = start + idx
				return
			}
		}
	} else {
		for i := e.curLine; i >= 0; i-- {
			line := []rune(e.lines[i])
			end := len(line)
			if i == e.curLine {
				end = min(e.curCol, len(line))
			}
			if idx := lastIndexRunes(line[:end], pat); idx >= 0 {
				e.curLine = i
				e.curCol = idx
				return
			}
		}
	}
	e.status = "Pattern not found"
}

func (e *viEditor) searchRepeat() {
	if e.lastFind == "" {
		e.status = "No previous search"
		return
	}
	e.search(e.lastFind, e.lastFwd)
}

func (e *viEditor) searchRepeatOpposite() {
	if e.lastFind == "" {
		e.status = "No previous search"
		return
	}
	e.search(e.lastFind, !e.lastFwd)
}

func runeAt(s string, idx int) rune {
	r := []rune(s)
	if idx < 0 || idx >= len(r) {
		return 0
	}
	return r[idx]
}

func isWord(r rune) bool {
	return r == '_' || ('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z') || ('0' <= r && r <= '9')
}

func isSpace(r rune) bool {
	return r == ' ' || r == '\t'
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func indexRunes(s, sub []rune) int {
	if len(sub) == 0 {
		return 0
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		match := true
		for j := range sub {
			if s[i+j] != sub[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

func lastIndexRunes(s, sub []rune) int {
	if len(sub) == 0 {
		return len(s)
	}
	for i := len(s) - len(sub); i >= 0; i-- {
		match := true
		for j := range sub {
			if s[i+j] != sub[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

func applyViRaw(termios *unix.Termios) {
	termios.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP | unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON
	termios.Oflag &^= unix.OPOST
	termios.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN
	termios.Cflag &^= unix.CSIZE | unix.PARENB
	termios.Cflag |= unix.CS8
	termios.Cc[unix.VMIN] = 1
	termios.Cc[unix.VTIME] = 0
}
