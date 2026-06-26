package klibc

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/agentbusybox/pkg/applet"
)

func init() {
	applet.Register(&applet.Applet{Name: "nuke", Short: "Remove directories recursively", Func: runNuke})
	applet.Register(&applet.Applet{Name: "resume", Short: "Resume from suspend-to-disk image", Func: runResume})
	applet.Register(&applet.Applet{Name: "run-init", Short: "Switch from initramfs to real root", Func: runRunInit})
}

func runNuke(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "nuke: missing directory\n")
		return 1
	}
	for _, name := range args[1:] {
		clean := filepath.Clean(name)
		if clean == "." || clean == ".." || clean == string(os.PathSeparator) {
			fmt.Fprintf(os.Stderr, "nuke: refusing to remove %s\n", name)
			continue
		}
		_ = os.RemoveAll(name)
	}
	return 0
}

func runResume(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "resume: missing block device\n")
		return 1
	}
	dev, err := resumeDevice(args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "resume: %v\n", err)
		return 1
	}
	if len(args) > 2 {
		if err := os.WriteFile("/sys/power/resume_offset", []byte(args[2]), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "resume: %v\n", err)
			return 1
		}
	}
	if err := os.WriteFile("/sys/power/resume", []byte(dev), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "resume: %v\n", err)
		return 1
	}
	return 1
}

func resumeDevice(name string) (string, error) {
	if strings.Contains(name, ":") {
		parts := strings.SplitN(name, ":", 2)
		if _, err := strconv.Atoi(parts[0]); err == nil {
			if _, err := strconv.Atoi(parts[1]); err == nil {
				return name, nil
			}
		}
	}
	base := strings.TrimPrefix(name, "/dev/")
	if data, err := os.ReadFile(filepath.Join("/sys/class/block", base, "dev")); err == nil {
		return strings.TrimSpace(string(data)), nil
	}
	return "", fmt.Errorf("invalid resume device: %s", name)
}

func runRunInit(args []string) int {
	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "run-init: not supported on %s\n", runtime.GOOS)
		return 1
	}
	console := ""
	capList := ""
	dryRun := false
	operands := make([]string, 0, len(args))
	operands = append(operands, args[0])
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-c":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "run-init: option %s requires an argument\n", args[i])
				return 1
			}
			i++
			console = args[i]
		case "-d":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "run-init: option %s requires an argument\n", args[i])
				return 1
			}
			i++
			capList = args[i]
		case "-n":
			dryRun = true
		default:
			operands = append(operands, args[i])
		}
	}
	if len(operands) < 3 {
		fmt.Fprintf(os.Stderr, "run-init: usage: run-init [-d CAPS] [-n] [-c CONSOLE_DEV] NEW_ROOT NEW_INIT [ARGS...]\n")
		return 1
	}
	if capList != "" {
		if err := dropCapabilities(capList); err != nil {
			fmt.Fprintf(os.Stderr, "run-init: %v\n", err)
			return 1
		}
	}
	return executeSwitchRoot(operands[1], operands[2:], console, dryRun, "run-init")
}

func executeSwitchRoot(newRoot string, initArgv []string, console string, dryRun bool, name string) int {
	if err := os.Chdir(newRoot); err != nil {
		fmt.Fprintf(os.Stderr, "%s: chdir %s: %v\n", name, newRoot, err)
		return 1
	}
	rootInfo, err := os.Stat("/")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: stat /: %v\n", name, err)
		return 1
	}
	newInfo, err := os.Stat(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: stat %s: %v\n", name, newRoot, err)
		return 1
	}
	rootStat, ok1 := rootInfo.Sys().(*syscall.Stat_t)
	newStat, ok2 := newInfo.Sys().(*syscall.Stat_t)
	if !ok1 || !ok2 {
		fmt.Fprintf(os.Stderr, "%s: unsupported stat result\n", name)
		return 1
	}
	if rootStat.Dev == newStat.Dev {
		fmt.Fprintf(os.Stderr, "%s: %s must be a mountpoint\n", name, newRoot)
		return 1
	}
	if !dryRun && os.Getpid() != 1 {
		fmt.Fprintf(os.Stderr, "%s: must be run as PID 1\n", name)
		return 1
	}
	var stfs unix.Statfs_t
	if err := unix.Statfs("/", &stfs); err != nil {
		fmt.Fprintf(os.Stderr, "%s: statfs /: %v\n", name, err)
		return 1
	}
	if stfs.Type != unix.RAMFS_MAGIC && stfs.Type != unix.TMPFS_MAGIC {
		fmt.Fprintf(os.Stderr, "%s: root filesystem is not ramfs/tmpfs\n", name)
		return 1
	}
	if !dryRun {
		if err := deleteInitramfsContents("/", rootStat.Dev); err != nil {
			fmt.Fprintf(os.Stderr, "%s: cleanup failed: %v\n", name, err)
			return 1
		}
		if err := unix.Mount(".", "/", "", unix.MS_MOVE, ""); err != nil {
			fmt.Fprintf(os.Stderr, "%s: error moving root: %v\n", name, err)
			return 1
		}
	}
	if err := unix.Chroot("."); err != nil {
		fmt.Fprintf(os.Stderr, "%s: chroot: %v\n", name, err)
		return 1
	}
	if err := unix.Chdir("/"); err != nil {
		fmt.Fprintf(os.Stderr, "%s: chdir: %v\n", name, err)
		return 1
	}
	if console != "" {
		if err := reopenTTY(console); err != nil {
			fmt.Fprintf(os.Stderr, "%s: console %s: %v\n", name, console, err)
			return 1
		}
	}
	if dryRun {
		if st, err := os.Stat(initArgv[0]); err == nil && st.Mode()&0111 != 0 {
			return 0
		}
		fmt.Fprintf(os.Stderr, "%s: can't execute %q\n", name, initArgv[0])
		return 1
	}
	if err := syscall.Exec(initArgv[0], initArgv, os.Environ()); err != nil {
		fmt.Fprintf(os.Stderr, "%s: can't execute %q: %v\n", name, initArgv[0], err)
		return 1
	}
	return 0
}

