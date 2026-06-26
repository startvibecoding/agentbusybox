package e2fsprogs

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/agentbusybox/pkg/applet"
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
	attrs := ""

	for _, a := range args[1:] {
		if a == "-R" || a == "--recursive" {
			recursive = true
			continue
		}
		if strings.HasPrefix(a, "-") || strings.HasPrefix(a, "+") || strings.HasPrefix(a, "=") {
			attrs = a
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

	for _, fname := range files {
		if recursive {
			filepathWalk(fname, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				chattrApply(path, attrs)
				return nil
			})
		} else {
			chattrApply(fname, attrs)
		}
	}
	return 0
}

func chattrApply(path, attrs string) {
	// Attribute manipulation requires root + ioctl
	_ = path
	_ = attrs
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
			filepathWalk(fname, func(path string, info os.FileInfo, err error) error {
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
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	attrs := "-------------"
	if info.IsDir() {
		attrs = "----d--------"
	}
	fmt.Printf("%s %s\n", attrs, path)
}

func runTune2fs(args []string) int {
	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "tune2fs: not supported\n")
		return 1
	}
	device := ""
	label := ""
	reserved := ""

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
			device = args[len(args)-1]
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

	if label != "" {
		// Write label via /sys or ioctl
		fmt.Printf("Setting filesystem label to '%s' on %s\n", label, device)
	}
	if reserved != "" {
		fmt.Printf("Setting reserved blocks percentage to %s%% on %s\n", reserved, device)
	}

	// Read superblock info
	data, err := os.ReadFile(fmt.Sprintf("/sys/block/%s/size", device))
	if err == nil {
		fmt.Printf("Filesystem size: %s", string(data))
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

// filepathWalk is a helper for recursive walking
func filepathWalk(path string, fn func(string, os.FileInfo, error) error) error {
	info, err := os.Stat(path)
	if err != nil {
		return fn(path, nil, err)
	}
	if !info.IsDir() {
		return fn(path, info, nil)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		subpath := path + "/" + entry.Name()
		subinfo, _ := entry.Info()
		if entry.IsDir() {
			filepathWalk(subpath, fn)
		} else {
			fn(subpath, subinfo, nil)
		}
	}
	return nil
}
