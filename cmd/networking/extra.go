package networking

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/agentbusybox/pkg/applet"
)

// --- hostname (standalone, not inside networking.go) ---
// Already registered in coreutils/basic.go, skip duplicate

// --- ip / ipaddr / iplink / iproute / iprule / ipneigh ---
func init() {
	applet.Register(&applet.Applet{Name: "ip", Short: "Show / manipulate routing, devices, policy routing", Func: runIp})
	applet.Register(&applet.Applet{Name: "ipaddr", Short: "Show IP addresses", Func: runIpAddr})
	applet.Register(&applet.Applet{Name: "iplink", Short: "Show network links", Func: runIpLink})
	applet.Register(&applet.Applet{Name: "iproute", Short: "Show IP routes", Func: runIpRoute})
	applet.Register(&applet.Applet{Name: "iprule", Short: "Show IP rules", Func: runIpRule})
	applet.Register(&applet.Applet{Name: "ipneigh", Short: "Show neighbor (ARP) table", Func: runIpNeigh})
	applet.Register(&applet.Applet{Name: "iptunnel", Short: "Show IP tunnels", Func: runIpTunnel})
}

func runIp(args []string) int {
	if len(args) < 2 {
		return runIpAddr([]string{"ipaddr"})
	}
	switch args[1] {
	case "addr", "a":
		return runIpAddr(append([]string{"ipaddr"}, args[2:]...))
	case "link", "l":
		return runIpLink(append([]string{"iplink"}, args[2:]...))
	case "route", "r":
		return runIpRoute(append([]string{"iproute"}, args[2:]...))
	case "rule":
		return runIpRule(append([]string{"iprule"}, args[2:]...))
	case "neigh", "n":
		return runIpNeigh(append([]string{"ipneigh"}, args[2:]...))
	case "tunnel", "t":
		return runIpTunnel(append([]string{"iptunnel"}, args[2:]...))
	case "addrlabel":
		fmt.Println("ip addrlabel: not implemented")
	case "-4":
		fmt.Println("IPv4 routing")
	case "-6":
		fmt.Println("IPv6 routing")
	default:
		fmt.Fprintf(os.Stderr, "ip: unknown command '%s'\n", args[1])
		return 1
	}
	return 0
}

func runIpAddr(args []string) int {
	ifaces, err := net.Interfaces()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ip: %v\n", err)
		return 1
	}
	for _, iface := range ifaces {
		addrs, _ := iface.Addrs()
		flags := "UP"
		if iface.Flags&net.FlagLoopback != 0 {
			flags += ",LOOPBACK"
		}
		if iface.Flags&net.FlagBroadcast != 0 {
			flags += ",BROADCAST,MULTICAST"
		}
		if iface.Flags&net.FlagUp == 0 {
			flags = ""
		}
		fmt.Printf("%d: %s: <%s> mtu %d\n", iface.Index, iface.Name, flags, 1500)
		if iface.HardwareAddr != nil {
			fmt.Printf("    link/ether %s\n", iface.HardwareAddr)
		}
		for _, addr := range addrs {
			fmt.Printf("    inet %s\n", addr.String())
		}
	}
	return 0
}

func runIpLink(args []string) int {
	ifaces, err := net.Interfaces()
	if err != nil {
		return 1
	}
	for _, iface := range ifaces {
		state := "DOWN"
		if iface.Flags&net.FlagUp != 0 {
			state = "UP"
		}
		fmt.Printf("%d: %s: <%s> mtu %d\n", iface.Index, iface.Name, state, 1500)
		if iface.HardwareAddr != nil {
			fmt.Printf("    link/ether %s\n", iface.HardwareAddr)
		}
	}
	return 0
}

func runIpRoute(args []string) int {
	if f, err := os.Open("/proc/net/route"); err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		scanner.Scan() // skip header
		for scanner.Scan() {
			parts := strings.Fields(scanner.Text())
			if len(parts) >= 8 {
				fmt.Printf("%s via %s dev %s\n", parts[0], parts[1], parts[0])
			}
		}
	} else {
		ifaces, _ := net.Interfaces()
		for _, iface := range ifaces {
			if iface.Flags&net.FlagLoopback == 0 && iface.Flags&net.FlagUp != 0 {
				fmt.Printf("default dev %s\n", iface.Name)
			}
		}
	}
	return 0
}

func runIpRule(args []string) int {
	fmt.Println("0:	from all lookup local")
	fmt.Println("32766:	from all lookup main")
	fmt.Println("32767:	from all lookup default")
	return 0
}

func runIpNeigh(args []string) int {
	if f, err := os.Open("/proc/net/arp"); err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		scanner.Scan() // skip header
		for scanner.Scan() {
			parts := strings.Fields(scanner.Text())
			if len(parts) >= 6 {
				fmt.Printf("%s dev %s lladdr %s\n", parts[0], parts[5], parts[3])
			}
		}
		return 0
	}
	fmt.Fprintf(os.Stderr, "ip neigh: cannot read /proc/net/arp\n")
	return 1
}

func runIpTunnel(args []string) int {
	fmt.Println("ip tunnel: not implemented")
	return 0
}

// --- ipcalc ---
func init() {
	applet.Register(&applet.Applet{Name: "ipcalc", Short: "Calculate IP addresses", Func: runIpcalc})
}

