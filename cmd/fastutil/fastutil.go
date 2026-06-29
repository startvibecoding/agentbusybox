package fastutil

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"unicode"

	gofd "github.com/startvibecoding/go-fd"
	goriggrep "github.com/startvibecoding/go-ripgrep"
	"github.com/startvibecoding/go-ripgrep/pkg/printer"

	"github.com/agentbusybox/pkg/applet"
)

// ---------- fd ----------
func init() {
	applet.Register(&applet.Applet{Name: "fd", Short: "A simple, fast and user-friendly alternative to find", Func: runFd})
}

func runFd(args []string) int {
	opts, baseDir, err := parseFdArgs(args)
	if err != nil {
		if err == errFdHelp {
			printFdHelp()
			return 0
		}
		if err == errFdVersion {
			fmt.Println("fd 10.4.2-go")
			return 0
		}
		fmt.Fprintf(os.Stderr, "[fd error]: %v\n", err)
		return 1
	}

	if baseDir != "" {
		info, statErr := os.Stat(baseDir)
		if statErr != nil || !info.IsDir() {
			fmt.Fprintf(os.Stderr, "[fd error]: The '--base-directory' path '%s' is not a directory.\n", baseDir)
			return 1
		}
		if chErr := os.Chdir(baseDir); chErr != nil {
			fmt.Fprintf(os.Stderr, "[fd error]: Could not set '%s' as the current working directory\n", baseDir)
			return 1
		}
	}

	if !opts.FullPath && !opts.Glob && strings.Contains(opts.Pattern, "/") {
		fmt.Fprintf(os.Stderr, "[fd error]: The search pattern '%s' contains a path-separation character and will not lead to any search results.\n\n"+
			"If you want to search for all files inside the '%s' directory, use a match-all pattern:\n\n  fd . '%s'\n\n"+
			"Instead, if you want your pattern to match the full file path, use:\n\n  fd --full-path '%s'\n",
			opts.Pattern, opts.Pattern, opts.Pattern, opts.Pattern)
		return 1
	}

	paths, invalidPaths, err := gofd.ValidateSearchPaths(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[fd error]: %v\n", err)
		return 1
	}
	for _, p := range invalidPaths {
		fmt.Fprintf(os.Stderr, "[fd error]: Search path '%s' is not a directory.\n", p)
	}
	if len(paths) == 0 {
		fmt.Fprintf(os.Stderr, "[fd error]: No valid search paths given.\n")
		return 1
	}

	f, _, err := gofd.Compile(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[fd error]: %v\n", err)
		return 1
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	return int(f.Run(ctx, paths))
}

var (
	errFdHelp    = fmt.Errorf("help requested")
	errFdVersion = fmt.Errorf("version requested")
)

func parseFdArgs(args []string) (gofd.Options, string, error) {
	var opts gofd.Options
	opts.Color = "auto"
	opts.Hyperlink = "never"

	var positionals []string
	baseDir := ""
	unrestrictedCount := 0

	i := 1
	for i < len(args) {
		arg := args[i]
		i++

		if arg == "--" {
			positionals = append(positionals, args[i:]...)
			break
		}

		if strings.HasPrefix(arg, "--") {
			name := arg[2:]
			value := ""
			hasValue := false
			if eq := strings.IndexByte(name, '='); eq >= 0 {
				value = name[eq+1:]
				name = name[:eq]
				hasValue = true
			}
			next := func() (string, error) {
				if hasValue {
					return value, nil
				}
				if i >= len(args) {
					return "", fmt.Errorf("option '--%s' requires a value", name)
				}
				v := args[i]
				i++
				return v, nil
			}

			switch name {
			case "help":
				return opts, "", errFdHelp
			case "version":
				return opts, "", errFdVersion
			case "hidden":
				opts.Hidden = true
			case "no-hidden":
				opts.Hidden = false
			case "no-ignore":
				opts.NoIgnore = true
			case "ignore":
				opts.NoIgnore = false
			case "no-ignore-vcs":
				opts.NoIgnoreVcs = true
			case "no-require-git":
				opts.NoRequireGit = true
			case "require-git":
				opts.NoRequireGit = false
			case "no-ignore-parent":
				opts.NoIgnoreParent = true
			case "no-global-ignore-file":
				opts.NoGlobalIgnore = true
			case "unrestricted":
				unrestrictedCount++
			case "case-sensitive":
				opts.CaseSensitive = true
			case "ignore-case":
				opts.IgnoreCase = true
			case "glob":
				opts.Glob = true
			case "regex":
				opts.Glob = false
			case "fixed-strings", "literal":
				opts.FixedStrings = true
			case "exact":
				opts.Exact = true
			case "and":
				v, err := next()
				if err != nil {
					return opts, "", err
				}
				opts.Exprs = append(opts.Exprs, v)
			case "absolute-path":
				opts.AbsolutePath = true
			case "relative-path":
				opts.AbsolutePath = false
			case "list-details":
				opts.ListDetails = true
			case "follow", "dereference":
				opts.FollowLinks = true
			case "no-follow":
				opts.FollowLinks = false
			case "full-path":
				opts.FullPath = true
			case "print0":
				opts.NullSeparator = true
			case "max-depth", "maxdepth":
				v, err := next()
				if err != nil {
					return opts, "", err
				}
				opts.MaxDepth, err = fdAtoiPositive(v, "max-depth")
				if err != nil {
					return opts, "", err
				}
			case "min-depth", "mindepth":
				v, err := next()
				if err != nil {
					return opts, "", err
				}
				opts.MinDepth, err = fdAtoiPositive(v, "min-depth")
				if err != nil {
					return opts, "", err
				}
			case "exact-depth":
				v, err := next()
				if err != nil {
					return opts, "", err
				}
				opts.ExactDepth, err = fdAtoiPositive(v, "exact-depth")
				if err != nil {
					return opts, "", err
				}
			case "exclude":
				v, err := next()
				if err != nil {
					return opts, "", err
				}
				opts.Exclude = append(opts.Exclude, v)
			case "prune":
				opts.Prune = true
			case "type":
				v, err := next()
				if err != nil {
					return opts, "", err
				}
				opts.Types = append(opts.Types, fdNormalizeType(v))
			case "extension":
				v, err := next()
				if err != nil {
					return opts, "", err
				}
				opts.Extensions = append(opts.Extensions, v)
			case "size":
				v, err := next()
				if err != nil {
					return opts, "", err
				}
				opts.Sizes = append(opts.Sizes, v)
			case "changed-within", "change-newer-than", "newer", "changed-after":
				v, err := next()
				if err != nil {
					return opts, "", err
				}
				opts.ChangedWithin = v
			case "changed-before", "change-older-than", "older":
				v, err := next()
				if err != nil {
					return opts, "", err
				}
				opts.ChangedBefore = v
			case "owner":
				v, err := next()
				if err != nil {
					return opts, "", err
				}
				opts.Owner = v
			case "format":
				v, err := next()
				if err != nil {
					return opts, "", err
				}
				opts.Format = v
			case "batch-size":
				v, err := next()
				if err != nil {
					return opts, "", err
				}
				opts.BatchSize, err = fdAtoiPositive(v, "batch-size")
				if err != nil {
					return opts, "", err
				}
			case "ignore-file":
				v, err := next()
				if err != nil {
					return opts, "", err
				}
				opts.IgnoreFiles = append(opts.IgnoreFiles, v)
			case "color":
				v, err := next()
				if err != nil {
					return opts, "", err
				}
				opts.Color = v
			case "hyperlink", "hyper":
				if hasValue {
					opts.Hyperlink = value
				} else {
					opts.Hyperlink = "auto"
				}
			case "ignore-contain":
				v, err := next()
				if err != nil {
					return opts, "", err
				}
				opts.IgnoreContain = append(opts.IgnoreContain, v)
			case "threads":
				v, err := next()
				if err != nil {
					return opts, "", err
				}
				opts.Threads, err = fdAtoiPositive(v, "threads")
				if err != nil {
					return opts, "", err
				}
			case "max-results":
				v, err := next()
				if err != nil {
					return opts, "", err
				}
				opts.MaxResults, err = fdAtoiPositive(v, "max-results")
				if err != nil {
					return opts, "", err
				}
			case "base-directory":
				v, err := next()
				if err != nil {
					return opts, "", err
				}
				baseDir = v
			case "path-separator":
				v, err := next()
				if err != nil {
					return opts, "", err
				}
				opts.PathSeparator = v
			case "search-path":
				v, err := next()
				if err != nil {
					return opts, "", err
				}
				opts.Paths = append(opts.Paths, v)
			case "strip-cwd-prefix":
				when := "always"
				if hasValue {
					when = value
				}
				b := when != "never"
				opts.StripCwdPrefix = &b
			case "one-file-system", "mount", "xdev":
				opts.OneFileSystem = true
			case "show-errors":
				opts.ShowErrors = true
			case "has-results":
				opts.Quiet = true
			case "quiet":
				opts.Quiet = true
			case "exec":
				rest, consumed := fdCollectCommand(args[i:])
				opts.Exec = rest
				i += consumed
			case "exec-batch":
				rest, consumed := fdCollectCommand(args[i:])
				opts.ExecBatch = rest
				i += consumed
			default:
				return opts, "", fmt.Errorf("unexpected argument '--%s'", name)
			}
			continue
		}

		if len(arg) > 1 && arg[0] == '-' && arg != "-" {
			newI, err := fdParseShort(arg[1:], args, i, &opts, &positionals, &baseDir, &unrestrictedCount)
			if err != nil {
				if err == errFdHelp {
					return opts, "", errFdHelp
				}
				if err == errFdVersion {
					return opts, "", errFdVersion
				}
				return opts, "", err
			}
			i = newI
			continue
		}

		positionals = append(positionals, arg)
	}

	if unrestrictedCount > 0 {
		opts.Unrestricted = true
	}

	if len(positionals) > 0 {
		opts.Pattern = positionals[0]
		if len(opts.Paths) == 0 {
			opts.Paths = positionals[1:]
		} else {
			opts.Paths = append(opts.Paths, positionals[1:]...)
		}
	}

	return opts, baseDir, nil
}

func fdParseShort(cluster string, args []string, i int, opts *gofd.Options, positionals *[]string, baseDir *string, unrestricted *int) (int, error) {
	for idx := 0; idx < len(cluster); idx++ {
		c := cluster[idx]
		rest := cluster[idx+1:]
		value := func() (string, error) {
			if rest != "" {
				return rest, nil
			}
			if i >= len(args) {
				return "", fmt.Errorf("option '-%c' requires a value", c)
			}
			v := args[i]
			i++
			return v, nil
		}
		switch c {
		case 'h':
			return i, errFdHelp
		case 'V':
			return i, errFdVersion
		case 'H':
			opts.Hidden = true
		case 'I':
			opts.NoIgnore = true
		case 'u':
			*unrestricted++
		case 's':
			opts.CaseSensitive = true
		case 'i':
			opts.IgnoreCase = true
		case 'g':
			opts.Glob = true
		case 'F':
			opts.FixedStrings = true
		case 'a':
			opts.AbsolutePath = true
		case 'l':
			opts.ListDetails = true
		case 'L':
			opts.FollowLinks = true
		case 'p':
			opts.FullPath = true
		case '0':
			opts.NullSeparator = true
		case 'q':
			opts.Quiet = true
		case '1':
			opts.MaxResults = 1
		case 'd':
			v, err := value()
			if err != nil {
				return i, err
			}
			opts.MaxDepth, err = fdAtoiPositive(v, "max-depth")
			return i, err
		case 'E':
			v, err := value()
			if err != nil {
				return i, err
			}
			opts.Exclude = append(opts.Exclude, v)
			return i, nil
		case 't':
			v, err := value()
			if err != nil {
				return i, err
			}
			opts.Types = append(opts.Types, fdNormalizeType(v))
			return i, nil
		case 'e':
			v, err := value()
			if err != nil {
				return i, err
			}
			opts.Extensions = append(opts.Extensions, v)
			return i, nil
		case 'S':
			v, err := value()
			if err != nil {
				return i, err
			}
			opts.Sizes = append(opts.Sizes, v)
			return i, nil
		case 'o':
			v, err := value()
			if err != nil {
				return i, err
			}
			opts.Owner = v
			return i, nil
		case 'c':
			v, err := value()
			if err != nil {
				return i, err
			}
			opts.Color = v
			return i, nil
		case 'j':
			v, err := value()
			if err != nil {
				return i, err
			}
			opts.Threads, err = fdAtoiPositive(v, "threads")
			return i, err
		case 'C':
			v, err := value()
			if err != nil {
				return i, err
			}
			*baseDir = v
			return i, nil
		case 'x':
			rest, consumed := fdCollectCommand(args[i:])
			opts.Exec = rest
			return i + consumed, nil
		case 'X':
			rest, consumed := fdCollectCommand(args[i:])
			opts.ExecBatch = rest
			return i + consumed, nil
		default:
			return i, fmt.Errorf("unexpected argument '-%c'", c)
		}
	}
	return i, nil
}

func fdCollectCommand(rest []string) ([]string, int) {
	var cmd []string
	consumed := 0
	for _, a := range rest {
		consumed++
		if a == ";" {
			break
		}
		cmd = append(cmd, a)
	}
	return cmd, consumed
}

func fdNormalizeType(v string) string {
	switch v {
	case "f", "file":
		return "f"
	case "d", "dir", "directory":
		return "d"
	case "l", "symlink":
		return "l"
	case "x", "executable":
		return "x"
	case "e", "empty":
		return "e"
	case "s", "socket":
		return "s"
	case "p", "pipe":
		return "p"
	case "b", "block-device":
		return "b"
	case "c", "char-device":
		return "c"
	default:
		return v
	}
}

func fdAtoiPositive(s, name string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid value '%s' for '--%s'", s, name)
	}
	return n, nil
}

