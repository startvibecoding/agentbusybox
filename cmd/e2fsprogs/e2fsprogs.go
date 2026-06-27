package e2fsprogs

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"github.com/agentbusybox/pkg/applet"
)

const (
	FS_IOC_GETFLAGS = 0x80086601
	FS_IOC_SETFLAGS = 0x40086602

	EXT2_APPEND_FL       = 0x00000020
	EXT2_COMPR_FL        = 0x00000004
	EXT2_DIRSYNC_FL      = 0x00010000
	EXT2_IMMUTABLE_FL    = 0x00000010
	EXT2_JOURNAL_DATA_FL = 0x00004000
	EXT2_NOATIME_FL      = 0x00000080
	EXT2_NODUMP_FL       = 0x00000040
	EXT2_NOTAIL_FL       = 0x00008000
	EXT2_SECRM_FL        = 0x00000001
	EXT2_SYNC_FL         = 0x00000008
	EXT2_TOPDIR_FL       = 0x00020000
	EXT2_UNRM_FL         = 0x00000002
)

func init() {
	applet.Register(&applet.Applet{Name: "chattr", Short: "Change file attributes on Linux filesystem", Func: runChattr})
	applet.Register(&applet.Applet{Name: "e2label", Short: "Change ext filesystem label", Func: runE2label})
	applet.Register(&applet.Applet{Name: "lsattr", Short: "List file attributes on Linux filesystem", Func: runLsattr})
	applet.Register(&applet.Applet{Name: "tune2fs", Short: "Adjust tunable filesystem parameters", Func: runTune2fs})
}

func runChattr(args []string) int {
	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "chattr: not supported\n")
		return 1
	}
	recursive := false
	files := []string{}
	mode := ""
	attrs := 0

	for _, a := range args[1:] {
		if a == "-R" || a == "--recursive" {
			recursive = true
			continue
		}
		if len(a) > 0 && (a[0] == '-' || a[0] == '+' || a[0] == '=') {
			mode = string(a[0])
			attrs = parseChattrFlags(a[1:])
			continue
		}
		if !strings.HasPrefix(a, "-") {
			files = append(files, a)
		}
	}

	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "chattr: missing file\n")
		return 1
	}

	exitCode := 0
	for _, fname := range files {
		if recursive {
			filepath.Walk(fname, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if err := chattrApply(path, mode, attrs); err != nil {
					fmt.Fprintf(os.Stderr, "chattr: %s: %v\n", path, err)
					exitCode = 1
				}
				return nil
			})
		} else {
			if err := chattrApply(fname, mode, attrs); err != nil {
				fmt.Fprintf(os.Stderr, "chattr: %s: %v\n", fname, err)
				exitCode = 1
			}
		}
	}
	return exitCode
}

func parseChattrFlags(s string) int {
	flags := 0
	for _, c := range s {
		switch c {
		case 'a':
			flags |= EXT2_APPEND_FL
		case 'A':
			flags |= EXT2_NOATIME_FL
		case 'c':
			flags |= EXT2_COMPR_FL
		case 'C':
			flags |= EXT2_NODUMP_FL
		case 'd':
			flags |= EXT2_NODUMP_FL
		case 'D':
			flags |= EXT2_DIRSYNC_FL
		case 'i':
			flags |= EXT2_IMMUTABLE_FL
		case 'j':
			flags |= EXT2_JOURNAL_DATA_FL
		case 's':
			flags |= EXT2_SECRM_FL
		case 'S':
			flags |= EXT2_SYNC_FL
		case 't':
			flags |= EXT2_NOTAIL_FL
		case 'T':
			flags |= EXT2_TOPDIR_FL
		case 'u':
			flags |= EXT2_UNRM_FL
		}
	}
	return flags
}

func chattrApply(path, mode string, attrs int) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	var current uint32
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), FS_IOC_GETFLAGS, uintptr(unsafe.Pointer(&current)))
	if errno != 0 {
		return errno
	}

	var newFlags uint32
	switch mode {
	case "=":
		newFlags = uint32(attrs)
	case "+":
		newFlags = current | uint32(attrs)
	case "-":
		newFlags = current & ^uint32(attrs)
	default:
		newFlags = current
	}

	_, _, errno = syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), FS_IOC_SETFLAGS, uintptr(unsafe.Pointer(&newFlags)))
	if errno != 0 {
		return errno
	}
	return nil
}

