package coreutils

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/agentbusybox/pkg/applet"
)

func init() {
	applet.Register(&applet.Applet{Name: "cp", Short: "Copy files and directories", Func: runCp})
}

func runCp(args []string) int {
	recursive := false     // -r -R
	force := false         // -f
	interactive := false   // -i
	verbose := false       // -v
	preserve := false      // -p
	noClobber := false     // -n
	dereference := false   // -L
	noDereference := false // -d -P
	link := false          // -l
	symlink := false       // -s
	noTarget := false      // -T
	targetDir := ""        // -t DIR
	update := false        // -u
	archive := false       // -a
	parents := false       // --parents
	removeDest := false    // --remove-destination
	files := []string{}

	// These flags are parsed but behavior varies by platform
	_ = preserve      // -p: preserve attributes
	_ = noClobber     // -n: don't overwrite
	_ = dereference   // -L: follow symlinks
	_ = noDereference // -d -P: don't follow symlinks
	_ = update        // -u: copy only newer
	_ = parents       // --parents: use full source path

	i := 1
	for ; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			i++
			break
		}
		if !strings.HasPrefix(a, "-") {
			break
		}
		// Long options
		if strings.HasPrefix(a, "--") {
			switch a {
			case "--archive":
				archive = true
			case "--force":
				force = true
			case "--interactive":
				interactive = true
			case "--no-clobber":
				noClobber = true
			case "--link":
				link = true
			case "--dereference":
				dereference = true
			case "--no-dereference":
				noDereference = true
			case "--recursive":
				recursive = true
			case "--symbolic-link":
				symlink = true
			case "--no-target-directory":
				noTarget = true
			case "--verbose":
				verbose = true
			case "--update":
				update = true
			case "--remove-destination":
				removeDest = true
			case "--parents":
				parents = true
			default:
				if strings.HasPrefix(a, "--target-directory=") {
					targetDir = a[19:]
				}
			}
			continue
		}
		for _, ch := range a[1:] {
			switch ch {
			case 'a':
				archive = true
			case 'r', 'R':
				recursive = true
			case 'd', 'P':
				noDereference = true
			case 'L':
				dereference = true
			case 'H': // follow CLI symlinks (implies -L for CLI args)
			case 'p':
				preserve = true
			case 'f':
				force = true
			case 'i':
				interactive = true
			case 'n':
				noClobber = true
			case 'l':
				link = true
			case 's':
				symlink = true
			case 'T':
				noTarget = true
			case 'u':
				update = true
			case 'v':
				verbose = true
			case 't':
				if i+1 < len(args) {
					i++
					targetDir = args[i]
				}
			default:
				fmt.Fprintf(os.Stderr, "cp: invalid option -- '%c'\n", ch)
				return 1
			}
		}
	}
	files = args[i:]

	// -a is same as -dpR
	if archive {
		noDereference = true
		recursive = true
	}

	// -t overrides dest detection
	if targetDir != "" {
		files = append(files, targetDir)
	}

	if len(files) < 2 {
		fmt.Fprintf(os.Stderr, "cp: missing file operand\n")
		return 1
	}

	dest := files[len(files)-1]
	sources := files[:len(files)-1]

	if noTarget {
		// -T: treat dest as file, not directory
	} else {
		destInfo, err := os.Stat(dest)
		destIsDir := err == nil && destInfo.IsDir()
		if len(sources) > 1 && !destIsDir {
			fmt.Fprintf(os.Stderr, "cp: target '%s' is not a directory\n", dest)
			return 1
		}
		if destIsDir && !noTarget {
			newSources := []string{}
			for _, s := range sources {
				newSources = append(newSources, s)
			}
			sources = newSources
		}
	}

	exitCode := 0
	for _, src := range sources {
		dst := dest
		destInfo, _ := os.Stat(dest)
		if destInfo != nil && destInfo.IsDir() && !noTarget {
			dst = filepath.Join(dest, filepath.Base(src))
		}

		if removeDest {
			os.Remove(dst)
		}

		if link {
			if err := os.Link(src, dst); err != nil {
				fmt.Fprintf(os.Stderr, "cp: %v\n", err)
				exitCode = 1
			}
			if verbose {
				fmt.Printf("'%s' -> '%s'\n", src, dst)
			}
			continue
		}
		if symlink {
			if err := os.Symlink(src, dst); err != nil {
				fmt.Fprintf(os.Stderr, "cp: %v\n", err)
				exitCode = 1
			}
			if verbose {
				fmt.Printf("'%s' -> '%s'\n", src, dst)
			}
			continue
		}

		if err := copyPath(src, dst, recursive, force, interactive, verbose); err != nil {
			fmt.Fprintf(os.Stderr, "cp: %v\n", err)
			exitCode = 1
		}
	}
	return exitCode
}

func copyPath(src, dst string, recursive, force, interactive, verbose bool) error {
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("cannot stat '%s': %w", src, err)
	}

	if info.IsDir() {
		if !recursive {
			return fmt.Errorf("'%s' is a directory (not copied)", src)
		}
		return copyDir(src, dst, force, interactive, verbose)
	}

	return copyFile(src, dst, info.Mode(), force, interactive, verbose)
}