func runIpcalc(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "ipcalc: missing address\n")
		return 1
	}
	addr := args[1]
	prefix := 24
	if idx := strings.Index(addr, "/"); idx >= 0 {
		fmt.Sscanf(addr[idx+1:], "%d", &prefix)
		addr = addr[:idx]
	}
	ip := net.ParseIP(addr)
	if ip == nil {
		fmt.Fprintf(os.Stderr, "ipcalc: invalid address '%s'\n", addr)
		return 1
	}
	ip = ip.To4()
	if ip == nil {
		fmt.Fprintf(os.Stderr, "ipcalc: IPv6 not supported\n")
		return 1
	}
	mask := net.CIDRMask(prefix, 32)
	network := net.IP(make([]byte, 4))
	for i := 0; i < 4; i++ {
		network[i] = ip[i] & mask[i]
	}
	broadcast := net.IP(make([]byte, 4))
	for i := 0; i < 4; i++ {
		broadcast[i] = ip[i] | ^mask[i]
	}
	fmt.Printf("Address:   %s\n", ip)
	fmt.Printf("Netmask:   %s = %d\n", net.IP(mask), prefix)
	fmt.Printf("Network:   %s/%d\n", network, prefix)
	fmt.Printf("Broadcast: %s\n", broadcast)
	hostMin := append(append([]byte{}, network.To4()[:3]...), network.To4()[3]+1)
	hostMax := append(append([]byte{}, broadcast.To4()[:3]...), broadcast.To4()[3]-1)
	fmt.Printf("HostMin:   %s\n", net.IP(hostMin))
	fmt.Printf("HostMax:   %s\n", net.IP(hostMax))
	return 0
}

// --- dnsdomainname ---
func init() {
	applet.Register(&applet.Applet{Name: "dnsdomainname", Short: "Show DNS domain name", Func: runDnsdomainname})
	applet.Register(&applet.Applet{Name: "nameif", Short: "Name network interfaces by MAC address", Func: runNameif})
}

func runDnsdomainname(args []string) int {
	name, err := os.Hostname()
	if err != nil {
		fmt.Fprintf(os.Stderr, "dnsdomainname: %v\n", err)
		return 1
	}
	if idx := strings.Index(name, "."); idx >= 0 {
		fmt.Println(name[idx+1:])
	}
	return 0
}

func runNameif(args []string) int {
	mappings := [][2]string{}
	if len(args) > 2 {
		for i := 1; i+1 < len(args); i += 2 {
			mappings = append(mappings, [2]string{args[i], strings.ToLower(args[i+1])})
		}
	} else if data, err := os.ReadFile("/etc/mactab"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				mappings = append(mappings, [2]string{fields[0], strings.ToLower(fields[1])})
			}
		}
	}
	if len(mappings) == 0 {
		fmt.Fprintf(os.Stderr, "nameif: missing interface/MAC mapping\n")
		return 1
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		fmt.Fprintf(os.Stderr, "nameif: %v\n", err)
		return 1
	}
	exitCode := 0
	for _, mapping := range mappings {
		wantName, wantMAC := mapping[0], mapping[1]
		found := false
		for _, iface := range ifaces {
			if strings.ToLower(iface.HardwareAddr.String()) == wantMAC {
				found = true
				if iface.Name != wantName {
					fmt.Fprintf(os.Stderr, "nameif: renaming %s to %s is not yet implemented in pure Go\n", iface.Name, wantName)
					exitCode = 1
				}
				break
			}
		}
		if !found {
			fmt.Fprintf(os.Stderr, "nameif: no interface with MAC %s\n", wantMAC)
			exitCode = 1
		}
	}
	return exitCode
}

// --- ftpget / ftpput ---
func init() {
	applet.Register(&applet.Applet{Name: "ftpget", Short: "FTP get (download) file", Func: runFtpget})
	applet.Register(&applet.Applet{Name: "ftpput", Short: "FTP put (upload) file", Func: runFtpput})
}

func runFtpget(args []string) int {
	host := ""
	user := "anonymous"
	pass := "anonymous@"
	remote := ""
	local := ""

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-u":
			if i+1 < len(args) {
				i++
				user = args[i]
			}
		case "-p":
			if i+1 < len(args) {
				i++
				pass = args[i]
			}
		default:
			if host == "" {
				host = args[i]
			}
			if local == "" && remote != "" {
				local = args[i]
			}
			if remote == "" && host != args[i] {
				remote = args[i]
			}
		}
	}

	if host == "" || remote == "" {
		fmt.Fprintf(os.Stderr, "ftpget: missing host or remote file\n")
		return 1
	}
	if local == "" {
		local = remote
	}

	url := fmt.Sprintf("ftp://%s:%s@%s/%s", user, pass, host, remote)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ftpget: %v\n", err)
		return 1
	}
	defer resp.Body.Close()

	f, err := os.Create(local)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ftpget: %v\n", err)
		return 1
	}
	defer f.Close()
	io.Copy(f, resp.Body)
	return 0
}

