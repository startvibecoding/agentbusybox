# AgentBusyBox

[![Go](https://img.shields.io/badge/Go-1.26-blue)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-green)](LICENSE)

A full-featured BusyBox implementation written entirely in Go — a single static binary containing ~389 Unix/Linux utilities, with built-in rootfs generation for containers.

## Features

- **Single binary** — one self-contained executable with ~389 embedded applets
- **Pure Go** — no external system command dependencies (no `exec.Command` for applet implementations)
- **Multi-platform** — Linux (primary), macOS, Windows
- **Container-ready** — built-in rootfs generation with `--tar`, `--tar.gz`, and `--minimal` options
- **Docker/Podman compatible** — generate importable rootfs tarballs for container images
- **Symlink dispatch** — works via symlinks (traditional BusyBox mode) or direct invocation (`agentbusybox ls`)
- **Runit support** — complete runit process supervision suite
- **SELinux utilities** — full SELinux policy management
- **Shell** — interactive shell with script execution support

## Applet Categories

| Category | Description | Example Applets |
|----------|-------------|-----------------|
| `coreutils` | Core Unix utilities | `ls`, `cat`, `cp`, `mv`, `rm`, `mkdir`, `echo`, `sort`, `wc`, `head`, `tail`, `cut`, `uniq`, `tr`, `sha256sum`, `md5sum` |
| `textproc` | Text processing | `grep`, `sed`, `diff`, `cmp`, `xargs` |
| `fileutil` | File operations | `find`, `stat`, `du`, `df`, `ln`, `chmod`, `chown`, `tree`, `file`, `readlink`, `realpath` |
| `archival` | Archives & compression | `tar`, `gzip`, `bzip2`, `xz`, `zip`, `unzip`, `cpio`, `ar` |
| `networking` | Network tools | `wget`, `curl`, `ping`, `nc`, `ifconfig`, `ip`, `nslookup`, `netstat`, `telnet`, `httpd` |
| `process` | Process management | `ps`, `top`, `kill`, `free`, `pgrep`, `pkill`, `lsof`, `pstree` |
| `shell` | Shell & scripting | `sh`, `ash`, `hush`, `printf`, `shuf` |
| `editors` | Text editors | `vi`, `ed`, `awk`, `patch` |
| `login` | Authentication | `login`, `su`, `passwd`, `adduser`, `addgroup`, `mkpasswd` |
| `util-linux` | Linux utilities | `mount`, `umount`, `fdisk`, `blkid`, `dmesg`, `flock`, `losetup`, `unshare` |
| `init` | System init | `init`, `halt`, `reboot`, `poweroff`, `bootchartd` |
| `runit` | Service supervision | `runsv`, `runsvdir`, `sv`, `chpst`, `svlogd` |
| `selinux` | SELinux tools | `chcon`, `getenforce`, `setenforce`, `restorecon`, `sestatus` |
| `misc` | Miscellaneous | `less`, `xxd`, `time`, `timeout`, `watch`, `nohup`, `bc`, `crond` |
| `fastutil` | Fast file search | `fd` (powered by go-fd / go-ripgrep) |
| `modutils` | Kernel modules | `insmod`, `lsmod`, `rmmod`, `modprobe`, `modinfo` |
| `sysklogd` | Logging | `syslogd`, `klogd`, `logger`, `logread` |
| `debianutils` | Debian tools | `run-parts`, `start-stop-daemon` |
| `console` | Console control | `clear`, `reset`, `chvt`, `setfont`, `openvt` |
| `e2fsprogs` | Ext filesystem | `chattr`, `lsattr`, `tune2fs` |
| `mailutils` | Mail tools | `makemime`, `reformime`, `sendmail`, `popmaildir` |
| `printutils` | Print tools | `lpd`, `lpq`, `lpr` |
| `klibc` | klibc utilities | `nuke`, `resume`, `run-init` |
| `rootfs` | Rootfs generation | `rootfs` (generates container root filesystem) |

## Quick Start

### Build

```bash
# Clone the repository
git clone https://github.com/startvibecoding/agentbusybox.git
cd agentbusybox

# Build for current platform
make build

# Build for all platforms
make build-all
```

### Usage

```bash
# Direct invocation
./bin/agentbusybox ls -la /tmp

# Symlink mode
ln -s ./bin/agentbusybox /usr/local/bin/ls
ls -la

# List all applets
./bin/agentbusybox --list

# Get help
./bin/agentbusybox --help
```

### Container Rootfs

Generate a minimal root filesystem for containers:

```bash
# Generate as a directory
./bin/agentbusybox rootfs ./rootfs

# Generate as a compressed tarball
./bin/agentbusybox rootfs rootfs.tar.gz --tar.gz

# Generate a Docker/Podman-ready image
make build-docker-image

# Import into Docker
docker import bin/rootfs.tar.gz agentbusybox:latest
docker run -it agentbusybox:latest /bin/sh

# Import into Podman
podman import bin/rootfs.tar.gz agentbusybox:latest
podman run -it agentbusybox:latest /bin/sh
```

## Build Targets

| Target | Description |
|--------|-------------|
| `make build` | Build for current platform |
| `make build-all` | Cross-compile for Linux/macOS/Windows |
| `make build-linux` | Build Linux binaries (amd64, arm64) |
| `make build-darwin` | Build macOS binaries (amd64, arm64) |
| `make build-windows` | Build Windows binaries (amd64, arm64) |
| `make build-docker-image` | Build rootfs tarball for Docker/Podman |
| `make dist` | Build all distribution packages |
| `make install` | Install via `go install` |
| `make test` | Run tests with race detection |
| `make lint` | Run golangci-lint |
| `make fmt` | Format code with gofmt/goimports |
| `make applets` | List all registered applets |
| `make clean` | Remove build artifacts |

## Architecture

AgentBusyBox uses a plugin-like **applet registry** pattern:

1. Each applet registers itself in `init()` via `applet.Register()`
2. Blank imports in `main.go` trigger registration at startup
3. `applet.Dispatch()` resolves the applet name from `os.Args[0]` (symlink mode) or `os.Args[1]` (busybox mode)

### Adding a New Applet

```go
// In cmd/myutils/myapplet.go
package myutils

import "github.com/agentbusybox/pkg/applet"

func init() {
    applet.Register(&applet.Applet{
        Name:  "myapplet",
        Short: "Description of my applet",
        Func:  runMyApplet,
    })
}

func runMyApplet(args []string) int {
    // args[0] is the applet name
    // User arguments start at args[1:]
    return 0
}
```

Then add a blank import in `main.go`:

```go
_ "github.com/agentbusybox/cmd/myutils"
```

## Design Principles

- **Pure Go implementations** — all applets use Go standard libraries (`os`, `syscall`, `net`, `crypto/*`, `compress/*`, etc.) instead of shelling out to system commands
- **No external dependencies** beyond `golang.org/x/sys` and two indirect packages (`go-fd`, `go-ripgrep`)
- **Cross-platform** — uses `pkg/platform` helpers and `runtime.GOOS` checks
- **Small binary** — stripped with `-ldflags "-s -w"` and optionally UPX-compressed

## Project Structure

```
agentbusybox/
├── main.go                  # Entry point with blank imports for all cmd packages
├── Makefile                 # Build system
├── cmd/                     # Applet implementations by category
│   ├── coreutils/           # ls, cat, cp, mv, rm, ...
│   ├── networking/          # wget, curl, ping, nc, ip, ...
│   ├── process/             # ps, top, kill, free, ...
│   ├── shell/               # sh, ash, hush
│   ├── archival/            # tar, gzip, zip, ...
│   ├── textproc/            # grep, sed, diff, ...
│   ├── fileutil/            # find, stat, du, df, ...
│   ├── util-linux/          # mount, fdisk, dmesg, ...
│   ├── init/                # init, reboot, halt, ...
│   ├── runit/               # runsv, sv, chpst, ...
│   ├── selinux/             # chcon, setenforce, ...
│   ├── rootfs/              # rootfs generation
│   └── ...                  # 26 categories total
└── pkg/
    ├── applet/              # Core registry: Register(), Dispatch(), List()
    └── platform/            # Cross-platform abstractions
```

## License

MIT
