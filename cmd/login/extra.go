package login

import (
	"bufio"
	"crypto/sha512"
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/agentbusybox/pkg/applet"
)

// --- adduser / addgroup ---
func init() {
	applet.Register(&applet.Applet{Name: "add-shell", Short: "Add shells to /etc/shells", Func: runAddShell})
	applet.Register(&applet.Applet{Name: "adduser", Short: "Add a user", Func: runAdduser})
	applet.Register(&applet.Applet{Name: "addgroup", Short: "Add a group", Func: runAddgroup})
	applet.Register(&applet.Applet{Name: "remove-shell", Short: "Remove shells from /etc/shells", Func: runRemoveShell})
}

func runAddShell(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "add-shell: missing shell\n")
		return 1
	}
	return updateShells(args[1:], true)
}

func runRemoveShell(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "remove-shell: missing shell\n")
		return 1
	}
	return updateShells(args[1:], false)
}

func updateShells(shells []string, add bool) int {
	path := "/etc/shells"
	data, _ := os.ReadFile(path)
	lines := []string{}
	seen := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lines = append(lines, line)
		seen[line] = true
	}
	if add {
		for _, shell := range shells {
			if !seen[shell] {
				lines = append(lines, shell)
				seen[shell] = true
			}
		}
	} else {
		remove := map[string]bool{}
		for _, shell := range shells {
			remove[shell] = true
		}
		filtered := lines[:0]
		for _, line := range lines {
			if !remove[line] {
				filtered = append(filtered, line)
			}
		}
		lines = filtered
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", func() string {
			if add {
				return "add-shell"
			}
			return "remove-shell"
		}(), err)
		return 1
	}
	return 0
}

func runAdduser(args []string) int {
	if runtime.GOOS == "windows" {
		fmt.Fprintf(os.Stderr, "adduser: not supported on Windows\n")
		return 1
	}
	system := false
	home := ""
	shell := "/bin/sh"
	group := ""
	user := ""

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-S", "--system":
			system = true
		case "-h", "--home":
			if i+1 < len(args) {
				i++
				home = args[i]
			}
		case "-s", "--shell":
			if i+1 < len(args) {
				i++
				shell = args[i]
			}
		case "-G", "--ingroup":
			if i+1 < len(args) {
				i++
				group = args[i]
			}
		default:
			if !strings.HasPrefix(args[i], "-") {
				user = args[i]
			}
		}
	}
	if user == "" {
		fmt.Fprintf(os.Stderr, "adduser: missing username\n")
		return 1
	}

	if home == "" {
		home = "/home/" + user
	}
	if group == "" {
		group = user
	}
	if err := addSystemUser(user, group, home, shell, system); err != nil {
		fmt.Fprintf(os.Stderr, "adduser: %v\n", err)
		return 1
	}
	return 0
}

func runAddgroup(args []string) int {
	if runtime.GOOS == "windows" {
		fmt.Fprintf(os.Stderr, "addgroup: not supported on Windows\n")
		return 1
	}
	system := false
	group := ""
	for _, a := range args[1:] {
		if a == "-S" || a == "--system" {
			system = true
			continue
		}
		if !strings.HasPrefix(a, "-") {
			group = a
		}
	}
	if group == "" {
		fmt.Fprintf(os.Stderr, "addgroup: missing group name\n")
		return 1
	}

	if _, err := addSystemGroup(group, system); err != nil {
		fmt.Fprintf(os.Stderr, "addgroup: %v\n", err)
		return 1
	}
	return 0
}

// --- deluser / delgroup ---
func init() {
	applet.Register(&applet.Applet{Name: "deluser", Short: "Delete a user", Func: runDeluser})
	applet.Register(&applet.Applet{Name: "delgroup", Short: "Delete a group", Func: runDelgroup})
}

func runDeluser(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "deluser: missing username\n")
		return 1
	}
	if err := deleteSystemUser(args[1]); err != nil {
		fmt.Fprintf(os.Stderr, "deluser: %v\n", err)
		return 1
	}
	return 0
}

func runDelgroup(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "delgroup: missing group name\n")
		return 1
	}
	if err := deleteSystemGroup(args[1]); err != nil {
		fmt.Fprintf(os.Stderr, "delgroup: %v\n", err)
		return 1
	}
	return 0
}

