package selinux

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"github.com/agentbusybox/pkg/applet"
	"golang.org/x/sys/unix"
)

func init() {
	applet.Register(&applet.Applet{Name: "chcon", Short: "Change SELinux security context", Func: runChcon})
	applet.Register(&applet.Applet{Name: "getenforce", Short: "Get SELinux enforcing mode", Func: runGetenforce})
	applet.Register(&applet.Applet{Name: "getsebool", Short: "Get SELinux boolean value", Func: runGetsebool})
	applet.Register(&applet.Applet{Name: "load_policy", Short: "Load SELinux policy", Func: runLoadPolicy})
	applet.Register(&applet.Applet{Name: "matchpathcon", Short: "Get default SELinux context for path", Func: runMatchpathcon})
	applet.Register(&applet.Applet{Name: "restorecon", Short: "Restore SELinux security context", Func: runSetfiles})
	applet.Register(&applet.Applet{Name: "runcon", Short: "Run command with SELinux context", Func: runRuncon})
	applet.Register(&applet.Applet{Name: "selinuxenabled", Short: "Exit successfully if SELinux is enabled", Func: runSelinuxenabled})
	applet.Register(&applet.Applet{Name: "sestatus", Short: "Show SELinux status", Func: runSestatus})
	applet.Register(&applet.Applet{Name: "setenforce", Short: "Set SELinux enforcing mode", Func: runSetenforce})
	applet.Register(&applet.Applet{Name: "setfiles", Short: "Set SELinux security contexts", Func: runSetfiles})
	applet.Register(&applet.Applet{Name: "setsebool", Short: "Set SELinux boolean value", Func: runSetsebool})
}

func selinuxPath(parts ...string) string {
	all := append([]string{"/sys/fs/selinux"}, parts...)
	return strings.Join(all, "/")
}

func selinuxEnabled() bool {
	if st, err := os.Stat("/sys/fs/selinux"); err == nil && st.IsDir() {
		return true
	}
	if data, err := os.ReadFile("/proc/filesystems"); err == nil {
		return strings.Contains(string(data), "selinuxfs")
	}
	return false
}

func runGetenforce(args []string) int {
	if !selinuxEnabled() {
		fmt.Println("Disabled")
		return 0
	}
	data, err := os.ReadFile(selinuxPath("enforce"))
	if err != nil {
		fmt.Println("Permissive")
		return 0
	}
	if strings.TrimSpace(string(data)) == "1" {
		fmt.Println("Enforcing")
	} else {
		fmt.Println("Permissive")
	}
	return 0
}

func runSelinuxenabled(args []string) int {
	if selinuxEnabled() {
		return 0
	}
	return 1
}

func runSestatus(args []string) int {
	status := "disabled"
	mode := "disabled"
	if selinuxEnabled() {
		status = "enabled"
		if data, err := os.ReadFile(selinuxPath("enforce")); err == nil && strings.TrimSpace(string(data)) == "1" {
			mode = "enforcing"
		} else {
			mode = "permissive"
		}
	}
	fmt.Printf("SELinux status:                 %s\n", status)
	fmt.Printf("Current mode:                   %s\n", mode)
	return 0
}

func runSetenforce(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "setenforce: missing mode\n")
		return 1
	}
	mode := args[1]
	switch strings.ToLower(mode) {
	case "1", "enforcing":
		mode = "1"
	case "0", "permissive":
		mode = "0"
	default:
		fmt.Fprintf(os.Stderr, "setenforce: invalid mode %s\n", args[1])
		return 1
	}
	if err := os.WriteFile(selinuxPath("enforce"), []byte(mode), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "setenforce: %v\n", err)
		return 1
	}
	return 0
}

func runGetsebool(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "getsebool: missing boolean name\n")
		return 1
	}
	for _, name := range args[1:] {
		data, err := os.ReadFile(selinuxPath("booleans", name))
		if err != nil {
			fmt.Fprintf(os.Stderr, "getsebool: %s: %v\n", name, err)
			return 1
		}
		fields := strings.Fields(string(data))
		state := "off"
		if len(fields) > 0 && fields[0] == "1" {
			state = "on"
		}
		fmt.Printf("%s --> %s\n", name, state)
	}
	return 0
}

