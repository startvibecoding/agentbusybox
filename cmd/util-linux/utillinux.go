package utillinux

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"github.com/agentbusybox/pkg/applet"
	"golang.org/x/sys/unix"
)

func init() {
	applet.Register(&applet.Applet{Name: "mount", Short: "Mount a filesystem", Func: runMount})
	applet.Register(&applet.Applet{Name: "umount", Short: "Unmount a filesystem", Func: runUmount})
	applet.Register(&applet.Applet{Name: "swapon", Short: "Enable devices for paging", Func: runSwapon})
	applet.Register(&applet.Applet{Name: "swapoff", Short: "Disable devices for paging", Func: runSwapoff})
	applet.Register(&applet.Applet{Name: "fdisk", Short: "Manipulate disk partition table", Func: runFdisk})
	applet.Register(&applet.Applet{Name: "blkid", Short: "Print block device attributes", Func: runBlkid})
	applet.Register(&applet.Applet{Name: "dmesg", Short: "Print or control kernel ring buffer", Func: runDmesg})
	applet.Register(&applet.Applet{Name: "fsck.minix", Short: "Check MINIX filesystem", Func: runFsckMinix})
	applet.Register(&applet.Applet{Name: "losetup", Short: "Set up and control loop devices", Func: runLosetup})
	applet.Register(&applet.Applet{Name: "mke2fs", Short: "Build an ext2 filesystem", Func: runMkfsExt2})
	applet.Register(&applet.Applet{Name: "mkdosfs", Short: "Build a FAT filesystem", Func: runMkfsVfat})
	applet.Register(&applet.Applet{Name: "mkfs.ext2", Short: "Build an ext2 filesystem", Func: runMkfsExt2})
	applet.Register(&applet.Applet{Name: "mkfs.minix", Short: "Build a MINIX filesystem", Func: runMkfsMinix})
	applet.Register(&applet.Applet{Name: "mkfs.reiser", Short: "Build a ReiserFS filesystem", Func: runMkfsReiser})
	applet.Register(&applet.Applet{Name: "mkfs.vfat", Short: "Build a FAT filesystem", Func: runMkfsVfat})
	applet.Register(&applet.Applet{Name: "mkswap", Short: "Set up a Linux swap area", Func: runMkswap})
	applet.Register(&applet.Applet{Name: "more", Short: "File perusal filter", Func: runMore})
	applet.Register(&applet.Applet{Name: "hexdump", Short: "Dump files in hex", Func: runHexdump})
	applet.Register(&applet.Applet{Name: "hd", Short: "Dump files in hex", Func: runHexdump})
	applet.Register(&applet.Applet{Name: "renice", Short: "Alter priority of running processes", Func: runRenice})
	applet.Register(&applet.Applet{Name: "chrt", Short: "Manipulate real-time attributes", Func: runChrt})
	applet.Register(&applet.Applet{Name: "taskset", Short: "Set or retrieve a process CPU affinity", Func: runTaskset})
	applet.Register(&applet.Applet{Name: "nsenter", Short: "Run program with namespaces of other processes", Func: runNsenter})
	applet.Register(&applet.Applet{Name: "unshare", Short: "Run program with some namespaces unshared", Func: runUnshare})
	applet.Register(&applet.Applet{Name: "fstrim", Short: "Discard unused blocks on a mounted filesystem", Func: runFstrim})
	applet.Register(&applet.Applet{Name: "lsns", Short: "List namespaces", Func: runLsns})
	applet.Register(&applet.Applet{Name: "fsfreeze", Short: "Freeze/thaw an ext2/ext3/ext4 filesystem", Func: runFsfreeze})
	applet.Register(&applet.Applet{Name: "findmnt", Short: "Find a filesystem", Func: runFindmnt})
	applet.Register(&applet.Applet{Name: "partprobe", Short: "Inform OS of partition table changes", Func: runPartprobe})
	applet.Register(&applet.Applet{Name: "mkfs", Short: "Build a Linux filesystem", Func: runMkfs})
	applet.Register(&applet.Applet{Name: "fsck", Short: "Check and repair a Linux filesystem", Func: runFsck})
}

func runMount(args []string) int {
	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "mount: not supported on this platform\n")
		return 1
	}

	if len(args) == 1 {
		// Show mounted filesystems
		f, err := os.Open("/proc/mounts")
		if err != nil {
			f, err = os.Open("/etc/mtab")
			if err != nil {
				fmt.Fprintf(os.Stderr, "mount: cannot read mount table\n")
				return 1
			}
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			parts := strings.Fields(scanner.Text())
			if len(parts) >= 3 {
				fmt.Printf("%s on %s type %s (%s)\n", parts[0], parts[1], parts[2], strings.Join(parts[3:], ","))
			}
		}
		return 0
	}

	source := ""
	target := ""
	fstype := ""
	options := "defaults"
	all := false
	operands := []string{}

	for i := 1; i < len(args); i++ {
		a := args[i]
		switch a {
		case "-a":
			all = true
		case "-r":
			options += ",ro"
		case "-w":
			options += ",rw"
		case "-t":
			if i+1 < len(args) {
				i++
				fstype = args[i]
			}
		case "-o":
			if i+1 < len(args) {
				i++
				options = args[i]
			}
		default:
			if strings.HasPrefix(a, "-t") && len(a) > 2 {
				fstype = a[2:]
			} else if strings.HasPrefix(a, "-o") && len(a) > 2 {
				options = a[2:]
			} else if !strings.HasPrefix(a, "-") {
				operands = append(operands, a)
			}
		}
	}

	if all {
		return mountAllFromFstab()
	}

	switch len(operands) {
	case 1:
		source, target, fstype, options = lookupFstabMount(operands[0])
	case 2:
		source, target = operands[0], operands[1]
	default:
		fmt.Fprintf(os.Stderr, "mount: missing device or mount point\n")
		return 1
	}
	if target == "" {
		fmt.Fprintf(os.Stderr, "mount: cannot find %s in /etc/fstab\n", operands[0])
		return 1
	}
	if fstype == "" {
		fstype = "auto"
	}
	if err := mountOne(source, target, fstype, options); err != nil {
		fmt.Fprintf(os.Stderr, "mount: %s on %s: %v\n", source, target, err)
		return 1
	}
	return 0
}