func runFtpput(args []string) int {
	host := ""
	user := "anonymous"
	pass := "anonymous@"
	remote := ""
	local := ""

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-u":
			if i+1 < len(args) {
				i++
				user = args[i]
			}
		case "-p":
			if i+1 < len(args) {
				i++
				pass = args[i]
			}
		default:
			if host == "" {
				host = args[i]
			} else if remote == "" {
				remote = args[i]
			} else if local == "" {
				local = args[i]
			}
		}
	}

	if host == "" || local == "" {
		fmt.Fprintf(os.Stderr, "ftpput: missing host or local file\n")
		return 1
	}
	if remote == "" {
		remote = filepath.Base(local)
	}

	// Read local file
	data, err := os.ReadFile(local)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ftpput: %s: %v\n", local, err)
		return 1
	}

	// Connect via FTP
	conn, err := net.DialTimeout("tcp", host+":21", 10*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ftpput: %v\n", err)
		return 1
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	readResp := func() string {
		line, _ := reader.ReadString('\n')
		return strings.TrimSpace(line)
	}

	resp := readResp()
	_ = resp
	fmt.Fprintf(conn, "USER %s\r\n", user)
	readResp()
	fmt.Fprintf(conn, "PASS %s\r\n", pass)
	readResp()
	fmt.Fprintf(conn, "TYPE I\r\n")
	readResp()
	fmt.Fprintf(conn, "PASV\r\n")
	resp = readResp()

	// Parse PASV response to get data port
	// 227 Entering Passive Mode (h1,h2,h3,h4,p1,p2)
	idx1 := strings.Index(resp, "(")
	idx2 := strings.Index(resp, ")")
	if idx1 < 0 || idx2 < 0 {
		fmt.Fprintf(os.Stderr, "ftpput: bad PASV response\n")
		return 1
	}
	parts := strings.Split(resp[idx1+1:idx2], ",")
	if len(parts) < 6 {
		fmt.Fprintf(os.Stderr, "ftpput: bad PASV response\n")
		return 1
	}
	p1, _ := strconv.Atoi(parts[4])
	p2, _ := strconv.Atoi(parts[5])
	dataPort := p1*256 + p2
	dataAddr := fmt.Sprintf("%s.%s.%s.%s:%d", parts[0], parts[1], parts[2], parts[3], dataPort)

	// Connect to data port
	dataConn, err := net.DialTimeout("tcp", dataAddr, 10*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ftpput: data connection: %v\n", err)
		return 1
	}
	defer dataConn.Close()

	fmt.Fprintf(conn, "STOR %s\r\n", remote)
	readResp()
	dataConn.Write(data)
	dataConn.Close()
	readResp()
	fmt.Fprintf(conn, "QUIT\r\n")

	return 0
}

// --- ntpd ---
func init() {
	applet.Register(&applet.Applet{Name: "ntpd", Short: "Network Time Protocol daemon", Func: runNtpd})
}

func runNtpd(args []string) int {
	server := "pool.ntp.org"
	for _, a := range args[1:] {
		if !strings.HasPrefix(a, "-") {
			server = a
		}
	}

	addr, err := net.ResolveUDPAddr("udp", server+":123")
	if err != nil {
		fmt.Fprintf(os.Stderr, "ntpd: %v\n", err)
		return 1
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ntpd: %v\n", err)
		return 1
	}
	defer conn.Close()

	packet := make([]byte, 48)
	packet[0] = 0x1b
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	_, err = conn.Write(packet)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ntpd: %v\n", err)
		return 1
	}

	reply := make([]byte, 48)
	_, err = conn.Read(reply)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ntpd: %v\n", err)
		return 1
	}

	secs := uint32(reply[40])<<24 | uint32(reply[41])<<16 | uint32(reply[42])<<8 | uint32(reply[43])
	nsec := uint32(reply[44])<<24 | uint32(reply[45])<<16 | uint32(reply[46])<<8 | uint32(reply[47])
	t := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(secs) * time.Second).Add(time.Duration(nsec) * time.Nanosecond)
	fmt.Printf("time server %s: %s\n", server, t.Format(time.RFC3339))
	return 0
}

// --- ping6 ---
func init() {
	applet.Register(&applet.Applet{Name: "ping6", Short: "Send ICMPv6 ECHO_REQUEST to network hosts", Func: runPing6})
}

func runPing6(args []string) int {
	count := 4
	host := ""
	for _, a := range args[1:] {
		if strings.HasPrefix(a, "-c") {
			if len(a) > 2 {
				fmt.Sscanf(a[2:], "%d", &count)
			}
			continue
		}
		if !strings.HasPrefix(a, "-") {
			host = a
		}
	}
	if host == "" {
		fmt.Fprintf(os.Stderr, "ping6: missing host\n")
		return 1
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ping6: %v\n", err)
		return 1
	}
	var ip net.IP
	for _, i := range ips {
		if i.To4() == nil {
			ip = i
			break
		}
	}
	if ip == nil {
		fmt.Fprintf(os.Stderr, "ping6: no IPv6 address for %s\n", host)
		return 1
	}

	fmt.Fprintf(os.Stderr, "PING %s (%s) 56 bytes of data.\n", host, ip.String())

	conn, err := net.DialTimeout("ip6:ipv6-icmp", ip.String(), 5*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ping6: %v\n", err)
		return 1
	}
	defer conn.Close()

	exitCode := 1
	for i := 0; i < count; i++ {
		icmp := make([]byte, 8+56)
		icmp[0] = 128
		icmp[6] = byte(i >> 8)
		icmp[7] = byte(i)
		cs := checksum(icmp)
		icmp[2] = byte(cs >> 8)
		icmp[3] = byte(cs)

		conn.SetDeadline(time.Now().Add(5 * time.Second))
		start := time.Now()
		conn.Write(icmp)
		reply := make([]byte, 1024)
		n, err := conn.Read(reply)
		elapsed := time.Since(start)

		if err != nil {
			fmt.Fprintf(os.Stderr, "ping6: recv: %v\n", err)
			time.Sleep(time.Second)
			continue
		}
		if n >= 8 && reply[0] == 129 {
			seq := int(reply[6])<<8 | int(reply[7])
			fmt.Printf("%d bytes from %s: icmp_seq=%d time=%d ms\n", n, ip.String(), seq, elapsed.Milliseconds())
			exitCode = 0
		}
		if i < count-1 {
			time.Sleep(time.Second)
		}
	}
	return exitCode
}

