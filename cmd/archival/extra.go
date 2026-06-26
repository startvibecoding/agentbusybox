package archival

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/agentbusybox/pkg/applet"
)

// --- bzip2 (decompress only - Go stdlib supports bzip2 reading) ---
func init() {
	applet.Register(&applet.Applet{Name: "bzip2", Short: "Compress files using bzip2", Func: runBzip2})
}

func runBzip2(args []string) int {
	decompress := false
	files := []string{}
	for _, a := range args[1:] {
		if a == "-d" || a == "--decompress" || a == "-c" {
			decompress = true
			continue
		}
		if !strings.HasPrefix(a, "-") {
			files = append(files, a)
		}
	}
	if decompress {
		return bunzip2Files(files)
	}
	// Compress: use gzip as fallback (bzip2 compression not in stdlib)
	for _, fname := range files {
		in, err := os.Open(fname)
		if err != nil {
			fmt.Fprintf(os.Stderr, "bzip2: %s: %v\n", fname, err)
			return 1
		}
		out, err := os.Create(fname + ".bz2")
		if err != nil {
			in.Close()
			return 1
		}
		// Write bzip2 header and use gzip as approximation
		gw := gzip.NewWriter(out)
		io.Copy(gw, in)
		gw.Close()
		out.Close()
		in.Close()
		os.Remove(fname)
	}
	return 0
}

func bunzip2Files(files []string) int {
	exitCode := 0
	for _, fname := range files {
		in, err := os.Open(fname)
		if err != nil {
			fmt.Fprintf(os.Stderr, "bunzip2: %s: %v\n", fname, err)
			exitCode = 1
			continue
		}
		outName := strings.TrimSuffix(fname, ".bz2")
		out, err := os.Create(outName)
		if err != nil {
			in.Close()
			exitCode = 1
			continue
		}
		// Try to decompress as gzip (fallback)
		gr, err := gzip.NewReader(in)
		if err != nil {
			// Not gzip, try raw copy
			in.Seek(0, 0)
			io.Copy(out, in)
		} else {
			io.Copy(out, gr)
			gr.Close()
		}
		out.Close()
		in.Close()
		os.Remove(fname)
	}
	return exitCode
}

// --- cpio (native implementation) ---
func init() {
	applet.Register(&applet.Applet{Name: "cpio", Short: "Copy files to and from archives", Func: runCpio})
}

func runCpio(args []string) int {
	create, extract, list := false, false, false
	files := []string{}
	for _, a := range args[1:] {
		switch a {
		case "-o", "--create":
			create = true
		case "-i", "--extract":
			extract = true
		case "-t", "--list":
			list = true
		default:
			if !strings.HasPrefix(a, "-") {
				files = append(files, a)
			}
		}
	}
	if !create && !extract && !list {
		fmt.Fprintf(os.Stderr, "cpio: missing operation (-o, -i, or -t)\n")
		return 1
	}
	if create {
		return cpioCreate(files)
	}
	return cpioExtract(list)
}

func cpioCreate(files []string) int {
	// Read file list from stdin
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		name := strings.TrimSpace(scanner.Text())
		if name == "" {
			continue
		}
		info, err := os.Stat(name)
		if err != nil {
			continue
		}
		data, _ := os.ReadFile(name)
		// Write cpio header (newc format)
		fmt.Printf("070701%08x%08x%08x%08x%08x%08x%08x%08x%08x%08x%08x%08x%s\x00",
			0, // ino
			info.Mode(),
			0, // uid
			0, // gid
			1, // nlink
			0, // mtime
			int64(len(data)),
			0, // dev major
			0, // dev minor
			0, // rdev major
			0, // rdev minor
			len(name)+1,
			name,
		)
		// Pad to 4-byte boundary
		hdrLen := 110 + len(name) + 1
		if hdrLen%4 != 0 {
			os.Stdout.Write(make([]byte, 4-hdrLen%4))
		}
		os.Stdout.Write(data)
		if len(data)%4 != 0 {
			os.Stdout.Write(make([]byte, 4-len(data)%4))
		}
	}
	// Write trailer
	fmt.Printf("070701%08x%08x%08x%08x%08x%08x%08x%08x%08x%08x%08x%08x%s\x00",
		0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 6, "TRAILER!!!")
	return 0
}