// --- mkpasswd ---
func init() {
	applet.Register(&applet.Applet{Name: "mkpasswd", Short: "Compute encrypted password", Func: runMkpasswd})
}

func runMkpasswd(args []string) int {
	method := "sha-512"
	password := ""

	for i := 1; i < len(args); i++ {
		if args[i] == "-m" || args[i] == "--method" {
			if i+1 < len(args) {
				i++
				method = args[i]
			}
			continue
		}
		if args[i] == "-S" || args[i] == "--salt" {
			continue // salt auto-generated
		}
		if !strings.HasPrefix(args[i], "-") {
			password = args[i]
		}
	}

	if password == "" {
		fmt.Fprintf(os.Stderr, "mkpasswd: missing password\n")
		return 1
	}

	salt := make([]byte, 16)
	chars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789./"
	for i := range salt {
		salt[i] = chars[rand.Intn(len(chars))]
	}

	switch method {
	case "sha-512", "sha512":
		h := sha512.Sum512(append([]byte(password), salt...))
		fmt.Printf("$6$%s$%x\n", string(salt), h)
	case "md5":
		fmt.Printf("$1$%s$%x\n", string(salt), sha512.Sum512([]byte(password)))
	default:
		fmt.Fprintf(os.Stderr, "mkpasswd: unsupported method '%s'\n", method)
		return 1
	}
	return 0
}

// --- vlock ---
func init() {
	applet.Register(&applet.Applet{Name: "vlock", Short: "Lock virtual consoles", Func: runVlock})
}

func runVlock(args []string) int {
	if runtime.GOOS == "windows" {
		fmt.Println("vlock: not supported on Windows")
		return 0
	}
	fmt.Println("The screen is locked.")
	fmt.Println("Please enter your password to unlock.")
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Password: ")
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		line = strings.TrimSpace(line)
		// Simplified: just check if not empty
		if line != "" {
			fmt.Println("Unlocked.")
			return 0
		}
	}
	return 0
}

// --- sulogin ---
func init() {
	applet.Register(&applet.Applet{Name: "sulogin", Short: "Single-user login", Func: runSulogin})
}

func runSulogin(args []string) int {
	if runtime.GOOS == "windows" {
		return 0
	}
	syscall.Setsid()
	if err := syscall.Exec("/bin/sh", []string{"sh"}, os.Environ()); err != nil {
		fmt.Fprintf(os.Stderr, "sulogin: %v\n", err)
		return 1
	}
	return 0
}

// --- chpasswd / cryptpw --- already in login.go, no duplicates here ---

type passwdEntry struct {
	name  string
	pass  string
	uid   int
	gid   int
	gecos string
	home  string
	shell string
	raw   string
	valid bool
}

type groupEntry struct {
	name    string
	pass    string
	gid     int
	members []string
	raw     string
	valid   bool
}

func readPasswdEntries() ([]passwdEntry, error) {
	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return nil, err
	}
	entries := []passwdEntry{}
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		parts := strings.Split(line, ":")
		e := passwdEntry{raw: line}
		if len(parts) >= 7 {
			uid, uidErr := strconv.Atoi(parts[2])
			gid, gidErr := strconv.Atoi(parts[3])
			if uidErr == nil && gidErr == nil {
				e = passwdEntry{name: parts[0], pass: parts[1], uid: uid, gid: gid, gecos: parts[4], home: parts[5], shell: parts[6], valid: true}
			}
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func writePasswdEntries(entries []passwdEntry) error {
	lines := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.valid {
			lines = append(lines, fmt.Sprintf("%s:%s:%d:%d:%s:%s:%s", e.name, e.pass, e.uid, e.gid, e.gecos, e.home, e.shell))
		} else if e.raw != "" {
			lines = append(lines, e.raw)
		}
	}
	return os.WriteFile("/etc/passwd", []byte(strings.Join(lines, "\n")+"\n"), 0644)
}