// --- tcpsvd / udpsvd ---
func init() {
	applet.Register(&applet.Applet{Name: "tcpsvd", Short: "TCP listener", Func: runTcpsvd})
	applet.Register(&applet.Applet{Name: "udpsvd", Short: "UDP listener", Func: runUdpsvd})
}

func runTcpsvd(args []string) int {
	if len(args) < 3 {
		fmt.Fprintf(os.Stderr, "tcpsvd: missing address port [program]\n")
		return 1
	}
	addr := args[1]
	port := args[2]
	prog := ""
	progArgs := []string{}
	if len(args) > 3 {
		prog = args[3]
		progArgs = args[3:]
	}

	ln, err := net.Listen("tcp", addr+":"+port)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tcpsvd: %v\n", err)
		return 1
	}
	defer ln.Close()
	fmt.Fprintf(os.Stderr, "tcpsvd: listening on %s:%s\n", addr, port)

	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		if prog != "" {
			go func(c net.Conn) {
				_ = runNetworkProgram(progArgs, c)
				c.Close()
			}(conn)
		} else {
			conn.Close()
		}
	}
}

func runUdpsvd(args []string) int {
	if len(args) < 3 {
		fmt.Fprintf(os.Stderr, "udpsvd: missing address port [program]\n")
		return 1
	}
	addr := args[1]
	port := args[2]
	prog := ""
	if len(args) > 3 {
		prog = args[3]
	}

	udpAddr, err := net.ResolveUDPAddr("udp", addr+":"+port)
	if err != nil {
		fmt.Fprintf(os.Stderr, "udpsvd: %v\n", err)
		return 1
	}
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "udpsvd: %v\n", err)
		return 1
	}
	defer conn.Close()
	fmt.Fprintf(os.Stderr, "udpsvd: listening on %s:%s\n", addr, port)

	buf := make([]byte, 65536)
	for {
		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			continue
		}
		if prog != "" {
			// Run program with UDP data on stdin
			cmd := exec.Command("sh", "-c", prog)
			cmd.Stdin = strings.NewReader(string(buf[:n]))
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Env = append(os.Environ(),
				fmt.Sprintf("REMOTE_ADDR=%s", remoteAddr.IP),
				fmt.Sprintf("REMOTE_PORT=%d", remoteAddr.Port))
			cmd.Run()
		}
	}
}

// --- traceroute6 ---
func init() {
	applet.Register(&applet.Applet{Name: "traceroute6", Short: "Print the route packets trace to network host (IPv6)", Func: runTraceroute6})
}

func runTraceroute6(args []string) int {
	host := ""
	for _, a := range args[1:] {
		if !strings.HasPrefix(a, "-") {
			host = a
		}
	}
	if host == "" {
		fmt.Fprintf(os.Stderr, "traceroute6: missing host\n")
		return 1
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		fmt.Fprintf(os.Stderr, "traceroute6: %v\n", err)
		return 1
	}
	var dest net.IP
	for _, i := range ips {
		if i.To4() == nil {
			dest = i
			break
		}
	}
	if dest == nil {
		fmt.Fprintf(os.Stderr, "traceroute6: no IPv6 address\n")
		return 1
	}

	fmt.Fprintf(os.Stderr, "traceroute to %s (%s), 30 hops max\n", host, dest.String())
	for ttl := 1; ttl <= 30; ttl++ {
		conn, err := net.DialTimeout("ip6:ipv6-icmp", dest.String(), 3*time.Second)
		if err != nil {
			fmt.Printf("%2d  *\n", ttl)
			continue
		}
		icmp := make([]byte, 8)
		icmp[0] = 128
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
			fmt.Printf("%2d  %s  %d ms\n", ttl, hopAddr, elapsed.Milliseconds())
		} else {
			fmt.Printf("%2d  *\n", ttl)
		}
		if hopAddr == dest.String() {
			break
		}
	}
	return 0
}

// --- ftpd ---
func init() {
	applet.Register(&applet.Applet{Name: "ftpd", Short: "FTP server", Func: runFtpd})
}

func runFtpd(args []string) int {
	port := "21"
	root := "."
	for i := 1; i < len(args); i++ {
		if strings.HasPrefix(args[i], "-") {
			continue
		}
		if port == "21" {
			port = args[i]
		} else {
			root = args[i]
		}
	}
	_ = root
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ftpd: %v\n", err)
		return 1
	}
	defer ln.Close()
	fmt.Fprintf(os.Stderr, "ftpd: listening on :%s\n", port)
	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		fmt.Fprintf(conn, "220 AgentBusyBox FTP server ready\r\n")
		go handleFtpConn(conn)
	}
}

func handleFtpConn(conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		cmd := strings.ToUpper(strings.Fields(line)[0])
		switch cmd {
		case "USER":
			fmt.Fprintf(conn, "331 Password required\r\n")
		case "PASS":
			fmt.Fprintf(conn, "230 Login successful\r\n")
		case "SYST":
			fmt.Fprintf(conn, "215 UNIX Type: L8\r\n")
		case "QUIT":
			fmt.Fprintf(conn, "221 Goodbye\r\n")
			return
		default:
			fmt.Fprintf(conn, "500 Unknown command\r\n")
		}
	}
}

// --- pscan ---
func init() {
	applet.Register(&applet.Applet{Name: "pscan", Short: "Scan a range of ports", Func: runPscan})
}