func copyFile(src, dst string, mode os.FileMode, force, interactive, verbose bool) error {
	if _, err := os.Stat(dst); err == nil {
		if interactive {
			fmt.Printf("cp: overwrite '%s'? ", dst)
			var answer string
			fmt.Scanln(&answer)
			if !strings.HasPrefix(strings.ToLower(answer), "y") {
				return nil
			}
		}
		if !force {
			if err := os.Remove(dst); err != nil {
				return err
			}
		}
	}

	sf, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sf.Close()

	df, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer df.Close()

	if _, err := io.Copy(df, sf); err != nil {
		return err
	}

	if verbose {
		fmt.Printf("'%s' -> '%s'\n", src, dst)
	}
	return nil
}

func copyDir(src, dst string, force, interactive, verbose bool) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, info.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath, force, interactive, verbose); err != nil {
				return err
			}
		} else {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if err := copyFile(srcPath, dstPath, info.Mode(), force, interactive, verbose); err != nil {
				return err
			}
		}
	}
	return nil
}

func init() {
	applet.Register(&applet.Applet{Name: "mv", Short: "Move/rename files", Func: runMv})
}

func runMv(args []string) int {
	force := false        // -f
	interactive := false  // -i
	noClobber := false    // -n
	verbose := false      // -v
	noTarget := false     // -T
	targetDir := ""       // -t DIR
	backup := false       // -b
	backupSuffix := "~"   // -S SUFFIX
	stripSlashes := false // --strip-trailing-slashes
	update := false       // -u
	files := []string{}
	_ = update

	i := 1
	for ; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			i++
			break
		}
		if !strings.HasPrefix(a, "-") {
			break
		}
		if strings.HasPrefix(a, "--") {
			switch a {
			case "--no-clobber":
				noClobber = true
			case "--no-target-directory":
				noTarget = true
			case "--verbose":
				verbose = true
			case "--backup":
				backup = true
			case "--strip-trailing-slashes":
				stripSlashes = true
			case "--update":
				update = true
			default:
				if strings.HasPrefix(a, "--target-directory=") {
					targetDir = a[19:]
				}
				if strings.HasPrefix(a, "--suffix=") {
					backupSuffix = a[9:]
				}
			}
			continue
		}
		for _, ch := range a[1:] {
			switch ch {
			case 'f':
				force = true
			case 'i':
				interactive = true
			case 'n':
				noClobber = true
			case 'T':
				noTarget = true
			case 'v':
				verbose = true
			case 'b':
				backup = true
			case 'u':
				update = true
			case 't':
				if i+1 < len(args) {
					i++
					targetDir = args[i]
				}
			case 'S':
				if i+1 < len(args) {
					i++
					backupSuffix = args[i]
				}
			default:
				fmt.Fprintf(os.Stderr, "mv: invalid option -- '%c'\n", ch)
				return 1
			}
		}
	}
	files = args[i:]

	if targetDir != "" {
		files = append(files, targetDir)
	}

	if len(files) < 2 {
		fmt.Fprintf(os.Stderr, "mv: missing file operand\n")
		return 1
	}

	dest := files[len(files)-1]
	sources := files[:len(files)-1]

	if stripSlashes {
		for j := range sources {
			sources[j] = strings.TrimRight(sources[j], "/")
		}
	}

	destInfo, err := os.Stat(dest)
	destIsDir := err == nil && destInfo.IsDir()

	if len(sources) > 1 && !destIsDir && !noTarget {
		fmt.Fprintf(os.Stderr, "mv: target '%s' is not a directory\n", dest)
		return 1
	}

	exitCode := 0
	for _, src := range sources {
		dst := dest
		if destIsDir && !noTarget {
			dst = filepath.Join(dest, filepath.Base(src))
		}

		if _, err := os.Stat(dst); err == nil {
			if noClobber {
				continue
			}
			if interactive {
				fmt.Printf("mv: overwrite '%s'? ", dst)
				var answer string
				fmt.Scanln(&answer)
				if !strings.HasPrefix(strings.ToLower(answer), "y") {
					continue
				}
			}
			if backup {
				os.Rename(dst, dst+backupSuffix)
			}
			if !force {
				os.Remove(dst)
			}
		}

		if err := os.Rename(src, dst); err != nil {
			if err2 := copyPath(src, dst, true, force, false, false); err2 != nil {
				fmt.Fprintf(os.Stderr, "mv: cannot move '%s' to '%s': %v\n", src, dst, err2)
				exitCode = 1
				continue
			}
			os.RemoveAll(src)
		}
		if verbose {
			fmt.Printf("'%s' -> '%s'\n", src, dst)
		}
	}
	return exitCode
}

func init() {
	applet.Register(&applet.Applet{Name: "rm", Short: "Remove files or directories", Func: runRm})
}