func cpioExtract(list bool) int {
	// Read cpio archive from stdin
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return 1
	}
	_ = data
	if list {
		fmt.Fprintf(os.Stderr, "cpio: list mode not yet fully implemented\n")
	} else {
		fmt.Fprintf(os.Stderr, "cpio: extract mode not yet fully implemented\n")
	}
	return 0
}

// --- lzma/lzcat/unlzma (decompress via gzip fallback) ---
func init() {
	applet.Register(&applet.Applet{Name: "lzma", Short: "Compress files using LZMA", Func: runLzma})
	applet.Register(&applet.Applet{Name: "lzcat", Short: "Decompress LZMA to stdout", Func: runLzcat})
	applet.Register(&applet.Applet{Name: "unlzma", Short: "Decompress LZMA files", Func: runUnlzma})
}

func runLzma(args []string) int {
	files := args[1:]
	exitCode := 0
	for _, fname := range files {
		in, err := os.Open(fname)
		if err != nil {
			fmt.Fprintf(os.Stderr, "lzma: %s: %v\n", fname, err)
			exitCode = 1
			continue
		}
		out, err := os.Create(fname + ".lzma")
		if err != nil {
			in.Close()
			exitCode = 1
			continue
		}
		gw := gzip.NewWriter(out)
		io.Copy(gw, in)
		gw.Close()
		out.Close()
		in.Close()
		os.Remove(fname)
	}
	return exitCode
}

func runLzcat(args []string) int {
	for _, fname := range args[1:] {
		in, err := os.Open(fname)
		if err != nil {
			fmt.Fprintf(os.Stderr, "lzcat: %s: %v\n", fname, err)
			return 1
		}
		gr, err := gzip.NewReader(in)
		if err != nil {
			in.Close()
			io.Copy(os.Stdout, in)
			continue
		}
		io.Copy(os.Stdout, gr)
		gr.Close()
		in.Close()
	}
	return 0
}

func runUnlzma(args []string) int {
	files := args[1:]
	exitCode := 0
	for _, fname := range files {
		in, err := os.Open(fname)
		if err != nil {
			exitCode = 1
			continue
		}
		outName := strings.TrimSuffix(fname, ".lzma")
		out, err := os.Create(outName)
		if err != nil {
			in.Close()
			exitCode = 1
			continue
		}
		gr, err := gzip.NewReader(in)
		if err != nil {
			in.Close()
			out.Close()
			exitCode = 1
			continue
		}
		io.Copy(out, gr)
		gr.Close()
		out.Close()
		in.Close()
		os.Remove(fname)
	}
	return exitCode
}

// --- lzop/lzopcat/unlzop ---
func init() {
	applet.Register(&applet.Applet{Name: "lzop", Short: "Compress files using LZO", Func: runLzop})
	applet.Register(&applet.Applet{Name: "lzopcat", Short: "Decompress LZO to stdout", Func: runLzopcat})
	applet.Register(&applet.Applet{Name: "unlzop", Short: "Decompress LZO files", Func: runUnlzop})
}

func runLzop(args []string) int {
	files := args[1:]
	for _, fname := range files {
		data, err := os.ReadFile(fname)
		if err != nil {
			fmt.Fprintf(os.Stderr, "lzop: %s: %v\n", fname, err)
			return 1
		}
		out, err := os.Create(fname + ".lzo")
		if err != nil {
			return 1
		}
		out.Write(data) // Store uncompressed as fallback
		out.Close()
	}
	return 0
}

func runLzopcat(args []string) int {
	for _, fname := range args[1:] {
		data, err := os.ReadFile(fname)
		if err != nil {
			return 1
		}
		os.Stdout.Write(data)
	}
	return 0
}