func runUmount(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "umount: missing mount point\n")
		return 1
	}
	flags := 0
	targets := []string{}
	for _, a := range args[1:] {
		switch a {
		case "-f":
			flags |= unix.MNT_FORCE
		case "-l":
			flags |= unix.MNT_DETACH
		case "-n", "-r":
		default:
			if !strings.HasPrefix(a, "-") {
				targets = append(targets, a)
			}
		}
	}
	if len(targets) == 0 {
		fmt.Fprintf(os.Stderr, "umount: missing mount point\n")
		return 1
	}
	exitCode := 0
	for _, target := range targets {
		if err := unix.Unmount(target, flags); err != nil {
			fmt.Fprintf(os.Stderr, "umount: %s: %v\n", target, err)
			exitCode = 1
		}
	}
	return exitCode
}

func runSwapon(args []string) int {
	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "swapon: not supported\n")
		return 1
	}
	devices := swapDevicesFromArgs(args)
	if len(devices) == 0 {
		fmt.Fprintf(os.Stderr, "swapon: missing device\n")
		return 1
	}
	exitCode := 0
	for _, dev := range devices {
		if err := linuxSwapon(dev, 0); err != nil {
			fmt.Fprintf(os.Stderr, "swapon: %s: %v\n", dev, err)
			exitCode = 1
		}
	}
	return exitCode
}

func runSwapoff(args []string) int {
	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "swapoff: not supported\n")
		return 1
	}
	devices := swapDevicesFromArgs(args)
	if len(devices) == 0 {
		fmt.Fprintf(os.Stderr, "swapoff: missing device\n")
		return 1
	}
	exitCode := 0
	for _, dev := range devices {
		if err := linuxSwapoff(dev); err != nil {
			fmt.Fprintf(os.Stderr, "swapoff: %s: %v\n", dev, err)
			exitCode = 1
		}
	}
	return exitCode
}

func runFdisk(args []string) int {
	list := len(args) == 1
	for _, a := range args[1:] {
		if a == "-l" {
			list = true
		}
	}
	if list {
		return listBlockDisks("fdisk")
	}
	fmt.Fprintf(os.Stderr, "fdisk: interactive partition editing is not yet implemented in pure Go\n")
	return 1
}

func runBlkid(args []string) int {
	devices := []string{}
	for _, a := range args[1:] {
		if !strings.HasPrefix(a, "-") {
			devices = append(devices, a)
		}
	}
	if len(devices) == 0 {
		devices = allBlockDevicePaths()
	}
	exitCode := 0
	for _, dev := range devices {
		info, err := probeBlockID(dev)
		if err != nil {
			exitCode = 1
			continue
		}
		if info.typ == "" && info.uuid == "" && info.label == "" {
			continue
		}
		fmt.Printf("%s:", dev)
		if info.uuid != "" {
			fmt.Printf(" UUID=\"%s\"", info.uuid)
		}
		if info.label != "" {
			fmt.Printf(" LABEL=\"%s\"", info.label)
		}
		if info.typ != "" {
			fmt.Printf(" TYPE=\"%s\"", info.typ)
		}
		fmt.Println()
	}
	return exitCode
}

func runLosetup(args []string) int {
	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "losetup: not supported\n")
		return 1
	}
	if len(args) == 1 || args[1] == "-a" {
		return listLoopDevices()
	}
	if args[1] == "-f" {
		dev, err := firstFreeLoop()
		if err != nil {
			fmt.Fprintf(os.Stderr, "losetup: %v\n", err)
			return 1
		}
		fmt.Println(dev)
		return 0
	}
	if args[1] == "-d" && len(args) > 2 {
		return detachLoop(args[2])
	}
	operands := []string{}
	for _, a := range args[1:] {
		if !strings.HasPrefix(a, "-") {
			operands = append(operands, a)
		}
	}
	if len(operands) == 2 {
		return attachLoop(operands[0], operands[1])
	}
	fmt.Fprintf(os.Stderr, "losetup: usage: losetup [-a|-f|-d LOOPDEV|LOOPDEV FILE]\n")
	return 1
}

func runMkswap(args []string) int {
	devices := []string{}
	for _, a := range args[1:] {
		if !strings.HasPrefix(a, "-") {
			devices = append(devices, a)
		}
	}
	if len(devices) == 0 {
		fmt.Fprintf(os.Stderr, "mkswap: missing device\n")
		return 1
	}
	exitCode := 0
	for _, dev := range devices {
		if err := writeSwapSignature(dev); err != nil {
			fmt.Fprintf(os.Stderr, "mkswap: %s: %v\n", dev, err)
			exitCode = 1
		}
	}
	return exitCode
}

func runDmesg(args []string) int {
	if runtime.GOOS == "linux" {
		size, err := unix.Klogctl(unix.SYSLOG_ACTION_SIZE_BUFFER, nil)
		if err == nil && size > 0 {
			buf := make([]byte, size)
			n, err := unix.Klogctl(unix.SYSLOG_ACTION_READ_ALL, buf)
			if err == nil {
				os.Stdout.Write(buf[:n])
				return 0
			}
		}
		data, err := os.ReadFile("/var/log/dmesg")
		if err != nil {
			data, err = os.ReadFile("/proc/kmsg")
			if err != nil {
				fmt.Fprintf(os.Stderr, "dmesg: %v\n", err)
				return 1
			}
		}
		os.Stdout.Write(data)
		return 0
	}
	fmt.Fprintf(os.Stderr, "dmesg: not supported\n")
	return 1
}

type fstabEntry struct {
	source string
	target string
	fstype string
	opts   string
}

func readFstab() []fstabEntry {
	data, err := os.ReadFile("/etc/fstab")
	if err != nil {
		return nil
	}
	entries := []fstabEntry{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 4 {
			entries = append(entries, fstabEntry{source: fields[0], target: fields[1], fstype: fields[2], opts: fields[3]})
		}
	}
	return entries
}

func lookupFstabMount(operand string) (string, string, string, string) {
	for _, entry := range readFstab() {
		if entry.source == operand || entry.target == operand {
			return resolveDeviceSpec(entry.source), entry.target, entry.fstype, entry.opts
		}
	}
	return "", "", "", ""
}

