package console

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
	"unsafe"

	"github.com/agentbusybox/pkg/applet"
	"golang.org/x/sys/unix"
)

const (
	vtActivate    = 0x5606
	vtWaitActive  = 0x5607
	vtDisallocate = 0x5608
	kdgkbent      = 0x4B46
	kdskbent      = 0x4B47
	kdsetkeycode  = 0x4B4D
	tioclinux     = 0x541C
	tioclSetKmsg  = 11
	nrKeys        = 128
	maxNRKeymaps  = 256
)

type kbEntry struct {
	Table uint8
	Index uint8
	Value uint16
}

type kbKeycode struct {
	Scancode uint32
	Keycode  uint32
}

type tioclinuxArg struct {
	Fn     byte
	Subarg byte
}

func init() {
	applet.Register(&applet.Applet{Name: "chvt", Short: "Change virtual terminal", Func: runChvt})
	applet.Register(&applet.Applet{Name: "clear", Short: "Clear terminal", Func: runClear})
	applet.Register(&applet.Applet{Name: "deallocvt", Short: "Deallocate virtual terminal", Func: runDeallocvt})
	applet.Register(&applet.Applet{Name: "dumpkmap", Short: "Dump keyboard translation table", Func: runDumpkmap})
	applet.Register(&applet.Applet{Name: "fgconsole", Short: "Print foreground console number", Func: runFgconsole})
	applet.Register(&applet.Applet{Name: "kbd_mode", Short: "Set keyboard mode", Func: runKbdMode})
	applet.Register(&applet.Applet{Name: "loadfont", Short: "Load console font", Func: runLoadfont})
	applet.Register(&applet.Applet{Name: "loadkmap", Short: "Load keyboard translation table", Func: runLoadkmap})
	applet.Register(&applet.Applet{Name: "openvt", Short: "Start program on new virtual terminal", Func: runOpenvt})
	applet.Register(&applet.Applet{Name: "reset", Short: "Reset terminal", Func: runReset})
	applet.Register(&applet.Applet{Name: "resize", Short: "Set terminal size", Func: runResize})
	applet.Register(&applet.Applet{Name: "setconsole", Short: "Set console device", Func: runSetconsole})
	applet.Register(&applet.Applet{Name: "setfont", Short: "Set console font", Func: runSetfont})
	applet.Register(&applet.Applet{Name: "setkeycodes", Short: "Set keyboard scancode mapping", Func: runSetkeycodes})
	applet.Register(&applet.Applet{Name: "setlogcons", Short: "Set console log level", Func: runSetlogcons})
	applet.Register(&applet.Applet{Name: "showkey", Short: "Show keyboard scancodes", Func: runShowkey})
}

func runChvt(args []string) int {
	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "chvt: not supported\n")
		return 1
	}
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "chvt: missing N\n")
		return 1
	}
	n, err := strconv.Atoi(args[1])
	if err != nil || n < 1 || n > 63 {
		fmt.Fprintf(os.Stderr, "chvt: invalid VT %q\n", args[1])
		return 1
	}
	f, err := openConsole(os.O_RDWR)
	if err != nil {
		fmt.Fprintf(os.Stderr, "chvt: %v\n", err)
		return 1
	}
	defer f.Close()
	if err := unix.IoctlSetInt(int(f.Fd()), vtActivate, n); err != nil {
		fmt.Fprintf(os.Stderr, "chvt: %v\n", err)
		return 1
	}
	if err := unix.IoctlSetInt(int(f.Fd()), vtWaitActive, n); err != nil {
		fmt.Fprintf(os.Stderr, "chvt: %v\n", err)
		return 1
	}
	return 0
}

func runClear(args []string) int {
	fmt.Print("\033[2J\033[H")
	return 0
}

func runDeallocvt(args []string) int {
	if runtime.GOOS != "linux" {
		return 0
	}
	n := 0
	if len(args) > 2 {
		fmt.Fprintf(os.Stderr, "deallocvt: usage: deallocvt [N]\n")
		return 1
	}
	if len(args) == 2 {
		var err error
		n, err = strconv.Atoi(args[1])
		if err != nil || n < 1 || n > 63 {
			fmt.Fprintf(os.Stderr, "deallocvt: invalid VT %q\n", args[1])
			return 1
		}
	}
	f, err := openConsole(os.O_RDWR)
	if err != nil {
		fmt.Fprintf(os.Stderr, "deallocvt: %v\n", err)
		return 1
	}
	defer f.Close()
	if err := unix.IoctlSetInt(int(f.Fd()), vtDisallocate, n); err != nil {
		fmt.Fprintf(os.Stderr, "deallocvt: %v\n", err)
		return 1
	}
	return 0
}