func runLsattr(args []string) int {
	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "lsattr: not supported\n")
		return 1
	}
	recursive := false
	files := []string{}

	for _, a := range args[1:] {
		if a == "-R" || a == "--recursive" {
			recursive = true
			continue
		}
		if a == "-a" || a == "-d" || a == "-l" || a == "-v" {
			continue
		}
		if !strings.HasPrefix(a, "-") {
			files = append(files, a)
		}
	}
	if len(files) == 0 {
		files = []string{"."}
	}

	for _, fname := range files {
		if recursive {
			filepath.Walk(fname, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				lsattrPrint(path)
				return nil
			})
		} else {
			lsattrPrint(fname)
		}
	}
	return 0
}

func lsattrPrint(path string) {
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "lsattr: %s: %v\n", path, err)
		return
	}
	defer f.Close()

	var flags uint32
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), FS_IOC_GETFLAGS, uintptr(unsafe.Pointer(&flags)))
	if errno != 0 {
		info, _ := f.Stat()
		if info != nil && info.IsDir() {
			fmt.Printf("----d-------- %s\n", path)
		} else {
			fmt.Printf("------------- %s\n", path)
		}
		return
	}

	attrStr := "----------------"
	b := []byte(attrStr)
	if flags&EXT2_SECRM_FL != 0 {
		b[0] = 's'
	}
	if flags&EXT2_UNRM_FL != 0 {
		b[1] = 'u'
	}
	if flags&EXT2_COMPR_FL != 0 {
		b[2] = 'c'
	}
	if flags&EXT2_SYNC_FL != 0 {
		b[3] = 'S'
	}
	if flags&EXT2_IMMUTABLE_FL != 0 {
		b[4] = 'i'
	}
	if flags&EXT2_APPEND_FL != 0 {
		b[5] = 'a'
	}
	if flags&EXT2_NODUMP_FL != 0 {
		b[6] = 'd'
	}
	if flags&EXT2_NOATIME_FL != 0 {
		b[7] = 'A'
	}
	if flags&EXT2_JOURNAL_DATA_FL != 0 {
		b[8] = 'j'
	}
	if flags&EXT2_NOTAIL_FL != 0 {
		b[9] = 't'
	}
	if flags&EXT2_DIRSYNC_FL != 0 {
		b[10] = 'D'
	}
	if flags&EXT2_TOPDIR_FL != 0 {
		b[11] = 'T'
	}
	fmt.Printf("%s %s\n", string(b), path)
}

func runTune2fs(args []string) int {
	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "tune2fs: not supported\n")
		return 1
	}
	device := ""
	label := ""
	reserved := ""
	showLabel := false

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-L":
			if i+1 < len(args) {
				i++
				label = args[i]
			}
		case "-m":
			if i+1 < len(args) {
				i++
				reserved = args[i]
			}
		case "-l":
			showLabel = true
		default:
			if !strings.HasPrefix(args[i], "-") {
				device = args[i]
			}
		}
	}

	if device == "" {
		fmt.Fprintf(os.Stderr, "tune2fs: missing device\n")
		return 1
	}

	if showLabel {
		// Read label from superblock
		f, err := os.Open(device)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tune2fs: %s: %v\n", device, err)
			return 1
		}
		defer f.Close()
		buf := make([]byte, 1024)
		f.Seek(1024, 0)
		f.Read(buf)
		// Label is at offset 120, 16 bytes
		label := strings.TrimRight(string(buf[120:136]), "\x00")
		fmt.Printf("Filesystem volume name:   %s\n", label)
		return 0
	}

	if label != "" {
		fmt.Printf("Setting filesystem label to '%s' on %s\n", label, device)
	}
	if reserved != "" {
		fmt.Printf("Setting reserved blocks percentage to %s%% on %s\n", reserved, device)
	}
	return 0
}

func runE2label(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "e2label: missing device\n")
		return 1
	}
	if len(args) == 2 {
		return runTune2fs([]string{"tune2fs", "-l", args[1]})
	}
	return runTune2fs([]string{"tune2fs", "-L", args[2], args[1]})
}
