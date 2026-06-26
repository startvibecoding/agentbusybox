package networking

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/agentbusybox/pkg/applet"
)

func init() {
	applet.Register(&applet.Applet{Name: "wget", Short: "Download files from the web", Func: runWget})
}

func runWget(args []string) int {
	output := ""          // -O FILE
	quiet := false        // -q
	serverResp := false   // -S
	dirPrefix := "."      // -P DIR
	userAgent := ""       // -U STR
	timeout := 0          // -T SEC
	headers := []string{} // --header STR
	postData := ""        // --post-data STR
	spider := false       // --spider
	noCheckCert := false  // --no-check-certificate
	urls := []string{}

	for i := 1; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			i++
			urls = append(urls, args[i:]...)
			break
		}
		if strings.HasPrefix(a, "--") {
			switch a {
			case "--quiet":
				quiet = true
			case "--server-response":
				serverResp = true
			case "--spider":
				spider = true
			case "--no-check-certificate":
				noCheckCert = true
			default:
				if strings.HasPrefix(a, "--output-document=") {
					output = a[18:]
				}
				if strings.HasPrefix(a, "--directory-prefix=") {
					dirPrefix = a[19:]
				}
				if strings.HasPrefix(a, "--user-agent=") {
					userAgent = a[13:]
				}
				if strings.HasPrefix(a, "--timeout=") {
					fmt.Sscanf(a[10:], "%d", &timeout)
				}
				if strings.HasPrefix(a, "--header=") {
					headers = append(headers, a[9:])
				}
				if strings.HasPrefix(a, "--post-data=") {
					postData = a[12:]
				}
			}
			continue
		}
		if strings.HasPrefix(a, "-") && len(a) > 1 {
			for _, ch := range a[1:] {
				switch ch {
				case 'q':
					quiet = true
				case 'S':
					serverResp = true
				case 'c': // continue (accepted, ignored)
				case 'O':
					if i+1 < len(args) {
						i++
						output = args[i]
					} else if len(a) > 2 {
						output = a[2:]
					}
				case 'P':
					if i+1 < len(args) {
						i++
						dirPrefix = args[i]
					}
				case 'U':
					if i+1 < len(args) {
						i++
						userAgent = args[i]
					}
				case 'T':
					if i+1 < len(args) {
						i++
						fmt.Sscanf(args[i], "%d", &timeout)
					}
				}
			}
			continue
		}
		urls = append(urls, a)
	}

	if len(urls) == 0 {
		fmt.Fprintf(os.Stderr, "wget: missing URL\n")
		return 1
	}

	_ = serverResp
	_ = dirPrefix
	_ = headers
	_ = postData
	_ = spider
	_ = noCheckCert

	exitCode := 0
	for _, url := range urls {
		outName := output
		if outName == "" {
			parts := strings.Split(url, "/")
			outName = parts[len(parts)-1]
			if outName == "" {
				outName = "index.html"
			}
			outName = filepath.Join(dirPrefix, outName)
		}

		if !quiet {
			fmt.Fprintf(os.Stderr, "Connecting to %s...\n", url)
		}

		client := &http.Client{}
		if timeout > 0 {
			client.Timeout = time.Duration(timeout) * time.Second
		}

		var req *http.Request
		var err error
		if postData != "" {
			req, err = http.NewRequest("POST", url, strings.NewReader(postData))
		} else {
			req, err = http.NewRequest("GET", url, nil)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "wget: %v\n", err)
			exitCode = 1
			continue
		}

		for _, h := range headers {
			parts := strings.SplitN(h, ":", 2)
			if len(parts) == 2 {
				req.Header.Set(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
			}
		}
		if userAgent != "" {
			req.Header.Set("User-Agent", userAgent)
		}

		resp, err := client.Do(req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "wget: %v\n", err)
			exitCode = 1
			continue
		}
		defer resp.Body.Close()

		if spider {
			if resp.StatusCode >= 400 {
				exitCode = 1
			}
			if !quiet {
				fmt.Fprintf(os.Stderr, "%s %d\n", url, resp.StatusCode)
			}
			continue
		}

		if resp.StatusCode != http.StatusOK {
			fmt.Fprintf(os.Stderr, "wget: HTTP error %d\n", resp.StatusCode)
			exitCode = 1
			continue
		}

		var outFile *os.File
		if outName == "-" {
			outFile = os.NewFile(1, "/dev/stdout")
		} else {
			outFile, err = os.Create(outName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "wget: %v\n", err)
				exitCode = 1
				continue
			}
		}
		n, _ := io.Copy(outFile, resp.Body)
		if outName != "-" {
			outFile.Close()
		}

		if !quiet {
			fmt.Fprintf(os.Stderr, "'%s' saved [%d]\n", outName, n)
		}
	}
	return exitCode
}