func deleteInitramfsContents(dir string, rootDev uint64) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := deleteInitramfsEntry(filepath.Join(dir, entry.Name()), rootDev); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func deleteInitramfsEntry(path string, rootDev uint64) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || uint64(stat.Dev) != rootDev {
		return nil
	}
	if !info.IsDir() {
		return os.Remove(path)
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := deleteInitramfsEntry(filepath.Join(path, entry.Name()), rootDev); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return os.Remove(path)
}

func reopenTTY(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	fd := int(f.Fd())
	for _, target := range []int{0, 1, 2} {
		if err := unix.Dup2(fd, target); err != nil {
			return err
		}
	}
	return nil
}

func dropCapabilities(list string) error {
	hdr := unix.CapUserHeader{Version: unix.LINUX_CAPABILITY_VERSION_3}
	data := [2]unix.CapUserData{}
	if err := unix.Capget(&hdr, &data[0]); err != nil {
		return err
	}
	for _, item := range strings.Split(list, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		capID, err := parseCapability(item)
		if err != nil {
			return err
		}
		idx := capID / 32
		mask := uint32(1) << uint(capID%32)
		data[idx].Inheritable &^= mask
		if _, _, errno := unix.Syscall6(unix.SYS_PRCTL, unix.PR_CAPBSET_READ, uintptr(capID), 0, 0, 0, 0); errno == 0 {
			if _, _, errno := unix.Syscall6(unix.SYS_PRCTL, unix.PR_CAPBSET_DROP, uintptr(capID), 0, 0, 0, 0); errno != 0 {
				return errno
			}
		}
	}
	return unix.Capset(&hdr, &data[0])
}

func parseCapability(name string) (int, error) {
	if n, err := strconv.Atoi(name); err == nil && n >= 0 {
		return n, nil
	}
	lookup := map[string]int{
		"cap_chown":              0,
		"cap_dac_override":       1,
		"cap_dac_read_search":    2,
		"cap_fowner":             3,
		"cap_fsetid":             4,
		"cap_kill":               5,
		"cap_setgid":             6,
		"cap_setuid":             7,
		"cap_setpcap":            8,
		"cap_linux_immutable":    9,
		"cap_net_bind_service":   10,
		"cap_net_broadcast":      11,
		"cap_net_admin":          12,
		"cap_net_raw":            13,
		"cap_ipc_lock":           14,
		"cap_ipc_owner":          15,
		"cap_sys_module":         16,
		"cap_sys_rawio":          17,
		"cap_sys_chroot":         18,
		"cap_sys_ptrace":         19,
		"cap_sys_pacct":          20,
		"cap_sys_admin":          21,
		"cap_sys_boot":           22,
		"cap_sys_nice":           23,
		"cap_sys_resource":       24,
		"cap_sys_time":           25,
		"cap_sys_tty_config":     26,
		"cap_mknod":              27,
		"cap_lease":              28,
		"cap_audit_write":        29,
		"cap_audit_control":      30,
		"cap_setfcap":            31,
		"cap_mac_override":       32,
		"cap_mac_admin":          33,
		"cap_syslog":             34,
		"cap_wake_alarm":         35,
		"cap_block_suspend":      36,
		"cap_audit_read":         37,
		"cap_perfmon":            38,
		"cap_bpf":                39,
		"cap_checkpoint_restore": 40,
	}
	if id, ok := lookup[strings.ToLower(name)]; ok {
		return id, nil
	}
	return 0, fmt.Errorf("unknown capability %q", name)
}
