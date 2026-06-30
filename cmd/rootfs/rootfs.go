package rootfs

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/agentbusybox/pkg/applet"
)

func init() {
	applet.Register(&applet.Applet{
		Name:  "rootfs",
		Short: "Generate a minimal root filesystem",
		Func:  runRootfs,
		Usage: `Usage: rootfs [OPTIONS] [OUTPUT]

Generate a minimal root filesystem for containers.

Options:
  -z            Output as tar.gz (shorthand for --tar.gz)
  --tar         Output as uncompressed tar
  --tar.gz      Output as gzip-compressed tar
  --minimal     Create a minimal rootfs (fewer directories)
  -b, --bin DIR Binary directory name (default: bin)
  --src FILE    Path to agentbusybox binary (default: auto-detect)

Examples:
  rootfs ./myrootfs          Create rootfs directory
  rootfs rootfs.tar.gz -z    Create tar.gz archive
  rootfs --minimal ./rootfs  Create minimal rootfs`,
	})
}

func runRootfs(args []string) int {
	outputDir := "rootfs"
	format := "dir" // dir, tar, tar.gz
	binDir := "bin" // bin or busybox
	minimal := false
	srcBin := ""

	for i := 1; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--help", "-h":
			fmt.Println(`Usage: rootfs [OPTIONS] [OUTPUT]

Generate a minimal root filesystem for containers.

Options:
  -z            Output as tar.gz (shorthand for --tar.gz)
  --tar         Output as uncompressed tar
  --tar.gz      Output as gzip-compressed tar
  --minimal     Create a minimal rootfs (fewer directories)
  -b, --bin DIR Binary directory name (default: bin)
  --src FILE    Path to agentbusybox binary (default: auto-detect)

Examples:
  rootfs ./myrootfs          Create rootfs directory
  rootfs rootfs.tar.gz -z    Create tar.gz archive
  rootfs --minimal ./rootfs  Create minimal rootfs`)
			return 0
		case "-z":
			format = "tar.gz"
		case "--tar":
			format = "tar"
		case "--tar.gz", "--tgz":
			format = "tar.gz"
		case "--minimal":
			minimal = true
		case "--bin", "-b":
			if i+1 < len(args) {
				i++
				binDir = args[i]
			}
		case "--src", "--binary":
			if i+1 < len(args) {
				i++
				srcBin = args[i]
			}
		default:
			if !strings.HasPrefix(a, "-") {
				outputDir = a
			}
		}
	}

	// Build self path
	selfPath := srcBin
	if selfPath == "" {
		var err error
		selfPath, err = os.Executable()
		if err != nil {
			selfPath = "agentbusybox"
		}
	}

	fmt.Fprintf(os.Stderr, "Creating rootfs in %s (format: %s)...\n", outputDir, format)

	switch format {
	case "tar", "tar.gz":
		return createRootfsTar(outputDir, selfPath, binDir, format == "tar.gz", minimal)
	default:
		return createRootfsDir(outputDir, selfPath, binDir, minimal)
	}
}

func createRootfsDir(outputDir, selfPath, binDir string, minimal bool) int {
	// Create directory structure
	dirs := []string{
		"bin", "sbin", "usr/bin", "usr/sbin", "usr/local/bin",
		"etc", "etc/init.d", "etc/network",
		"tmp", "var", "var/tmp", "var/log", "var/run", "var/spool",
		"proc", "sys", "dev", "dev/pts", "dev/shm",
		"root", "home", "opt", "mnt", "media",
		"lib", "lib64", "usr/lib", "usr/share",
		"run",
	}

	if !minimal {
		dirs = append(dirs,
			"usr/share/udhcpc",
			"etc/default",
			"var/cache",
			"var/lib",
		)
	}

	for _, d := range dirs {
		path := filepath.Join(outputDir, d)
		if err := os.MkdirAll(path, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "rootfs: mkdir %s: %v\n", d, err)
			return 1
		}
	}

	// Copy binary to bin/agentbusybox
	binPath := filepath.Join(outputDir, "bin", "agentbusybox")
	if err := copyFile(selfPath, binPath); err != nil {
		fmt.Fprintf(os.Stderr, "rootfs: copy binary: %v\n", err)
		return 1
	}
	os.Chmod(binPath, 0755)

	// Create symlinks for all applets
	names := applet.Names()
	created := 0
	for _, name := range names {
		if name == "rootfs" {
			continue
		} // don't symlink rootfs itself
		linkPath := filepath.Join(outputDir, "bin", name)
		// Remove existing if any
		os.Remove(linkPath)
		if err := os.Symlink("agentbusybox", linkPath); err != nil {
			fmt.Fprintf(os.Stderr, "rootfs: symlink %s: %v\n", name, err)
		} else {
			created++
		}
	}

	// Also create symlinks in sbin for system commands
	sbinApps := []string{
		"init", "halt", "poweroff", "reboot", "linuxrc",
		"mount", "umount", "swapon", "swapoff", "fdisk",
		"mkswap", "mkfs", "fsck", "mkfs.ext2", "mkfs.vfat",
		"blkid", "losetup", "mdev", "insmod", "lsmod", "rmmod",
		"modprobe", "depmod", "syslogd", "klogd",
		"hwclock", "chroot", "pivot_root", "switch_root",
		"adduser", "addgroup", "deluser", "delgroup",
		"passwd", "login", "su", "sulogin", "getty",
		"start-stop-daemon", "run-parts",
	}
	for _, name := range sbinApps {
		linkPath := filepath.Join(outputDir, "sbin", name)
		os.Remove(linkPath)
		os.Symlink("../bin/agentbusybox", linkPath)
	}

	// Create basic etc files
	createEtcFiles(outputDir)

	fmt.Fprintf(os.Stderr, "rootfs: created %d applet symlinks in %s/bin/\n", created, outputDir)
	fmt.Fprintf(os.Stderr, "rootfs: rootfs ready at %s/\n", outputDir)
	return 0
}

