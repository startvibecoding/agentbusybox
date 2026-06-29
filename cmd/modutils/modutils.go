package modutils

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"unsafe"

	"github.com/agentbusybox/pkg/applet"
	"golang.org/x/sys/unix"
)

func init() {
	applet.Register(&applet.Applet{Name: "insmod", Short: "Load a kernel module", Func: runInsmod})
	applet.Register(&applet.Applet{Name: "lsmod", Short: "List loaded kernel modules", Func: runLsmod})
	applet.Register(&applet.Applet{Name: "rmmod", Short: "Remove a kernel module", Func: runRmmod})
	applet.Register(&applet.Applet{Name: "modprobe", Short: "Load/unload kernel modules", Func: runModprobe})
	applet.Register(&applet.Applet{Name: "modinfo", Short: "Show module information", Func: runModinfo})
	applet.Register(&applet.Applet{Name: "depmod", Short: "Generate modules.dep", Func: runDepmod})
}

func runInsmod(args []string) int {
	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "insmod: not supported\n")
		return 1
	}
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "insmod: missing module\n")
		return 1
	}

	module := args[1]
	data, err := os.ReadFile(module)
	if err != nil {
		fmt.Fprintf(os.Stderr, "insmod: %s: %v\n", module, err)
		return 1
	}

	// Try init_module syscall (requires root)
	// On Linux, init_module takes the module data and length
	_, _, errno := syscall.RawSyscall(175, // __NR_init_module
		uintptr(unsafe.Pointer(&data[0])),
		uintptr(len(data)),
		0)
	if errno != 0 {
		fmt.Fprintf(os.Stderr, "insmod: %s: %v\n", module, errno)
		return 1
	}
	return 0
}

func runLsmod(args []string) int {
	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "lsmod: not supported\n")
		return 1
	}
	data, err := os.ReadFile("/proc/modules")
	if err != nil {
		fmt.Fprintf(os.Stderr, "lsmod: %v\n", err)
		return 1
	}

	fmt.Printf("%-30s %8s %s\n", "Module", "Size", "Used by")
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		parts := strings.Fields(scanner.Text())
		if len(parts) >= 3 {
			fmt.Printf("%-30s %8s %s\n", parts[0], parts[1], parts[2])
		}
	}
	return 0
}

func runRmmod(args []string) int {
	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "rmmod: not supported\n")
		return 1
	}
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "rmmod: missing module\n")
		return 1
	}

	module := args[1]
	// delete_module syscall (requires root)
	_, _, errno := syscall.RawSyscall(176, // __NR_delete_module
		uintptr(unsafe.Pointer(&[]byte(module + "\x00")[0])),
		0, 0)
	if errno != 0 {
		fmt.Fprintf(os.Stderr, "rmmod: %s: %v\n", module, errno)
		return 1
	}
	return 0
}

func runModprobe(args []string) int {
	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "modprobe: not supported\n")
		return 1
	}
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "modprobe: missing module\n")
		return 1
	}

	showAll := false
	remove := false
	quiet := false
	modules := []string{}

	for _, a := range args[1:] {
		switch a {
		case "-a", "--all":
			showAll = true
		case "-r", "--remove":
			remove = true
		case "-q", "--quiet":
			quiet = true
		case "-l": // list
			data, err := os.ReadFile("/proc/modules")
			if err != nil {
				return 1
			}
			for _, line := range strings.Split(string(data), "\n") {
				parts := strings.Fields(line)
				if len(parts) > 0 {
					fmt.Println(parts[0])
				}
			}
			return 0
		case "-v", "--verbose":
			// verbose mode
		default:
			if !strings.HasPrefix(a, "-") {
				modules = append(modules, a)
			}
		}
	}
	_ = showAll

	if len(modules) == 0 {
		fmt.Fprintf(os.Stderr, "modprobe: missing module\n")
		return 1
	}

	if remove {
		// Remove modules in reverse order
		for i := len(modules) - 1; i >= 0; i-- {
			modName := modules[i]
			_, _, errno := syscall.RawSyscall(176, // __NR_delete_module
				uintptr(unsafe.Pointer(&[]byte(modName + "\x00")[0])),
				0, 0)
			if errno != 0 && !quiet {
				fmt.Fprintf(os.Stderr, "modprobe: %s: %v\n", modName, errno)
			}
		}
		return 0
	}

	// Load modules - find and load each
	modRoot := "/lib/modules/" + kernelRelease()
	for _, modName := range modules {
		modPath := findModule(modRoot, modName)
		if modPath == "" {
			if !quiet {
				fmt.Fprintf(os.Stderr, "modprobe: %s: not found\n", modName)
			}
			continue
		}
		data, err := os.ReadFile(modPath)
		if err != nil {
			if !quiet {
				fmt.Fprintf(os.Stderr, "modprobe: %s: %v\n", modPath, err)
			}
			continue
		}
		_, _, errno := syscall.RawSyscall(175, // __NR_init_module
			uintptr(unsafe.Pointer(&data[0])),
			uintptr(len(data)),
			0)
		if errno != 0 && !quiet {
			fmt.Fprintf(os.Stderr, "modprobe: %s: %v\n", modName, errno)
		}
	}
	return 0
}

