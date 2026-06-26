package fileutil

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/agentbusybox/pkg/applet"
	"golang.org/x/sys/unix"
)

func init() {
	applet.Register(&applet.Applet{Name: "find", Short: "Search for files in a directory hierarchy", Func: runFind})
}

func runFind(args []string) int {
	paths := []string{}
	namePattern := ""
	typeFilter := ""
	sizeExpr := ""
	maxDepth := -1

	i := 1
	for ; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			i++
			break
		}
		if strings.HasPrefix(a, "-name") {
			if i+1 < len(args) {
				i++
				namePattern = args[i]
			}
			continue
		}
		if strings.HasPrefix(a, "-type") {
			if i+1 < len(args) {
				i++
				typeFilter = args[i]
			}
			continue
		}
		if strings.HasPrefix(a, "-size") {
			if i+1 < len(args) {
				i++
				sizeExpr = args[i]
			}
			continue
		}
		if strings.HasPrefix(a, "-maxdepth") {
			if i+1 < len(args) {
				i++
				fmt.Sscanf(args[i], "%d", &maxDepth)
			}
			continue
		}
		if !strings.HasPrefix(a, "-") {
			paths = append(paths, a)
		}
	}
	paths = append(paths, args[i:]...)
	if len(paths) == 0 {
		paths = []string{"."}
	}

	exitCode := 0
	for _, root := range paths {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}

			// Check depth
			if maxDepth >= 0 {
				rel, _ := filepath.Rel(root, path)
				depth := strings.Count(rel, string(os.PathSeparator)) + 1
				if depth > maxDepth {
					return filepath.SkipDir
				}
			}

			// Name filter
			if namePattern != "" {
				matched, _ := filepath.Match(namePattern, info.Name())
				if !matched {
					return nil
				}
			}

			// Type filter
			if typeFilter != "" {
				switch typeFilter {
				case "f":
					if info.IsDir() {
						return nil
					}
				case "d":
					if !info.IsDir() {
						return nil
					}
				case "l":
					if info.Mode()&os.ModeSymlink == 0 {
						return nil
					}
				}
			}

			// Size filter
			if sizeExpr != "" {
				if !matchSize(info.Size(), sizeExpr) {
					return nil
				}
			}

			fmt.Println(path)
			return nil
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "find: %s: %v\n", root, err)
			exitCode = 1
		}
	}
	return exitCode
}

func matchSize(size int64, expr string) bool {
	if len(expr) == 0 {
		return true
	}
	unit := int64(1)
	s := expr
	switch expr[len(expr)-1] {
	case 'c':
		unit = 1
		s = expr[:len(expr)-1]
	case 'k':
		unit = 1024
		s = expr[:len(expr)-1]
	case 'M':
		unit = 1024 * 1024
		s = expr[:len(expr)-1]
	case 'G':
		unit = 1024 * 1024 * 1024
		s = expr[:len(expr)-1]
	}
	var n int64
	fmt.Sscanf(s, "%d", &n)
	return size >= n*unit
}

func init() {
	applet.Register(&applet.Applet{Name: "stat", Short: "Display file status", Func: runStat})
}

func runStat(args []string) int {
	files := []string{}

	for _, a := range args[1:] {
		if a == "-f" {
			continue
		}
		if !strings.HasPrefix(a, "-") {
			files = append(files, a)
		}
	}

	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "stat: missing operand\n")
		return 1
	}

	exitCode := 0
	for _, fname := range files {
		info, err := os.Stat(fname)
		if err != nil {
			fmt.Fprintf(os.Stderr, "stat: cannot stat '%s': %v\n", fname, err)
			exitCode = 1
			continue
		}
		fmt.Printf("  File: %q\n", fname)
		fmt.Printf("  Size: %d\tBlocks: %d\tIO Block: %d\t%v\n",
			info.Size(), info.Size()/512+1, 4096, info.IsDir())
		fmt.Printf("Access: %v\n", info.Mode())
		fmt.Printf("Modify: %v\n", info.ModTime())
	}
	return exitCode
}