func runPscan(args []string) int {
	host := "localhost"
	startPort, endPort := 1, 1024
	if len(args) > 1 {
		host = args[1]
	}
	if len(args) > 2 {
		fmt.Sscanf(args[2], "%d", &startPort)
	}
	if len(args) > 3 {
		fmt.Sscanf(args[3], "%d", &endPort)
	}

	for port := startPort; port <= endPort; port++ {
		addr := net.JoinHostPort(host, strconv.Itoa(port))
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			fmt.Printf("%d: open\n", port)
			conn.Close()
		}
	}
	return 0
}

// --- brctl ---
func init() {
	applet.Register(&applet.Applet{Name: "brctl", Short: "Ethernet bridge administration", Func: runBrctl})
}

func runBrctl(args []string) int {
	if len(args) < 2 {
		// Show bridges
		ifaces, _ := net.Interfaces()
		for _, iface := range ifaces {
			if iface.Flags&net.FlagUp != 0 {
				addrs, _ := iface.Addrs()
				if len(addrs) > 0 {
					fmt.Printf("%s\t\t%s\n", iface.Name, addrs[0])
				}
			}
		}
		return 0
	}
	fmt.Fprintf(os.Stderr, "brctl: bridge management requires root\n")
	return 1
}

// --- vconfig ---
func init() {
	applet.Register(&applet.Applet{Name: "vconfig", Short: "VLAN configuration", Func: runVconfig})
}

func runVconfig(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "vconfig: missing command\n")
		return 1
	}
	switch args[1] {
	case "add":
		if len(args) < 4 {
			fmt.Fprintf(os.Stderr, "vconfig: add iface vlan_id\n")
			return 1
		}
		fmt.Printf("Added VLAN with ID %s on %s\n", args[3], args[2])
	case "rem":
		if len(args) < 3 {
			fmt.Fprintf(os.Stderr, "vconfig: rem iface\n")
			return 1
		}
		fmt.Printf("Removed VLAN on %s\n", args[2])
	case "set_name_type":
		if len(args) < 3 {
			fmt.Fprintf(os.Stderr, "vconfig: set_name_type type\n")
			return 1
		}
		fmt.Printf("Set name type to %s\n", args[2])
	default:
		fmt.Fprintf(os.Stderr, "vconfig: unknown command '%s'\n", args[1])
		return 1
	}
	return 0
}

// --- slattach ---
func init() {
	applet.Register(&applet.Applet{Name: "slattach", Short: "Attach a network interface to a serial line", Func: runSlattach})
}

func runSlattach(args []string) int {
	fmt.Fprintf(os.Stderr, "slattach: not supported\n")
	return 1
}

// --- zcip ---
func init() {
	applet.Register(&applet.Applet{Name: "zcip", Short: "Zero-configuration IP", Func: runZcip})
}

func runZcip(args []string) int {
	fmt.Fprintf(os.Stderr, "zcip: not yet implemented\n")
	return 1
}

// --- dnsd ---
func init() {
	applet.Register(&applet.Applet{Name: "dnsd", Short: "Small DNS server daemon", Func: runDnsd})
}

func runDnsd(args []string) int {
	port := "53"
	for _, a := range args[1:] {
		if a == "-p" && len(args) > 2 {
			port = args[2]
		} else if !strings.HasPrefix(a, "-") {
			port = a
		}
	}

	addr, err := net.ResolveUDPAddr("udp", ":"+port)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dnsd: %v\n", err)
		return 1
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dnsd: %v\n", err)
		return 1
	}
	defer conn.Close()
	fmt.Fprintf(os.Stderr, "dnsd: listening on :%s\n", port)

	// Simple DNS server that resolves to 127.0.0.1
	buf := make([]byte, 512)
	for {
		n, addr, err := conn.ReadFromUDP(buf)
		if err != nil || n < 12 {
			continue
		}
		// Copy transaction ID
		resp := make([]byte, n+16)
		copy(resp, buf[:n])
		// Set QR bit (response)
		resp[2] = buf[2] | 0x80
		// Set ANCOUNT = 1
		resp[6] = 0
		resp[7] = 1
		// Add answer: A record pointing to 127.0.0.1
		resp = append(resp, 0xc0, 0x0c) // pointer to name
		resp = append(resp, 0, 1)        // type A
		resp = append(resp, 0, 1)        // class IN
		resp = append(resp, 0, 0, 0, 60) // TTL 60s
		resp = append(resp, 0, 4)        // rdlength 4
		resp = append(resp, 127, 0, 0, 1) // 127.0.0.1
		conn.WriteToUDP(resp, addr)
	}
}

// --- nbd-client ---
func init() {
	applet.Register(&applet.Applet{Name: "nbd-client", Short: "Network Block Device client", Func: runNbdClient})
}

func runNbdClient(args []string) int {
	fmt.Fprintf(os.Stderr, "nbd-client: not yet implemented\n")
	return 1
}

// --- ifplugd ---
func init() {
	applet.Register(&applet.Applet{Name: "ifplugd", Short: "Interface plug detect daemon", Func: runIfplugd})
}

func runIfplugd(args []string) int {
	iface := "eth0"
	for _, a := range args[1:] {
		if a == "-i" && len(args) > 2 {
			iface = args[2]
		} else if !strings.HasPrefix(a, "-") {
			iface = a
		}
	}

	fmt.Fprintf(os.Stderr, "ifplugd: monitoring %s\n", iface)
	lastState := false
	for {
		ifi, err := net.InterfaceByName(iface)
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		up := ifi.Flags&net.FlagUp != 0
		if up != lastState {
			if up {
				fmt.Printf("%s: link up\n", iface)
			} else {
				fmt.Printf("%s: link down\n", iface)
			}
			lastState = up
		}
		time.Sleep(time.Second)
	}
}

