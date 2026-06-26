package mailutils

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/agentbusybox/pkg/applet"
)

func init() {
	applet.Register(&applet.Applet{Name: "makemime", Short: "Create MIME messages", Func: runMakemime})
	applet.Register(&applet.Applet{Name: "popmaildir", Short: "Retrieve mail into a maildir", Func: runPopmaildir})
	applet.Register(&applet.Applet{Name: "reformime", Short: "Parse MIME messages", Func: runReformime})
	applet.Register(&applet.Applet{Name: "sendmail", Short: "Send mail through SMTP", Func: runSendmail})
}

func runMakemime(args []string) int {
	files := []string{}
	for _, a := range args[1:] {
		if strings.HasPrefix(a, "-") {
			continue
		}
		files = append(files, a)
	}
	boundary := fmt.Sprintf("agentbusybox-%d", time.Now().UnixNano())
	fmt.Printf("MIME-Version: 1.0\r\n")
	if len(files) == 0 {
		fmt.Printf("Content-Type: text/plain; charset=utf-8\r\n\r\n")
		io.Copy(os.Stdout, os.Stdin)
		return 0
	}
	fmt.Printf("Content-Type: multipart/mixed; boundary=%q\r\n\r\n", boundary)
	fmt.Printf("--%s\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n", boundary)
	fmt.Printf("\r\n")
	for _, name := range files {
		data, err := os.ReadFile(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "makemime: %s: %v\n", name, err)
			return 1
		}
		ctype := mime.TypeByExtension(filepath.Ext(name))
		if ctype == "" {
			ctype = "application/octet-stream"
		}
		fmt.Printf("--%s\r\n", boundary)
		fmt.Printf("Content-Type: %s; name=%q\r\n", ctype, filepath.Base(name))
		fmt.Printf("Content-Transfer-Encoding: base64\r\n")
		fmt.Printf("Content-Disposition: attachment; filename=%q\r\n\r\n", filepath.Base(name))
		enc := base64.StdEncoding.EncodeToString(data)
		for len(enc) > 76 {
			fmt.Println(enc[:76])
			enc = enc[76:]
		}
		if enc != "" {
			fmt.Println(enc)
		}
	}
	fmt.Printf("--%s--\r\n", boundary)
	return 0
}

func runReformime(args []string) int {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "reformime: %v\n", err)
		return 1
	}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || line == "\r" {
			break
		}
		fmt.Println(line)
	}
	return 0
}

func runSendmail(args []string) int {
	server := os.Getenv("SMTPHOST")
	if server == "" {
		server = "127.0.0.1:25"
	}
	from := ""
	recipients := []string{}
	readRecipients := false
	for i := 1; i < len(args); i++ {
		switch {
		case args[i] == "-t":
			readRecipients = true
		case args[i] == "-f" && i+1 < len(args):
			i++
			from = args[i]
		case args[i] == "-S" && i+1 < len(args):
			i++
			server = args[i]
		case strings.HasPrefix(args[i], "-"):
		default:
			recipients = append(recipients, args[i])
		}
	}
	if !strings.Contains(server, ":") {
		server += ":25"
	}
	message, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sendmail: %v\n", err)
		return 1
	}
	if readRecipients {
		recipients = append(recipients, recipientsFromHeaders(string(message))...)
	}
	if from == "" {
		from = os.Getenv("USER")
		if from == "" {
			from = "agentbusybox"
		}
		if !strings.Contains(from, "@") {
			host, _ := os.Hostname()
			if host == "" {
				host = "localhost"
			}
			from += "@" + host
		}
	}
	if len(recipients) == 0 {
		fmt.Fprintf(os.Stderr, "sendmail: no recipients\n")
		return 1
	}
	conn, err := net.DialTimeout("tcp", server, 30*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sendmail: %v\n", err)
		return 1
	}
	defer conn.Close()
	r := bufio.NewReader(conn)
	if !smtpReadOK(r, 220) {
		return 1
	}
	host, _ := os.Hostname()
	if host == "" {
		host = "localhost"
	}
	if !smtpCmd(conn, r, 250, "HELO %s\r\n", host) {
		return 1
	}
	if !smtpCmd(conn, r, 250, "MAIL FROM:<%s>\r\n", from) {
		return 1
	}
	for _, rcpt := range recipients {
		if !smtpCmd(conn, r, 250, "RCPT TO:<%s>\r\n", rcpt) {
			return 1
		}
	}
	if !smtpCmd(conn, r, 354, "DATA\r\n") {
		return 1
	}
	conn.Write(message)
	if len(message) == 0 || message[len(message)-1] != '\n' {
		conn.Write([]byte("\r\n"))
	}
	conn.Write([]byte(".\r\n"))
	if !smtpReadOK(r, 250) {
		return 1
	}
	smtpCmd(conn, r, 221, "QUIT\r\n")
	return 0
}

func recipientsFromHeaders(msg string) []string {
	recipients := []string{}
	scanner := bufio.NewScanner(strings.NewReader(msg))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			break
		}
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "to:") || strings.HasPrefix(lower, "cc:") || strings.HasPrefix(lower, "bcc:") {
			parts := strings.SplitN(line, ":", 2)
			for _, addr := range strings.Split(parts[1], ",") {
				addr = strings.TrimSpace(addr)
				if i := strings.LastIndex(addr, "<"); i >= 0 {
					if j := strings.LastIndex(addr, ">"); j > i {
						addr = addr[i+1 : j]
					}
				}
				if addr != "" {
					recipients = append(recipients, addr)
				}
			}
		}
	}
	return recipients
}

func smtpCmd(conn net.Conn, r *bufio.Reader, want int, format string, args ...any) bool {
	fmt.Fprintf(conn, format, args...)
	return smtpReadOK(r, want)
}

func smtpReadOK(r *bufio.Reader, want int) bool {
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			fmt.Fprintf(os.Stderr, "sendmail: %v\n", err)
			return false
		}
		if len(line) < 3 {
			continue
		}
		var code int
		fmt.Sscanf(line[:3], "%d", &code)
		if len(line) > 3 && line[3] == '-' {
			continue
		}
		if code != want {
			fmt.Fprintf(os.Stderr, "sendmail: SMTP error: %s", line)
			return false
		}
		return true
	}
}

func runPopmaildir(args []string) int {
	fmt.Fprintf(os.Stderr, "popmaildir: not yet implemented in pure Go\n")
	return 1
}