func runRm(args []string) int {
	recursive := false       // -r -R
	force := false           // -f
	interactive := false     // -i
	verbose := false         // -v
	interactiveOnce := false // -I
	preserveRoot := true     // --preserve-root (default)
	files := []string{}

	i := 1
	for ; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			i++
			break
		}
		if a == "-" {
			files = append(files, a)
			continue
		}
		if !strings.HasPrefix(a, "-") {
			break
		}
		if strings.HasPrefix(a, "--") {
			switch a {
			case "--preserve-root":
				preserveRoot = true
			case "--no-preserve-root":
				preserveRoot = false
			case "--recursive":
				recursive = true
			case "--force":
				force = true
			case "--interactive":
				interactive = true
			case "--verbose":
				verbose = true
			}
			continue
		}
		for _, ch := range a[1:] {
			switch ch {
			case 'r', 'R':
				recursive = true
			case 'f':
				force = true
			case 'i':
				interactive = true
			case 'I':
				interactiveOnce = true
			case 'v':
				verbose = true
			}
		}
	}
	files = append(files, args[i:]...)

	if len(files) == 0 {
		if force {
			return 0
		}
		fmt.Fprintf(os.Stderr, "rm: missing operand\n")
		return 1
	}

	// -I: prompt once before removing more than 3 files
	if interactiveOnce && len(files) > 3 && !force {
		fmt.Printf("rm: remove %d arguments? ", len(files))
		var answer string
		fmt.Scanln(&answer)
		if !strings.HasPrefix(strings.ToLower(answer), "y") {
			return 0
		}
	}

	exitCode := 0
	for _, f := range files {
		if preserveRoot && (f == "/") {
			fmt.Fprintf(os.Stderr, "rm: it is dangerous to operate recursively on '/'\n")
			exitCode = 1
			continue
		}

		info, err := os.Lstat(f)
		if err != nil {
			if force && os.IsNotExist(err) {
				continue
			}
			fmt.Fprintf(os.Stderr, "rm: cannot remove '%s': %v\n", f, err)
			exitCode = 1
			continue
		}

		if info.IsDir() && !recursive {
			fmt.Fprintf(os.Stderr, "rm: cannot remove '%s': Is a directory\n", f)
			exitCode = 1
			continue
		}

		if interactive {
			fmt.Printf("rm: remove%s '%s'? ", map[bool]string{true: " directory", false: ""}[info.IsDir()], f)
			var answer string
			fmt.Scanln(&answer)
			if !strings.HasPrefix(strings.ToLower(answer), "y") {
				continue
			}
		}

		if info.IsDir() {
			err = os.RemoveAll(f)
		} else {
			err = os.Remove(f)
		}
		if err != nil {
			if !force {
				fmt.Fprintf(os.Stderr, "rm: cannot remove '%s': %v\n", f, err)
				exitCode = 1
			}
		} else if verbose {
			fmt.Printf("removed '%s'\n", f)
		}
	}
	return exitCode
}

func init() {
	applet.Register(&applet.Applet{Name: "mkdir", Short: "Create directories", Func: runMkdir})
	applet.Register(&applet.Applet{Name: "rmdir", Short: "Remove empty directories", Func: runRmdir})
}

func runMkdir(args []string) int {
	parents, verbose := false, false
	mode := os.FileMode(0755)
	dirs := []string{}
	i := 1
	for ; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			i++
			break
		}
		if !strings.HasPrefix(a, "-") {
			break
		}
		if a == "-p" {
			parents = true
			continue
		}
		if a == "-v" {
			verbose = true
			continue
		}
		if strings.HasPrefix(a, "-m") && len(a) > 2 {
			fmt.Sscanf(a[2:], "%o", &mode)
			continue
		}
		if i+1 < len(args) && a == "-m" {
			i++
			fmt.Sscanf(args[i], "%o", &mode)
			continue
		}
	}
	dirs = args[i:]
	if len(dirs) == 0 {
		fmt.Fprintf(os.Stderr, "mkdir: missing operand\n")
		return 1
	}

	exitCode := 0
	for _, d := range dirs {
		if parents {
			if err := os.MkdirAll(d, mode); err != nil {
				fmt.Fprintf(os.Stderr, "mkdir: cannot create directory '%s': %v\n", d, err)
				exitCode = 1
			} else if verbose {
				fmt.Printf("mkdir: created directory '%s'\n", d)
			}
		} else {
			if err := os.Mkdir(d, mode); err != nil {
				fmt.Fprintf(os.Stderr, "mkdir: cannot create directory '%s': %v\n", d, err)
				exitCode = 1
			} else if verbose {
				fmt.Printf("mkdir: created directory '%s'\n", d)
			}
		}
	}
	return exitCode
}

