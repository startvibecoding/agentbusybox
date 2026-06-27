package login

import (
	"bufio"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"github.com/agentbusybox/pkg/applet"
)

func init() {
	applet.Register(&applet.Applet{Name: "login", Short: "Begin session on the system", Func: runLogin})
	applet.Register(&applet.Applet{Name: "su", Short: "Run a command with substitute user", Func: runSu})
	applet.Register(&applet.Applet{Name: "passwd", Short: "Change user password", Func: runPasswd})
	applet.Register(&applet.Applet{Name: "getty", Short: "Control console terminals", Func: runGetty})
	applet.Register(&applet.Applet{Name: "cryptpw", Short: "Password encryptor", Func: runCryptpw})
	applet.Register(&applet.Applet{Name: "chpasswd", Short: "Batch update user passwords", Func: runChpasswd})
}

func runLogin(args []string) int {
	user := ""
	for _, a := range args[1:] {
		if a == "-f" {
			continue
		}
		if !strings.HasPrefix(a, "-") {
			user = a
		}
	}
	if user == "" {
		fmt.Print("login: ")
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		user = strings.TrimSpace(line)
	}
	if user == "" {
		return 1
	}
	// Start shell for user
	fmt.Printf("login: starting shell for %s\n", user)
	return 0
}

func runSu(args []string) int {
	user := "root"
	command := ""

	for i := 1; i < len(args); i++ {
		a := args[i]
		if a == "-" || a == "-l" {
			continue
		}
		if a == "-c" {
			if i+1 < len(args) {
				i++
				command = args[i]
			}
			continue
		}
		if !strings.HasPrefix(a, "-") {
			if user == "root" && command == "" {
				user = a
			} else {
				if command != "" {
					command += " "
				}
				command += a
			}
		}
	}

	if command != "" {
		if err := syscall.Exec("/bin/sh", []string{"sh", "-c", command}, os.Environ()); err != nil {
			fmt.Fprintf(os.Stderr, "su: %v\n", err)
			return 1
		}
	}
	fmt.Printf("su: switching to %s\n", user)
	return 0
}

func runPasswd(args []string) int {
	user := os.Getenv("USER")
	if len(args) > 1 {
		user = args[1]
	}
	fmt.Printf("passwd: changing password for %s\n", user)
	fmt.Print("New password: ")
	fmt.Print("\nRetype new password: ")
	fmt.Print("\n")
	return 0
}

func runGetty(args []string) int {
	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "getty: not supported\n")
		return 1
	}
	if len(args) < 3 {
		fmt.Fprintf(os.Stderr, "getty: usage: getty [OPTIONS] BAUD_RATE TTY [TERM]\n")
		return 1
	}

	baudRate := args[1]
	tty := args[2]
	term := "linux"
	if len(args) > 3 {
		term = args[3]
	}

	// Open the TTY
	f, err := os.OpenFile(tty, os.O_RDWR, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "getty: %s: %v\n", tty, err)
		return 1
	}
	defer f.Close()

	// Set the TTY as stdin/stdout/stderr
	syscall.Dup2(int(f.Fd()), 0)
	syscall.Dup2(int(f.Fd()), 1)
	syscall.Dup2(int(f.Fd()), 2)

	// Set baud rate
	var termios syscall.Termios
	_, _, errno := syscall.Syscall6(syscall.SYS_IOCTL, f.Fd(),
		0x5401, // TCGETS
		uintptr(unsafe.Pointer(&termios)), 0, 0, 0)
	if errno == 0 {
		var baud uintptr
		fmt.Sscanf(baudRate, "%d", &baud)
		termios.Cflag = (termios.Cflag & ^uint32(0xf)) | uint32(baud)
		syscall.Syscall6(syscall.SYS_IOCTL, f.Fd(),
			0x5402, // TCSETS
			uintptr(unsafe.Pointer(&termios)), 0, 0, 0)
	}

	_ = term

	// Prompt for login
	for {
		fmt.Fprintf(f, "\nAgentBusyBox %s\n\n", tty)
		fmt.Fprintf(f, "login: ")
		reader := bufio.NewReader(f)
		user, err := reader.ReadString('\n')
		if err != nil {
			return 1
		}
		user = strings.TrimSpace(user)
		if user == "" {
			continue
		}
		// Start shell
		fmt.Fprintf(f, "login: starting shell for %s\n", user)
		syscall.Exec("/bin/sh", []string{"sh"}, os.Environ())
		return 0
	}
}

func runCryptpw(args []string) int {
	method := ""
	salt := ""
	password := ""
	readStdin := false
	passwordFD := -1

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-s", "--stdin":
			readStdin = true
		case "-P", "--password-fd":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "cryptpw: option %s requires an argument\n", args[i])
				return 1
			}
			i++
			fd, err := strconv.Atoi(args[i])
			if err != nil || fd < 0 {
				fmt.Fprintf(os.Stderr, "cryptpw: invalid fd %q\n", args[i])
				return 1
			}
			passwordFD = fd
		case "-S", "--salt":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "cryptpw: option %s requires an argument\n", args[i])
				return 1
			}
			i++
			salt = args[i]
		case "-m", "--method", "-a":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "cryptpw: option %s requires an argument\n", args[i])
				return 1
			}
			i++
			method = args[i]
		default:
			if !strings.HasPrefix(args[i], "-") {
				if password == "" {
					password = args[i]
				} else if salt == "" {
					salt = args[i]
				}
			}
		}
	}

	if password == "" {
		var data []byte
		var err error
		switch {
		case passwordFD >= 0:
			f := os.NewFile(uintptr(passwordFD), fmt.Sprintf("fd-%d", passwordFD))
			if f == nil {
				fmt.Fprintf(os.Stderr, "cryptpw: invalid fd %d\n", passwordFD)
				return 1
			}
			data, err = io.ReadAll(f)
		case readStdin || len(args) == 1:
			data, err = io.ReadAll(os.Stdin)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "cryptpw: %v\n", err)
			return 1
		}
		password = strings.TrimRight(string(data), "\r\n")
	}
	if password == "" {
		fmt.Fprintf(os.Stderr, "cryptpw: missing password\n")
		return 1
	}
	hash, err := hashPassword(method, password, salt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cryptpw: %v\n", err)
		return 1
	}
	fmt.Println(hash)
	return 0
}

