package coreutils

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/agentbusybox/pkg/applet"
)

func init() {
	applet.Register(&applet.Applet{Name: "dd", Short: "Copy and convert a file", Func: runDd})
}

func runDd(args []string) int {
	ifile := "/dev/stdin"
	ofile := "/dev/stdout"
	bs := 512
	count := -1 // -1 = unlimited
	skip := 0
	seek := 0
	conv := ""
	iflag := ""
	oflag := ""
	status := "default" // default, noxfer, none

	for _, a := range args[1:] {
		if a == "--" {
			continue
		}
		parts := strings.SplitN(a, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, val := parts[0], parts[1]
		switch key {
		case "if":
			ifile = val
		case "of":
			ofile = val
		case "bs":
			fmt.Sscanf(val, "%d", &bs)
		case "count":
			fmt.Sscanf(val, "%d", &count)
		case "skip":
			fmt.Sscanf(val, "%d", &skip)
		case "seek":
			fmt.Sscanf(val, "%d", &seek)
		case "conv":
			conv = val
		case "iflag":
			iflag = val
		case "oflag":
			oflag = val
		case "status":
			status = val
		// Size suffixes
		case "cbs", "ibs", "obs":
			n := parseDdSize(val)
			if key == "cbs" { /* ignore */
			} else {
				bs = n
			}
		}
	}

	_ = conv
	_ = iflag
	_ = oflag

	var inf *os.File
	var err error
	if ifile == "/dev/stdin" {
		inf = os.Stdin
	} else {
		inf, err = os.Open(ifile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "dd: %s: %v\n", ifile, err)
			return 1
		}
		defer inf.Close()
	}

	// Skip input
	if skip > 0 {
		if _, err := inf.Seek(int64(skip*bs), io.SeekStart); err != nil {
			// If seek fails, read and discard
			buf := make([]byte, bs)
			for i := 0; i < skip; i++ {
				if _, err := inf.Read(buf); err != nil {
					break
				}
			}
		}
	}

	var outf *os.File
	if ofile == "/dev/stdout" {
		outf = os.Stdout
	} else {
		flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
		if strings.Contains(conv, "notrunc") {
			flags = os.O_WRONLY | os.O_CREATE
		}
		if strings.Contains(conv, "append") {
			flags = os.O_WRONLY | os.O_CREATE | os.O_APPEND
		}
		outf, err = os.OpenFile(ofile, flags, 0666)
		if err != nil {
			fmt.Fprintf(os.Stderr, "dd: %s: %v\n", ofile, err)
			return 1
		}
		defer outf.Close()
	}

	// Seek output
	if seek > 0 {
		if _, err := outf.Seek(int64(seek*bs), io.SeekStart); err != nil {
			buf := make([]byte, bs)
			for i := 0; i < seek; i++ {
				outf.Write(buf)
			}
		}
	}

	buf := make([]byte, bs)
	totalIn := 0
	totalOut := 0

	for i := 0; count < 0 || i < count; i++ {
		n, readErr := inf.Read(buf)
		if n > 0 {
			totalIn += n
			data := buf[:n]

			// Conv transformations
			if strings.Contains(conv, "ucase") {
				data = []byte(strings.ToUpper(string(data)))
			}
			if strings.Contains(conv, "lcase") {
				data = []byte(strings.ToLower(string(data)))
			}
			if strings.Contains(conv, "swab") {
				for j := 0; j+1 < len(data); j += 2 {
					data[j], data[j+1] = data[j+1], data[j]
				}
			}

			written, writeErr := outf.Write(data)
			if writeErr != nil {
				fmt.Fprintf(os.Stderr, "dd: write error: %v\n", writeErr)
				return 1
			}
			totalOut += written
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			fmt.Fprintf(os.Stderr, "dd: read error: %v\n", readErr)
			return 1
		}
	}

	if status != "none" && status != "noxfer" {
		fmt.Fprintf(os.Stderr, "%d+%d records in\n", totalIn/bs, totalIn%bs)
		fmt.Fprintf(os.Stderr, "%d+%d records out\n", totalOut/bs, totalOut%bs)
		fmt.Fprintf(os.Stderr, "%d bytes (%s) copied\n", totalIn, humanSize(totalIn))
	}

	return 0
}

func parseDdSize(s string) int {
	mult := 1
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return 0
	}
	last := s[len(s)-1]
	switch last {
	case 'c':
		mult = 1
		s = s[:len(s)-1]
	case 'w':
		mult = 2
		s = s[:len(s)-1]
	case 'b':
		mult = 512
		s = s[:len(s)-1]
	case 'K', 'k':
		mult = 1024
		s = s[:len(s)-1]
	case 'M', 'm':
		mult = 1024 * 1024
		s = s[:len(s)-1]
	case 'G', 'g':
		mult = 1024 * 1024 * 1024
		s = s[:len(s)-1]
	}
	var n int
	fmt.Sscanf(s, "%d", &n)
	return n * mult
}

func humanSize(n int) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	f := float64(n)
	for _, u := range units {
		if f < 1024 {
			return fmt.Sprintf("%.1f %s", f, u)
		}
		f /= 1024
	}
	return fmt.Sprintf("%.1f PB", f)
}