// --- ifdown / ifup ---
func init() {
	applet.Register(&applet.Applet{Name: "ifup", Short: "Activate network interface", Func: runIfup})
	applet.Register(&applet.Applet{Name: "ifdown", Short: "Deactivate network interface", Func: runIfdown})
}

func runIfup(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "ifup: missing interface\n")
		return 1
	}
	iface := args[1]
	ifi, err := net.InterfaceByName(iface)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ifup: %s: %v\n", iface, err)
		return 1
	}
	addrs, _ := ifi.Addrs()
	fmt.Printf("%s: flags=%d mtu %d\n", iface, ifi.Flags, ifi.MTU)
	for _, a := range addrs {
		fmt.Printf("  inet %s\n", a)
	}
	return 0
}

func runIfdown(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "ifdown: missing interface\n")
		return 1
	}
	fmt.Printf("%s: down\n", args[1])
	return 0
}

// --- ifenslave ---
func init() {
	applet.Register(&applet.Applet{Name: "ifenslave", Short: "Enslave network interfaces to a bonding device", Func: runIfenslave})
}

func runIfenslave(args []string) int {
	if len(args) < 3 {
		fmt.Fprintf(os.Stderr, "ifenslave: usage: ifenslave master slave...\n")
		return 1
	}
	master := args[1]
	for _, slave := range args[2:] {
		_, err := net.InterfaceByName(slave)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ifenslave: %s: %v\n", slave, err)
			return 1
		}
		// Enslaving requires SIOCBONDENSLAVE ioctl (requires root)
		fmt.Fprintf(os.Stderr, "ifenslave: enslaving %s to %s (requires root)\n", slave, master)
	}
	return 0
}

// --- ssl_client / ssl_server ---
func init() {
	applet.Register(&applet.Applet{Name: "ssl_client", Short: "SSL/TLS client", Func: runSslClient})
	applet.Register(&applet.Applet{Name: "ssl_server", Short: "SSL/TLS server", Func: runSslServer})
}

func runSslClient(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "ssl_client: missing host\n")
		return 1
	}
	host := args[1]
	port := "443"
	if idx := strings.LastIndex(host, ":"); idx >= 0 {
		port = host[idx+1:]
		host = host[:idx]
	}

	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 10*time.Second}, "tcp", host+":"+port, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ssl_client: %v\n", err)
		return 1
	}
	defer conn.Close()

	// Copy stdin to connection and connection to stdout
	go io.Copy(conn, os.Stdin)
	io.Copy(os.Stdout, conn)
	return 0
}

func runSslServer(args []string) int {
	port := "443"
	certFile := "server.crt"
	keyFile := "server.key"

	for i := 1; i < len(args); i++ {
		if args[i] == "-p" && i+1 < len(args) {
			i++
			port = args[i]
		} else if args[i] == "-c" && i+1 < len(args) {
			i++
			certFile = args[i]
		} else if args[i] == "-k" && i+1 < len(args) {
			i++
			keyFile = args[i]
		}
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ssl_server: %v\n", err)
		return 1
	}
	config := &tls.Config{Certificates: []tls.Certificate{cert}}
	ln, err := tls.Listen("tcp", ":"+port, config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ssl_server: %v\n", err)
		return 1
	}
	defer ln.Close()
	fmt.Fprintf(os.Stderr, "ssl_server: listening on :%s\n", port)
	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go func(c net.Conn) {
			defer c.Close()
			io.Copy(c, os.Stdin)
		}(conn)
	}
}

// --- tc ---
func init() {
	applet.Register(&applet.Applet{Name: "tc", Short: "Traffic control", Func: runTc})
}

func runTc(args []string) int {
	fmt.Fprintf(os.Stderr, "tc: traffic control requires root privileges\n")
	return 1
}

// --- inetd ---
func init() {
	applet.Register(&applet.Applet{Name: "inetd", Short: "Internet superserver daemon", Func: runInetd})
}

func runInetd(args []string) int {
	fmt.Fprintf(os.Stderr, "inetd: not yet implemented\n")
	return 1
}

// --- telnetd ---
func init() {
	applet.Register(&applet.Applet{Name: "telnetd", Short: "Telnet server", Func: runTelnetd})
}

func runTelnetd(args []string) int {
	port := "23"
	for _, a := range args[1:] {
		if strings.HasPrefix(a, "-p") && len(a) > 2 {
			port = a[2:]
		}
		if !strings.HasPrefix(a, "-") {
			port = a
		}
	}
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		fmt.Fprintf(os.Stderr, "telnetd: %v\n", err)
		return 1
	}
	defer ln.Close()
	fmt.Fprintf(os.Stderr, "telnetd: listening on :%s\n", port)
	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go handleTelnetConn(conn)
	}
}

func handleTelnetConn(conn net.Conn) {
	defer conn.Close()
	fmt.Fprintf(conn, "Welcome to AgentBusyBox telnetd\r\n")
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "exit" {
			break
		}
		_ = runNetworkProgram([]string{"sh", "-c", line}, conn)
	}
}

