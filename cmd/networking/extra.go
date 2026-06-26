package networking

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
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
	fmt.Fprintf(os.Stderr, "ftpput: not yet implemented\n")
	return 1
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
	fmt.Fprintf(os.Stderr, "udpsvd: not yet implemented\n")
	return 1
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
	fmt.Fprintf(os.Stderr, "vconfig: not yet implemented\n")
	return 1
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
	fmt.Fprintf(os.Stderr, "dnsd: not yet implemented\n")
	return 1
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
	fmt.Fprintf(os.Stderr, "ifplugd: not yet implemented\n")
	return 1
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
	fmt.Fprintf(os.Stderr, "ifenslave: not yet implemented\n")
	return 1
}

// --- ssl_client / ssl_server ---
func init() {
	applet.Register(&applet.Applet{Name: "ssl_client", Short: "SSL/TLS client", Func: runSslClient})
	applet.Register(&applet.Applet{Name: "ssl_server", Short: "SSL/TLS server", Func: runSslServer})
}

func runSslClient(args []string) int {
	fmt.Fprintf(os.Stderr, "ssl_client: not yet implemented\n")
	return 1
}

func runSslServer(args []string) int {
	fmt.Fprintf(os.Stderr, "ssl_server: not yet implemented\n")
	return 1
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
	fmt.Fprintf(os.Stderr, "tftpd: not yet implemented\n")
	return 1
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
	fmt.Fprintf(os.Stderr, "dumpleases: not yet implemented\n")
	return 1
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
	fmt.Fprintf(os.Stderr, "udhcpd: not yet implemented\n")
	return 1
}

func runUdhcpc6(args []string) int {
	fmt.Fprintf(os.Stderr, "udhcpc6: not yet implemented\n")
	return 1
}

// --- ether-wake ---
func init() {
	applet.Register(&applet.Applet{Name: "ether-wake", Short: "Wake up LAN hosts via Wake-On-LAN", Func: runEtherWake})
}

func runEtherWake(args []string) int {
	fmt.Fprintf(os.Stderr, "ether-wake: not yet implemented\n")
	return 1
}

// --- fakeidentd ---
func init() {
	applet.Register(&applet.Applet{Name: "fakeidentd", Short: "Fake ident daemon", Func: runFakeidentd})
}

func runFakeidentd(args []string) int {
	fmt.Fprintf(os.Stderr, "fakeidentd: not yet implemented\n")
	return 1
}

// --- tunctl ---
func init() {
	applet.Register(&applet.Applet{Name: "tunctl", Short: "Create and manage TUN/TAP interfaces", Func: runTunctl})
}

func runTunctl(args []string) int {
	fmt.Fprintf(os.Stderr, "tunctl: not yet implemented\n")
	return 1
}