func mountAllFromFstab() int {
	exitCode := 0
	for _, entry := range readFstab() {
		if entry.fstype == "swap" || mountOptionHas(entry.opts, "noauto") {
			continue
		}
		if err := mountOne(resolveDeviceSpec(entry.source), entry.target, entry.fstype, entry.opts); err != nil {
			fmt.Fprintf(os.Stderr, "mount: %s on %s: %v\n", entry.source, entry.target, err)
			exitCode = 1
		}
	}
	return exitCode
}

func mountOne(source, target, fstype, opts string) error {
	if strings.HasPrefix(source, "UUID=") || strings.HasPrefix(source, "LABEL=") {
		source = resolveDeviceSpec(source)
	}
	flags, data := parseMountOptions(opts)
	if fstype == "auto" {
		if info, err := probeBlockID(source); err == nil && info.typ != "" {
			fstype = info.typ
		} else {
			fstype = ""
		}
	}
	return unix.Mount(source, target, fstype, flags, data)
}

func parseMountOptions(opts string) (uintptr, string) {
	var flags uintptr
	data := []string{}
	for _, opt := range strings.Split(opts, ",") {
		switch strings.TrimSpace(opt) {
		case "", "defaults", "rw", "auto", "noauto":
		case "ro":
			flags |= unix.MS_RDONLY
		case "nosuid":
			flags |= unix.MS_NOSUID
		case "nodev":
			flags |= unix.MS_NODEV
		case "noexec":
			flags |= unix.MS_NOEXEC
		case "sync":
			flags |= unix.MS_SYNCHRONOUS
		case "dirsync":
			flags |= unix.MS_DIRSYNC
		case "noatime":
			flags |= unix.MS_NOATIME
		case "nodiratime":
			flags |= unix.MS_NODIRATIME
		case "relatime":
			flags |= unix.MS_RELATIME
		case "bind":
			flags |= unix.MS_BIND
		case "rbind":
			flags |= unix.MS_BIND | unix.MS_REC
		case "move":
			flags |= unix.MS_MOVE
		case "remount":
			flags |= unix.MS_REMOUNT
		case "rec":
			flags |= unix.MS_REC
		default:
			data = append(data, opt)
		}
	}
	return flags, strings.Join(data, ",")
}

func mountOptionHas(opts, want string) bool {
	for _, opt := range strings.Split(opts, ",") {
		if strings.TrimSpace(opt) == want {
			return true
		}
	}
	return false
}

func swapDevicesFromArgs(args []string) []string {
	all := false
	devices := []string{}
	for _, a := range args[1:] {
		if a == "-a" {
			all = true
			continue
		}
		if !strings.HasPrefix(a, "-") {
			devices = append(devices, a)
		}
	}
	if all {
		for _, entry := range readFstab() {
			if entry.fstype == "swap" && !mountOptionHas(entry.opts, "noauto") {
				devices = append(devices, resolveDeviceSpec(entry.source))
			}
		}
	}
	return devices
}

func linuxSwapon(path string, flags int) error {
	p, err := unix.BytePtrFromString(path)
	if err != nil {
		return err
	}
	_, _, errno := unix.Syscall(unix.SYS_SWAPON, uintptr(unsafe.Pointer(p)), uintptr(flags), 0)
	if errno != 0 {
		return errno
	}
	return nil
}

func linuxSwapoff(path string) error {
	p, err := unix.BytePtrFromString(path)
	if err != nil {
		return err
	}
	_, _, errno := unix.Syscall(unix.SYS_SWAPOFF, uintptr(unsafe.Pointer(p)), 0, 0)
	if errno != 0 {
		return errno
	}
	return nil
}

func writeSwapSignature(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	page := int64(os.Getpagesize())
	if page < 4096 {
		page = 4096
	}
	if _, err := f.WriteAt([]byte("SWAPSPACE2"), page-10); err != nil {
		return err
	}
	fmt.Printf("Setting up swapspace on %s\n", path)
	return nil
}

type blockID struct {
	uuid  string
	label string
	typ   string
}

const (
	ioctlFIFREEZE = 0xc0045877
	ioctlFITHAW   = 0xc0045878
	ioctlFITRIM   = 0xc0185879
)

type fstrimRange struct {
	Start  uint64
	Len    uint64
	Minlen uint64
}

func resolveDeviceSpec(spec string) string {
	if strings.HasPrefix(spec, "UUID=") {
		if path := findByDiskDir("/dev/disk/by-uuid", strings.TrimPrefix(spec, "UUID=")); path != "" {
			return path
		}
	}
	if strings.HasPrefix(spec, "LABEL=") {
		if path := findByDiskDir("/dev/disk/by-label", strings.TrimPrefix(spec, "LABEL=")); path != "" {
			return path
		}
	}
	return spec
}

func findByDiskDir(dir, name string) string {
	path := filepath.Join(dir, name)
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return ""
}

func allBlockDevicePaths() []string {
	entries, err := os.ReadDir("/sys/class/block")
	if err != nil {
		return nil
	}
	paths := []string{}
	for _, entry := range entries {
		paths = append(paths, filepath.Join("/dev", entry.Name()))
	}
	return paths
}

func probeBlockID(dev string) (blockID, error) {
	info := blockID{}
	if byUUID := symlinkNameForDevice("/dev/disk/by-uuid", dev); byUUID != "" {
		info.uuid = byUUID
	}
	if byLabel := symlinkNameForDevice("/dev/disk/by-label", dev); byLabel != "" {
		info.label = byLabel
	}
	f, err := os.Open(dev)
	if err != nil {
		return info, err
	}
	defer f.Close()
	if typ, label, uuid := probeExt(f); typ != "" {
		info.typ = typ
		if info.label == "" {
			info.label = label
		}
		if info.uuid == "" {
			info.uuid = uuid
		}
		return info, nil
	}
	if typ, label := probeFAT(f); typ != "" {
		info.typ = typ
		if info.label == "" {
			info.label = label
		}
		return info, nil
	}
	if typ, label, uuid := probeXFS(f); typ != "" {
		info.typ = typ
		if info.label == "" {
			info.label = label
		}
		if info.uuid == "" {
			info.uuid = uuid
		}
		return info, nil
	}
	if isSwap(f) {
		info.typ = "swap"
		return info, nil
	}
	if label := probeISO9660(f); label != "" {
		info.typ = "iso9660"
		if info.label == "" {
			info.label = label
		}
		return info, nil
	}
	return info, nil
}