func init() {
	applet.Register(&applet.Applet{Name: "curl", Short: "Transfer data from or to a server", Func: runCurl})
}

func runCurl(args []string) int {
	output := ""
	method := "GET"
	headers := []string{}
	url := ""

	for i := 1; i < len(args); i++ {
		a := args[i]
		if a == "-o" && i+1 < len(args) {
			i++
			output = args[i]
			continue
		}
		if a == "-X" && i+1 < len(args) {
			i++
			method = args[i]
			continue
		}
		if a == "-H" && i+1 < len(args) {
			i++
			headers = append(headers, args[i])
			continue
		}
		if !strings.HasPrefix(a, "-") {
			url = a
		}
	}

	if url == "" {
		fmt.Fprintf(os.Stderr, "curl: missing URL\n")
		return 1
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "curl: %v\n", err)
		return 1
	}

	for _, h := range headers {
		parts := strings.SplitN(h, ":", 2)
		if len(parts) == 2 {
			req.Header.Set(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "curl: %v\n", err)
		return 1
	}
	defer resp.Body.Close()

	var w io.Writer
	if output != "" {
		f, err := os.Create(output)
		if err != nil {
			fmt.Fprintf(os.Stderr, "curl: %v\n", err)
			return 1
		}
		defer f.Close()
		w = f
	} else {
		w = os.Stdout
	}

	io.Copy(w, resp.Body)
	return 0
}

func init() {
	applet.Register(&applet.Applet{Name: "ping", Short: "Send ICMP ECHO_REQUEST to network hosts", Func: runPing})
}

func runPing(args []string) int {
	count := 4
	host := ""

	for _, a := range args[1:] {
		if strings.HasPrefix(a, "-c") {
			if len(a) > 2 {
				fmt.Sscanf(a[2:], "%d", &count)
			} else {
				count = 4
			}
			continue
		}
		if !strings.HasPrefix(a, "-") {
			host = a
		}
	}

	if host == "" {
		fmt.Fprintf(os.Stderr, "ping: missing host\n")
		return 1
	}

	// Resolve host
	ips, err := net.LookupIP(host)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ping: %s: %v\n", host, err)
		return 1
	}
	ip := ips[0]

	fmt.Fprintf(os.Stderr, "PING %s (%s) 56 bytes of data.\n", host, ip.String())

	// Native ICMP implementation
	conn, err := net.DialTimeout("ip4:icmp", ip.String(), 5*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ping: %v\n", err)
		return 1
	}
	defer conn.Close()

	exitCode := 1
	for i := 0; i < count; i++ {
		// Build ICMP echo request
		icmp := make([]byte, 8+56)
		icmp[0] = 8            // Echo request
		icmp[1] = 0            // Code
		icmp[2] = 0            // Checksum (high)
		icmp[3] = 0            // Checksum (low)
		icmp[4] = 0            // ID high
		icmp[5] = 0            // ID low
		icmp[6] = byte(i >> 8) // Seq high
		icmp[7] = byte(i)      // Seq low

		// Calculate checksum
		cs := checksum(icmp)
		icmp[2] = byte(cs >> 8)
		icmp[3] = byte(cs)

		conn.SetDeadline(time.Now().Add(5 * time.Second))
		start := time.Now()
		_, err := conn.Write(icmp)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ping: send: %v\n", err)
			time.Sleep(time.Second)
			continue
		}

		// Read reply
		reply := make([]byte, 1024)
		n, err := conn.Read(reply)
		elapsed := time.Since(start)

		if err != nil {
			fmt.Fprintf(os.Stderr, "ping: recv: %v\n", err)
			time.Sleep(time.Second)
			continue
		}

		// Parse ICMP echo reply
		if n >= 8 && reply[0] == 0 {
			seq := int(reply[6])<<8 | int(reply[7])
			fmt.Printf("%d bytes from %s: icmp_seq=%d time=%d ms\n", n, ip.String(), seq, elapsed.Milliseconds())
			exitCode = 0
		}

		if i < count-1 {
			time.Sleep(time.Second)
		}
	}

	fmt.Fprintf(os.Stderr, "--- %s ping statistics ---\n", host)
	return exitCode
}