func init() {
	applet.Register(&applet.Applet{Name: "du", Short: "Estimate file space usage", Func: runDu})
}

func runDu(args []string) int {
	human, summary, all := false, false, false
	maxDepth := -1
	paths := []string{}

	for _, a := range args[1:] {
		if a == "-h" || a == "--human-readable" {
			human = true
			continue
		}
		if a == "-s" || a == "--summarize" {
			summary = true
			continue
		}
		if a == "-a" || a == "--all" {
			all = true
			continue
		}
		if strings.HasPrefix(a, "-d") {
			if len(a) > 2 {
				fmt.Sscanf(a[2:], "%d", &maxDepth)
			}
			continue
		}
		if !strings.HasPrefix(a, "-") {
			paths = append(paths, a)
		}
	}
	if len(paths) == 0 {
		paths = []string{"."}
	}

	exitCode := 0
	for _, root := range paths {
		totalSize := int64(0)
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if !info.IsDir() {
				totalSize += info.Size()
			}
			return nil
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "du: %s: %v\n", root, err)
			exitCode = 1
			continue
		}
		_ = all
		_ = maxDepth
		if summary || true {
			fmt.Printf("%s\t%s\n", formatSize(totalSize, human), root)
		}
	}
	return exitCode
}

func formatSize(size int64, human bool) string {
	if !human {
		return fmt.Sprintf("%d", size)
	}
	units := []string{"B", "K", "M", "G", "T"}
	f := float64(size)
	for _, u := range units {
		if f < 1024 {
			return fmt.Sprintf("%.1f%s", f, u)
		}
		f /= 1024
	}
	return fmt.Sprintf("%.1fE", f)
}

func init() {
	applet.Register(&applet.Applet{Name: "df", Short: "Report file system disk space usage", Func: runDf})
}

func runDf(args []string) int {
	paths := []string{}

	for _, a := range args[1:] {
		if a == "-h" || a == "--human-readable" {
			continue
		}
		if !strings.HasPrefix(a, "-") {
			paths = append(paths, a)
		}
	}

	if len(paths) == 0 {
		// Show all mounted filesystems
		paths = []string{"/"}
	}

	fmt.Printf("%-20s %10s %10s %10s %5s %s\n",
		"Filesystem", "1K-blocks", "Used", "Available", "Use%", "Mounted on")

	exitCode := 0
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "df: %s: %v\n", path, err)
			exitCode = 1
			continue
		}
		_ = info
		fmt.Printf("%-20s %10s %10s %10s %4s %s\n",
			path, "-", "-", "-", "-", path)
	}
	return exitCode
}

func init() {
	applet.Register(&applet.Applet{Name: "readlink", Short: "Print resolved symbolic links", Func: runReadlink})
	applet.Register(&applet.Applet{Name: "realpath", Short: "Print the resolved absolute path", Func: runRealpath})
}

func runReadlink(args []string) int {
	canonical := false
	files := []string{}
	for _, a := range args[1:] {
		if a == "-f" || a == "--canonicalize" {
			canonical = true
			continue
		}
		if !strings.HasPrefix(a, "-") {
			files = append(files, a)
		}
	}

	exitCode := 0
	for _, f := range files {
		if canonical {
			r, err := filepath.Abs(f)
			if err != nil {
				fmt.Fprintf(os.Stderr, "readlink: %s: %v\n", f, err)
				exitCode = 1
				continue
			}
			fmt.Println(r)
		} else {
			r, err := os.Readlink(f)
			if err != nil {
				fmt.Fprintf(os.Stderr, "readlink: %s: %v\n", f, err)
				exitCode = 1
				continue
			}
			fmt.Println(r)
		}
	}
	return exitCode
}