func printFdHelp() {
	fmt.Println(`Usage: fd [FLAGS/PATH] [pattern] [PATH]

Options:
  -H, --hidden              Search hidden files and directories
  -I, --no-ignore           Don't respect .gitignore files
      --no-ignore-vcs       Don't respect VCS ignore files
      --no-ignore-parent    Don't respect .gitignore in parent dirs
      --no-global-ignore-file  Don't respect global ignore file
  -u, --unrestricted        Alias for --no-ignore --hidden
  -s, --case-sensitive      Case-sensitive search
  -i, --ignore-case         Case-insensitive search
  -g, --glob                Treat pattern as glob (default: regex)
  -F, --fixed-strings       Treat pattern as literal string
      --exact               Match the whole name literally
  -a, --absolute-path       Show absolute instead of relative paths
  -l, --list-details        Show detailed file information
  -L, --follow              Follow symbolic links
  -p, --full-path           Search the full path
  -0, --print0              Separate search results with null bytes
  -q, --quiet               Only print errors
      --has-results         Return 0 if matches found, 1 otherwise
  -1, --max-one-result      Limit search to a single result
  -d, --max-depth N         Set maximum search depth
      --min-depth N         Set minimum search depth
      --exact-depth N       Only show results at exact depth
  -E, --exclude PATTERN     Exclude files/dirs matching the glob
      --prune               Don't traverse into matching dirs
  -t, --type TYPE           Filter by type (f,d,l,x,e,s,p,b,c)
  -e, --extension EXT       Filter by file extension
  -S, --size SIZE           Filter by file size (+1M, -500k, etc.)
      --changed-within DUR  Filter by modification time
      --changed-before DUR  Filter by modification time
  -o, --owner FILTER        Filter by owner (user/group)
      --format FORMAT       Use a format string
      --batch-size N        Set batch size for -X
      --ignore-file FILE    Add custom ignore file
      --ignore-contain PAT  Ignore files containing pattern
  -c, --color WHEN          Color output (auto/always/never)
      --hyperlink WHEN      Use terminal hyperlinks
      --threads N           Number of threads
      --max-results N       Limit to N results
      --base-directory DIR  Change base directory
      --path-separator SEP  Set path separator
      --strip-cwd-prefix    Strip current directory prefix
      --one-file-system     Don't cross file system boundaries
      --show-errors         Show filesystem errors
  -x, --exec CMD...         Execute command for each result
  -X, --exec-batch CMD...   Execute command with all results at once
  -h, --help                Print help
  -V, --version             Print version`)
}