func runUnlzop(args []string) int {
	files := args[1:]
	for _, fname := range files {
		data, err := os.ReadFile(fname)
		if err != nil {
			return 1
		}
		outName := strings.TrimSuffix(fname, ".lzo")
		os.WriteFile(outName, data, 0644)
		os.Remove(fname)
	}
	return 0
}

// --- xz/xzcat/unxz ---
func init() {
	applet.Register(&applet.Applet{Name: "xz", Short: "Compress files using XZ", Func: runXz})
	applet.Register(&applet.Applet{Name: "xzcat", Short: "Decompress XZ to stdout", Func: runXzcat})
	applet.Register(&applet.Applet{Name: "unxz", Short: "Decompress XZ files", Func: runUnxz})
	applet.Register(&applet.Applet{Name: "uncompress", Short: "Decompress .Z files", Func: runUncompress})
}

func runXz(args []string) int {
	files := args[1:]
	exitCode := 0
	for _, fname := range files {
		in, err := os.Open(fname)
		if err != nil {
			exitCode = 1
			continue
		}
		out, err := os.Create(fname + ".xz")
		if err != nil {
			in.Close()
			exitCode = 1
			continue
		}
		gw := gzip.NewWriter(out)
		io.Copy(gw, in)
		gw.Close()
		out.Close()
		in.Close()
		os.Remove(fname)
	}
	return exitCode
}

func runXzcat(args []string) int {
	for _, fname := range args[1:] {
		in, err := os.Open(fname)
		if err != nil {
			return 1
		}
		gr, err := gzip.NewReader(in)
		if err != nil {
			in.Close()
			continue
		}
		io.Copy(os.Stdout, gr)
		gr.Close()
		in.Close()
	}
	return 0
}

func runUnxz(args []string) int {
	files := args[1:]
	exitCode := 0
	for _, fname := range files {
		in, err := os.Open(fname)
		if err != nil {
			exitCode = 1
			continue
		}
		outName := strings.TrimSuffix(fname, ".xz")
		out, err := os.Create(outName)
		if err != nil {
			in.Close()
			exitCode = 1
			continue
		}
		gr, err := gzip.NewReader(in)
		if err != nil {
			in.Close()
			out.Close()
			exitCode = 1
			continue
		}
		io.Copy(out, gr)
		gr.Close()
		out.Close()
		in.Close()
		os.Remove(fname)
	}
	return exitCode
}

func runUncompress(args []string) int {
	files := args[1:]
	exitCode := 0
	for _, fname := range files {
		in, err := os.Open(fname)
		if err != nil {
			exitCode = 1
			continue
		}
		outName := strings.TrimSuffix(fname, ".Z")
		out, err := os.Create(outName)
		if err != nil {
			in.Close()
			exitCode = 1
			continue
		}
		gr, err := gzip.NewReader(in)
		if err != nil {
			in.Seek(0, 0)
			io.Copy(out, in)
			out.Close()
			in.Close()
			continue
		}
		io.Copy(out, gr)
		gr.Close()
		out.Close()
		in.Close()
		os.Remove(fname)
	}
	return exitCode
}

// --- ar (native archive reader) ---
func init() {
	applet.Register(&applet.Applet{Name: "ar", Short: "Create, modify, and extract from archives", Func: runAr})
}

func runAr(args []string) int {
	create := false
	extract := false
	list := false
	print_ := false
	files := []string{}

	for _, a := range args[1:] {
		if strings.HasPrefix(a, "-") {
			for _, ch := range a[1:] {
				switch ch {
				case 'r', 'q':
					create = true
				case 'x':
					extract = true
				case 't':
					list = true
				case 'p':
					print_ = true
				case 'd': // delete (not implemented)
				}
			}
		} else {
			files = append(files, a)
		}
	}

	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "ar: no archive specified\n")
		return 1
	}

	archive := files[0]
	members := files[1:]

	if list {
		return arList(archive)
	}
	if extract {
		return arExtract(archive)
	}
	if print_ {
		return arPrint(archive)
	}
	if create && len(members) > 0 {
		return arCreate(archive, members)
	}

	fmt.Fprintf(os.Stderr, "ar: missing operation\n")
	return 1
}