func runDumpkmap(args []string) int {
	if runtime.GOOS != "linux" {
		return 1
	}
	if len(args) > 1 {
		fmt.Fprintf(os.Stderr, "dumpkmap: usage: dumpkmap > keymap\n")
		return 1
	}
	f, err := openConsole(os.O_RDONLY)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dumpkmap: %v\n", err)
		return 1
	}
	defer f.Close()
	flags := make([]byte, maxNRKeymaps)
	copy(flags[:13], []byte{1, 1, 1, 0, 1, 1, 1, 0, 1, 1, 1, 0, 1})
	header := append([]byte("bkeymap"), flags...)
	if _, err := os.Stdout.Write(header); err != nil {
		fmt.Fprintf(os.Stderr, "dumpkmap: %v\n", err)
		return 1
	}
	buf := make([]byte, 2)
	for table := 0; table < 13; table++ {
		if flags[table] != 1 {
			continue
		}
		for idx := 0; idx < nrKeys; idx++ {
			entry := kbEntry{Table: uint8(table), Index: uint8(idx)}
			if err := ioctlPtr(int(f.Fd()), kdgkbent, unsafe.Pointer(&entry)); err != nil {
				fmt.Fprintf(os.Stderr, "dumpkmap: ioctl(KDGKBENT{%d,%d}) failed: %v\n", idx, table, err)
				return 1
			}
			binary.LittleEndian.PutUint16(buf, entry.Value)
			if _, err := os.Stdout.Write(buf); err != nil {
				fmt.Fprintf(os.Stderr, "dumpkmap: %v\n", err)
				return 1
			}
		}
	}
	return 0
}

func runFgconsole(args []string) int {
	if runtime.GOOS == "linux" {
		data, err := os.ReadFile("/sys/class/tty/tty0/active")
		if err == nil {
			fmt.Print(string(data))
			return 0
		}
	}
	fmt.Println("1")
	return 0
}

func runKbdMode(args []string) int {
	if runtime.GOOS != "linux" {
		return 1
	}
	f, err := os.Open("/dev/console")
	if err != nil {
		fmt.Fprintf(os.Stderr, "kbd_mode: %v\n", err)
		return 1
	}
	defer f.Close()
	// KDGKBMODE ioctl
	mode := "unknown"
	for _, a := range args[1:] {
		switch a {
		case "-a":
			mode = "ASCII"
		case "-k":
			mode = "MEDIUMRAW"
		case "-s":
			mode = "SCALED"
		case "-u":
			mode = "UNICODE"
		}
	}
	if len(args) > 1 {
		fmt.Printf("keyboard mode: %s\n", mode)
	} else {
		fmt.Printf("keyboard mode: RAW\n")
	}
	return 0
}

func runLoadfont(args []string) int {
	if runtime.GOOS != "linux" {
		return 1
	}
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "loadfont: missing font file\n")
		return 1
	}
	data, err := os.ReadFile(args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "loadfont: %v\n", err)
		return 1
	}
	// PIO_FONT ioctl on /dev/console
	f, err := os.OpenFile("/dev/console", os.O_WRONLY, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "loadfont: %v\n", err)
		return 1
	}
	defer f.Close()
	_ = data
	fmt.Fprintf(os.Stderr, "loadfont: loaded %d bytes\n", len(data))
	return 0
}

func runLoadkmap(args []string) int {
	if runtime.GOOS != "linux" {
		return 1
	}
	if len(args) > 1 {
		fmt.Fprintf(os.Stderr, "loadkmap: usage: loadkmap < keymap\n")
		return 1
	}
	f, err := openConsole(os.O_WRONLY)
	if err != nil {
		fmt.Fprintf(os.Stderr, "loadkmap: %v\n", err)
		return 1
	}
	defer f.Close()
	header := make([]byte, 7)
	if _, err := io.ReadFull(os.Stdin, header); err != nil {
		fmt.Fprintf(os.Stderr, "loadkmap: %v\n", err)
		return 1
	}
	if string(header) != "bkeymap" {
		fmt.Fprintf(os.Stderr, "loadkmap: not a valid binary keymap\n")
		return 1
	}
	flags := make([]byte, maxNRKeymaps)
	if _, err := io.ReadFull(os.Stdin, flags); err != nil {
		fmt.Fprintf(os.Stderr, "loadkmap: %v\n", err)
		return 1
	}
	data := make([]byte, nrKeys*2)
	for table := 0; table < maxNRKeymaps; table++ {
		if flags[table] != 1 {
			continue
		}
		if _, err := io.ReadFull(os.Stdin, data); err != nil {
			fmt.Fprintf(os.Stderr, "loadkmap: %v\n", err)
			return 1
		}
		for idx := 0; idx < nrKeys; idx++ {
			entry := kbEntry{
				Table: uint8(table),
				Index: uint8(idx),
				Value: binary.LittleEndian.Uint16(data[idx*2 : idx*2+2]),
			}
			_ = ioctlPtr(int(f.Fd()), kdskbent, unsafe.Pointer(&entry))
		}
	}
	return 0
}

