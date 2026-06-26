package printutils

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/agentbusybox/pkg/applet"
)

func init() {
	applet.Register(&applet.Applet{Name: "lpd", Short: "Line printer daemon", Func: runLpd})
	applet.Register(&applet.Applet{Name: "lpq", Short: "Show print queue", Func: runLpq})
	applet.Register(&applet.Applet{Name: "lpr", Short: "Send files to print queue", Func: runLpr})
}

func parsePrinter(args []string) (queue, server, user string, files []string) {
	queue = os.Getenv("PRINTER")
	if queue == "" {
		queue = "lp"
	}
	user = os.Getenv("USER")
	if user == "" {
		user = "agentbusybox"
	}
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-P":
			if i+1 < len(args) {
				i++
				queue = args[i]
			}
		case "-U":
			if i+1 < len(args) {
				i++
				user = args[i]
			}
		default:
			if strings.HasPrefix(args[i], "-P") && len(args[i]) > 2 {
				queue = args[i][2:]
			} else if strings.HasPrefix(args[i], "-U") && len(args[i]) > 2 {
				user = args[i][2:]
			} else if !strings.HasPrefix(args[i], "-") {
				files = append(files, args[i])
			}
		}
	}
	server = "localhost:515"
	if at := strings.Index(queue, "@"); at >= 0 {
		server = queue[at+1:]
		queue = queue[:at]
	}
	if !strings.Contains(server, ":") {
		server += ":515"
	}
	return queue, server, user, files
}

func runLpq(args []string) int {
	queue, server, _, _ := parsePrinter(args)
	conn, err := net.DialTimeout("tcp", server, 10*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "lpq: %v\n", err)
		return 1
	}
	defer conn.Close()
	fmt.Fprintf(conn, "\x04%s\n", queue)
	io.Copy(os.Stdout, conn)
	return 0
}

func runLpr(args []string) int {
	queue, server, user, files := parsePrinter(args)
	if len(files) == 0 {
		files = []string{"-"}
	}
	conn, err := net.DialTimeout("tcp", server, 10*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "lpr: %v\n", err)
		return 1
	}
	defer conn.Close()
	reader := bufio.NewReader(conn)
	fmt.Fprintf(conn, "\x02%s\n", queue)
	if ack, _ := reader.ReadByte(); ack != 0 {
		fmt.Fprintf(os.Stderr, "lpr: queue rejected\n")
		return 1
	}
	host, _ := os.Hostname()
	if host == "" {
		host = "localhost"
	}
	job := os.Getpid() % 1000
	for _, name := range files {
		var data []byte
		var err error
		title := name
		if name == "-" {
			data, err = io.ReadAll(os.Stdin)
			title = "stdin"
		} else {
			data, err = os.ReadFile(name)
			title = filepath.Base(name)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "lpr: %s: %v\n", name, err)
			return 1
		}
		df := fmt.Sprintf("dfA%03d%s", job, host)
		cf := fmt.Sprintf("cfA%03d%s", job, host)
		control := fmt.Sprintf("H%s\nP%s\nJ%s\nld%s\n", host, user, title, df)
		if !lprSendFile(conn, reader, 2, cf, []byte(control)) {
			return 1
		}
		if !lprSendFile(conn, reader, 3, df, data) {
			return 1
		}
	}
	return 0
}

func lprSendFile(conn net.Conn, r *bufio.Reader, kind byte, name string, data []byte) bool {
	fmt.Fprintf(conn, "%c%d %s\n", kind, len(data), name)
	if ack, _ := r.ReadByte(); ack != 0 {
		fmt.Fprintf(os.Stderr, "lpr: file rejected\n")
		return false
	}
	conn.Write(data)
	conn.Write([]byte{0})
	if ack, _ := r.ReadByte(); ack != 0 {
		fmt.Fprintf(os.Stderr, "lpr: transfer failed\n")
		return false
	}
	return true
}

func runLpd(args []string) int {
	spool := "."
	if len(args) > 1 && !strings.HasPrefix(args[1], "-") {
		spool = args[1]
	}
	if err := os.MkdirAll(spool, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "lpd: %v\n", err)
		return 1
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "lpd: %v\n", err)
		return 1
	}
	name := filepath.Join(spool, fmt.Sprintf("job-%d", time.Now().UnixNano()))
	if err := os.WriteFile(name, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "lpd: %v\n", err)
		return 1
	}
	return 0
}