func checksum(data []byte) uint16 {
	var sum uint32
	for i := 0; i+1 < len(data); i += 2 {
		sum += uint32(data[i])<<8 | uint32(data[i+1])
	}
	if len(data)%2 == 1 {
		sum += uint32(data[len(data)-1]) << 8
	}
	for sum>>16 != 0 {
		sum = (sum & 0xffff) + (sum >> 16)
	}
	return uint16(^sum)
}

func init() {
	applet.Register(&applet.Applet{Name: "nc", Short: "TCP/IP swiss army knife", Func: runNc})
	applet.Register(&applet.Applet{Name: "netcat", Short: "TCP/IP swiss army knife", Func: runNc})
}

func runNc(args []string) int {
	listen := false
	host := ""
	port := ""

	for _, a := range args[1:] {
		if a == "-l" {
			listen = true
			continue
		}
		if !strings.HasPrefix(a, "-") {
			if host == "" {
				host = a
			} else {
				port = a
			}
		}
	}

	if listen {
		ln, err := net.Listen("tcp", host+":"+port)
		if err != nil {
			fmt.Fprintf(os.Stderr, "nc: %v\n", err)
			return 1
		}
		defer ln.Close()

		conn, err := ln.Accept()
		if err != nil {
			fmt.Fprintf(os.Stderr, "nc: %v\n", err)
			return 1
		}
		defer conn.Close()

		go io.Copy(conn, os.Stdin)
		io.Copy(os.Stdout, conn)
		return 0
	}

	if host == "" || port == "" {
		fmt.Fprintf(os.Stderr, "nc: missing host/port\n")
		return 1
	}

	conn, err := net.Dial("tcp", host+":"+port)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nc: %v\n", err)
		return 1
	}
	defer conn.Close()

	go io.Copy(conn, os.Stdin)
	io.Copy(os.Stdout, conn)
	return 0
}

func init() {
	applet.Register(&applet.Applet{Name: "nslookup", Short: "Query DNS servers", Func: runNslookup})
}

func runNslookup(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "nslookup: missing host\n")
		return 1
	}

	host := args[1]
	ips, err := net.LookupHost(host)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nslookup: %v\n", err)
		return 1
	}

	fmt.Printf("Server:\t\t127.0.0.53\n")
	fmt.Printf("Address:\t127.0.0.53#53\n\n")
	fmt.Printf("Name:\t%s\n", host)
	for _, ip := range ips {
		fmt.Printf("Address: %s\n", ip)
	}
	return 0
}

func init() {
	applet.Register(&applet.Applet{Name: "ifconfig", Short: "Configure network interfaces", Func: runIfconfig})
}

func runIfconfig(args []string) int {
	ifaces, err := net.Interfaces()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ifconfig: %v\n", err)
		return 1
	}

	for _, iface := range ifaces {
		addrs, _ := iface.Addrs()
		fmt.Printf("%s: flags=%d mtu %d\n", iface.Name, iface.Flags, iface.MTU)
		if len(addrs) > 0 {
			fmt.Printf("        inet %s\n", addrs[0].String())
		}
		if iface.HardwareAddr != nil {
			fmt.Printf("        ether %s\n", iface.HardwareAddr)
		}
		fmt.Println()
	}
	return 0
}