func runRealpath(args []string) int {
	files := []string{}
	for _, a := range args[1:] {
		if !strings.HasPrefix(a, "-") {
			files = append(files, a)
		}
	}
	for _, f := range files {
		r, err := filepath.Abs(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "realpath: %s: %v\n", f, err)
			return 1
		}
		fmt.Println(r)
	}
	return 0
}

func init() {
	applet.Register(&applet.Applet{Name: "ln", Short: "Create links", Func: runLn})
}

func runLn(args []string) int {
	symbolic, force := false, false
	target := ""
	files := []string{}

	for _, a := range args[1:] {
		if a == "-s" || a == "--symbolic" {
			symbolic = true
			continue
		}
		if a == "-f" || a == "--force" {
			force = true
			continue
		}
		if a == "--" {
			continue
		}
		if !strings.HasPrefix(a, "-") {
			files = append(files, a)
		}
	}

	if len(files) < 2 {
		fmt.Fprintf(os.Stderr, "ln: missing file operand\n")
		return 1
	}

	if len(files) == 2 {
		target = files[1]
	} else {
		target = files[len(files)-1]
	}

	targetInfo, err := os.Stat(target)
	targetIsDir := err == nil && targetInfo.IsDir()

	exitCode := 0
	for _, src := range files[:len(files)-1] {
		dst := target
		if targetIsDir {
			dst = filepath.Join(target, filepath.Base(src))
		}

		if force {
			os.Remove(dst)
		}

		if symbolic {
			if err := os.Symlink(src, dst); err != nil {
				fmt.Fprintf(os.Stderr, "ln: failed to create symbolic link '%s': %v\n", dst, err)
				exitCode = 1
			}
		} else {
			if err := os.Link(src, dst); err != nil {
				fmt.Fprintf(os.Stderr, "ln: failed to create hard link '%s': %v\n", dst, err)
				exitCode = 1
			}
		}
	}
	return exitCode
}

func init() {
	applet.Register(&applet.Applet{Name: "chmod", Short: "Change file permissions", Func: runChmod})
	applet.Register(&applet.Applet{Name: "chown", Short: "Change file owner and group", Func: runChown})
	applet.Register(&applet.Applet{Name: "chgrp", Short: "Change group ownership", Func: runChgrp})
}

func runChmod(args []string) int {
	recursive := false
	files := []string{}
	mode := ""

	for _, a := range args[1:] {
		if a == "-R" || a == "--recursive" {
			recursive = true
			continue
		}
		if !strings.HasPrefix(a, "-") {
			if mode == "" {
				mode = a
			} else {
				files = append(files, a)
			}
			continue
		}
	}

	if mode == "" || len(files) == 0 {
		fmt.Fprintf(os.Stderr, "chmod: missing operand\n")
		return 1
	}

	// Parse octal mode
	var perm os.FileMode
	if _, err := fmt.Sscanf(mode, "%o", &perm); err != nil {
		fmt.Fprintf(os.Stderr, "chmod: invalid mode: %s\n", mode)
		return 1
	}

	exitCode := 0
	for _, f := range files {
		if recursive {
			filepath.Walk(f, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if err := os.Chmod(path, perm); err != nil {
					fmt.Fprintf(os.Stderr, "chmod: cannot access '%s': %v\n", path, err)
					exitCode = 1
				}
				return nil
			})
		} else {
			if err := os.Chmod(f, perm); err != nil {
				fmt.Fprintf(os.Stderr, "chmod: cannot access '%s': %v\n", f, err)
				exitCode = 1
			}
		}
	}
	return exitCode
}