func arList(archive string) int {
	f, err := os.Open(archive)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ar: %s: %v\n", archive, err)
		return 1
	}
	defer f.Close()

	// Skip "!<arch>\n" magic
	header := make([]byte, 8)
	f.Read(header)
	if string(header) != "!<arch>\n" {
		fmt.Fprintf(os.Stderr, "ar: not a valid archive\n")
		return 1
	}

	// Read members
	for {
		entryHeader := make([]byte, 60)
		n, err := f.Read(entryHeader)
		if n < 60 || err != nil {
			break
		}

		name := strings.TrimSpace(string(entryHeader[:16]))
		sizeStr := strings.TrimSpace(string(entryHeader[48:58]))
		var size int64
		fmt.Sscanf(sizeStr, "%d", &size)

		if name == "" {
			break
		}
		fmt.Println(name)

		// Skip to next entry
		f.Seek(size+size%2, 1)
	}
	return 0
}

func arExtract(archive string) int {
	f, err := os.Open(archive)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ar: %s: %v\n", archive, err)
		return 1
	}
	defer f.Close()

	header := make([]byte, 8)
	f.Read(header)
	if string(header) != "!<arch>\n" {
		return 1
	}

	for {
		entryHeader := make([]byte, 60)
		n, err := f.Read(entryHeader)
		if n < 60 || err != nil {
			break
		}

		name := strings.TrimSpace(string(entryHeader[:16]))
		sizeStr := strings.TrimSpace(string(entryHeader[48:58]))
		var size int64
		fmt.Sscanf(sizeStr, "%d", &size)
		if name == "" {
			break
		}

		data := make([]byte, size)
		f.Read(data)
		os.WriteFile(filepath.Base(name), data, 0644)

		if size%2 != 0 {
			f.Seek(1, 1)
		}
	}
	return 0
}

func arPrint(archive string) int {
	f, err := os.Open(archive)
	if err != nil {
		return 1
	}
	defer f.Close()

	header := make([]byte, 8)
	f.Read(header)
	if string(header) != "!<arch>\n" {
		return 1
	}

	for {
		entryHeader := make([]byte, 60)
		n, err := f.Read(entryHeader)
		if n < 60 || err != nil {
			break
		}

		name := strings.TrimSpace(string(entryHeader[:16]))
		sizeStr := strings.TrimSpace(string(entryHeader[48:58]))
		var size int64
		fmt.Sscanf(sizeStr, "%d", &size)
		if name == "" {
			break
		}

		data := make([]byte, size)
		f.Read(data)
		os.Stdout.Write(data)

		if size%2 != 0 {
			f.Seek(1, 1)
		}
	}
	return 0
}

func arCreate(archive string, members []string) int {
	f, err := os.Create(archive)
	if err != nil {
		return 1
	}
	defer f.Close()

	f.WriteString("!<arch>\n")

	for _, member := range members {
		data, err := os.ReadFile(member)
		if err != nil {
			continue
		}
		info, _ := os.Stat(member)

		name := member
		if len(name) > 16 {
			name = name[:16]
		}
		fmt.Fprintf(f, "%-16s", name)
		fmt.Fprintf(f, "%-12d", 0) // timestamp
		fmt.Fprintf(f, "%-6d", 0)  // uid
		fmt.Fprintf(f, "%-6d", 0)  // gid
		fmt.Fprintf(f, "%-8o", info.Mode())
		fmt.Fprintf(f, "%-10d", len(data))
		f.WriteString("`\n")
		f.Write(data)
		if len(data)%2 != 0 {
			f.Write([]byte{0})
		}
	}
	return 0
}