func init() {
	applet.Register(&applet.Applet{Name: "route", Short: "Show/manipulate the IP routing table", Func: runRoute})
}

func runRoute(args []string) int {
	// Simplified route output
	fmt.Printf("Kernel IP routing table\n")
	fmt.Printf("Destination     Gateway         Genmask         Flags Metric Ref    Use Iface\n")

	// Try to read from /proc/net/route on Linux
	if f, err := os.Open("/proc/net/route"); err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		scanner.Scan() // skip header
		for scanner.Scan() {
			parts := strings.Fields(scanner.Text())
			if len(parts) >= 8 {
				fmt.Printf("%-16s %-16s %-16s %-5s %-6s %-4s %s %s\n",
					parts[0], parts[1], parts[2], parts[3], parts[6], parts[7], "0", parts[0])
			}
		}
	} else {
		fmt.Printf("0.0.0.0         0.0.0.0         0.0.0.0         U     0      0        0 eth0\n")
	}
	return 0
}

func init() {
	applet.Register(&applet.Applet{Name: "netstat", Short: "Network statistics", Func: runNetstat})
}

func runNetstat(args []string) int {
	showAll := false
	showTCP := false
	showUDP := false
	showListen := false

	for _, a := range args[1:] {
		if a == "-a" {
			showAll = true
		}
		if a == "-t" {
			showTCP = true
		}
		if a == "-u" {
			showUDP = true
		}
		if a == "-l" {
			showListen = true
		}
	}

	if !showTCP && !showUDP {
		showTCP = true
		showUDP = true
	}

	if showAll {
		fmt.Printf("Active Internet connections")
	} else {
		fmt.Printf("Active Internet connections (only servers)")
	}
	if showListen {
		fmt.Printf(" (only servers)")
	}
	fmt.Println()
	fmt.Printf("Proto Recv-Q Send-Q Local Address           Foreign Address         State\n")

	// Try /proc/net/tcp and /proc/net/udp
	for _, proto := range []string{"tcp", "udp"} {
		if proto == "tcp" && !showTCP {
			continue
		}
		if proto == "udp" && !showUDP {
			continue
		}

		path := "/proc/net/" + proto
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		scanner.Scan() // skip header
		for scanner.Scan() {
			parts := strings.Fields(scanner.Text())
			if len(parts) >= 4 {
				fmt.Printf("%-5s %6s %6s %-23s %-23s %s\n",
					proto, parts[0], parts[1], formatAddr(parts[1]), formatAddr(parts[2]), parts[3])
			}
		}
	}
	return 0
}

func formatAddr(hex string) string {
	// Simplified hex address parsing
	return hex
}

func init() {
	applet.Register(&applet.Applet{Name: "telnet", Short: "Telnet client", Func: runTelnet})
	applet.Register(&applet.Applet{Name: "tftp", Short: "TFTP client", Func: runTftp})
	applet.Register(&applet.Applet{Name: "traceroute", Short: "Print the route packets trace to network host", Func: runTraceroute})
}

func runTelnet(args []string) int {
	if len(args) < 3 {
		fmt.Fprintf(os.Stderr, "telnet: missing host/port\n")
		return 1
	}
	host := args[1]
	port := args[2]

	conn, err := net.DialTimeout("tcp", host+":"+port, 10*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "telnet: %v\n", err)
		return 1
	}
	defer conn.Close()

	go io.Copy(conn, os.Stdin)
	io.Copy(os.Stdout, conn)
	return 0
}

func runTftp(args []string) int {
	fmt.Fprintf(os.Stderr, "tftp: not yet implemented\n")
	return 1
}