func createRootfsTar(outputFile, selfPath, binDir string, gz bool, minimal bool) int {
	ext := ".tar"
	if gz {
		ext = ".tar.gz"
	}
	if !strings.HasSuffix(outputFile, ext) {
		outputFile += ext
	}

	f, err := os.Create(outputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rootfs: %v\n", err)
		return 1
	}
	defer f.Close()

	var w io.Writer = f
	if gz {
		gw := gzip.NewWriter(f)
		defer gw.Close()
		w = gw
	}

	tw := tar.NewWriter(w)
	defer tw.Close()

	// Create directories
	dirs := []string{
		"bin/", "sbin/", "usr/", "usr/bin/", "usr/sbin/",
		"etc/", "etc/init.d/",
		"tmp/", "var/", "var/tmp/", "var/log/",
		"proc/", "sys/", "dev/", "dev/pts/",
		"root/", "home/", "opt/", "mnt/",
		"lib/", "lib64/",
		"run/",
	}
	for _, d := range dirs {
		tw.WriteHeader(&tar.Header{
			Name:     d,
			Typeflag: tar.TypeDir,
			Mode:     0755,
		})
	}

	// Add binary
	binData, err := os.ReadFile(selfPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rootfs: read binary: %v\n", err)
		return 1
	}
	tw.WriteHeader(&tar.Header{
		Name: "bin/agentbusybox",
		Mode: 0755,
		Size: int64(len(binData)),
	})
	tw.Write(binData)

	// Add symlinks
	names := applet.Names()
	created := 0
	for _, name := range names {
		if name == "rootfs" {
			continue
		}
		tw.WriteHeader(&tar.Header{
			Name:     "bin/" + name,
			Typeflag: tar.TypeSymlink,
			Linkname: "agentbusybox",
			Mode:     0777,
		})
		created++
	}

	// Add sbin symlinks for system commands
	sbinApps := []string{
		"init", "halt", "poweroff", "reboot", "linuxrc",
		"mount", "umount", "swapon", "swapoff", "fdisk",
		"mkswap", "mkfs", "fsck", "mkfs.ext2", "mkfs.vfat",
		"blkid", "losetup", "mdev", "insmod", "lsmod", "rmmod",
		"modprobe", "depmod", "syslogd", "klogd",
		"hwclock", "chroot", "pivot_root", "switch_root",
		"adduser", "addgroup", "deluser", "delgroup",
		"passwd", "login", "su", "sulogin", "getty",
		"start-stop-daemon", "run-parts",
	}
	for _, name := range sbinApps {
		tw.WriteHeader(&tar.Header{
			Name:     "sbin/" + name,
			Typeflag: tar.TypeSymlink,
			Linkname: "../bin/agentbusybox",
			Mode:     0777,
		})
	}

	// Add etc files
	addEtcToTar(tw)

	fmt.Fprintf(os.Stderr, "rootfs: created %s with %d applet symlinks\n", outputFile, created)
	return 0
}