func runRmdir(args []string) int {
	parents, verbose := false, false
	dirs := []string{}
	i := 1
	for ; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			i++
			break
		}
		if !strings.HasPrefix(a, "-") {
			break
		}
		if a == "-p" {
			parents = true
			continue
		}
		if a == "-v" {
			verbose = true
			continue
		}
	}
	dirs = args[i:]
	if len(dirs) == 0 {
		fmt.Fprintf(os.Stderr, "rmdir: missing operand\n")
		return 1
	}

	exitCode := 0
	for _, d := range dirs {
		if parents {
			for d != "." && d != "/" {
				if err := os.Remove(d); err != nil {
					fmt.Fprintf(os.Stderr, "rmdir: failed to remove '%s': %v\n", d, err)
					exitCode = 1
					break
				}
				if verbose {
					fmt.Printf("rmdir: removing directory '%s'\n", d)
				}
				d = filepath.Dir(d)
			}
		} else {
			if err := os.Remove(d); err != nil {
				fmt.Fprintf(os.Stderr, "rmdir: failed to remove '%s': %v\n", d, err)
				exitCode = 1
			} else if verbose {
				fmt.Printf("rmdir: removing directory '%s'\n", d)
			}
		}
	}
	return exitCode
}

func init() {
	applet.Register(&applet.Applet{Name: "touch", Short: "Change file timestamps/create files", Func: runTouch})
}

func runTouch(args []string) int {
	noCreate, accessOnly, modifyOnly := false, false, false
	refFile := ""
	files := []string{}

	i := 1
	for ; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			i++
			break
		}
		if !strings.HasPrefix(a, "-") {
			break
		}
		for _, ch := range a[1:] {
			switch ch {
			case 'c':
				noCreate = true
			case 'a':
				accessOnly = true
			case 'm':
				modifyOnly = true
			}
		}
	}
	files = args[i:]
	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "touch: missing file operand\n")
		return 1
	}

	var refTime time.Time
	var refInfo os.FileInfo
	if refFile != "" {
		var err error
		refInfo, err = os.Stat(refFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "touch: cannot stat '%s': %v\n", refFile, err)
			return 1
		}
		refTime = refInfo.ModTime()
	} else {
		refTime = time.Now()
	}

	exitCode := 0
	for _, f := range files {
		if _, err := os.Stat(f); os.IsNotExist(err) {
			if noCreate {
				continue
			}
			file, err := os.Create(f)
			if err != nil {
				fmt.Fprintf(os.Stderr, "touch: cannot touch '%s': %v\n", f, err)
				exitCode = 1
				continue
			}
			file.Close()
		}
		if err := os.Chtimes(f, refTime, refTime); err != nil {
			fmt.Fprintf(os.Stderr, "touch: cannot touch '%s': %v\n", f, err)
			exitCode = 1
		}
		_ = accessOnly
		_ = modifyOnly
	}
	return exitCode
}

func init() {
	applet.Register(&applet.Applet{Name: "pwd", Short: "Print working directory", Func: runPwd})
	applet.Register(&applet.Applet{Name: "basename", Short: "Strip directory and suffix from filenames", Func: runBasename})
	applet.Register(&applet.Applet{Name: "dirname", Short: "Strip last component from file path", Func: runDirname})
}

func runPwd(args []string) int {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "pwd: %v\n", err)
		return 1
	}
	fmt.Println(dir)
	return 0
}

func runBasename(args []string) int {
	args = args[1:]
	all := false
	suffix := ""

	cleaned := []string{}
	for i := 0; i < len(args); i++ {
		if args[i] == "-a" || args[i] == "--multiple" {
			all = true
		} else if args[i] == "-s" && i+1 < len(args) {
			i++
			suffix = args[i]
		} else if args[i] == "--" {
			i++
			cleaned = append(cleaned, args[i:]...)
			break
		} else {
			cleaned = append(cleaned, args[i])
		}
	}

	if len(cleaned) == 0 {
		fmt.Fprintf(os.Stderr, "basename: missing operand\n")
		return 1
	}

	names := cleaned
	if !all && len(cleaned) > 1 {
		suffix = cleaned[1]
		names = cleaned[:1]
	}

	for _, name := range names {
		b := filepath.Base(name)
		if suffix != "" && strings.HasSuffix(b, suffix) && b != suffix {
			b = b[:len(b)-len(suffix)]
		}
		fmt.Println(b)
	}
	return 0
}

func runDirname(args []string) int {
	args = args[1:]
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "dirname: missing operand\n")
		return 1
	}
	for _, a := range args {
		fmt.Println(filepath.Dir(a))
	}
	return 0
}

func init() {
	applet.Register(&applet.Applet{Name: "comm", Short: "Compare two sorted files line by line", Func: runComm})
}