func symlinkNameForDevice(dir, dev string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	want, err := filepath.EvalSymlinks(dev)
	if err != nil {
		want = dev
	}
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		resolved, err := filepath.EvalSymlinks(path)
		if err == nil && resolved == want {
			return entry.Name()
		}
	}
	return ""
}

func probeExt(f *os.File) (string, string, string) {
	buf := make([]byte, 2048)
	if _, err := f.ReadAt(buf, 1024); err != nil {
		return "", "", ""
	}
	if buf[0x38] != 0x53 || buf[0x39] != 0xef {
		return "", "", ""
	}
	uuid := formatUUID(buf[0x68 : 0x68+16])
	label := strings.TrimRight(string(buf[0x78:0x78+16]), "\x00 ")
	return "ext2", label, uuid
}

func probeFAT(f *os.File) (string, string) {
	buf := make([]byte, 512)
	if _, err := f.ReadAt(buf, 0); err != nil {
		return "", ""
	}
	if buf[510] != 0x55 || buf[511] != 0xaa {
		return "", ""
	}
	if string(buf[82:90]) == "FAT32   " {
		return "vfat", strings.TrimSpace(string(buf[71:82]))
	}
	if string(buf[54:59]) == "FAT12" || string(buf[54:59]) == "FAT16" {
		return "vfat", strings.TrimSpace(string(buf[43:54]))
	}
	return "", ""
}

func probeXFS(f *os.File) (string, string, string) {
	buf := make([]byte, 256)
	if _, err := f.ReadAt(buf, 0); err != nil {
		return "", "", ""
	}
	if string(buf[:4]) != "XFSB" {
		return "", "", ""
	}
	return "xfs", strings.TrimRight(string(buf[108:120]), "\x00 "), formatUUID(buf[32:48])
}

func isSwap(f *os.File) bool {
	page := int64(os.Getpagesize())
	if page < 4096 {
		page = 4096
	}
	buf := make([]byte, 10)
	_, err := f.ReadAt(buf, page-10)
	return err == nil && (string(buf) == "SWAPSPACE2" || string(buf) == "SWAP-SPACE")
}

func probeISO9660(f *os.File) string {
	buf := make([]byte, 2048)
	if _, err := f.ReadAt(buf, 16*2048); err != nil {
		return ""
	}
	if buf[0] == 1 && string(buf[1:6]) == "CD001" {
		return strings.TrimSpace(string(buf[40:72]))
	}
	return ""
}

func formatUUID(b []byte) string {
	if len(b) != 16 {
		return ""
	}
	hexed := hex.EncodeToString(b)
	return fmt.Sprintf("%s-%s-%s-%s-%s", hexed[0:8], hexed[8:12], hexed[12:16], hexed[16:20], hexed[20:32])
}

func listLoopDevices() int {
	entries, err := os.ReadDir("/sys/block")
	if err != nil {
		fmt.Fprintf(os.Stderr, "losetup: %v\n", err)
		return 1
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "loop") {
			continue
		}
		backing := trimFile(filepath.Join("/sys/block", entry.Name(), "loop/backing_file"))
		if backing != "" {
			fmt.Printf("/dev/%s: %s\n", entry.Name(), backing)
		}
	}
	return 0
}

func firstFreeLoop() (string, error) {
	if f, err := os.OpenFile("/dev/loop-control", os.O_RDONLY, 0); err == nil {
		defer f.Close()
		n, err := unix.IoctlRetInt(int(f.Fd()), unix.LOOP_CTL_GET_FREE)
		if err == nil {
			return fmt.Sprintf("/dev/loop%d", n), nil
		}
	}
	for i := 0; i < 256; i++ {
		name := fmt.Sprintf("loop%d", i)
		if _, err := os.Stat(filepath.Join("/sys/block", name, "loop/backing_file")); os.IsNotExist(err) {
			return "/dev/" + name, nil
		}
	}
	return "", fmt.Errorf("no free loop device")
}

func attachLoop(loopdev, file string) int {
	loop, err := os.OpenFile(loopdev, os.O_RDWR, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "losetup: %s: %v\n", loopdev, err)
		return 1
	}
	defer loop.Close()
	backing, err := os.Open(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "losetup: %s: %v\n", file, err)
		return 1
	}
	defer backing.Close()
	if err := unix.IoctlSetInt(int(loop.Fd()), unix.LOOP_SET_FD, int(backing.Fd())); err != nil {
		fmt.Fprintf(os.Stderr, "losetup: %v\n", err)
		return 1
	}
	info := &unix.LoopInfo64{}
	copy(info.File_name[:], []byte(file))
	_ = unix.IoctlLoopSetStatus64(int(loop.Fd()), info)
	return 0
}

func detachLoop(loopdev string) int {
	loop, err := os.OpenFile(loopdev, os.O_RDONLY, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "losetup: %s: %v\n", loopdev, err)
		return 1
	}
	defer loop.Close()
	if err := unix.IoctlSetInt(int(loop.Fd()), unix.LOOP_CLR_FD, 0); err != nil {
		fmt.Fprintf(os.Stderr, "losetup: %v\n", err)
		return 1
	}
	return 0
}

func listBlockDisks(prefix string) int {
	entries, err := os.ReadDir("/sys/block")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", prefix, err)
		return 1
	}
	for _, entry := range entries {
		size, _ := readSysfsBlockBytes(entry.Name())
		fmt.Printf("Disk /dev/%s: %d bytes\n", entry.Name(), size)
	}
	return 0
}

func readSysfsBlockBytes(name string) (int64, error) {
	data, err := os.ReadFile(filepath.Join("/sys/class/block", name, "size"))
	if err != nil {
		data, err = os.ReadFile(filepath.Join("/sys/block", name, "size"))
	}
	if err != nil {
		return 0, err
	}
	sectors, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, err
	}
	return sectors * 512, nil
}

func trimFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func printLsblk() int {
	entries, err := os.ReadDir("/sys/class/block")
	if err != nil {
		fmt.Fprintf(os.Stderr, "lsblk: %v\n", err)
		return 1
	}
	mounts := currentMountpoints()
	fmt.Printf("%-16s %-7s %-10s %-6s %s\n", "NAME", "MAJ:MIN", "SIZE", "TYPE", "MOUNTPOINT")
	for _, entry := range entries {
		name := entry.Name()
		dev := trimFile(filepath.Join("/sys/class/block", name, "dev"))
		size, _ := readSysfsBlockBytes(name)
		typ := "disk"
		if _, err := os.Stat(filepath.Join("/sys/class/block", name, "partition")); err == nil {
			typ = "part"
		}
		fmt.Printf("%-16s %-7s %-10d %-6s %s\n", name, dev, size, typ, mounts["/dev/"+name])
	}
	return 0
}

func currentMountpoints() map[string]string {
	out := map[string]string{}
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return out
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 {
			out[fields[0]] = fields[1]
		}
	}
	return out
}

func parseSizeSuffix(s string) (uint64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty size")
	}
	multiplier := uint64(1)
	last := s[len(s)-1]
	switch last {
	case 'k', 'K':
		multiplier = 1024
		s = s[:len(s)-1]
	case 'm', 'M':
		multiplier = 1024 * 1024
		s = s[:len(s)-1]
	case 'g', 'G':
		multiplier = 1024 * 1024 * 1024
		s = s[:len(s)-1]
	case 't', 'T':
		multiplier = 1024 * 1024 * 1024 * 1024
		s = s[:len(s)-1]
	}
	value, err := strconv.ParseUint(s, 0, 64)
	if err != nil {
		return 0, err
	}
	return value * multiplier, nil
}

type namespaceInfo struct {
	inode   string
	typ     string
	nprocs  int
	pid     int
	command string
}

func collectNamespaces(onlyType string) ([]namespaceInfo, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	seen := map[string]*namespaceInfo{}
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		nsDir := filepath.Join("/proc", entry.Name(), "ns")
		nsEntries, err := os.ReadDir(nsDir)
		if err != nil {
			continue
		}
		command := trimFile(filepath.Join("/proc", entry.Name(), "comm"))
		if command == "" {
			command = trimFile(filepath.Join("/proc", entry.Name(), "cmdline"))
		}
		for _, nsEntry := range nsEntries {
			typ := nsEntry.Name()
			if onlyType != "" && typ != onlyType {
				continue
			}
			target, err := os.Readlink(filepath.Join(nsDir, typ))
			if err != nil {
				continue
			}
			inode := namespaceInode(target)
			if inode == "" {
				continue
			}
			key := typ + ":" + inode
			if existing := seen[key]; existing != nil {
				existing.nprocs++
				if pid < existing.pid {
					existing.pid = pid
					existing.command = command
				}
				continue
			}
			seen[key] = &namespaceInfo{inode: inode, typ: typ, nprocs: 1, pid: pid, command: command}
		}
	}
	out := make([]namespaceInfo, 0, len(seen))
	for _, ns := range seen {
		out = append(out, *ns)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].typ != out[j].typ {
			return out[i].typ < out[j].typ
		}
		return out[i].inode < out[j].inode
	})
	return out, nil
}

func namespaceInode(link string) string {
	start := strings.IndexByte(link, '[')
	end := strings.IndexByte(link, ']')
	if start < 0 || end <= start {
		return ""
	}
	return link[start+1 : end]
}

func deviceSizeBytes(path string) (uint64, error) {
	if size, err := readSysfsBlockBytes(filepath.Base(path)); err == nil {
		return uint64(size), nil
	}
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return blockSize64(f)
}

func cpuSetFromHex(mask string) unix.CPUSet {
	var set unix.CPUSet
	set.Zero()
	mask = strings.TrimPrefix(strings.ToLower(mask), "0x")
	value, err := strconv.ParseUint(mask, 16, 64)
	if err != nil {
		return set
	}
	for cpu := 0; cpu < 64; cpu++ {
		if value&(uint64(1)<<uint(cpu)) != 0 {
			set.Set(cpu)
		}
	}
	return set
}

func cpuSetHex(set *unix.CPUSet) string {
	var value uint64
	for cpu := 0; cpu < 64; cpu++ {
		if set.IsSet(cpu) {
			value |= uint64(1) << uint(cpu)
		}
	}
	return fmt.Sprintf("%x", value)
}

func execProgram(name string, argv []string) int {
	path := name
	if !strings.Contains(name, "/") {
		for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
			candidate := filepath.Join(dir, name)
			if st, err := os.Stat(candidate); err == nil && st.Mode()&0111 != 0 {
				path = candidate
				break
			}
		}
	}
	if err := syscall.Exec(path, argv, os.Environ()); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", name, err)
		return 127
	}
	return 0
}

func runMore(args []string) int {
	files := args[1:]
	if len(files) == 0 {
		buf := make([]byte, 4096)
		for {
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				os.Stdout.Write(buf[:n])
			}
			if err != nil {
				break
			}
		}
		return 0
	}
	for _, fname := range files {
		data, err := os.ReadFile(fname)
		if err != nil {
			fmt.Fprintf(os.Stderr, "more: %s: %v\n", fname, err)
			return 1
		}
		os.Stdout.Write(data)
	}
	return 0
}

func runHexdump(args []string) int {
	files := []string{}
	for _, a := range args[1:] {
		if !strings.HasPrefix(a, "-") {
			files = append(files, a)
		}
	}
	if len(files) == 0 {
		files = []string{"-"}
	}

	for _, fname := range files {
		var data []byte
		var err error
		if fname == "-" {
			data, err = os.ReadFile("/dev/stdin")
		} else {
			data, err = os.ReadFile(fname)
		}
		if err != nil {
			return 1
		}

		for i := 0; i < len(data); i += 16 {
			end := i + 16
			if end > len(data) {
				end = len(data)
			}
			fmt.Printf("%08x  ", i)
			for j := i; j < end; j++ {
				fmt.Printf("%02x ", data[j])
				if j == i+7 {
					fmt.Print(" ")
				}
			}
			fmt.Printf(" |")
			for j := i; j < end; j++ {
				if data[j] >= 32 && data[j] < 127 {
					fmt.Printf("%c", data[j])
				} else {
					fmt.Print(".")
				}
			}
			fmt.Println("|")
		}
	}
	return 0
}