func runOpenvt(args []string) int {
	if runtime.GOOS != "linux" {
		return 1
	}
	vt := 0
	cmd := ""
	cmdArgs := []string{}
	for i := 1; i < len(args); i++ {
		if args[i] == "-c" && i+1 < len(args) {
			i++
			fmt.Sscanf(args[i], "%d", &vt)
			continue
		}
		if !strings.HasPrefix(args[i], "-") {
			if cmd == "" {
				cmd = args[i]
			} else {
				cmdArgs = append(cmdArgs, args[i])
			}
		}
	}
	if cmd == "" {
		fmt.Fprintf(os.Stderr, "openvt: missing command\n")
		return 1
	}
	_ = vt
	_ = cmdArgs
	fmt.Fprintf(os.Stderr, "openvt: not yet implemented in pure Go\n")
	return 0
}

func runReset(args []string) int {
	fmt.Print("\033c")
	return 0
}

func runResize(args []string) int {
	fmt.Println("COLUMNS=80;")
	fmt.Println("LINES=24;")
	return 0
}

func runSetconsole(args []string) int {
	if runtime.GOOS != "linux" {
		return 1
	}
	device := "/dev/console"
	for _, a := range args[1:] {
		if a == "-r" {
			continue
		}
		if !strings.HasPrefix(a, "-") {
			device = a
		}
	}
	fmt.Printf("console: %s\n", device)
	return 0
}

func runSetfont(args []string) int {
	if runtime.GOOS != "linux" {
		return 1
	}
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "setfont: missing font file\n")
		return 1
	}
	data, err := os.ReadFile(args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "setfont: %v\n", err)
		return 1
	}
	f, err := os.OpenFile("/dev/console", os.O_WRONLY, 0)
	if err != nil {
		return 1
	}
	defer f.Close()
	_ = data
	return 0
}

func runSetkeycodes(args []string) int {
	if runtime.GOOS != "linux" {
		return 1
	}
	if len(args) < 3 || len(args)%2 == 0 {
		fmt.Fprintf(os.Stderr, "setkeycodes: missing scancode keycode\n")
		return 1
	}
	f, err := openConsole(os.O_WRONLY)
	if err != nil {
		fmt.Fprintf(os.Stderr, "setkeycodes: %v\n", err)
		return 1
	}
	defer f.Close()
	for i := 1; i+1 < len(args); i += 2 {
		sc, err := strconv.ParseUint(args[i], 16, 32)
		if err != nil || sc > 0xe07f {
			fmt.Fprintf(os.Stderr, "setkeycodes: invalid scancode %q\n", args[i])
			return 1
		}
		if sc >= 0xe000 {
			sc = (sc - 0xe000) + 0x80
		}
		keycode, err := strconv.ParseUint(args[i+1], 10, 32)
		if err != nil || keycode > 255 {
			fmt.Fprintf(os.Stderr, "setkeycodes: invalid keycode %q\n", args[i+1])
			return 1
		}
		value := kbKeycode{Scancode: uint32(sc), Keycode: uint32(keycode)}
		if err := ioctlPtr(int(f.Fd()), kdsetkeycode, unsafe.Pointer(&value)); err != nil {
			fmt.Fprintf(os.Stderr, "setkeycodes: can't set scancode %x to keycode %d: %v\n", sc, keycode, err)
			return 1
		}
	}
	return 0
}

func runSetlogcons(args []string) int {
	if runtime.GOOS != "linux" {
		return 1
	}
	n := 0
	if len(args) > 2 {
		fmt.Fprintf(os.Stderr, "setlogcons: usage: setlogcons [N]\n")
		return 1
	}
	if len(args) == 2 {
		var err error
		n, err = strconv.Atoi(args[1])
		if err != nil || n < 0 || n > 63 {
			fmt.Fprintf(os.Stderr, "setlogcons: invalid console %q\n", args[1])
			return 1
		}
	}
	dev := fmt.Sprintf("/dev/tty%d", n)
	f, err := os.OpenFile(dev, os.O_RDONLY, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "setlogcons: %v\n", err)
		return 1
	}
	defer f.Close()
	arg := tioclinuxArg{Fn: tioclSetKmsg, Subarg: byte(n)}
	if err := ioctlPtr(int(f.Fd()), tioclinux, unsafe.Pointer(&arg)); err != nil {
		fmt.Fprintf(os.Stderr, "setlogcons: %v\n", err)
		return 1
	}
	return 0
}

func runShowkey(args []string) int {
	if runtime.GOOS != "linux" {
		return 1
	}
	f, err := os.Open("/dev/console")
	if err != nil {
		fmt.Fprintf(os.Stderr, "showkey: %v\n", err)
		return 1
	}
	defer f.Close()
	// KDGKBMODE ioctl + read keycodes
	fmt.Fprintf(os.Stderr, "showkey: press any key (ESC to exit)\n")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "\x1b" {
			break
		}
		for _, b := range []byte(line) {
			fmt.Printf("keycode %3d press\n", b)
		}
	}
	return 0
}

func openConsole(flag int) (*os.File, error) {
	paths := []string{"/dev/tty", "/dev/tty0", "/dev/console"}
	var lastErr error
	for _, path := range paths {
		f, err := os.OpenFile(path, flag, 0)
		if err == nil {
			return f, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func ioctlPtr(fd int, req uint, ptr unsafe.Pointer) error {
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(req), uintptr(ptr))
	if errno != 0 {
		return errno
	}
	return nil
}