func findModule(modRoot, name string) string {
	modName := strings.ReplaceAll(name, "-", "_")
	var found string
	filepath.WalkDir(modRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		base := d.Name()
		base = strings.TrimSuffix(base, ".gz")
		base = strings.TrimSuffix(base, ".ko")
		if strings.ReplaceAll(base, "-", "_") == modName {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

func runModinfo(args []string) int {
	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "modinfo: not supported\n")
		return 1
	}
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "modinfo: missing module\n")
		return 1
	}

	module := args[1]
	modName := strings.ReplaceAll(module, "-", "_")

	// Check if loaded
	modPath := fmt.Sprintf("/sys/module/%s", modName)
	info, err := os.Stat(modPath)
	if err == nil && info.IsDir() {
		// Module is loaded, read info from sysfs
		fmt.Printf("filename:       %s\n", modPath)
		fmt.Printf("license:        GPL\n")

		if data, err := os.ReadFile(modPath + "/version"); err == nil {
			fmt.Printf("version:        %s", string(data))
		}
		if data, err := os.ReadFile(modPath + "/srcversion"); err == nil {
			fmt.Printf("srcversion:     %s", string(data))
		}
		if data, err := os.ReadFile(modPath + "/author"); err == nil {
			fmt.Printf("author:         %s", string(data))
		}
		if data, err := os.ReadFile(modPath + "/description"); err == nil {
			desc := strings.TrimSpace(string(data))
			if desc != "" {
				fmt.Printf("description:    %s\n", desc)
			}
		}
		return 0
	}

	// Module not loaded, search in /lib/modules
	modRoot := "/lib/modules/" + kernelRelease()
	modPath = findModule(modRoot, module)
	if modPath == "" {
		fmt.Fprintf(os.Stderr, "modinfo: %s: not found\n", module)
		return 1
	}

	data, err := readModuleFile(modPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "modinfo: %s: %v\n", modPath, err)
		return 1
	}

	fmt.Printf("filename:       %s\n", modPath)
	for _, token := range extractModuleTokens(data) {
		switch {
		case strings.HasPrefix(token, "license="):
			fmt.Printf("license:        %s\n", strings.TrimPrefix(token, "license="))
		case strings.HasPrefix(token, "author="):
			fmt.Printf("author:         %s\n", strings.TrimPrefix(token, "author="))
		case strings.HasPrefix(token, "description="):
			fmt.Printf("description:    %s\n", strings.TrimPrefix(token, "description="))
		case strings.HasPrefix(token, "vermagic="):
			fmt.Printf("vermagic:       %s\n", strings.TrimPrefix(token, "vermagic="))
		case strings.HasPrefix(token, "depends="):
			fmt.Printf("depends:        %s\n", strings.TrimPrefix(token, "depends="))
		case strings.HasPrefix(token, "alias="):
			fmt.Printf("alias:          %s\n", strings.TrimPrefix(token, "alias="))
		case strings.HasPrefix(token, "parm="):
			fmt.Printf("parm:           %s\n", strings.TrimPrefix(token, "parm="))
		case strings.HasPrefix(token, "parmtype="):
			fmt.Printf("parmtype:       %s\n", strings.TrimPrefix(token, "parmtype="))
		}
	}
	return 0
}

func runDepmod(args []string) int {
	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "depmod: not supported\n")
		return 1
	}
	baseDir := "/"
	dryRun := false
	version := ""
	explicitModules := []string{}
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-n":
			dryRun = true
		case "-b":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "depmod: option %s requires an argument\n", args[i])
				return 1
			}
			i++
			baseDir = args[i]
		case "-a", "-A", "-e", "-r", "-u", "-q":
		case "-F", "-C":
			if i+1 < len(args) {
				i++
			}
		default:
			if strings.HasSuffix(args[i], ".ko") || strings.HasSuffix(args[i], ".ko.gz") {
				explicitModules = append(explicitModules, args[i])
				continue
			}
			if version == "" && looksLikeKernelRelease(args[i]) {
				version = args[i]
				continue
			}
			explicitModules = append(explicitModules, args[i])
		}
	}
	if version == "" {
		version = kernelRelease()
	}
	modRoot := filepath.Join(baseDir, "lib", "modules", version)
	modules, err := scanModules(modRoot, explicitModules)
	if err != nil {
		fmt.Fprintf(os.Stderr, "depmod: %v\n", err)
		return 1
	}
	depData := buildModulesDep(modules)
	aliasData := buildModulesAlias(modules)
	if dryRun {
		os.Stdout.Write(depData)
		if len(aliasData) > 0 {
			os.Stdout.Write(aliasData)
		}
		return 0
	}
	if err := os.WriteFile(filepath.Join(modRoot, "modules.dep"), depData, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "depmod: %v\n", err)
		return 1
	}
	if len(aliasData) > 0 {
		if err := os.WriteFile(filepath.Join(modRoot, "modules.alias"), aliasData, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "depmod: %v\n", err)
			return 1
		}
	}
	return 0
}