func runXxd(args []string) int {
	return runHexdump(args)
}

func runRenice(args []string) int {
	if len(args) < 3 {
		fmt.Fprintf(os.Stderr, "renice: missing priority or pid\n")
		return 1
	}
	priority, err := strconv.Atoi(args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "renice: invalid priority '%s'\n", args[1])
		return 1
	}
	exitCode := 0
	for _, pidArg := range args[2:] {
		if strings.HasPrefix(pidArg, "-") {
			continue
		}
		pid, err := strconv.Atoi(pidArg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "renice: invalid pid '%s'\n", pidArg)
			exitCode = 1
			continue
		}
		if err := syscall.Setpriority(syscall.PRIO_PROCESS, pid, priority); err != nil {
			fmt.Fprintf(os.Stderr, "renice: %d: %v\n", pid, err)
			exitCode = 1
		}
	}
	return exitCode
}

func runChrt(args []string) int {
	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "chrt: not supported\n")
		return 1
	}
	pidMode := false
	policy := uint32(unix.SCHED_RR)
	operands := []string{}
	for _, a := range args[1:] {
		switch a {
		case "-p":
			pidMode = true
		case "-f":
			policy = unix.SCHED_FIFO
		case "-r":
			policy = unix.SCHED_RR
		case "-o":
			policy = 0
		default:
			if !strings.HasPrefix(a, "-") {
				operands = append(operands, a)
			}
		}
	}
	if pidMode {
		if len(operands) == 1 {
			pid, _ := strconv.Atoi(operands[0])
			attr, err := unix.SchedGetAttr(pid, 0)
			if err != nil {
				fmt.Fprintf(os.Stderr, "chrt: %v\n", err)
				return 1
			}
			fmt.Printf("pid %d's current scheduling policy: %d\n", pid, attr.Policy)
			fmt.Printf("pid %d's current scheduling priority: %d\n", pid, attr.Priority)
			return 0
		}
		if len(operands) >= 2 {
			prio, _ := strconv.Atoi(operands[0])
			pid, _ := strconv.Atoi(operands[1])
			attr := &unix.SchedAttr{Size: uint32(unsafe.Sizeof(unix.SchedAttr{})), Policy: policy, Priority: uint32(prio)}
			if err := unix.SchedSetAttr(pid, attr, 0); err != nil {
				fmt.Fprintf(os.Stderr, "chrt: %v\n", err)
				return 1
			}
			return 0
		}
	}
	if len(operands) >= 2 {
		prio, _ := strconv.Atoi(operands[0])
		attr := &unix.SchedAttr{Size: uint32(unsafe.Sizeof(unix.SchedAttr{})), Policy: policy, Priority: uint32(prio)}
		if err := unix.SchedSetAttr(0, attr, 0); err != nil {
			fmt.Fprintf(os.Stderr, "chrt: %v\n", err)
			return 1
		}
		return execProgram(operands[1], operands[1:])
	}
	fmt.Fprintf(os.Stderr, "chrt: usage: chrt [-f|-r|-o] PRIORITY COMMAND [ARGS...] or chrt -p [PRIORITY] PID\n")
	return 1
}

func runTaskset(args []string) int {
	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "taskset: not supported\n")
		return 1
	}
	pidMode := false
	operands := []string{}
	for _, a := range args[1:] {
		if a == "-p" {
			pidMode = true
			continue
		}
		if !strings.HasPrefix(a, "-") {
			operands = append(operands, a)
		}
	}
	if pidMode {
		if len(operands) == 1 {
			pid, _ := strconv.Atoi(operands[0])
			var set unix.CPUSet
			if err := unix.SchedGetaffinity(pid, &set); err != nil {
				fmt.Fprintf(os.Stderr, "taskset: %v\n", err)
				return 1
			}
			fmt.Printf("pid %d's current affinity mask: %s\n", pid, cpuSetHex(&set))
			return 0
		}
		if len(operands) >= 2 {
			set := cpuSetFromHex(operands[0])
			pid, _ := strconv.Atoi(operands[1])
			if err := unix.SchedSetaffinity(pid, &set); err != nil {
				fmt.Fprintf(os.Stderr, "taskset: %v\n", err)
				return 1
			}
			return 0
		}
	}
	if len(operands) >= 2 {
		set := cpuSetFromHex(operands[0])
		if err := unix.SchedSetaffinity(0, &set); err != nil {
			fmt.Fprintf(os.Stderr, "taskset: %v\n", err)
			return 1
		}
		return execProgram(operands[1], operands[1:])
	}
	fmt.Fprintf(os.Stderr, "taskset: usage: taskset MASK COMMAND [ARGS...] or taskset -p [MASK] PID\n")
	return 1
}

func runNsenter(args []string) int {
	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "nsenter: not supported\n")
		return 1
	}
	target := ""
	namespaces := []string{"mnt", "uts", "ipc", "net", "pid", "user", "cgroup"}
	command := []string{"/bin/sh"}
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-t", "--target":
			if i+1 < len(args) {
				i++
				target = args[i]
			}
		case "--mount":
			namespaces = []string{"mnt"}
		case "--uts":
			namespaces = []string{"uts"}
		case "--ipc":
			namespaces = []string{"ipc"}
		case "--net":
			namespaces = []string{"net"}
		case "--pid":
			namespaces = []string{"pid"}
		default:
			if !strings.HasPrefix(args[i], "-") {
				command = args[i:]
				i = len(args)
			}
		}
	}
	if target == "" {
		fmt.Fprintf(os.Stderr, "nsenter: missing --target PID\n")
		return 1
	}
	for _, ns := range namespaces {
		path := filepath.Join("/proc", target, "ns", ns)
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		err = unix.Setns(int(f.Fd()), 0)
		f.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "nsenter: %s: %v\n", path, err)
			return 1
		}
	}
	return execProgram(command[0], command)
}