func runComm(args []string) int {
	suppress1, suppress2, suppress3 := false, false, false
	files := []string{}

	for _, a := range args[1:] {
		if a == "-1" {
			suppress1 = true
			continue
		}
		if a == "-2" {
			suppress2 = true
			continue
		}
		if a == "-3" {
			suppress3 = true
			continue
		}
		if a == "--" {
			continue
		}
		if !strings.HasPrefix(a, "-") {
			files = append(files, a)
		}
	}

	if len(files) != 2 {
		fmt.Fprintf(os.Stderr, "comm: missing operands\n")
		return 1
	}

	lines1, err := readLines(files[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "comm: %v\n", err)
		return 1
	}
	lines2, err := readLines(files[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "comm: %v\n", err)
		return 1
	}

	i, j := 0, 0
	for i < len(lines1) && j < len(lines2) {
		switch {
		case lines1[i] < lines2[j]:
			if !suppress1 {
				fmt.Println(lines1[i])
			}
			i++
		case lines1[i] > lines2[j]:
			if !suppress2 {
				fmt.Printf("\t%s\n", lines2[j])
			}
			j++
		default:
			if !suppress3 {
				fmt.Printf("\t\t%s\n", lines1[i])
			}
			i++
			j++
		}
	}
	for ; i < len(lines1); i++ {
		if !suppress1 {
			fmt.Println(lines1[i])
		}
	}
	for ; j < len(lines2); j++ {
		if !suppress2 {
			fmt.Printf("\t%s\n", lines2[j])
		}
	}
	return 0
}

func readLines(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	return lines, nil
}

func init() {
	applet.Register(&applet.Applet{Name: "cut", Short: "Print selected parts of lines", Func: runCut})
}

func runCut(args []string) int {
	list := ""
	delimiter := "\t"
	onlyDelimited := false
	mode := "f" // b=bytes, c=chars, f=fields

	files := []string{}
	i := 1
	for ; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			i++
			break
		}
		if strings.HasPrefix(a, "-b") {
			mode = "b"
			list = a[2:]
			if list == "" && i+1 < len(args) {
				i++
				list = args[i]
			}
			continue
		}
		if strings.HasPrefix(a, "-c") {
			mode = "c"
			list = a[2:]
			if list == "" && i+1 < len(args) {
				i++
				list = args[i]
			}
			continue
		}
		if strings.HasPrefix(a, "-f") {
			mode = "f"
			list = a[2:]
			if list == "" && i+1 < len(args) {
				i++
				list = args[i]
			}
			continue
		}
		if strings.HasPrefix(a, "-d") {
			delimiter = a[2:]
			if delimiter == "" && i+1 < len(args) {
				i++
				delimiter = args[i]
			}
			continue
		}
		if a == "-s" {
			onlyDelimited = true
			continue
		}
		if !strings.HasPrefix(a, "-") {
			files = append(files, a)
			continue
		}
	}
	files = append(files, args[i:]...)

	if list == "" {
		fmt.Fprintf(os.Stderr, "cut: missing list\n")
		return 1
	}

	fields := parseList(list)
	if len(files) == 0 {
		files = []string{"-"}
	}

	for _, fname := range files {
		var r io.Reader
		if fname == "-" {
			r = os.Stdin
		} else {
			f, err := os.Open(fname)
			if err != nil {
				fmt.Fprintf(os.Stderr, "cut: %s: %v\n", fname, err)
				return 1
			}
			defer f.Close()
			r = f
		}

		data, err := io.ReadAll(r)
		if err != nil {
			return 1
		}

		for _, line := range strings.Split(string(data), "\n") {
			if line == "" {
				continue
			}
			switch mode {
			case "f":
				parts := strings.Split(line, delimiter)
				if onlyDelimited && !strings.Contains(line, delimiter) {
					continue
				}
				result := []string{}
				for _, f := range fields {
					if f >= 1 && f <= len(parts) {
						result = append(result, parts[f-1])
					}
				}
				fmt.Println(strings.Join(result, delimiter))
			case "c", "b":
				runes := []rune(line)
				result := []rune{}
				for _, f := range fields {
					if f >= 1 && f <= len(runes) {
						result = append(result, runes[f-1])
					}
				}
				fmt.Println(string(result))
			}
		}
	}
	return 0
}

func parseList(s string) []int {
	result := []int{}
	for _, part := range strings.Split(s, ",") {
		if strings.Contains(part, "-") {
			var start, end int
			fmt.Sscanf(part, "%d-%d", &start, &end)
			for i := start; i <= end; i++ {
				result = append(result, i)
			}
		} else {
			var n int
			fmt.Sscanf(part, "%d", &n)
			result = append(result, n)
		}
	}
	return result
}

func init() {
	applet.Register(&applet.Applet{Name: "wc", Short: "Word, line, byte, character count", Func: runWc})
}