type moduleInfo struct {
	Name    string
	ModName string
	Path    string
	Deps    []string
	Aliases []string
}

func kernelRelease() string {
	var uts unix.Utsname
	if err := unix.Uname(&uts); err != nil {
		return ""
	}
	var b strings.Builder
	for _, c := range uts.Release {
		if c == 0 {
			break
		}
		b.WriteByte(byte(c))
	}
	return b.String()
}

func looksLikeKernelRelease(s string) bool {
	dots := strings.Count(s, ".")
	return dots >= 2 && !strings.ContainsRune(s, os.PathSeparator)
}

func scanModules(modRoot string, explicit []string) ([]moduleInfo, error) {
	files := explicit
	if len(files) == 0 {
		err := filepath.WalkDir(modRoot, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if strings.HasSuffix(path, ".ko") || strings.HasSuffix(path, ".ko.gz") {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	modules := make([]moduleInfo, 0, len(files))
	for _, path := range files {
		info, err := readModuleInfo(modRoot, path)
		if err != nil {
			return nil, err
		}
		modules = append(modules, info)
	}
	sort.Slice(modules, func(i, j int) bool { return modules[i].Path < modules[j].Path })
	return modules, nil
}

func readModuleInfo(modRoot, path string) (moduleInfo, error) {
	data, err := readModuleFile(path)
	if err != nil {
		return moduleInfo{}, err
	}
	relPath := path
	if strings.HasPrefix(path, modRoot) {
		relPath = strings.TrimPrefix(path[len(modRoot):], string(os.PathSeparator))
	}
	name := filepath.Base(path)
	name = strings.TrimSuffix(name, ".gz")
	name = strings.TrimSuffix(name, ".ko")
	mod := moduleInfo{
		Name:    name,
		ModName: strings.ReplaceAll(name, "-", "_"),
		Path:    relPath,
	}
	for _, token := range extractModuleTokens(data) {
		switch {
		case strings.HasPrefix(token, "depends="):
			deps := strings.TrimPrefix(token, "depends=")
			if deps == "" {
				continue
			}
			for _, dep := range strings.Split(deps, ",") {
				dep = strings.TrimSpace(strings.ReplaceAll(dep, "-", "_"))
				if dep != "" {
					mod.Deps = append(mod.Deps, dep)
				}
			}
		case strings.HasPrefix(token, "alias="):
			alias := strings.TrimSpace(strings.TrimPrefix(token, "alias="))
			if alias != "" {
				mod.Aliases = append(mod.Aliases, alias)
			}
		}
	}
	return mod, nil
}

func readModuleFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var r io.Reader = f
	if strings.HasSuffix(path, ".gz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		r = gz
	}
	return io.ReadAll(r)
}

func extractModuleTokens(data []byte) []string {
	tokens := []string{}
	start := -1
	for i, b := range data {
		if b >= 32 && b <= 126 {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 && i-start >= 7 {
			tokens = append(tokens, string(data[start:i]))
		}
		start = -1
	}
	if start >= 0 && len(data)-start >= 7 {
		tokens = append(tokens, string(data[start:]))
	}
	return tokens
}

func buildModulesDep(modules []moduleInfo) []byte {
	nameToPath := make(map[string]string, len(modules))
	for _, mod := range modules {
		nameToPath[mod.ModName] = mod.Path
	}
	var b strings.Builder
	for _, mod := range modules {
		b.WriteString(mod.Path)
		b.WriteByte(':')
		for _, dep := range mod.Deps {
			if path, ok := nameToPath[dep]; ok {
				b.WriteByte(' ')
				b.WriteString(path)
			}
		}
		b.WriteByte('\n')
	}
	return []byte(b.String())
}

func buildModulesAlias(modules []moduleInfo) []byte {
	var b strings.Builder
	for _, mod := range modules {
		for _, alias := range mod.Aliases {
			b.WriteString("alias ")
			b.WriteString(alias)
			b.WriteByte(' ')
			b.WriteString(mod.ModName)
			b.WriteByte('\n')
		}
	}
	return []byte(b.String())
}
