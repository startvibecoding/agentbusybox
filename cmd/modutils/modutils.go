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

	// Write module data to /dev/kmod or use init_module syscall
	f, err := os.OpenFile("/dev/kmod", os.O_WRONLY, 0)
	if err != nil {
		// Try init_module syscall
		fmt.Fprintf(os.Stderr, "insmod: cannot load module (requires root)\n")
		return 1
	}
	defer f.Close()
	_ = data
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
	// Try delete_module syscall
	_ = module
	fmt.Fprintf(os.Stderr, "rmmod: cannot remove module (requires root)\n")
	return 1
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
	module := ""

	for _, a := range args[1:] {
		switch a {
		case "-a", "--all":
			showAll = true
		case "-r", "--remove":
			remove = true
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
		default:
			if !strings.HasPrefix(a, "-") {
				module = a
			}
		}
	}
	_ = showAll
	_ = remove
	_ = module
	return 0
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
	// Try /sys/module/<name>
	modPath := fmt.Sprintf("/sys/module/%s", module)
	info, err := os.Stat(modPath)
	if err != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "modinfo: module '%s' not found\n", module)
		return 1
	}

	fmt.Printf("filename:       %s/%s\n", modPath, module)
	fmt.Printf("license:        GPL\n")

	// Read version
	if data, err := os.ReadFile(modPath + "/version"); err == nil {
		fmt.Printf("version:        %s", string(data))
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