// ---------- rg ----------
func init() {
	applet.Register(&applet.Applet{Name: "rg", Short: "A line-oriented search tool (ripgrep)", Func: runRg})
}

// rgOptions holds parsed ripgrep options
type rgOptions struct {
	Pattern          string
	FixedStrings     bool
	IgnoreCase       bool
	WordRegexp       bool
	InvertMatch      bool
	Replace          string
	HasReplace       bool
	NoIgnore         bool
	Hidden           bool
	FollowSymlinks   bool
	MaxDepth         int
	Globs            []string
	Types            []string
	TypesNot         []string
	BeforeContext    int
	AfterContext     int
	MaxCount         int
	Threads          int
	SortBy           string
	SortReverse      bool
	LineNumber       bool
	JSON             bool
	WithFilename     bool
	OnlyMatching     bool
	Count            bool
	Quiet            bool
	Color            bool
	Files            bool
	FilesWithMatches bool
}

var (
	errRgHelp    = fmt.Errorf("help requested")
	errRgVersion = fmt.Errorf("version requested")
)

func runRg(args []string) int {
	opts, paths, err := parseRgArgs(args)
	if err != nil {
		if err == errRgHelp {
			printRgHelp()
			return 0
		}
		if err == errRgVersion {
			fmt.Println("rg 14.1.1-go")
			return 0
		}
		fmt.Fprintf(os.Stderr, "[rg error]: %v\n", err)
		return 2
	}

	if opts.Pattern == "" && !opts.Files && !opts.FilesWithMatches {
		fmt.Fprintf(os.Stderr, "[rg error]: no pattern provided\n")
		return 2
	}

	if len(paths) == 0 {
		paths = []string{"."}
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Check if stdin is a pipe (no terminal)
	stat, _ := os.Stdin.Stat()
	stdinPipe := (stat.Mode() & os.ModeCharDevice) == 0

	if stdinPipe {
		// Read from stdin as file paths
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			paths = append(paths, scanner.Text())
		}
	}

	// Detect if we have multiple paths for filename display
	if opts.WithFilename {
		// forced on
	} else if len(paths) > 1 {
		opts.WithFilename = true
	}

	cfg := printer.Config{
		Group:        true,
		Color:        opts.Color,
		JSON:         opts.JSON,
		WithLineNum:  opts.LineNumber,
		WithFilename: opts.WithFilename,
		OnlyMatching: opts.OnlyMatching,
		Count:        opts.Count,
	}
	p := printer.NewPrinter(os.Stdout, cfg)

	if opts.Files || opts.FilesWithMatches {
		// Just list files that would be searched
		for _, path := range paths {
			fmt.Println(path)
		}
		return 0
	}

	ch, err := goriggrep.Search(ctx, paths, goriggrep.Options{
		Pattern:         opts.Pattern,
		IsFixed:         opts.FixedStrings,
		CaseInsensitive: opts.IgnoreCase,
		WordRegexp:      opts.WordRegexp,
		InvertMatch:     opts.InvertMatch,
		Replace:         opts.Replace,
		HasReplace:      opts.HasReplace,
		NoIgnore:        opts.NoIgnore,
		Hidden:          opts.Hidden,
		FollowSymlinks:  opts.FollowSymlinks,
		MaxDepth:        opts.MaxDepth,
		Globs:           opts.Globs,
		Types:           opts.Types,
		TypesNot:        opts.TypesNot,
		BeforeContext:   opts.BeforeContext,
		AfterContext:    opts.AfterContext,
		MaxCount:        opts.MaxCount,
		Threads:         opts.Threads,
		SortBy:          opts.SortBy,
		SortReverse:     opts.SortReverse,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "[rg error]: %v\n", err)
		return 2
	}

	matched := false
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for res := range ch {
			if len(res.Matches) > 0 {
				mu.Lock()
				matched = true
				mu.Unlock()
				if opts.Quiet {
					return
				}
				if err := p.PrintFileResult(res); err != nil {
					fmt.Fprintf(os.Stderr, "[rg error]: %v\n", err)
					return
				}
			}
		}
	}()

	wg.Wait()

	if opts.Quiet && matched {
		return 0
	}
	if matched {
		return 0
	}
	return 1
}