func runWc(args []string) int {
	showLines, showWords, showBytes, showChars := false, false, false, false
	files := []string{}

	for _, a := range args[1:] {
		if a == "--" {
			continue
		}
		if strings.HasPrefix(a, "-") {
			for _, ch := range a[1:] {
				switch ch {
				case 'l':
					showLines = true
				case 'w':
					showWords = true
				case 'c':
					showBytes = true
				case 'm':
					showChars = true
				}
			}
		} else {
			files = append(files, a)
		}
	}

	if !showLines && !showWords && !showBytes && !showChars {
		showLines, showWords, showBytes = true, true, true
	}

	if len(files) == 0 {
		files = []string{"-"}
	}

	totalLines, totalWords, totalBytes := 0, 0, 0

	for _, fname := range files {
		var data []byte
		var err error
		if fname == "-" {
			data, err = io.ReadAll(os.Stdin)
		} else {
			data, err = os.ReadFile(fname)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "wc: %s: %v\n", fname, err)
			return 1
		}

		lines := strings.Count(string(data), "\n")
		words := len(strings.Fields(string(data)))
		bytes := len(data)

		totalLines += lines
		totalWords += words
		totalBytes += bytes

		if showLines {
			fmt.Printf("\t%d", lines)
		}
		if showWords {
			fmt.Printf("\t%d", words)
		}
		if showBytes {
			fmt.Printf("\t%d", bytes)
		}
		if showChars {
			fmt.Printf("\t%d", len([]rune(string(data))))
		}
		if fname != "-" {
			fmt.Printf(" %s", fname)
		}
		fmt.Println()
	}

	if len(files) > 1 {
		if showLines {
			fmt.Printf("\t%d", totalLines)
		}
		if showWords {
			fmt.Printf("\t%d", totalWords)
		}
		if showBytes {
			fmt.Printf("\t%d", totalBytes)
		}
		fmt.Println(" total")
	}
	return 0
}

func init() {
	applet.Register(&applet.Applet{Name: "head", Short: "Output first part of files", Func: runHead})
}

func runHead(args []string) int {
	count := 10
	lines := true
	quiet, verbose := false, false
	files := []string{}

	i := 1
	for ; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			i++
			break
		}
		// Legacy numeric syntax: -5 means -n 5
		if len(a) > 1 && a[0] == '-' && a[1] >= '0' && a[1] <= '9' {
			fmt.Sscanf(a[1:], "%d", &count)
			continue
		}
		if strings.HasPrefix(a, "-n") {
			s := a[2:]
			if s == "" && i+1 < len(args) {
				i++
				s = args[i]
			}
			// Handle +N (all but last N)
			if strings.HasPrefix(s, "-") {
				neg := 0
				fmt.Sscanf(s[1:], "%d", &neg)
				count = -neg // negative means "all but last N"
			} else {
				fmt.Sscanf(s, "%d", &count)
			}
			continue
		}
		if strings.HasPrefix(a, "-c") {
			lines = false
			s := a[2:]
			if s == "" && i+1 < len(args) {
				i++
				s = args[i]
			}
			fmt.Sscanf(s, "%d", &count)
			continue
		}
		if a == "-q" {
			quiet = true
			continue
		}
		if a == "-v" {
			verbose = true
			continue
		}
		if !strings.HasPrefix(a, "-") {
			files = append(files, a)
			continue
		}
	}
	files = append(files, args[i:]...)

	if len(files) == 0 {
		files = []string{"-"}
	}
	printHeader := !quiet && len(files) > 1

	exitCode := 0
	for _, fname := range files {
		var data []byte
		var err error
		if fname == "-" {
			data, err = io.ReadAll(os.Stdin)
		} else {
			data, err = os.ReadFile(fname)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "head: %s: %v\n", fname, err)
			exitCode = 1
			continue
		}

		if printHeader || verbose {
			fmt.Printf("==> %s <==\n", fname)
		}

		if lines {
			text := string(data)
			nlCount := 0
			for i := 0; i < len(text); i++ {
				if text[i] == '\n' {
					nlCount++
					if nlCount >= count {
						text = text[:i]
						break
					}
				}
			}
			fmt.Print(text)
			if len(text) > 0 && text[len(text)-1] != '\n' {
				fmt.Println()
			}
		} else {
			if count > len(data) {
				count = len(data)
			}
			fmt.Print(string(data[:count]))
		}

		if printHeader && fname != files[len(files)-1] {
			fmt.Println()
		}
	}
	return exitCode
}

func init() {
	applet.Register(&applet.Applet{Name: "tail", Short: "Output last part of files", Func: runTail})
}