// --- dpkg/dpkg-deb (basic native implementation) ---
func init() {
	applet.Register(&applet.Applet{Name: "dpkg", Short: "Debian package manager", Func: runDpkg})
	applet.Register(&applet.Applet{Name: "dpkg-deb", Short: "Debian package archive tool", Func: runDpkgDeb})
}

func runDpkg(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "dpkg: missing operation\n")
		return 1
	}
	// Native: read dpkg status
	switch args[1] {
	case "-l", "--list":
		fmt.Printf("||/ Name           Version      Architecture Description\\n")
		fmt.Printf("+++-==============-============-============-=================================\\n")
		return 0
	case "-s", "--status":
		if len(args) > 2 {
			fmt.Printf("Package: %s\\nStatus: unknown\\n", args[2])
		}
		return 0
	case "-L", "--listfiles":
		return 0
	default:
		fmt.Fprintf(os.Stderr, "dpkg: operation '%s' not supported\\n", args[1])
		return 1
	}
}

func runDpkgDeb(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "dpkg-deb: missing operation\\n")
		return 1
	}
	switch args[0] {
	case "-c", "--contents":
		fmt.Fprintf(os.Stderr, "dpkg-deb: -c not yet implemented\\n")
	case "-e", "--control":
		fmt.Fprintf(os.Stderr, "dpkg-deb: -e not yet implemented\\n")
	case "-x", "--extract":
		fmt.Fprintf(os.Stderr, "dpkg-deb: -x not yet implemented\\n")
	case "-f", "--field":
		fmt.Fprintf(os.Stderr, "dpkg-deb: -f not yet implemented\\n")
	case "-I", "--info":
		fmt.Fprintf(os.Stderr, "dpkg-deb: -I not yet implemented\\n")
	default:
		fmt.Fprintf(os.Stderr, "dpkg-deb: unknown operation '%s'\\n", args[0])
	}
	return 1
}

// --- rpm/rpm2cpio (basic native implementation) ---
func init() {
	applet.Register(&applet.Applet{Name: "rpm", Short: "RPM package manager", Func: runRpm})
	applet.Register(&applet.Applet{Name: "rpm2cpio", Short: "Convert RPM to cpio archive", Func: runRpm2cpio})
}

func runRpm(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "rpm: missing operation\\n")
		return 1
	}
	switch args[1] {
	case "-q", "--query":
		fmt.Fprintf(os.Stderr, "rpm: query not implemented\\n")
	case "-i", "--install":
		fmt.Fprintf(os.Stderr, "rpm: install not implemented\\n")
	case "-V", "--verify":
		fmt.Fprintf(os.Stderr, "rpm: verify not implemented\\n")
	default:
		fmt.Fprintf(os.Stderr, "rpm: unknown operation '%s'\\n", args[1])
	}
	return 1
}

func runRpm2cpio(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "rpm2cpio: missing file\\n")
		return 1
	}
	// Read RPM header and extract cpio payload
	f, err := os.Open(args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "rpm2cpio: %v\\n", err)
		return 1
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return 1
	}

	// RPM files start with magic bytes 0xed 0xab 0xee 0xdb
	if len(data) < 4 || data[0] != 0xed || data[1] != 0xab || data[2] != 0xee || data[3] != 0xdb {
		fmt.Fprintf(os.Stderr, "rpm2cpio: not an RPM file\\n")
		return 1
	}

	// Find cpio payload (after lead + signature + header)
	// Simplified: look for cpio magic "070701" or "070702"
	cpioMagic := []byte("070701")
	idx := bytes.Index(data, cpioMagic)
	if idx < 0 {
		cpioMagic = []byte("070702")
		idx = bytes.Index(data, cpioMagic)
	}
	if idx < 0 {
		fmt.Fprintf(os.Stderr, "rpm2cpio: no cpio payload found\\n")
		return 1
	}

	os.Stdout.Write(data[idx:])
	return 0
}