func runUnshare(args []string) int {
	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "unshare: not supported\n")
		return 1
	}
	flags := 0
	command := []string{"/bin/sh"}
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-m", "--mount":
			flags |= unix.CLONE_NEWNS
		case "-u", "--uts":
			flags |= unix.CLONE_NEWUTS
		case "-i", "--ipc":
			flags |= unix.CLONE_NEWIPC
		case "-n", "--net":
			flags |= unix.CLONE_NEWNET
		case "-p", "--pid":
			flags |= unix.CLONE_NEWPID
		case "-U", "--user":
			flags |= unix.CLONE_NEWUSER
		default:
			if !strings.HasPrefix(args[i], "-") {
				command = args[i:]
				i = len(args)
			}
		}
	}
	if flags == 0 {
		flags = unix.CLONE_NEWNS
	}
	if err := unix.Unshare(flags); err != nil {
		fmt.Fprintf(os.Stderr, "unshare: %v\n", err)
		return 1
	}
	return execProgram(command[0], command)
}

func runFstrim(args []string) int {
	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "fstrim: not supported\n")
		return 1
	}
	var trim fstrimRange
	trim.Len = ^uint64(0)
	verbose := false
	mountpoint := ""
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-o", "--offset":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "fstrim: option %s requires an argument\n", args[i])
				return 1
			}
			i++
			v, err := parseSizeSuffix(args[i])
			if err != nil {
				fmt.Fprintf(os.Stderr, "fstrim: invalid offset '%s'\n", args[i])
				return 1
			}
			trim.Start = v
		case "-l", "--length":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "fstrim: option %s requires an argument\n", args[i])
				return 1
			}
			i++
			v, err := parseSizeSuffix(args[i])
			if err != nil {
				fmt.Fprintf(os.Stderr, "fstrim: invalid length '%s'\n", args[i])
				return 1
			}
			trim.Len = v
		case "-m", "--minimum":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "fstrim: option %s requires an argument\n", args[i])
				return 1
			}
			i++
			v, err := parseSizeSuffix(args[i])
			if err != nil {
				fmt.Fprintf(os.Stderr, "fstrim: invalid minimum '%s'\n", args[i])
				return 1
			}
			trim.Minlen = v
		case "-v", "--verbose":
			verbose = true
		default:
			if strings.HasPrefix(args[i], "-") {
				fmt.Fprintf(os.Stderr, "fstrim: unknown option %s\n", args[i])
				return 1
			}
			if mountpoint != "" {
				fmt.Fprintf(os.Stderr, "fstrim: too many operands\n")
				return 1
			}
			mountpoint = args[i]
		}
	}
	if mountpoint == "" {
		fmt.Fprintf(os.Stderr, "fstrim: missing mountpoint\n")
		return 1
	}
	f, err := os.OpenFile(mountpoint, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fstrim: %s: %v\n", mountpoint, err)
		return 1
	}
	defer f.Close()
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, f.Fd(), ioctlFITRIM, uintptr(unsafe.Pointer(&trim)))
	if errno != 0 {
		fmt.Fprintf(os.Stderr, "fstrim: %s: %v\n", mountpoint, errno)
		return 1
	}
	if verbose {
		fmt.Printf("%s: %d bytes trimmed\n", mountpoint, trim.Len)
	}
	return 0
}

func runLsns(args []string) int {
	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "lsns: not supported\n")
		return 1
	}
	onlyType := ""
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-t", "--type":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "lsns: option %s requires an argument\n", args[i])
				return 1
			}
			i++
			onlyType = args[i]
		default:
			if strings.HasPrefix(args[i], "--type=") {
				onlyType = strings.TrimPrefix(args[i], "--type=")
			}
		}
	}
	namespaces, err := collectNamespaces(onlyType)
	if err != nil {
		fmt.Fprintf(os.Stderr, "lsns: %v\n", err)
		return 1
	}
	fmt.Printf("%-12s %-8s %-8s %-8s %s\n", "NS", "TYPE", "NPROCS", "PID", "COMMAND")
	for _, ns := range namespaces {
		fmt.Printf("%-12s %-8s %-8d %-8d %s\n", ns.inode, ns.typ, ns.nprocs, ns.pid, ns.command)
	}
	return 0
}

func runFsfreeze(args []string) int {
	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "fsfreeze: not supported\n")
		return 1
	}
	request := uintptr(0)
	mountpoint := ""
	for _, a := range args[1:] {
		switch a {
		case "-f", "--freeze":
			if request != 0 {
				fmt.Fprintf(os.Stderr, "fsfreeze: --freeze and --unfreeze are mutually exclusive\n")
				return 1
			}
			request = ioctlFIFREEZE
		case "-u", "--unfreeze":
			if request != 0 {
				fmt.Fprintf(os.Stderr, "fsfreeze: --freeze and --unfreeze are mutually exclusive\n")
				return 1
			}
			request = ioctlFITHAW
		default:
			if !strings.HasPrefix(a, "-") {
				mountpoint = a
			}
		}
	}
	if request == 0 || mountpoint == "" {
		fmt.Fprintf(os.Stderr, "fsfreeze: usage: fsfreeze --freeze|--unfreeze MOUNTPOINT\n")
		return 1
	}
	f, err := os.Open(mountpoint)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fsfreeze: %s: %v\n", mountpoint, err)
		return 1
	}
	defer f.Close()
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, f.Fd(), request, 0)
	if errno != 0 {
		fmt.Fprintf(os.Stderr, "fsfreeze: %s: %v\n", mountpoint, errno)
		return 1
	}
	return 0
}

func runFindmnt(args []string) int {
	if runtime.GOOS == "linux" {
		f, err := os.Open("/proc/mounts")
		if err != nil {
			return 1
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			parts := strings.Fields(scanner.Text())
			if len(parts) >= 3 {
				fmt.Printf("%-20s %s %s\n", parts[1], parts[0], parts[2])
			}
		}
		return 0
	}
	fmt.Fprintf(os.Stderr, "findmnt: not supported\n")
	return 1
}

func runPartprobe(args []string) int {
	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "partprobe: not supported\n")
		return 1
	}
	dryRun := false
	summary := false
	devices := []string{}
	for _, a := range args[1:] {
		switch a {
		case "-d", "--dry-run":
			dryRun = true
		case "-s", "--summary":
			summary = true
		default:
			if !strings.HasPrefix(a, "-") {
				devices = append(devices, a)
			}
		}
	}
	exitCode := 0
	for _, dev := range devices {
		if summary {
			if size, err := deviceSizeBytes(dev); err == nil {
				fmt.Printf("%s: %d bytes\n", dev, size)
			} else {
				fmt.Printf("%s\n", dev)
			}
		}
		if dryRun {
			continue
		}
		f, err := os.Open(dev)
		if err != nil {
			fmt.Fprintf(os.Stderr, "partprobe: %s: %v\n", dev, err)
			exitCode = 1
			continue
		}
		_, _, errno := unix.Syscall(unix.SYS_IOCTL, f.Fd(), uintptr(unix.BLKRRPART), 0)
		f.Close()
		if errno != 0 {
			fmt.Fprintf(os.Stderr, "partprobe: %s: %v\n", dev, errno)
			exitCode = 1
		}
	}
	return exitCode
}