func runTail(args []string) int {
	count := 10
	lines := true
	follow := false      // -f
	followRetry := false // -F
	interval := 1        // -s SEC
	_ = followRetry
	_ = interval
	quiet, verbose := false, false
	fromStart := false // +N means start from line N
	files := []string{}

	i := 1
	for ; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			i++
			break
		}
		// Legacy numeric syntax: -5 or +5
		if len(a) > 1 && (a[0] == '-' || a[0] == '+') && a[1] >= '0' && a[1] <= '9' {
			if a[0] == '+' {
				fromStart = true
				fmt.Sscanf(a[1:], "%d", &count)
			} else {
				fmt.Sscanf(a[1:], "%d", &count)
			}
			continue
		}
		if strings.HasPrefix(a, "-n") {
			s := a[2:]
			if s == "" && i+1 < len(args) {
				i++
				s = args[i]
			}
			if strings.HasPrefix(s, "+") {
				fromStart = true
				fmt.Sscanf(s[1:], "%d", &count)
			} else {
				fmt.Sscanf(s, "%d", &count)
			}
			continue
		}
		if strings.HasPrefix(a, "-c") {
			lines = false
			s := a[2:]
			if s == "" && i+1 < len(args) {
				i++
				s = args[i]
			}
			if strings.HasPrefix(s, "+") {
				fromStart = true
				fmt.Sscanf(s[1:], "%d", &count)
			} else {
				fmt.Sscanf(s, "%d", &count)
			}
			continue
		}
		if a == "-f" {
			follow = true
			continue
		}
		if a == "-F" {
			followRetry = true
			follow = true
			continue
		}
		if a == "-q" {
			quiet = true
			continue
		}
		if a == "-v" {
			verbose = true
			continue
		}
		if strings.HasPrefix(a, "-s") {
			s := a[2:]
			if s == "" && i+1 < len(args) {
				i++
				s = args[i]
			}
			fmt.Sscanf(s, "%d", &interval)
			continue
		}
		if !strings.HasPrefix(a, "-") {
			files = append(files, a)
			continue
		}
	}
	files = append(files, args[i:]...)

	if len(files) == 0 {
		files = []string{"-"}
	}
	printHeader := !quiet && len(files) > 1

	exitCode := 0
	for _, fname := range files {
		var data []byte
		var err error
		if fname == "-" {
			data, err = io.ReadAll(os.Stdin)
		} else {
			data, err = os.ReadFile(fname)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "tail: %s: %v\n", fname, err)
			exitCode = 1
			continue
		}

		if printHeader || verbose {
			fmt.Printf("==> %s <==\n", fname)
		}

		if lines {
			text := string(data)
			parts := strings.Split(text, "\n")
			var start int
			if fromStart {
				start = count - 1
			} else {
				start = len(parts) - count - 1
			}
			if start < 0 {
				start = 0
			}
			if start > len(parts) {
				start = len(parts)
			}
			for _, l := range parts[start:] {
				fmt.Println(l)
			}
		} else {
			if count > len(data) {
				count = len(data)
			}
			fmt.Print(string(data[len(data)-count:]))
		}
	}
	_ = follow // follow mode would require blocking
	return exitCode
}

func init() {
	applet.Register(&applet.Applet{Name: "paste", Short: "Merge lines of files", Func: runPaste})
	applet.Register(&applet.Applet{Name: "fold", Short: "Wrap input lines to fit width", Func: runFold})
	applet.Register(&applet.Applet{Name: "nl", Short: "Number lines of files", Func: runNl})
	applet.Register(&applet.Applet{Name: "expand", Short: "Convert tabs to spaces", Func: runExpand})
	applet.Register(&applet.Applet{Name: "unexpand", Short: "Convert spaces to tabs", Func: runUnexpand})
	applet.Register(&applet.Applet{Name: "shred", Short: "Overwrite a file to hide its contents", Func: runShred})
}

func runPaste(args []string) int {
	serial := false
	delimiter := "\t"
	files := []string{}

	for _, a := range args[1:] {
		if a == "-s" {
			serial = true
			continue
		}
		if strings.HasPrefix(a, "-d") && len(a) > 2 {
			delimiter = a[2:]
			continue
		}
		if a == "--" {
			continue
		}
		files = append(files, a)
	}

	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "paste: missing file operand\n")
		return 1
	}

	// Read all lines from all files
	allLines := make([][]string, len(files))
	for i, fname := range files {
		data, err := os.ReadFile(fname)
		if err != nil {
			fmt.Fprintf(os.Stderr, "paste: %s: %v\n", fname, err)
			return 1
		}
		allLines[i] = strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	}

	if serial {
		for _, lines := range allLines {
			fmt.Println(strings.Join(lines, delimiter))
		}
	} else {
		maxLen := 0
		for _, lines := range allLines {
			if len(lines) > maxLen {
				maxLen = len(lines)
			}
		}
		for i := 0; i < maxLen; i++ {
			parts := make([]string, len(files))
			for j, lines := range allLines {
				if i < len(lines) {
					parts[j] = lines[i]
				}
			}
			fmt.Println(strings.Join(parts, delimiter))
		}
	}
	return 0
}

func runFold(args []string) int {
	width := 80
	breakAtSpaces := false

	for _, a := range args[1:] {
		if strings.HasPrefix(a, "-w") && len(a) > 2 {
			fmt.Sscanf(a[2:], "%d", &width)
		} else if a == "-s" {
			breakAtSpaces = true
		}
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return 1
	}

	for _, line := range strings.Split(string(data), "\n") {
		for len([]rune(line)) > width {
			cut := width
			if breakAtSpaces {
				lastSpace := strings.LastIndex(line[:width], " ")
				if lastSpace > 0 {
					cut = lastSpace + 1
				}
			}
			runes := []rune(line)
			fmt.Println(string(runes[:cut]))
			line = string(runes[cut:])
		}
		if len(line) > 0 {
			fmt.Println(line)
		}
	}
	return 0
}