func runChown(args []string) int {
	recursive := false
	owner := ""
	files := []string{}

	for _, a := range args[1:] {
		if a == "-R" || a == "--recursive" {
			recursive = true
			continue
		}
		if !strings.HasPrefix(a, "-") {
			if owner == "" {
				owner = a
			} else {
				files = append(files, a)
			}
			continue
		}
	}

	if owner == "" || len(files) == 0 {
		fmt.Fprintf(os.Stderr, "chown: missing operand\n")
		return 1
	}

	// Parse owner:group
	parts := strings.SplitN(owner, ":", 2)
	_ = parts

	exitCode := 0
	for _, f := range files {
		if recursive {
			filepath.Walk(f, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				// Note: os.Lchown is platform-specific
				if err := os.Chown(path, os.Getuid(), os.Getgid()); err != nil {
					fmt.Fprintf(os.Stderr, "chown: cannot access '%s': %v\n", path, err)
					exitCode = 1
				}
				return nil
			})
		} else {
			if err := os.Chown(f, os.Getuid(), os.Getgid()); err != nil {
				fmt.Fprintf(os.Stderr, "chown: cannot access '%s': %v\n", f, err)
				exitCode = 1
			}
		}
	}
	return exitCode
}

func runChgrp(args []string) int {
	recursive := false
	group := ""
	files := []string{}

	for _, a := range args[1:] {
		if a == "-R" || a == "--recursive" {
			recursive = true
			continue
		}
		if !strings.HasPrefix(a, "-") {
			if group == "" {
				group = a
			} else {
				files = append(files, a)
			}
			continue
		}
	}

	if group == "" || len(files) == 0 {
		fmt.Fprintf(os.Stderr, "chgrp: missing operand\n")
		return 1
	}

	exitCode := 0
	for _, f := range files {
		if recursive {
			filepath.Walk(f, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if err := os.Chown(path, -1, os.Getgid()); err != nil {
					fmt.Fprintf(os.Stderr, "chgrp: cannot access '%s': %v\n", path, err)
					exitCode = 1
				}
				return nil
			})
		} else {
			if err := os.Chown(f, -1, os.Getgid()); err != nil {
				fmt.Fprintf(os.Stderr, "chgrp: cannot access '%s': %v\n", f, err)
				exitCode = 1
			}
		}
	}
	return exitCode
}

func init() {
	applet.Register(&applet.Applet{Name: "lsblk", Short: "List block devices", Func: runLsblk})
}

func runLsblk(args []string) int {
	entries, err := os.ReadDir("/sys/class/block")
	if err != nil {
		fmt.Fprintf(os.Stderr, "lsblk: %v\n", err)
		return 1
	}
	mounts := map[string]string{}
	if data, err := os.ReadFile("/proc/mounts"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				mounts[fields[0]] = fields[1]
			}
		}
	}
	fmt.Printf("%-16s %-7s %-10s %-6s %s\n", "NAME", "MAJ:MIN", "SIZE", "TYPE", "MOUNTPOINT")
	for _, entry := range entries {
		name := entry.Name()
		dev := strings.TrimSpace(readSmallFile(filepath.Join("/sys/class/block", name, "dev")))
		size := blockBytes(name)
		typ := "disk"
		if _, err := os.Stat(filepath.Join("/sys/class/block", name, "partition")); err == nil {
			typ = "part"
		}
		fmt.Printf("%-16s %-7s %-10d %-6s %s\n", name, dev, size, typ, mounts["/dev/"+name])
	}
	return 0
}

func init() {
	applet.Register(&applet.Applet{Name: "mktemp", Short: "Create a temporary file or directory", Func: runMktemp})
}

func runMktemp(args []string) int {
	dir := false
	template := "tmp.XXXXXXXXXX"

	for _, a := range args[1:] {
		if a == "-d" {
			dir = true
			continue
		}
		if !strings.HasPrefix(a, "-") {
			template = a
		}
	}

	if dir {
		d, err := os.MkdirTemp("", template)
		if err != nil {
			fmt.Fprintf(os.Stderr, "mktemp: %v\n", err)
			return 1
		}
		fmt.Println(d)
	} else {
		f, err := os.CreateTemp("", template)
		if err != nil {
			fmt.Fprintf(os.Stderr, "mktemp: %v\n", err)
			return 1
		}
		f.Close()
		fmt.Println(f.Name())
	}
	return 0
}

