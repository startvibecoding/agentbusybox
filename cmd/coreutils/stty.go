package coreutils

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/agentbusybox/pkg/applet"
	"golang.org/x/sys/unix"
)

func init() {
	applet.Register(&applet.Applet{Name: "stty", Short: "Print or change terminal characteristics", Func: runStty})
}

func runStty(args []string) int {
	showAll := false
	showG := false
	settings := []string{}

	for _, a := range args[1:] {
		switch a {
		case "-a", "--all":
			showAll = true
		case "-g", "--save":
			showG = true
		default:
			settings = append(settings, a)
		}
	}
	if runtime.GOOS == "windows" {
		fmt.Fprintf(os.Stderr, "stty: not supported on this platform\n")
		return 1
	}
	termios, fd, err := currentTermios()
	if err != nil {
		fmt.Fprintf(os.Stderr, "stty: %v\n", err)
		return 1
	}

	if showG {
		printSttyEncoded(termios)
		return 0
	}

	if showAll || len(settings) == 0 {
		printSttyReadable(termios, showAll)
		if len(settings) == 0 {
			return 0
		}
	}

	for _, s := range settings {
		switch s {
		case "echo":
			termios.Lflag |= unix.ECHO
		case "-echo":
			termios.Lflag &^= unix.ECHO
		case "icanon":
			termios.Lflag |= unix.ICANON
		case "-icanon":
			termios.Lflag &^= unix.ICANON
		case "raw":
			applyRaw(termios)
		case "sane":
			applySane(termios)
		default:
			fmt.Fprintf(os.Stderr, "stty: invalid argument '%s'\n", s)
			return 1
		}
	}
	if err := setCurrentTermios(fd, termios); err != nil {
		fmt.Fprintf(os.Stderr, "stty: %v\n", err)
		return 1
	}
	return 0
}

func currentTermios() (*unix.Termios, int, error) {
	for _, fd := range []int{0, 1, 2} {
		t, err := unix.IoctlGetTermios(fd, unix.TCGETS)
		if err == nil {
			return t, fd, nil
		}
	}
	return nil, -1, fmt.Errorf("no controlling terminal")
}

func setCurrentTermios(fd int, termios *unix.Termios) error {
	return unix.IoctlSetTermios(fd, unix.TCSETS, termios)
}

func applyRaw(termios *unix.Termios) {
	termios.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP | unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON
	termios.Oflag &^= unix.OPOST
	termios.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN
	termios.Cflag &^= unix.CSIZE | unix.PARENB
	termios.Cflag |= unix.CS8
	termios.Cc[unix.VMIN] = 1
	termios.Cc[unix.VTIME] = 0
}

func applySane(termios *unix.Termios) {
	termios.Lflag |= unix.ECHO | unix.ECHOE | unix.ECHOK | unix.ICANON | unix.ISIG | unix.IEXTEN
	termios.Iflag |= unix.ICRNL | unix.IXON
	termios.Oflag |= unix.OPOST
	termios.Cflag |= unix.CREAD | unix.CS8
	termios.Cc[unix.VINTR] = 3
	termios.Cc[unix.VQUIT] = 28
	termios.Cc[unix.VERASE] = 127
	termios.Cc[unix.VKILL] = 21
	termios.Cc[unix.VEOF] = 4
	termios.Cc[unix.VMIN] = 1
	termios.Cc[unix.VTIME] = 0
}

func printSttyReadable(termios *unix.Termios, verbose bool) {
	fmt.Printf("speed %d baud; line = 0;\n", sttySpeed(termios))
	flags := []string{}
	if termios.Lflag&unix.ECHO != 0 {
		flags = append(flags, "echo")
	} else {
		flags = append(flags, "-echo")
	}
	if termios.Lflag&unix.ICANON != 0 {
		flags = append(flags, "icanon")
	} else {
		flags = append(flags, "-icanon")
	}
	if verbose {
		if termios.Lflag&unix.ISIG != 0 {
			flags = append(flags, "isig")
		} else {
			flags = append(flags, "-isig")
		}
		if termios.Oflag&unix.OPOST != 0 {
			flags = append(flags, "opost")
		} else {
			flags = append(flags, "-opost")
		}
	}
	fmt.Println(strings.Join(flags, " "))
}

func printSttyEncoded(termios *unix.Termios) {
	fmt.Printf("%x:%x:%x:%x", termios.Iflag, termios.Oflag, termios.Cflag, termios.Lflag)
	for _, cc := range termios.Cc {
		fmt.Printf(":%x", cc)
	}
	fmt.Println()
}

func sttySpeed(termios *unix.Termios) int {
	switch termios.Cflag & unix.CBAUD {
	case unix.B0:
		return 0
	case unix.B50:
		return 50
	case unix.B75:
		return 75
	case unix.B110:
		return 110
	case unix.B134:
		return 134
	case unix.B150:
		return 150
	case unix.B200:
		return 200
	case unix.B300:
		return 300
	case unix.B600:
		return 600
	case unix.B1200:
		return 1200
	case unix.B1800:
		return 1800
	case unix.B2400:
		return 2400
	case unix.B4800:
		return 4800
	case unix.B9600:
		return 9600
	case unix.B19200:
		return 19200
	case unix.B38400:
		return 38400
	default:
		return 0
	}
}