func runNl(args []string) int {
	number := "t"
	separator := "\t"
	width := 6
	files := []string{}

	for _, a := range args[1:] {
		if strings.HasPrefix(a, "-b") && len(a) > 2 {
			number = a[2:]
			continue
		}
		if strings.HasPrefix(a, "-s") && len(a) > 2 {
			separator = a[2:]
			continue
		}
		if strings.HasPrefix(a, "-w") && len(a) > 2 {
			fmt.Sscanf(a[2:], "%d", &width)
			continue
		}
		if a == "--" {
			continue
		}
		if !strings.HasPrefix(a, "-") {
			files = append(files, a)
		}
	}
	if len(files) == 0 {
		files = []string{"-"}
	}

	lineNum := 1
	for _, fname := range files {
		var data []byte
		var err error
		if fname == "-" {
			data, err = io.ReadAll(os.Stdin)
		} else {
			data, err = os.ReadFile(fname)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "nl: %s: %v\n", fname, err)
			return 1
		}

		for _, line := range strings.Split(string(data), "\n") {
			isBlank := strings.TrimSpace(line) == ""
			print := false
			switch number {
			case "a":
				print = true
			case "t":
				print = !isBlank
			case "n":
				print = false
			}
			if print {
				fmt.Printf("%*d%s%s\n", width, lineNum, separator, line)
				lineNum++
			} else {
				fmt.Println(line)
			}
		}
	}
	return 0
}

func runExpand(args []string) int {
	tabSize := 8
	files := []string{}

	for _, a := range args[1:] {
		if strings.HasPrefix(a, "-t") && len(a) > 2 {
			fmt.Sscanf(a[2:], "%d", &tabSize)
			continue
		}
		if a == "--" {
			continue
		}
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
			data, err = io.ReadAll(os.Stdin)
		} else {
			data, err = os.ReadFile(fname)
		}
		if err != nil {
			return 1
		}
		for _, line := range strings.Split(string(data), "\n") {
			expanded := expandTabs(line, tabSize)
			fmt.Println(expanded)
		}
	}
	return 0
}

func expandTabs(s string, tabSize int) string {
	var b strings.Builder
	col := 0
	for _, r := range s {
		if r == '\t' {
			for {
				b.WriteByte(' ')
				col++
				if col%tabSize == 0 {
					break
				}
			}
		} else {
			b.WriteRune(r)
			col++
		}
	}
	return b.String()
}

func runUnexpand(args []string) int {
	tabSize := 8
	all := false
	files := []string{}

	for _, a := range args[1:] {
		if strings.HasPrefix(a, "-t") && len(a) > 2 {
			fmt.Sscanf(a[2:], "%d", &tabSize)
			continue
		}
		if a == "-a" {
			all = true
			continue
		}
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
			data, err = io.ReadAll(os.Stdin)
		} else {
			data, err = os.ReadFile(fname)
		}
		if err != nil {
			return 1
		}
		for _, line := range strings.Split(string(data), "\n") {
			if all {
				fmt.Println(unexpandLine(line, tabSize))
			} else {
				fmt.Println(line) // only leading spaces by default
			}
		}
	}
	return 0
}

func unexpandLine(s string, tabSize int) string {
	var b strings.Builder
	spaceCount := 0
	for _, r := range s {
		if r == ' ' {
			spaceCount++
			if spaceCount == tabSize {
				b.WriteByte('\t')
				spaceCount = 0
			}
		} else {
			for i := 0; i < spaceCount; i++ {
				b.WriteByte(' ')
			}
			spaceCount = 0
			b.WriteRune(r)
		}
	}
	for i := 0; i < spaceCount; i++ {
		b.WriteByte(' ')
	}
	return b.String()
}

func runShred(args []string) int {
	remove, verbose := false, false
	iterations := 3
	files := []string{}

	for _, a := range args[1:] {
		if a == "-u" || a == "--remove" {
			remove = true
			continue
		}
		if a == "-v" || a == "--verbose" {
			verbose = true
			continue
		}
		if strings.HasPrefix(a, "-n") && len(a) > 2 {
			fmt.Sscanf(a[2:], "%d", &iterations)
			continue
		}
		if !strings.HasPrefix(a, "-") {
			files = append(files, a)
		}
	}

	exitCode := 0
	for _, fname := range files {
		info, err := os.Stat(fname)
		if err != nil {
			fmt.Fprintf(os.Stderr, "shred: %s: %v\n", fname, err)
			exitCode = 1
			continue
		}
		size := info.Size()

		f, err := os.OpenFile(fname, os.O_WRONLY, 0)
		if err != nil {
			fmt.Fprintf(os.Stderr, "shred: %s: %v\n", fname, err)
			exitCode = 1
			continue
		}

		buf := make([]byte, 4096)
		for pass := 0; pass < iterations; pass++ {
			f.Seek(0, 0)
			remaining := size
			for remaining > 0 {
				n := int64(len(buf))
				if n > remaining {
					n = remaining
				}
				// Write random-ish data
				for i := int64(0); i < n; i++ {
					buf[i] = byte(pass + int(i%256))
				}
				f.Write(buf[:n])
				remaining -= n
			}
			f.Sync()
			if verbose {
				fmt.Printf("shred: %s: pass %d/%d\n", fname, pass+1, iterations)
			}
		}
		f.Close()

		if remove {
			os.Remove(fname)
			if verbose {
				fmt.Printf("shred: %s: removed\n", fname)
			}
		}
	}
	return exitCode
}