func init() {
	applet.Register(&applet.Applet{Name: "which", Short: "Locate a command", Func: runWhich})
}

func runWhich(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "which: missing operand\n")
		return 1
	}

	exitCode := 0
	for _, name := range args[1:] {
		path, err := execLookPath(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "which: no %s in (PATH)\n", name)
			exitCode = 1
		} else {
			fmt.Println(path)
		}
	}
	return exitCode
}

func execLookPath(name string) (string, error) {
	// Check if it's an absolute or relative path
	if strings.Contains(name, "/") || strings.Contains(name, string(os.PathSeparator)) {
		if _, err := os.Stat(name); err == nil {
			return name, nil
		}
		return "", fmt.Errorf("not found")
	}

	path := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(path) {
		full := filepath.Join(dir, name)
		if _, err := os.Stat(full); err == nil {
			return full, nil
		}
	}
	return "", fmt.Errorf("not found")
}

func init() {
	applet.Register(&applet.Applet{Name: "file", Short: "Determine file type", Func: runFile})
}

func runFile(args []string) int {
	files := []string{}
	for _, a := range args[1:] {
		if !strings.HasPrefix(a, "-") {
			files = append(files, a)
		}
	}

	for _, fname := range files {
		info, err := os.Stat(fname)
		if err != nil {
			fmt.Fprintf(os.Stderr, "file: %s: %v\n", fname, err)
			continue
		}
		if info.IsDir() {
			fmt.Printf("%s: directory\n", fname)
			continue
		}

		// Read first bytes to detect type
		f, err := os.Open(fname)
		if err != nil {
			continue
		}
		buf := make([]byte, 512)
		n, _ := f.Read(buf)
		f.Close()

		mime := "data"
		if n >= 4 {
			if string(buf[:4]) == "\x7fELF" {
				mime = "ELF executable"
			} else if string(buf[:2]) == "PK" {
				mime = "Zip archive"
			} else if string(buf[:3]) == "GIF" {
				mime = "GIF image"
			} else if buf[0] == 0xFF && buf[1] == 0xD8 {
				mime = "JPEG image"
			} else if buf[0] == 0x89 && string(buf[1:4]) == "PNG" {
				mime = "PNG image"
			} else if string(buf[:2]) == "#!" {
				mime = "script"
			} else if isText(buf[:n]) {
				mime = "text"
			}
		}
		fmt.Printf("%s: %s\n", fname, mime)
	}
	return 0
}

func isText(data []byte) bool {
	for _, b := range data {
		if b < 32 && b != '\n' && b != '\r' && b != '\t' {
			return false
		}
	}
	return true
}

func init() {
	applet.Register(&applet.Applet{Name: "tree", Short: "List contents of directories in a tree-like format", Func: runTree})
}

func runTree(args []string) int {
	maxDepth := -1
	path := "."
	for _, a := range args[1:] {
		if strings.HasPrefix(a, "-L") && len(a) > 2 {
			fmt.Sscanf(a[2:], "%d", &maxDepth)
		} else if !strings.HasPrefix(a, "-") {
			path = a
		}
	}

	dirs, files := 0, 0
	printTree(path, "", maxDepth, 0, &dirs, &files)
	fmt.Printf("\n%d directories, %d files\n", dirs, files)
	return 0
}