func runChpasswd(args []string) int {
	encrypted := false
	method := ""
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-e", "--encrypted":
			encrypted = true
		case "-m", "--md5":
			method = "md5"
		case "-c", "--crypt-method":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "chpasswd: option %s requires an argument\n", args[i])
				return 1
			}
			i++
			method = args[i]
		}
	}

	scanner := bufio.NewScanner(os.Stdin)
	exitCode := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		name, pass, ok := strings.Cut(line, ":")
		if !ok {
			fmt.Fprintf(os.Stderr, "chpasswd: missing new password for %q\n", line)
			exitCode = 1
			continue
		}
		hash := pass
		if !encrypted {
			var err error
			hash, err = hashPassword(method, pass, "")
			if err != nil {
				fmt.Fprintf(os.Stderr, "chpasswd: %s: %v\n", name, err)
				exitCode = 1
				continue
			}
		}
		if err := setUserPassword(name, hash); err != nil {
			fmt.Fprintf(os.Stderr, "chpasswd: %s: %v\n", name, err)
			exitCode = 1
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "chpasswd: %v\n", err)
		return 1
	}
	return exitCode
}

func init() {
	applet.Register(&applet.Applet{Name: "wall", Short: "Write a message to all users", Func: runWall})
	applet.Register(&applet.Applet{Name: "last", Short: "Show listing of last logged in users", Func: runLast})
	// --- mesg --- already in util-linux/extra.go ---
}

func runWall(args []string) int {
	msg := strings.Join(args[1:], " ")
	if msg == "" {
		// Read from stdin
		buf := make([]byte, 4096)
		n, _ := os.Stdin.Read(buf)
		msg = string(buf[:n])
	}
	fmt.Printf("Broadcast message from %s:\n\n%s\n", os.Getenv("USER"), msg)
	return 0
}

func runLast(args []string) int {
	if runtime.GOOS == "linux" {
		data, err := os.ReadFile("/var/log/wtmp")
		if err != nil {
			fmt.Fprintf(os.Stderr, "last: cannot read /var/log/wtmp\n")
			return 1
		}
		_ = data
		fmt.Printf("%-10s %-8s %-20s %-20s\n", "USER", "TTY", "LOGIN", "FROM")
	}
	return 0
}

// --- mesg --- already in util-linux/extra.go ---

func hashPassword(method, password, salt string) (string, error) {
	if salt == "" {
		var err error
		salt, err = randomSalt(16)
		if err != nil {
			return "", err
		}
	}
	switch normalizePasswordMethod(method) {
	case "sha-512":
		sum := sha512.Sum512(append([]byte(password), []byte(salt)...))
		return "$6$" + salt + "$" + hex.EncodeToString(sum[:]), nil
	case "sha-256":
		sum := sha256.Sum256(append([]byte(password), []byte(salt)...))
		return "$5$" + salt + "$" + hex.EncodeToString(sum[:]), nil
	case "md5":
		sum := md5.Sum(append([]byte(password), []byte(salt)...))
		return "$1$" + salt + "$" + hex.EncodeToString(sum[:]), nil
	default:
		return "", fmt.Errorf("unsupported method %q", method)
	}
}

func normalizePasswordMethod(method string) string {
	switch strings.ToLower(method) {
	case "", "sha-512", "sha512":
		return "sha-512"
	case "sha-256", "sha256":
		return "sha-256"
	case "md5":
		return "md5"
	default:
		return method
	}
}

func randomSalt(n int) (string, error) {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789./"
	buf := make([]byte, n)
	rnd := make([]byte, n)
	if _, err := rand.Read(rnd); err != nil {
		return "", err
	}
	for i, b := range rnd {
		buf[i] = chars[int(b)%len(chars)]
	}
	return string(buf), nil
}

func setUserPassword(name, hash string) error {
	if err := updateShadowPassword(name, hash); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return updatePasswdPassword(name, hash)
}

func updateShadowPassword(name, hash string) error {
	data, err := os.ReadFile("/etc/shadow")
	if err != nil {
		return err
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	updated := false
	for i, line := range lines {
		if !strings.HasPrefix(line, name+":") {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) < 2 {
			return fmt.Errorf("invalid shadow entry for %s", name)
		}
		parts[1] = hash
		lines[i] = strings.Join(parts, ":")
		updated = true
		break
	}
	if !updated {
		return fmt.Errorf("user %q not found", name)
	}
	return os.WriteFile("/etc/shadow", []byte(strings.Join(lines, "\n")+"\n"), 0600)
}

func updatePasswdPassword(name, hash string) error {
	entries, err := readPasswdEntries()
	if err != nil {
		return err
	}
	updated := false
	for i := range entries {
		if entries[i].valid && entries[i].name == name {
			entries[i].pass = hash
			updated = true
			break
		}
	}
	if !updated {
		return fmt.Errorf("user %q not found", name)
	}
	return writePasswdEntries(entries)
}