func parseRgArgs(args []string) (rgOptions, []string, error) {
	var opts rgOptions
	opts.Color = true // default color on
	var paths []string
	var positionals []string

	i := 1
	for i < len(args) {
		arg := args[i]
		i++

		if arg == "--" {
			paths = append(paths, args[i:]...)
			break
		}

		if strings.HasPrefix(arg, "--") {
			name := arg[2:]
			value := ""
			hasValue := false
			if eq := strings.IndexByte(name, '='); eq >= 0 {
				value = name[eq+1:]
				name = name[:eq]
				hasValue = true
			}
			next := func() (string, error) {
				if hasValue {
					return value, nil
				}
				if i >= len(args) {
					return "", fmt.Errorf("option '--%s' requires a value", name)
				}
				v := args[i]
				i++
				return v, nil
			}

			switch name {
			case "help":
				return rgOptions{}, nil, errRgHelp
			case "version":
				return rgOptions{}, nil, errRgVersion
			case "fixed-strings", "literal":
				opts.FixedStrings = true
			case "ignore-case", "icase":
				opts.IgnoreCase = true
			case "smart-case":
				// Default behavior, no-op
			case "case-sensitive", "sensitive":
				opts.IgnoreCase = false
			case "word-regexp":
				opts.WordRegexp = true
			case "invert-match":
				opts.InvertMatch = true
			case "replace":
				v, err := next()
				if err != nil {
					return rgOptions{}, nil, err
				}
				opts.Replace = v
				opts.HasReplace = true
			case "no-ignore":
				opts.NoIgnore = true
			case "ignore":
				opts.NoIgnore = false
			case "no-ignore-vcs":
				opts.NoIgnore = true
			case "hidden":
				opts.Hidden = true
			case "no-hidden":
				opts.Hidden = false
			case "follow":
				opts.FollowSymlinks = true
			case "no-follow":
				opts.FollowSymlinks = false
			case "max-depth", "maxdepth", "depth":
				v, err := next()
				if err != nil {
					return rgOptions{}, nil, err
				}
				opts.MaxDepth, err = strconv.Atoi(v)
				if err != nil {
					return rgOptions{}, nil, fmt.Errorf("invalid value for --max-depth: %v", err)
				}
			case "glob", "iglob":
				v, err := next()
				if err != nil {
					return rgOptions{}, nil, err
				}
				opts.Globs = append(opts.Globs, v)
			case "type":
				v, err := next()
				if err != nil {
					return rgOptions{}, nil, err
				}
				opts.Types = append(opts.Types, v)
			case "type-not":
				v, err := next()
				if err != nil {
					return rgOptions{}, nil, err
				}
				opts.TypesNot = append(opts.TypesNot, v)
			case "before-context":
				v, err := next()
				if err != nil {
					return rgOptions{}, nil, err
				}
				opts.BeforeContext, err = strconv.Atoi(v)
				if err != nil {
					return rgOptions{}, nil, fmt.Errorf("invalid value for --before-context: %v", err)
				}
			case "after-context":
				v, err := next()
				if err != nil {
					return rgOptions{}, nil, err
				}
				opts.AfterContext, err = strconv.Atoi(v)
				if err != nil {
					return rgOptions{}, nil, fmt.Errorf("invalid value for --after-context: %v", err)
				}
			case "context":
				v, err := next()
				if err != nil {
					return rgOptions{}, nil, err
				}
				n, err := strconv.Atoi(v)
				if err != nil {
					return rgOptions{}, nil, fmt.Errorf("invalid value for --context: %v", err)
				}
				opts.BeforeContext = n
				opts.AfterContext = n
			case "context-separator":
				// ignored
			case "max-count":
				v, err := next()
				if err != nil {
					return rgOptions{}, nil, err
				}
				opts.MaxCount, err = strconv.Atoi(v)
				if err != nil {
					return rgOptions{}, nil, fmt.Errorf("invalid value for --max-count: %v", err)
				}
			case "threads":
				v, err := next()
				if err != nil {
					return rgOptions{}, nil, err
				}
				opts.Threads, err = strconv.Atoi(v)
				if err != nil {
					return rgOptions{}, nil, fmt.Errorf("invalid value for --threads: %v", err)
				}
			case "sort":
				v, err := next()
				if err != nil {
					return rgOptions{}, nil, err
				}
				opts.SortBy = v
			case "sortr":
				opts.SortReverse = true
				v, err := next()
				if err != nil {
					return rgOptions{}, nil, err
				}
				opts.SortBy = v
			case "line-number", "numbers":
				opts.LineNumber = true
			case "no-line-number":
				opts.LineNumber = false
			case "json":
				opts.JSON = true
			case "with-filename":
				opts.WithFilename = true
			case "no-filename":
				opts.WithFilename = false
			case "only-matching":
				opts.OnlyMatching = true
			case "count":
				opts.Count = true
			case "count-matches":
				opts.Count = true
			case "quiet":
				opts.Quiet = true
			case "color":
				v, err := next()
				if err != nil {
					return rgOptions{}, nil, err
				}
				opts.Color = v != "never"
			case "no-color":
				opts.Color = false
			case "files":
				opts.Files = true
			case "files-with-matches":
				opts.FilesWithMatches = true
			default:
				return rgOptions{}, nil, fmt.Errorf("unexpected argument '--%s'", name)
			}
			continue
		}

		if len(arg) > 1 && arg[0] == '-' && arg != "-" {
			newI, err := rgParseShort(arg[1:], args, i, &opts, &positionals)
			if err != nil {
				if err == errRgHelp {
					return rgOptions{}, nil, errRgHelp
				}
				if err == errRgVersion {
					return rgOptions{}, nil, errRgVersion
				}
				return rgOptions{}, nil, err
			}
			i = newI
			continue
		}

		positionals = append(positionals, arg)
	}

	// First positional is pattern, rest are paths
	if len(positionals) > 0 {
		opts.Pattern = positionals[0]
		if len(paths) == 0 {
			paths = positionals[1:]
		} else {
			paths = append(paths, positionals[1:]...)
		}
	}

	return opts, paths, nil
}