func printTree(path, prefix string, maxDepth, depth int, dirs, files *int) {
	if maxDepth >= 0 && depth > maxDepth {
		return
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	for i, entry := range entries {
		isLast := i == len(entries)-1
		connector := "├── "
		if isLast {
			connector = "└── "
		}

		if entry.IsDir() {
			*dirs++
			fmt.Printf("%s%s%s/\n", prefix, connector, entry.Name())
			newPrefix := prefix
			if isLast {
				newPrefix += "    "
			} else {
				newPrefix += "│   "
			}
			printTree(filepath.Join(path, entry.Name()), newPrefix, maxDepth, depth+1, dirs, files)
		} else {
			*files++
			fmt.Printf("%s%s%s\n", prefix, connector, entry.Name())
		}
	}
}

func init() {
	applet.Register(&applet.Applet{Name: "mknod", Short: "Make block or character special files", Func: runMknod})
	applet.Register(&applet.Applet{Name: "mkfifo", Short: "Make FIFOs (named pipes)", Func: runMkfifo})
	applet.Register(&applet.Applet{Name: "truncate", Short: "Shrink or extend the size of a file", Func: runTruncate})
	applet.Register(&applet.Applet{Name: "unlink", Short: "Call the unlink function", Func: runUnlink})
	applet.Register(&applet.Applet{Name: "sync", Short: "Synchronize data to disk", Func: runSync})
}

func runMknod(args []string) int {
	if len(args) < 3 {
		fmt.Fprintf(os.Stderr, "mknod: missing operand\n")
		return 1
	}
	name := args[1]
	kind := args[2]
	mode := uint32(0666)
	dev := 0
	if kind == "p" {
		if err := unix.Mkfifo(name, mode); err != nil {
			fmt.Fprintf(os.Stderr, "mknod: %s: %v\n", name, err)
			return 1
		}
		return 0
	}
	if len(args) < 5 {
		fmt.Fprintf(os.Stderr, "mknod: missing major/minor\n")
		return 1
	}
	major, _ := strconv.Atoi(args[3])
	minor, _ := strconv.Atoi(args[4])
	if kind == "b" {
		mode |= unix.S_IFBLK
	} else if kind == "c" || kind == "u" {
		mode |= unix.S_IFCHR
	} else {
		fmt.Fprintf(os.Stderr, "mknod: invalid type %s\n", kind)
		return 1
	}
	dev = int(unix.Mkdev(uint32(major), uint32(minor)))
	if err := unix.Mknod(name, mode, dev); err != nil {
		fmt.Fprintf(os.Stderr, "mknod: %s: %v\n", name, err)
		return 1
	}
	return 0
}

func runMkfifo(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "mkfifo: missing operand\n")
		return 1
	}
	exitCode := 0
	for _, name := range args[1:] {
		if strings.HasPrefix(name, "-") {
			continue
		}
		if err := unix.Mkfifo(name, 0666); err != nil {
			fmt.Fprintf(os.Stderr, "mkfifo: %s: %v\n", name, err)
			exitCode = 1
		}
	}
	return exitCode
}

func runTruncate(args []string) int {
	size := int64(0)
	files := []string{}
	for _, a := range args[1:] {
		if strings.HasPrefix(a, "-s") && len(a) > 2 {
			fmt.Sscanf(a[2:], "%d", &size)
			continue
		}
		if !strings.HasPrefix(a, "-") {
			files = append(files, a)
		}
	}
	for _, f := range files {
		if err := os.Truncate(f, size); err != nil {
			fmt.Fprintf(os.Stderr, "truncate: %s: %v\n", f, err)
			return 1
		}
	}
	return 0
}

func runUnlink(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "unlink: missing operand\n")
		return 1
	}
	if err := os.Remove(args[1]); err != nil {
		fmt.Fprintf(os.Stderr, "unlink: %s: %v\n", args[1], err)
		return 1
	}
	return 0
}

func runSync(args []string) int {
	unix.Sync()
	return 0
}

func readSmallFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func blockBytes(name string) int64 {
	data := strings.TrimSpace(readSmallFile(filepath.Join("/sys/class/block", name, "size")))
	sectors, err := strconv.ParseInt(data, 10, 64)
	if err != nil {
		return 0
	}
	return sectors * 512
}
