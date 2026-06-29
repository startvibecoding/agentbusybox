package applet

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Applet represents a command that can be invoked via the busybox binary.
type Applet struct {
	Name   string
	Short  string
	Func   func(args []string) int
	NoFork bool // if true, run in-process (not in a subprocess)
	Usage    string // optional custom usage line, e.g. "ls [OPTIONS] [FILE...]"
	LongHelp string // detailed help text with option descriptions
}

var (
	registry = make(map[string]*Applet)
	order    []string // preserve insertion order
)

// Register adds an applet to the global registry.
func Register(a *Applet) {
	name := strings.ToLower(a.Name)
	if _, dup := registry[name]; dup {
		panic(fmt.Sprintf("duplicate applet: %s", name))
	}
	registry[name] = a
	order = append(order, name)
}

// Get returns the applet with the given name, or nil if not found.
func Get(name string) *Applet {
	return registry[strings.ToLower(name)]
}

// Names returns all registered applet names sorted alphabetically.
func Names() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// List prints all registered applets with their descriptions.
func List() {
	names := Names()
	maxLen := 0
	for _, n := range names {
		if len(n) > maxLen {
			maxLen = len(n)
		}
	}
	for _, n := range names {
		a := registry[n]
		fmt.Printf("  %-*s  %s\n", maxLen, a.Name, a.Short)
	}
}

// Dispatch determines the applet name from os.Args[0] or the first argument
// and runs it. Returns the exit code.
func Dispatch() int {
	progName := filepath.Base(os.Args[0])

	// Strip known extensions
	for _, ext := range []string{".exe", ".com"} {
		if strings.HasSuffix(strings.ToLower(progName), ext) {
			progName = progName[:len(progName)-len(ext)]
			break
		}
	}

	// Check if invoked as "busybox <applet> [args...]"
	lowerName := strings.ToLower(progName)
	if lowerName == "busybox" || lowerName == "agentbusybox" {
		return dispatchBusybox(os.Args[1:])
	}

	// Check if invoked directly as an applet (via symlink/copy)
	if a := Get(lowerName); a != nil {
		if containsHelp(os.Args) {
			PrintHelp(progName, a.Usage, a.Short, a.LongHelp)
			return 0
		}
		return a.Func(os.Args)
	}

	// Unknown command
	fmt.Fprintf(os.Stderr, "%s: applet not found\n", progName)
	return 1
}

// dispatchBusybox handles "busybox [applet] [args...]" invocation.
func dispatchBusybox(args []string) int {
	if len(args) == 0 {
		// No applet specified, show usage
		printBanner()
		fmt.Fprintf(os.Stderr, "\nUsage: busybox [applet] [args...]\n")
		fmt.Fprintf(os.Stderr, "\nCurrently defined applets:\n")
		List()
		return 0
	}

	appletName := strings.ToLower(args[0])

	// Handle special flags
	switch appletName {
	case "--list", "-l":
		List()
		return 0
	case "--help", "-h":
		printBanner()
		fmt.Fprintf(os.Stderr, "\nUsage: busybox [applet] [args...]\n")
		fmt.Fprintf(os.Stderr, "       busybox --list\n")
		fmt.Fprintf(os.Stderr, "\nCurrently defined applets:\n")
		List()
		return 0
	case "--version", "-v":
		printVersion()
		return 0
	}

	a := Get(appletName)
	if a == nil {
		fmt.Fprintf(os.Stderr, "busybox: unknown applet '%s'\n", appletName)
		return 1
	}

	// Handle --help for the target applet
	if containsHelp(args) {
		PrintHelp(appletName, a.Usage, a.Short, a.LongHelp)
		return 0
	}

	return a.Func(args)
}

func printBanner() {
	fmt.Fprintf(os.Stderr, "AgentBusyBox - A BusyBox implementation in Go\n")
}

func printVersion() {
	fmt.Println("AgentBusyBox v0.1.0 (Go implementation)")
}

// containsHelp checks if --help appears in args before -- (end of options).
// args[0] is expected to be the applet name.
func containsHelp(args []string) bool {
	for i := 1; i < len(args); i++ {
		if args[i] == "--" {
			return false
		}
		if args[i] == "--help" {
			return true
		}
	}
	return false
}

// PrintHelp prints formatted help information for an applet.
// If usage is empty, a default usage line is generated from the applet name.
func PrintHelp(appletName, usage, short, longHelp string) {
	if usage == "" {
		usage = appletName + " [OPTIONS] [ARGS...]"
	}
	fmt.Printf("Usage: %s\n\n", usage)
	if short != "" {
		fmt.Printf("%s\n\n", short)
	}
	if longHelp != "" {
		fmt.Printf("Options:\n%s\n", longHelp)
	}
}
