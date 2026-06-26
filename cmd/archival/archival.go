package archival

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/agentbusybox/pkg/applet"
)

func init() {
	applet.Register(&applet.Applet{Name: "tar", Short: "Archive files", Func: runTar})
}

func runTar(args []string) int {
	create, extract, list, verbose, gzip_ := false, false, false, false, false
	file := ""
	files := []string{}

	i := 1
	for ; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			i++
			break
		}
		if strings.HasPrefix(a, "-f") {
			if len(a) > 2 {
				file = a[2:]
			} else if i+1 < len(args) {
				i++
				file = args[i]
			}
			continue
		}
		if strings.HasPrefix(a, "-") {
			for _, ch := range a[1:] {
				switch ch {
				case 'c':
					create = true
				case 'x':
					extract = true
				case 't':
					list = true
				case 'v':
					verbose = true
				case 'z':
					gzip_ = true
				}
			}
			continue
		}
		files = append(files, a)
	}
	files = append(files, args[i:]...)

	if create && file != "" {
		return tarCreate(file, files, verbose, gzip_)
	}
	if extract && file != "" {
		return tarExtract(file, verbose)
	}
	if list && file != "" {
		return tarList(file, verbose)
	}

	fmt.Fprintf(os.Stderr, "tar: missing operation\n")
	return 1
}

func tarCreate(file string, files []string, verbose, gz bool) int {
	var w io.Writer
	if file == "-" {
		w = os.Stdout
	} else {
		f, err := os.Create(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tar: %v\n", err)
			return 1
		}
		defer f.Close()
		w = f
	}

	if gz {
		gw := gzip.NewWriter(w)
		defer gw.Close()
		w = gw
	}

	tw := tar.NewWriter(w)
	defer tw.Close()

	for _, root := range files {
		filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}

			header, err := tar.FileInfoHeader(info, "")
			if err != nil {
				return nil
			}
			header.Name = path

			if err := tw.WriteHeader(header); err != nil {
				return nil
			}

			if !info.IsDir() {
				f, err := os.Open(path)
				if err != nil {
					return nil
				}
				io.Copy(tw, f)
				f.Close()
			}

			if verbose {
				fmt.Println(path)
			}
			return nil
		})
	}
	return 0
}

func tarExtract(file string, verbose bool) int {
	f, err := os.Open(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tar: %v\n", err)
		return 1
	}
	defer f.Close()

	var r io.Reader = f
	if strings.HasSuffix(file, ".gz") || strings.HasSuffix(file, ".tgz") {
		gr, err := gzip.NewReader(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tar: %v\n", err)
			return 1
		}
		defer gr.Close()
		r = gr
	}

	tr := tar.NewReader(r)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "tar: %v\n", err)
			return 1
		}

		if verbose {
			fmt.Println(header.Name)
		}

		dir := filepath.Dir(header.Name)
		if dir != "." {
			os.MkdirAll(dir, 0755)
		}

		if header.Typeflag == tar.TypeDir {
			os.MkdirAll(header.Name, os.FileMode(header.Mode))
			continue
		}

		outFile, err := os.OpenFile(header.Name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(header.Mode))
		if err != nil {
			fmt.Fprintf(os.Stderr, "tar: %s: %v\n", header.Name, err)
			continue
		}
		io.Copy(outFile, tr)
		outFile.Close()
	}
	return 0
}

func tarList(file string, verbose bool) int {
	f, err := os.Open(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tar: %v\n", err)
		return 1
	}
	defer f.Close()

	var r io.Reader = f
	if strings.HasSuffix(file, ".gz") || strings.HasSuffix(file, ".tgz") {
		gr, err := gzip.NewReader(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tar: %v\n", err)
			return 1
		}
		defer gr.Close()
		r = gr
	}

	tr := tar.NewReader(r)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 1
		}
		if verbose {
			fmt.Printf("%s %8d %s\n", os.FileMode(header.Mode), header.Size, header.Name)
		} else {
			fmt.Println(header.Name)
		}
	}
	return 0
}

func init() {
	applet.Register(&applet.Applet{Name: "gzip", Short: "Compress files", Func: runGzip})
	applet.Register(&applet.Applet{Name: "gunzip", Short: "Decompress files", Func: runGunzip})
	applet.Register(&applet.Applet{Name: "zcat", Short: "Decompress to stdout", Func: runZcat})
}