func runTraceroute(args []string) int {
	host := ""
	maxHops := 30
	for _, a := range args[1:] {
		if strings.HasPrefix(a, "-m") && len(a) > 2 {
			fmt.Sscanf(a[2:], "%d", &maxHops)
			continue
		}
		if a == "-m" {
			continue
		}
		if !strings.HasPrefix(a, "-") {
			host = a
		}
	}
	if host == "" {
		fmt.Fprintf(os.Stderr, "traceroute: missing host\n")
		return 1
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		fmt.Fprintf(os.Stderr, "traceroute: %v\n", err)
		return 1
	}
	dest := ips[0]

	fmt.Fprintf(os.Stderr, "traceroute to %s (%s), %d hops max\n", host, dest.String(), maxHops)

	for ttl := 1; ttl <= maxHops; ttl++ {
		conn, err := net.DialTimeout("ip4:icmp", dest.String(), 3*time.Second)
		if err != nil {
			fmt.Printf("%2d  *\n", ttl)
			continue
		}

		icmp := make([]byte, 8)
		icmp[0] = 8
		icmp[6] = byte(ttl >> 8)
		icmp[7] = byte(ttl)
		cs := checksum(icmp)
		icmp[2] = byte(cs >> 8)
		icmp[3] = byte(cs)

		conn.SetDeadline(time.Now().Add(3 * time.Second))
		start := time.Now()
		conn.Write(icmp)

		reply := make([]byte, 1024)
		n, _ := conn.Read(reply)
		elapsed := time.Since(start)
		hopAddr := conn.RemoteAddr().String()
		conn.Close()

		if n > 0 {
			fmt.Printf("%2d  %s (%s)  %d ms\n", ttl, hopAddr, hopAddr, elapsed.Milliseconds())
		} else {
			fmt.Printf("%2d  *\n", ttl)
		}

		if hopAddr == dest.String() {
			break
		}
	}
	return 0
}

func init() {
	applet.Register(&applet.Applet{Name: "arp", Short: "Manipulate the system ARP cache", Func: runArp})
	applet.Register(&applet.Applet{Name: "arping", Short: "Send ARP REQUEST to a neighbour host", Func: runArping})
}

func runArp(args []string) int {
	f, err := os.Open("/proc/net/arp")
	if err != nil {
		fmt.Fprintf(os.Stderr, "arp: cannot open /proc/net/arp: %v\n", err)
		return 1
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}
	return 0
}

func runArping(args []string) int {
	fmt.Fprintf(os.Stderr, "arping: requires raw sockets (not supported)\n")
	return 1
}

func runHostname(args []string) int {
	if len(args) > 1 && !strings.HasPrefix(args[1], "-") {
		fmt.Fprintf(os.Stderr, "hostname: setting hostname not supported\n")
		return 1
	}
	name, err := os.Hostname()
	if err != nil {
		fmt.Fprintf(os.Stderr, "hostname: %v\n", err)
		return 1
	}
	fmt.Println(name)
	return 0
}

func init() {
	applet.Register(&applet.Applet{Name: "whois", Short: "Client for the whois directory service", Func: runWhois})
}

func runWhois(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "whois: missing query\n")
		return 1
	}

	server := "whois.iana.org"
	query := args[1]

	conn, err := net.DialTimeout("tcp", server+":43", 10*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "whois: %v\n", err)
		return 1
	}
	defer conn.Close()

	fmt.Fprintf(conn, "%s\r\n", query)
	io.Copy(os.Stdout, conn)
	return 0
}

func init() {
	applet.Register(&applet.Applet{Name: "httpd", Short: "Simple HTTP server", Func: runHttpd})
}

func runHttpd(args []string) int {
	port := "8080"
	root := "."

	for _, a := range args[1:] {
		if strings.HasPrefix(a, "-p") && len(a) > 2 {
			port = a[2:]
			continue
		}
		if !strings.HasPrefix(a, "-") {
			root = a
		}
	}

	fmt.Fprintf(os.Stderr, "httpd: serving %s on port %s\n", root, port)
	http.Handle("/", http.FileServer(http.Dir(root)))
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		fmt.Fprintf(os.Stderr, "httpd: %v\n", err)
		return 1
	}
	return 0
}