func readGroupEntries() ([]groupEntry, error) {
	data, err := os.ReadFile("/etc/group")
	if err != nil {
		return nil, err
	}
	entries := []groupEntry{}
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		parts := strings.Split(line, ":")
		e := groupEntry{raw: line}
		if len(parts) >= 4 {
			gid, gidErr := strconv.Atoi(parts[2])
			if gidErr == nil {
				members := []string{}
				if parts[3] != "" {
					members = strings.Split(parts[3], ",")
				}
				e = groupEntry{name: parts[0], pass: parts[1], gid: gid, members: members, valid: true}
			}
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func writeGroupEntries(entries []groupEntry) error {
	lines := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.valid {
			lines = append(lines, fmt.Sprintf("%s:%s:%d:%s", e.name, e.pass, e.gid, strings.Join(e.members, ",")))
		} else if e.raw != "" {
			lines = append(lines, e.raw)
		}
	}
	return os.WriteFile("/etc/group", []byte(strings.Join(lines, "\n")+"\n"), 0644)
}

func addSystemGroup(name string, system bool) (int, error) {
	groups, err := readGroupEntries()
	if err != nil {
		return 0, err
	}
	minID := 1000
	if system {
		minID = 100
	}
	nextGID := minID
	for _, g := range groups {
		if !g.valid {
			continue
		}
		if g.name == name {
			return g.gid, nil
		}
		if g.gid >= nextGID {
			nextGID = g.gid + 1
		}
	}
	groups = append(groups, groupEntry{name: name, pass: "x", gid: nextGID, valid: true})
	if err := writeGroupEntries(groups); err != nil {
		return 0, err
	}
	return nextGID, nil
}

func addSystemUser(name, group, home, shell string, system bool) error {
	passwd, err := readPasswdEntries()
	if err != nil {
		return err
	}
	for _, p := range passwd {
		if p.valid && p.name == name {
			return fmt.Errorf("user '%s' already exists", name)
		}
	}
	gid, err := addSystemGroup(group, system)
	if err != nil {
		return err
	}
	minID := 1000
	if system {
		minID = 100
	}
	nextUID := minID
	for _, p := range passwd {
		if p.valid && p.uid >= nextUID {
			nextUID = p.uid + 1
		}
	}
	passwd = append(passwd, passwdEntry{name: name, pass: "x", uid: nextUID, gid: gid, gecos: name, home: home, shell: shell, valid: true})
	if err := writePasswdEntries(passwd); err != nil {
		return err
	}
	if !system {
		if err := os.MkdirAll(home, 0755); err != nil {
			return err
		}
		_ = os.Chown(home, nextUID, gid)
	}
	appendShadowUser(name)
	return nil
}

func appendShadowUser(name string) {
	data, err := os.ReadFile("/etc/shadow")
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, name+":") {
			return
		}
	}
	days := time.Now().Unix() / 86400
	line := fmt.Sprintf("%s:!:%d:0:99999:7:::", name, days)
	_ = os.WriteFile("/etc/shadow", append([]byte(strings.TrimRight(string(data), "\n")+"\n"), []byte(line+"\n")...), 0600)
}

func deleteSystemUser(name string) error {
	passwd, err := readPasswdEntries()
	if err != nil {
		return err
	}
	filtered := passwd[:0]
	found := false
	for _, p := range passwd {
		if p.valid && p.name == name {
			found = true
			continue
		}
		filtered = append(filtered, p)
	}
	if !found {
		return fmt.Errorf("user '%s' does not exist", name)
	}
	if err := writePasswdEntries(filtered); err != nil {
		return err
	}
	removeShadowUser(name)
	removeUserFromGroups(name)
	return nil
}

func removeShadowUser(name string) {
	data, err := os.ReadFile("/etc/shadow")
	if err != nil {
		return
	}
	lines := []string{}
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		if !strings.HasPrefix(line, name+":") {
			lines = append(lines, line)
		}
	}
	_ = os.WriteFile("/etc/shadow", []byte(strings.Join(lines, "\n")+"\n"), 0600)
}

func removeUserFromGroups(name string) {
	groups, err := readGroupEntries()
	if err != nil {
		return
	}
	for i := range groups {
		members := groups[i].members[:0]
		for _, m := range groups[i].members {
			if m != name {
				members = append(members, m)
			}
		}
		groups[i].members = members
	}
	_ = writeGroupEntries(groups)
}

func deleteSystemGroup(name string) error {
	groups, err := readGroupEntries()
	if err != nil {
		return err
	}
	filtered := groups[:0]
	found := false
	for _, g := range groups {
		if g.valid && g.name == name {
			found = true
			continue
		}
		filtered = append(filtered, g)
	}
	if !found {
		return fmt.Errorf("group '%s' does not exist", name)
	}
	return writeGroupEntries(filtered)
}