func runMkfs(args []string) int {
	fmt.Fprintf(os.Stderr, "mkfs: not yet implemented in pure Go\n")
	return 1
}

func runFsck(args []string) int {
	fmt.Fprintf(os.Stderr, "fsck: not yet implemented in pure Go\n")
	return 1
}

func runFsckMinix(args []string) int {
	fmt.Fprintf(os.Stderr, "%s: not yet implemented in pure Go\n", args[0])
	return 1
}

func runMkfsExt2(args []string) int {
	fmt.Fprintf(os.Stderr, "%s: not yet implemented in pure Go\n", args[0])
	return 1
}

func runMkfsMinix(args []string) int {
	fmt.Fprintf(os.Stderr, "%s: not yet implemented in pure Go\n", args[0])
	return 1
}

func runMkfsReiser(args []string) int {
	fmt.Fprintf(os.Stderr, "%s: not yet implemented in pure Go\n", args[0])
	return 1
}

func runMkfsVfat(args []string) int {
	fmt.Fprintf(os.Stderr, "%s: not yet implemented in pure Go\n", args[0])
	return 1
}

func init() {
	applet.Register(&applet.Applet{Name: "lscpu", Short: "Display CPU information", Func: runLscpu})
	applet.Register(&applet.Applet{Name: "lspci", Short: "List PCI devices", Func: runLspci})
	applet.Register(&applet.Applet{Name: "lsusb", Short: "List USB devices", Func: runLsusb})
	applet.Register(&applet.Applet{Name: "lsdev", Short: "List devices", Func: runLsdev})
}

func runLsblk(args []string) int {
	return printLsblk()
}

func runLscpu(args []string) int {
	if runtime.GOOS == "linux" {
		data, err := os.ReadFile("/proc/cpuinfo")
		if err != nil {
			fmt.Fprintf(os.Stderr, "lscpu: %v\n", err)
			return 1
		}
		os.Stdout.Write(data)
		return 0
	}
	fmt.Fprintf(os.Stderr, "lscpu: not supported\n")
	return 1
}

func runLspci(args []string) int {
	entries, err := os.ReadDir("/sys/bus/pci/devices")
	if err != nil {
		fmt.Fprintf(os.Stderr, "lspci: %v\n", err)
		return 1
	}
	for _, entry := range entries {
		base := filepath.Join("/sys/bus/pci/devices", entry.Name())
		class := trimFile(filepath.Join(base, "class"))
		vendor := trimFile(filepath.Join(base, "vendor"))
		device := trimFile(filepath.Join(base, "device"))
		fmt.Printf("%s Class %s: %s:%s\n", entry.Name(), strings.TrimPrefix(class, "0x"), strings.TrimPrefix(vendor, "0x"), strings.TrimPrefix(device, "0x"))
	}
	return 0
}

func runLsusb(args []string) int {
	entries, err := os.ReadDir("/sys/bus/usb/devices")
	if err != nil {
		fmt.Fprintf(os.Stderr, "lsusb: %v\n", err)
		return 1
	}
	for _, entry := range entries {
		base := filepath.Join("/sys/bus/usb/devices", entry.Name())
		vendor := trimFile(filepath.Join(base, "idVendor"))
		product := trimFile(filepath.Join(base, "idProduct"))
		if vendor == "" || product == "" {
			continue
		}
		man := trimFile(filepath.Join(base, "manufacturer"))
		prod := trimFile(filepath.Join(base, "product"))
		bus := trimFile(filepath.Join(base, "busnum"))
		devnum := trimFile(filepath.Join(base, "devnum"))
		fmt.Printf("Bus %03s Device %03s: ID %s:%s %s %s\n", bus, devnum, vendor, product, man, prod)
	}
	return 0
}

func runLsdev(args []string) int {
	path := "/dev"
	if len(args) > 1 && !strings.HasPrefix(args[1], "-") {
		path = args[1]
	}
	if runtime.GOOS == "linux" && path == "/dev" {
		return listLinuxDevices()
	}
	return listDeviceDirectory(path)
}

func listLinuxDevices() int {
	classEntries, err := os.ReadDir("/sys/class")
	if err != nil {
		return listDeviceDirectory("/dev")
	}
	type devRow struct {
		class string
		name  string
		major string
		path  string
	}
	rows := []devRow{}
	for _, classEntry := range classEntries {
		if !classEntry.IsDir() {
			continue
		}
		base := filepath.Join("/sys/class", classEntry.Name())
		entries, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			devID := trimFile(filepath.Join(base, entry.Name(), "dev"))
			if devID == "" {
				continue
			}
			rows = append(rows, devRow{
				class: classEntry.Name(),
				name:  entry.Name(),
				major: devID,
				path:  filepath.Join("/dev", entry.Name()),
			})
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].class != rows[j].class {
			return rows[i].class < rows[j].class
		}
		return rows[i].name < rows[j].name
	})
	for _, row := range rows {
		fmt.Printf("%-16s %-20s %-8s %s\n", row.class, row.name, row.major, row.path)
	}
	return 0
}

func listDeviceDirectory(path string) int {
	entries, err := os.ReadDir(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "lsdev: %s: %v\n", path, err)
		return 1
	}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		mode := info.Mode()
		kind := "file"
		switch {
		case mode&os.ModeDevice != 0 && mode&os.ModeCharDevice != 0:
			kind = "char"
		case mode&os.ModeDevice != 0:
			kind = "block"
		case mode.IsDir():
			kind = "dir"
		case mode&os.ModeSymlink != 0:
			kind = "link"
		}
		fmt.Printf("%-8s %s\n", kind, filepath.Join(path, entry.Name()))
	}
	return 0
}