func runSetsebool(args []string) int {
	if len(args) < 3 {
		fmt.Fprintf(os.Stderr, "setsebool: usage: setsebool BOOLEAN VALUE [BOOLEAN VALUE...]\n")
		return 1
	}
	persistent := false
	pairs := []string{}
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-P":
			persistent = true
		default:
			pairs = append(pairs, args[i])
		}
	}
	if len(pairs) == 0 || len(pairs)%2 != 0 {
		fmt.Fprintf(os.Stderr, "setsebool: usage: setsebool BOOLEAN VALUE [BOOLEAN VALUE...]\n")
		return 1
	}
	if persistent {
		fmt.Fprintf(os.Stderr, "setsebool: -P is not supported in pure Go\n")
		return 1
	}
	exitCode := 0
	for i := 0; i < len(pairs); i += 2 {
		name := pairs[i]
		value, err := normalizeSelinuxBool(pairs[i+1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "setsebool: %s: %v\n", name, err)
			exitCode = 1
			continue
		}
		if err := os.WriteFile(selinuxPath("booleans", name), []byte(value), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "setsebool: %s: %v\n", name, err)
			exitCode = 1
		}
	}
	return exitCode
}

func runLoadPolicy(args []string) int {
	fmt.Fprintf(os.Stderr, "load_policy: not yet implemented in pure Go\n")
	return 1
}

func runChcon(args []string) int {
	recursive := false
	dereference := true
	context := ""
	files := []string{}

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-R", "--recursive":
			recursive = true
		case "-h", "--no-dereference":
			dereference = false
		case "--reference":
			fmt.Fprintf(os.Stderr, "chcon: --reference is not yet implemented in pure Go\n")
			return 1
		case "-u", "-r", "-t", "-l":
			fmt.Fprintf(os.Stderr, "chcon: component mode is not yet implemented in pure Go\n")
			return 1
		default:
			if strings.HasPrefix(args[i], "--reference=") {
				fmt.Fprintf(os.Stderr, "chcon: --reference is not yet implemented in pure Go\n")
				return 1
			}
			if strings.HasPrefix(args[i], "-") {
				continue
			}
			if context == "" {
				context = args[i]
			} else {
				files = append(files, args[i])
			}
		}
	}

	if context == "" || len(files) == 0 {
		fmt.Fprintf(os.Stderr, "chcon: usage: chcon [-R] [-h] CONTEXT FILE...\n")
		return 1
	}

	exitCode := 0
	for _, target := range files {
		if recursive {
			err := filepath.Walk(target, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				return setSELinuxContext(path, context, dereference)
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "chcon: %s: %v\n", target, err)
				exitCode = 1
			}
			continue
		}
		if err := setSELinuxContext(target, context, dereference); err != nil {
			fmt.Fprintf(os.Stderr, "chcon: %s: %v\n", target, err)
			exitCode = 1
		}
	}
	return exitCode
}

func runMatchpathcon(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "matchpathcon: missing path\n")
		return 1
	}
	for _, path := range args[1:] {
		if strings.HasPrefix(path, "-") {
			continue
		}
		fmt.Printf("%s\t<<none>>\n", path)
	}
	return 0
}

func runRuncon(args []string) int {
	fmt.Fprintf(os.Stderr, "runcon: not yet implemented in pure Go\n")
	return 1
}

func runSetfiles(args []string) int {
	fmt.Fprintf(os.Stderr, "%s: not yet implemented in pure Go\n", args[0])
	return 1
}

func normalizeSelinuxBool(v string) (string, error) {
	switch strings.ToLower(v) {
	case "1", "on", "true":
		return "1", nil
	case "0", "off", "false":
		return "0", nil
	default:
		return "", fmt.Errorf("invalid value %q", v)
	}
}

func setSELinuxContext(path, context string, dereference bool) error {
	data := []byte(context)
	if dereference {
		return unix.Setxattr(path, "security.selinux", data, 0)
	}
	pathBytes := append([]byte(path), 0)
	nameBytes := append([]byte("security.selinux"), 0)
	_, _, errno := unix.Syscall6(unix.SYS_LSETXATTR,
		uintptr(unsafe.Pointer(&pathBytes[0])),
		uintptr(unsafe.Pointer(&nameBytes[0])),
		uintptr(unsafeBytesPointer(data)),
		uintptr(len(data)),
		0,
		0)
	if errno != 0 {
		return errno
	}
	return nil
}

func unsafeBytesPointer(b []byte) unsafe.Pointer {
	if len(b) == 0 {
		return nil
	}
	return unsafe.Pointer(&b[0])
}