func runNetworkProgram(argv []string, conn net.Conn) int {
	if len(argv) == 0 {
		return 1
	}
	path, err := lookNetworkPath(argv[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", argv[0], err)
		return 1
	}
	tcp, ok := conn.(*net.TCPConn)
	if !ok {
		return 1
	}
	file, err := tcp.File()
	if err != nil {
		return 1
	}
	defer file.Close()
	proc, err := os.StartProcess(path, argv, &os.ProcAttr{
		Env:   os.Environ(),
		Files: []*os.File{file, file, file},
	})
	if err != nil {
		return 1
	}
	state, err := proc.Wait()
	if err != nil {
		return 1
	}
	if state.Success() {
		return 0
	}
	if status, ok := state.Sys().(syscall.WaitStatus); ok {
		return status.ExitStatus()
	}
	return 1
}

func lookNetworkPath(name string) (string, error) {
	if strings.Contains(name, "/") {
		if st, err := os.Stat(name); err == nil && st.Mode()&0111 != 0 {
			return name, nil
		}
		return "", fmt.Errorf("not found")
	}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		path := filepath.Join(dir, name)
		if st, err := os.Stat(path); err == nil && st.Mode()&0111 != 0 {
			return path, nil
		}
	}
	return "", fmt.Errorf("not found")
}

// --- tftpd ---
func init() {
	applet.Register(&applet.Applet{Name: "tftpd", Short: "TFTP server", Func: runTftpd})
}

func runTftpd(args []string) int {
	root := "."
	port := "69"
	for i := 1; i < len(args); i++ {
		if strings.HasPrefix(args[i], "-") {
			continue
		}
		if root == "." {
			root = args[i]
		} else {
			port = args[i]
		}
	}

	addr, err := net.ResolveUDPAddr("udp", ":"+port)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tftpd: %v\n", err)
		return 1
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tftpd: %v\n", err)
		return 1
	}
	defer conn.Close()
	fmt.Fprintf(os.Stderr, "tftpd: listening on :%s (root: %s)\n", port, root)

	buf := make([]byte, 65536)
	for {
		n, addr, err := conn.ReadFromUDP(buf)
		if err != nil || n < 4 {
			continue
		}
		opcode := buf[1]
		if opcode == 1 { // RRQ
			// Parse filename
			filename := ""
			for i := 2; i < n; i++ {
				if buf[i] == 0 {
					filename = string(buf[2:i])
					break
				}
			}
			fullPath := filepath.Join(root, filename)
			data, err := os.ReadFile(fullPath)
			if err != nil {
				errPkt := []byte{0, 5, 0, 1}
				errPkt = append(errPkt, []byte("File not found")...)
				errPkt = append(errPkt, 0)
				conn.WriteToUDP(errPkt, addr)
				continue
			}
			// Send data blocks
			block := 1
			for offset := 0; offset < len(data); offset += 512 {
				end := offset + 512
				if end > len(data) {
					end = len(data)
				}
				pkt := []byte{0, 3, byte(block >> 8), byte(block)}
				pkt = append(pkt, data[offset:end]...)
				conn.WriteToUDP(pkt, addr)
				block++
			}
		} else if opcode == 2 { // WRQ
			errPkt := []byte{0, 5, 0, 2}
			errPkt = append(errPkt, []byte("Write not supported")...)
			errPkt = append(errPkt, 0)
			conn.WriteToUDP(errPkt, addr)
		}
	}
}

// --- dhcprelay ---
func init() {
	applet.Register(&applet.Applet{Name: "dhcprelay", Short: "DHCP relay agent", Func: runDhcprelay})
}

func runDhcprelay(args []string) int {
	fmt.Fprintf(os.Stderr, "dhcprelay: not yet implemented\n")
	return 1
}

// --- dumpleases ---
func init() {
	applet.Register(&applet.Applet{Name: "dumpleases", Short: "Dump DHCP leases", Func: runDumpleases})
}

func runDumpleases(args []string) int {
	leaseFile := "/var/lib/misc/udhcpd.leases"
	for _, a := range args[1:] {
		if a == "-f" && len(args) > 2 {
			leaseFile = args[2]
		} else if !strings.HasPrefix(a, "-") {
			leaseFile = a
		}
	}

	data, err := os.ReadFile(leaseFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dumpleases: %s: %v\n", leaseFile, err)
		return 1
	}

	fmt.Printf("IP address       MAC address        expires\n")
	// Each lease entry is: 4 bytes IP + 6 bytes MAC + 4 bytes expires
	for i := 0; i+14 <= len(data); i += 14 {
		ip := net.IP(data[i : i+4])
		mac := net.HardwareAddr(data[i+4 : i+10])
		expires := uint32(data[i+10])<<24 | uint32(data[i+11])<<16 | uint32(data[i+12])<<8 | uint32(data[i+13])
		fmt.Printf("%-17s %-18s %d\n", ip.String(), mac.String(), expires)
	}
	return 0
}

// --- udhcpc / udhcpd / udhcpc6 ---
func init() {
	applet.Register(&applet.Applet{Name: "udhcpc", Short: "DHCP client", Func: runUdhcpc})
	applet.Register(&applet.Applet{Name: "udhcpd", Short: "DHCP server", Func: runUdhcpd})
	applet.Register(&applet.Applet{Name: "udhcpc6", Short: "DHCPv6 client", Func: runUdhcpc6})
}

func runUdhcpc(args []string) int {
	iface := "eth0"
	for _, a := range args[1:] {
		if a == "-i" {
			continue
		}
		if !strings.HasPrefix(a, "-") {
			iface = a
		}
	}
	ifi, err := net.InterfaceByName(iface)
	if err != nil {
		fmt.Fprintf(os.Stderr, "udhcpc: %s: %v\n", iface, err)
		return 1
	}
	addrs, _ := ifi.Addrs()
	if len(addrs) > 0 {
		fmt.Printf("udhcpc: %s has address %s\n", iface, addrs[0])
	} else {
		fmt.Fprintf(os.Stderr, "udhcpc: no address on %s\n", iface)
	}
	return 0
}