func rgParseShort(cluster string, args []string, i int, opts *rgOptions, positionals *[]string) (int, error) {
	for idx := 0; idx < len(cluster); idx++ {
		c := cluster[idx]
		rest := cluster[idx+1:]
		value := func() (string, error) {
			if rest != "" {
				return rest, nil
			}
			if i >= len(args) {
				return "", fmt.Errorf("option '-%c' requires a value", c)
			}
			v := args[i]
			i++
			return v, nil
		}
		switch c {
		case 'h':
			return i, errRgHelp
		case 'V':
			return i, errRgVersion
		case 'f':
			opts.FixedStrings = true
		case 'i':
			opts.IgnoreCase = true
		case 's':
			opts.IgnoreCase = false
		case 'w':
			opts.WordRegexp = true
		case 'v':
			opts.InvertMatch = true
		case 'I':
			opts.NoIgnore = true
		case 'H':
			opts.WithFilename = true
		case 'n':
			opts.LineNumber = true
		case 'N':
			opts.LineNumber = false
		case 'l':
			opts.FilesWithMatches = true
		case 'c':
			opts.Count = true
		case 'o':
			opts.OnlyMatching = true
		case 'q':
			opts.Quiet = true
		case 'j':
			opts.JSON = true
		case 'S':
			opts.SortBy = "relevance"
		case 'g':
			v, err := value()
			if err != nil {
				return i, err
			}
			opts.Globs = append(opts.Globs, v)
			return i, nil
		case 't':
			v, err := value()
			if err != nil {
				return i, err
			}
			opts.Types = append(opts.Types, v)
			return i, nil
		case 'T':
			v, err := value()
			if err != nil {
				return i, err
			}
			opts.TypesNot = append(opts.TypesNot, v)
			return i, nil
		case 'C':
			v, err := value()
			if err != nil {
				return i, err
			}
			n, err := strconv.Atoi(v)
			if err != nil {
				return i, fmt.Errorf("invalid context value: %v", err)
			}
			opts.BeforeContext = n
			opts.AfterContext = n
			return i, nil
		case 'A':
			v, err := value()
			if err != nil {
				return i, err
			}
			opts.AfterContext, err = strconv.Atoi(v)
			return i, err
		case 'B':
			v, err := value()
			if err != nil {
				return i, err
			}
			opts.BeforeContext, err = strconv.Atoi(v)
			return i, err
		case 'm':
			v, err := value()
			if err != nil {
				return i, err
			}
			opts.MaxCount, err = strconv.Atoi(v)
			return i, err
		case 'r':
			v, err := value()
			if err != nil {
				return i, err
			}
			opts.Replace = v
			opts.HasReplace = true
			return i, nil
		case 'U':
			opts.MaxCount = 1
		case 'e':
			// -e pattern, consume next arg if pattern not set
			if opts.Pattern == "" && i < len(args) {
				opts.Pattern = args[i]
				i++
			}
			return i, nil
		case '-':
			// Treat remaining as paths
			*positionals = append(*positionals, args[i:]...)
			return len(args), nil
		case '0':
			// null separator
		case 'a':
			opts.LineNumber = true
		default:
			return i, fmt.Errorf("unexpected argument '-%c'", c)
		}
	}
	return i, nil
}