func runGzip(args []string) int {
	files := args[1:]
	exitCode := 0
	for _, fname := range files {
		inFile, err := os.Open(fname)
		if err != nil {
			fmt.Fprintf(os.Stderr, "gzip: %s: %v\n", fname, err)
			exitCode = 1
			continue
		}

		outName := fname + ".gz"
		outFile, err := os.Create(outName)
		if err != nil {
			inFile.Close()
			exitCode = 1
			continue
		}

		gw := gzip.NewWriter(outFile)
		io.Copy(gw, inFile)
		gw.Close()
		outFile.Close()
		inFile.Close()
		os.Remove(fname)
	}
	return exitCode
}

func runGunzip(args []string) int {
	files := args[1:]
	exitCode := 0
	for _, fname := range files {
		inFile, err := os.Open(fname)
		if err != nil {
			fmt.Fprintf(os.Stderr, "gunzip: %s: %v\n", fname, err)
			exitCode = 1
			continue
		}

		gr, err := gzip.NewReader(inFile)
		if err != nil {
			inFile.Close()
			fmt.Fprintf(os.Stderr, "gunzip: %s: %v\n", fname, err)
			exitCode = 1
			continue
		}

		outName := strings.TrimSuffix(fname, ".gz")
		outFile, err := os.Create(outName)
		if err != nil {
			gr.Close()
			inFile.Close()
			exitCode = 1
			continue
		}

		io.Copy(outFile, gr)
		gr.Close()
		outFile.Close()
		inFile.Close()
		os.Remove(fname)
	}
	return exitCode
}

func runZcat(args []string) int {
	for _, fname := range args[1:] {
		inFile, err := os.Open(fname)
		if err != nil {
			fmt.Fprintf(os.Stderr, "zcat: %s: %v\n", fname, err)
			return 1
		}
		gr, err := gzip.NewReader(inFile)
		if err != nil {
			inFile.Close()
			return 1
		}
		io.Copy(os.Stdout, gr)
		gr.Close()
		inFile.Close()
	}
	return 0
}

func init() {
	applet.Register(&applet.Applet{Name: "bunzip2", Short: "Decompress bzip2 files", Func: runBunzip2})
	applet.Register(&applet.Applet{Name: "bzcat", Short: "Decompress bzip2 to stdout", Func: runBzcat})
}

func runBunzip2(args []string) int {
	files := args[1:]
	exitCode := 0
	for _, fname := range files {
		inFile, err := os.Open(fname)
		if err != nil {
			fmt.Fprintf(os.Stderr, "bunzip2: %s: %v\n", fname, err)
			exitCode = 1
			continue
		}

		br := bzip2.NewReader(inFile)
		outName := strings.TrimSuffix(fname, ".bz2")
		outFile, err := os.Create(outName)
		if err != nil {
			inFile.Close()
			exitCode = 1
			continue
		}

		io.Copy(outFile, br)
		outFile.Close()
		inFile.Close()
		os.Remove(fname)
	}
	return exitCode
}

func runBzcat(args []string) int {
	for _, fname := range args[1:] {
		inFile, err := os.Open(fname)
		if err != nil {
			fmt.Fprintf(os.Stderr, "bzcat: %s: %v\n", fname, err)
			return 1
		}
		br := bzip2.NewReader(inFile)
		io.Copy(os.Stdout, br)
		inFile.Close()
	}
	return 0
}

func init() {
	applet.Register(&applet.Applet{Name: "unzip", Short: "Extract zip archives", Func: runUnzip})
	applet.Register(&applet.Applet{Name: "zip", Short: "Create zip archives", Func: runZip})
}

func runUnzip(args []string) int {
	files := args[1:]
	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "unzip: missing archive\n")
		return 1
	}

	archive := files[0]
	r, err := zip.OpenReader(archive)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unzip: %s: %v\n", archive, err)
		return 1
	}
	defer r.Close()

	for _, f := range r.File {
		fmt.Println(f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(f.Name, 0755)
			continue
		}
		os.MkdirAll(filepath.Dir(f.Name), 0755)
		outFile, err := os.Create(f.Name)
		if err != nil {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			continue
		}
		io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()
	}
	return 0
}

func runZip(args []string) int {
	// Simplified zip creation
	files := args[1:]
	if len(files) < 2 {
		fmt.Fprintf(os.Stderr, "zip: missing archive or files\n")
		return 1
	}

	archive := files[0]
	sources := files[1:]

	outFile, err := os.Create(archive)
	if err != nil {
		return 1
	}
	defer outFile.Close()

	w := zip.NewWriter(outFile)
	defer w.Close()

	for _, src := range sources {
		info, err := os.Stat(src)
		if err != nil {
			continue
		}
		header, _ := zip.FileInfoHeader(info)
		header.Name = src
		fw, err := w.CreateHeader(header)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			f, err := os.Open(src)
			if err != nil {
				continue
			}
			io.Copy(fw, f)
			f.Close()
		}
	}
	return 0
}