func createEtcFiles(outputDir string) {
	etc := filepath.Join(outputDir, "etc")

	os.WriteFile(filepath.Join(etc, "hostname"), []byte("agentbusybox\n"), 0644)
	os.WriteFile(filepath.Join(etc, "hosts"), []byte("127.0.0.1\tlocalhost\n::1\t\tlocalhost\n"), 0644)
	os.WriteFile(filepath.Join(etc, "resolv.conf"), []byte("nameserver 8.8.8.8\nnameserver 8.8.4.4\n"), 0644)

	os.WriteFile(filepath.Join(etc, "passwd"), []byte(
		"root:x:0:0:root:/root:/bin/sh\n"+
			"nobody:x:65534:65534:nobody:/nonexistent:/usr/sbin/nologin\n"), 0644)

	os.WriteFile(filepath.Join(etc, "group"), []byte(
		"root:x:0:\n"+
			"nobody:x:65534:\n"+
			"tty:x:5:\n"+
			"disk:x:6:\n"+
			"staff:x:50:\n"), 0644)

	os.WriteFile(filepath.Join(etc, "shadow"), []byte(
		"root:*:19745:0:99999:7:::\n"+
			"nobody:*:19745:0:99999:7:::\n"), 0640)

	os.WriteFile(filepath.Join(etc, "nsswitch.conf"), []byte(
		"hosts: files dns\n"+
			"networks: files\n"), 0644)

	os.WriteFile(filepath.Join(etc, "protocols"), []byte(
		"ip\t0\tIP\t\tinternet protocol, pseudo protocol number\n"+
			"icmp\t1\tICMP\t\tinternet control message protocol\n"+
			"tcp\t6\tTCP\t\ttransmission control protocol\n"+
			"udp\t17\tUDP\t\tuser datagram protocol\n"), 0644)

	os.WriteFile(filepath.Join(etc, "services"), []byte(
		"ssh\t\t22/tcp\n"+
			"http\t\t80/tcp\n"+
			"https\t\t443/tcp\n"+
			"domain\t\t53/tcp\n"+
			"domain\t\t53/udp\n"), 0644)

	os.WriteFile(filepath.Join(etc, "profile"), []byte(
		"export PATH=/bin:/sbin:/usr/bin:/usr/sbin\n"+
			"export HOME=/root\n"+
			"export HOSTNAME=$(hostname)\n"+
			"export PS1='\\u@\\h:\\w\\$ '\n"), 0644)

	os.WriteFile(filepath.Join(etc, "motd"), []byte(
		"Welcome to AgentBusyBox!\n"+
			"A full-featured BusyBox implementation in Go.\n\n"), 0644)

	os.WriteFile(filepath.Join(etc, "inittab"), []byte(
		"::sysinit:/bin/mount -t proc proc /proc\n"+
			"::sysinit:/bin/mount -t sysfs sysfs /sys\n"+
			"::sysinit:/bin/mount -t devtmpfs devtmpfs /dev\n"+
			"::respawn:/bin/sh\n"+
			"::ctrlaltdel:/bin/reboot\n"), 0644)

	os.WriteFile(filepath.Join(etc, "fstab"), []byte(
		"proc\t/proc\tproc\tdefaults\t0 0\n"+
			"sysfs\t/sys\tsysfs\tdefaults\t0 0\n"+
			"devtmpfs\t/dev\tdevtmpfs\tdefaults\t0 0\n"+
			"tmpfs\t/tmp\ttmpfs\tdefaults\t0 0\n"), 0644)

	// init.d/rcS
	rcS := filepath.Join(outputDir, "etc", "init.d", "rcS")
	os.WriteFile(rcS, []byte(
		"#!/bin/sh\n"+
			"mount -t proc proc /proc\n"+
			"mount -t sysfs sysfs /sys\n"+
			"mount -t devtmpfs devtmpfs /dev 2>/dev/null\n"+
			"mkdir -p /dev/pts /dev/shm\n"+
			"mount -t devpts devpts /dev/pts\n"+
			"echo 'AgentBusyBox init complete'\n"), 0755)

	// udhcpc default script
	udhcpc := filepath.Join(outputDir, "usr", "share", "udhcpc", "default.script")
	os.MkdirAll(filepath.Dir(udhcpc), 0755)
	os.WriteFile(udhcpc, []byte(
		"#!/bin/sh\n"+
			"case \"$1\" in\n"+
			"  deconfig)\n"+
			"    ip addr flush dev $interface\n"+
			"    ;;\n"+
			"  renew|bound)\n"+
			"    ip addr add $ip/$mask dev $interface\n"+
			"    ip route add default via $router\n"+
			"    echo \"nameserver $dns\" > /etc/resolv.conf\n"+
			"    ;;\n"+
			"esac\n"), 0755)
}

func addEtcToTar(tw *tar.Writer) {
	files := map[string]string{
		"etc/hostname":    "agentbusybox\n",
		"etc/hosts":       "127.0.0.1\tlocalhost\n::1\t\tlocalhost\n",
		"etc/resolv.conf": "nameserver 8.8.8.8\n",
		"etc/passwd":      "root:x:0:0:root:/root:/bin/sh\nnobody:x:65534:65534:nobody:/nonexistent:/usr/sbin/nologin\n",
		"etc/group":       "root:x:0:\nnobody:x:65534:\n",
		"etc/profile":     "export PATH=/bin:/sbin:/usr/bin:/usr/sbin\nexport HOME=/root\n",
		"etc/motd":        "Welcome to AgentBusyBox!\n",
		"etc/inittab":     "::sysinit:/bin/mount -t proc proc /proc\n::respawn:/bin/sh\n",
		"etc/fstab":       "proc /proc proc defaults 0 0\ntmpfs /tmp tmpfs defaults 0 0\n",
	}
	for name, content := range files {
		tw.WriteHeader(&tar.Header{
			Name: name,
			Mode: 0644,
			Size: int64(len(content)),
		})
		tw.Write([]byte(content))
	}
	// rcS script
	rcS := "#!/bin/sh\nmount -t proc proc /proc\nmount -t sysfs sysfs /sys\necho 'AgentBusyBox init complete'\n"
	tw.WriteHeader(&tar.Header{
		Name: "etc/init.d/rcS",
		Mode: 0755,
		Size: int64(len(rcS)),
	})
	tw.Write([]byte(rcS))
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