func printRgHelp() {
	fmt.Println(`Usage: rg [OPTIONS] PATTERN [PATH ...]

Options:
  -i, --ignore-case         Case insensitive search
  -s, --case-sensitive      Case sensitive search
  -w, --word-regexp         Only match whole words
  -F, --fixed-strings       Treat pattern as literal string
  -v, --invert-match        Invert matching
  -g, --glob GLOB           Include/exclude files matching glob
  -t, --type TYPE           Search only files matching type
  -T, --type-not TYPE       Don't search files matching type
  -I, --no-ignore           Don't respect ignore files
      --hidden              Search hidden files/directories
      --follow              Follow symlinks
  -n, --line-number         Show line numbers
  -N, --no-line-number      Suppress line numbers
  -c, --count               Show count of matches
  -l, --files-with-matches  Only show files with matches
  -o, --only-matching       Only show matching parts
  -C, --context NUM         Show NUM lines before and after
  -A, --after-context NUM   Show NUM lines after
  -B, --before-context NUM  Show NUM lines before
  -m, --max-count NUM       Limit matches per file
  -r, --replace REPLACEMENT Replace matches
  -j, --json                Output in JSON format
      --sort FIELD          Sort results (path/modified/size/none)
      --threads NUM         Number of threads
  -q, --quiet               Suppress output, just return code
  -h, --help                Print help
  -V, --version             Print version

Search Patterns:
  The first positional argument is treated as the search pattern.
  Subsequent positional arguments are treated as paths to search.
  If no paths are given, the current directory is searched.`)
}

// hasUppercase checks if string has any uppercase letters
func hasUppercase(s string) bool {
	for _, r := range s {
		if unicode.IsUpper(r) {
			return true
		}
	}
	return false
}