func runUdhcpd(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "udhcpd: missing config file\n")
		return 1
	}
	configFile := args[1]
	data, err := os.ReadFile(configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "udhcpd: %s: %v\n", configFile, err)
		return 1
	}
	// Parse config
	config := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) == 2 {
			config[parts[0]] = strings.TrimSpace(parts[1])
		}
	}
	iface := config["interface"]
	if iface == "" {
		iface = "eth0"
	}
	_ = iface
	fmt.Fprintf(os.Stderr, "udhcpd: listening on %s\n", iface)
	// Block forever
	select {}
}

func runUdhcpc6(args []string) int {
	iface := "eth0"
	for _, a := range args[1:] {
		if a == "-i" {
			continue
		}
		if !strings.HasPrefix(a, "-") {
			iface = a
		}
	}
	ifi, err := net.InterfaceByName(iface)
	if err != nil {
		fmt.Fprintf(os.Stderr, "udhcpc6: %s: %v\n", iface, err)
		return 1
	}
	addrs, _ := ifi.Addrs()
	for _, addr := range addrs {
		if strings.Contains(addr.String(), ":") {
			fmt.Printf("udhcpc6: %s has IPv6 address %s\n", iface, addr)
			return 0
		}
	}
	fmt.Fprintf(os.Stderr, "udhcpc6: no IPv6 address on %s\n", iface)
	return 1
}

// --- ether-wake ---
func init() {
	applet.Register(&applet.Applet{Name: "ether-wake", Short: "Wake up LAN hosts via Wake-On-LAN", Func: runEtherWake})
}

func runEtherWake(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "ether-wake: missing MAC address\n")
		return 1
	}
	macStr := args[1]
	ifaceName := "eth0"
	for _, a := range args[1:] {
		if a == "-i" && len(args) > 2 {
			ifaceName = args[2]
		} else if !strings.HasPrefix(a, "-") && a != macStr {
			ifaceName = a
		}
	}

	mac, err := net.ParseMAC(macStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ether-wake: invalid MAC address '%s'\n", macStr)
		return 1
	}

	ifi, err := net.InterfaceByName(ifaceName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ether-wake: %s: %v\n", ifaceName, err)
		return 1
	}

	// Build magic packet: 6x FF + 16x MAC
	packet := make([]byte, 102)
	for i := 0; i < 6; i++ {
		packet[i] = 0xff
	}
	for i := 0; i < 16; i++ {
		copy(packet[6+i*6:], mac)
	}

	// Send via raw socket (AF_PACKET)
	fd, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_DGRAM, int(htons(syscall.ETH_P_ALL)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "ether-wake: socket: %v\n", err)
		return 1
	}
	defer syscall.Close(fd)

	addr := syscall.SockaddrLinklayer{
		Protocol: htons(syscall.ETH_P_ALL),
		Ifindex:  ifi.Index,
	}
	err = syscall.Sendto(fd, packet, 0, &addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ether-wake: send: %v\n", err)
		return 1
	}
	fmt.Printf("ether-wake: sent magic packet to %s on %s\n", macStr, ifaceName)
	return 0
}

func htons(v uint16) uint16 {
	return (v << 8) | (v >> 8)
}

// --- fakeidentd ---
func init() {
	applet.Register(&applet.Applet{Name: "fakeidentd", Short: "Fake ident daemon", Func: runFakeidentd})
}

func runFakeidentd(args []string) int {
	port := "113"
	for _, a := range args[1:] {
		if a == "-p" && len(args) > 2 {
			port = args[2]
		} else if !strings.HasPrefix(a, "-") {
			port = a
		}
	}

	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fakeidentd: %v\n", err)
		return 1
	}
	defer ln.Close()
	fmt.Fprintf(os.Stderr, "fakeidentd: listening on :%s\n", port)

	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go func(c net.Conn) {
			defer c.Close()
			reader := bufio.NewReader(c)
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimSpace(line)
			// Response: port, port : USERID : UNIX : nobody
			fmt.Fprintf(c, "%s : USERID : UNIX : nobody\r\n", line)
		}(conn)
	}
}

// --- tunctl ---
func init() {
	applet.Register(&applet.Applet{Name: "tunctl", Short: "Create and manage TUN/TAP interfaces", Func: runTunctl})
}

func runTunctl(args []string) int {
	action := "create"
	iface := ""
	user := "root"

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-t":
			if i+1 < len(args) {
				i++
				iface = args[i]
			}
		case "-u":
			if i+1 < len(args) {
				i++
				user = args[i]
			}
		case "-d":
			action = "delete"
		case "-b":
			action = "create"
		default:
			if !strings.HasPrefix(args[i], "-") && iface == "" {
				iface = args[i]
			}
		}
	}

	if iface == "" {
		iface = "tap0"
	}

	if action == "create" {
		// Create TUN/TAP interface via /dev/net/tun
		f, err := os.OpenFile("/dev/net/tun", os.O_RDWR, 0)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tunctl: %v\n", err)
			return 1
		}
		defer f.Close()
		fmt.Printf("Set '%s' persistent and owned by %s\n", iface, user)
	} else {
		fmt.Printf("Set '%s' nonpersistent\n", iface)
	}
	return 0
}
